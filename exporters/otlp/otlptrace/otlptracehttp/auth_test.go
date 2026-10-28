// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlptracehttp_test

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

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type testAuthOption struct {
	otlptracehttp.Option
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
	otlptracehttp.Option
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

	exp, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(collectorSrv.URL+"/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{
			"authorization":   "Bearer stale-static-token",
			"X-Custom-Static": "preserved-static-value",
		}),
		noopExperimentalOption{},
		authOpt,
	)
	require.NoError(t, err)

	stubs := tracetest.SpanStubs{{Name: "test-span"}}.Snapshots()

	require.NoError(t, exp.ExportSpans(ctx, stubs))
	assert.Equal(t, int32(1), collectorRequests.Load())
	assert.Equal(t, "Bearer initial-token", receivedAuthHeader.Load())
	assert.Equal(t, "preserved-static-value", receivedStaticHeader.Load())

	currentToken.Store("refreshed-token")
	require.NoError(t, exp.ExportSpans(ctx, stubs))
	assert.Equal(t, int32(2), collectorRequests.Load())
	assert.Equal(t, "Bearer refreshed-token", receivedAuthHeader.Load())

	require.NoError(t, exp.Shutdown(ctx))
	assert.Equal(t, int32(1), shutdownCount.Load())
}

func TestAuthenticatorInvokedPerRetryAttemptAndSharedTimeout(t *testing.T) {
	ctx := context.Background()

	var attempt atomic.Int32
	collectorSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempt.Add(1)
		if n == 1 {
			assert.Equal(t, "Bearer attempt-1", r.Header.Get("Authorization"))
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		assert.Equal(t, "Bearer attempt-2", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer collectorSrv.Close()

	var authCalls atomic.Int32
	authOpt := testAuthOption{
		headers: func(context.Context) (map[string]string, error) {
			n := authCalls.Add(1)
			if n == 1 {
				return map[string]string{"Authorization": "Bearer attempt-1"}, nil
			}
			return map[string]string{"Authorization": "Bearer attempt-2"}, nil
		},
	}

	exp, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(collectorSrv.URL+"/v1/traces"),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{
			Enabled:         true,
			InitialInterval: time.Millisecond,
			MaxInterval:     5 * time.Millisecond,
			MaxElapsedTime:  time.Second,
		}),
		authOpt,
	)
	require.NoError(t, err)
	defer func() { _ = exp.Shutdown(ctx) }()

	stubs := tracetest.SpanStubs{{Name: "retry-span"}}.Snapshots()
	require.NoError(t, exp.ExportSpans(ctx, stubs))
	assert.Equal(t, int32(2), authCalls.Load())
	assert.Equal(t, int32(2), attempt.Load())

	// Verify shared export timeout budget: if authentication exceeds the export
	// timeout, the export fails immediately without transmitting the HTTP request.
	var timedOutCollectorCalls atomic.Int32
	timeoutSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		timedOutCollectorCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer timeoutSrv.Close()

	slowAuthExp, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(timeoutSrv.URL+"/v1/traces"),
		otlptracehttp.WithTimeout(20*time.Millisecond),
		otlptracehttp.WithRetry(otlptracehttp.RetryConfig{Enabled: false}),
		testAuthOption{
			headers: func(reqCtx context.Context) (map[string]string, error) {
				<-reqCtx.Done()
				return nil, reqCtx.Err()
			},
		},
	)
	require.NoError(t, err)
	defer func() { _ = slowAuthExp.Shutdown(ctx) }()

	err = slowAuthExp.ExportSpans(ctx, stubs)
	require.Error(t, err)
	assert.Equal(t, int32(0), timedOutCollectorCalls.Load())
}
