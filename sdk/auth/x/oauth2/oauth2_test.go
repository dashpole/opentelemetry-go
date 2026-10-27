// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOAuth2TokenCachingAndRefresh(t *testing.T) {
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "my-client-id", user)
		assert.Equal(t, "my-client-secret", pass)

		require.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
		assert.Equal(t, "otel.write otel.traces", r.Form.Get("scope"))
		assert.Equal(t, "https://collector.example.com", r.Form.Get("audience"))

		callNum := tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "token-" + string(rune('0'+callNum)),
			"token_type":   "Bearer",
			"expires_in":   120,
		})
	}))
	defer srv.Close()

	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var clockMu sync.Mutex
	clockNow := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return now
	}
	advanceClock := func(d time.Duration) {
		clockMu.Lock()
		defer clockMu.Unlock()
		now = now.Add(d)
	}

	auth, err := New(
		Config{
			TokenURL:     srv.URL + "/oauth/token",
			ClientID:     "my-client-id",
			ClientSecret: "my-client-secret",
			Scopes:       []string{"otel.write", "otel.traces"},
			Audience:     "https://collector.example.com",
		},
		WithExpirySkew(30*time.Second),
		optionFunc(func(o options) options {
			o.now = clockNow
			return o
		}),
	)
	require.NoError(t, err)

	ctx := context.Background()

	// 1. Initial call fetches token-1.
	hdrs, err := auth.Headers(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token-1"}, hdrs)
	assert.Equal(t, int32(1), tokenCalls.Load())

	// 2. Subsequent call within validity window (e.g. +60s < 120s-30s) uses cached token.
	advanceClock(60 * time.Second)
	hdrs, err = auth.Headers(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token-1"}, hdrs)
	assert.Equal(t, int32(1), tokenCalls.Load())

	// 3. Advance clock into expiry skew window (+35s -> 95s >= 90s); triggers refresh to token-2.
	advanceClock(35 * time.Second)
	hdrs, err = auth.Headers(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Authorization": "Bearer token-2"}, hdrs)
	assert.Equal(t, int32(2), tokenCalls.Load())

	// 4. Shutdown is idempotent and prevents further token resolution.
	require.NoError(t, auth.Shutdown(ctx))
	require.NoError(t, auth.Shutdown(ctx))
	_, err = auth.Headers(ctx)
	assert.ErrorIs(t, err, ErrShutdown)
}

func TestOAuth2ConcurrentAccessAndContextCancellation(t *testing.T) {
	var tokenCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "concurrent-token",
			"token_type":   "Bearer",
			"expires_in":   300,
		})
	}))
	defer srv.Close()

	auth, err := New(Config{
		TokenURL:     srv.URL,
		ClientID:     "cid",
		ClientSecret: "csecret",
	})
	require.NoError(t, err)
	defer func() { _ = auth.Shutdown(context.Background()) }()

	// Cancelled context must fail immediately without making a network request.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = auth.Headers(cancelCtx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(0), tokenCalls.Load())

	// Concurrent callers share the cached token after first fetch.
	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			h, hErr := auth.Headers(context.Background())
			assert.NoError(t, hErr)
			assert.Equal(t, "Bearer concurrent-token", h["Authorization"])
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), tokenCalls.Load())
}
