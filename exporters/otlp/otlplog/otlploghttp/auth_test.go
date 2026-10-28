// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlploghttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/log"
)

type testAuthOption struct {
	otlploghttp.Option
	headers  func(context.Context) (map[string]string, error)
	shutdown func(context.Context) error
}

func (testAuthOption) Experimental() {}

func (o testAuthOption) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return testAuthRoundTripper{base: base, headers: o.headers}, nil
}

type testAuthRoundTripper struct {
	base    http.RoundTripper
	headers func(context.Context) (map[string]string, error)
}

func (rt testAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.headers == nil {
		return rt.base.RoundTrip(req)
	}
	hdrs, err := rt.headers(req.Context())
	if err != nil {
		return nil, err
	}
	cloned := req.Clone(req.Context())
	for k, v := range hdrs {
		for existingKey := range cloned.Header {
			if strings.EqualFold(existingKey, k) {
				delete(cloned.Header, existingKey)
			}
		}
		cloned.Header.Set(k, v)
	}
	return rt.base.RoundTrip(cloned)
}

func (o testAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	if o.shutdown != nil {
		return o.shutdown(ctx)
	}
	return nil
}

type noopExperimentalOption struct {
	otlploghttp.Option
}

func (noopExperimentalOption) Experimental() {}

func TestAuthenticatorEndToEndAndHeaderPrecedence(t *testing.T) {
	ctx := context.Background()

	var currentToken atomic.Value
	currentToken.Store("initial-token")

	var collectorRequests atomic.Int32
	var receivedAuthHeader atomic.Value
	var receivedStaticHeader atomic.Value

	collectorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		collectorRequests.Add(1)
		authHeader := r.Header.Get("Authorization")
		receivedAuthHeader.Store(authHeader)
		receivedStaticHeader.Store(r.Header.Get("X-Custom-Static"))

		expected := "Bearer " + currentToken.Load().(string)
		if authHeader != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer collectorSrv.Close()

	var shutdownCount atomic.Int32
	authOpt := testAuthOption{
		headers: func(context.Context) (map[string]string, error) {
			return map[string]string{
				"Authorization": "Bearer " + currentToken.Load().(string),
			}, nil
		},
		shutdown: func(context.Context) error {
			shutdownCount.Add(1)
			return nil
		},
	}

	exp, err := otlploghttp.New(
		ctx,
		otlploghttp.WithEndpointURL(collectorSrv.URL+"/v1/logs"),
		otlploghttp.WithHeaders(map[string]string{
			"authorization":   "Bearer stale-static-token",
			"X-Custom-Static": "preserved-static-value",
		}),
		noopExperimentalOption{},
		authOpt,
	)
	require.NoError(t, err)

	records := make([]log.Record, 1)

	require.NoError(t, exp.Export(ctx, records))
	assert.Equal(t, int32(1), collectorRequests.Load())
	assert.Equal(t, "Bearer initial-token", receivedAuthHeader.Load())
	assert.Equal(t, "preserved-static-value", receivedStaticHeader.Load())

	currentToken.Store("refreshed-token")
	require.NoError(t, exp.Export(ctx, records))
	assert.Equal(t, int32(2), collectorRequests.Load())
	assert.Equal(t, "Bearer refreshed-token", receivedAuthHeader.Load())

	require.NoError(t, exp.Shutdown(ctx))
	assert.Equal(t, int32(1), shutdownCount.Load())

	// Verify shared export timeout budget.
	beforeCalls := collectorRequests.Load()
	slowAuthExp, err := otlploghttp.New(
		ctx,
		otlploghttp.WithEndpointURL(collectorSrv.URL+"/v1/logs"),
		otlploghttp.WithTimeout(20*time.Millisecond),
		otlploghttp.WithRetry(otlploghttp.RetryConfig{Enabled: false}),
		testAuthOption{
			headers: func(reqCtx context.Context) (map[string]string, error) {
				<-reqCtx.Done()
				return nil, reqCtx.Err()
			},
		},
	)
	require.NoError(t, err)
	defer func() { _ = slowAuthExp.Shutdown(ctx) }()

	err = slowAuthExp.Export(ctx, records)
	require.Error(t, err)
	assert.Equal(t, beforeCalls, collectorRequests.Load())
}
