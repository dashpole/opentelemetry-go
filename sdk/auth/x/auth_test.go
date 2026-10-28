// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	authx "go.opentelemetry.io/otel/sdk/auth/x"
	"go.opentelemetry.io/otel/sdk/auth/x/oauth2"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type grpcCollector struct {
	coltracepb.UnimplementedTraceServiceServer
	colmetricpb.UnimplementedMetricsServiceServer
	collogpb.UnimplementedLogsServiceServer
	expectedToken string
	requests      atomic.Int32
}

func (c *grpcCollector) checkAuth(ctx context.Context) error {
	c.requests.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	vals := md.Get("authorization")
	if len(vals) == 0 || vals[0] != "Bearer "+c.expectedToken {
		return status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return nil
}

func (c *grpcCollector) Export(ctx context.Context, _ *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	if err := c.checkAuth(ctx); err != nil {
		return nil, err
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type grpcMetricCollector struct {
	colmetricpb.UnimplementedMetricsServiceServer
	parent *grpcCollector
}

func (m *grpcMetricCollector) Export(ctx context.Context, _ *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	if err := m.parent.checkAuth(ctx); err != nil {
		return nil, err
	}
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

type grpcLogCollector struct {
	collogpb.UnimplementedLogsServiceServer
	parent *grpcCollector
}

func (l *grpcLogCollector) Export(ctx context.Context, _ *collogpb.ExportLogsServiceRequest) (*collogpb.ExportLogsServiceResponse, error) {
	if err := l.parent.checkAuth(ctx); err != nil {
		return nil, err
	}
	return &collogpb.ExportLogsServiceResponse{}, nil
}

func TestAllSixOTLPExportersWithOAuth2Authenticator(t *testing.T) {
	ctx := context.Background()
	const expectedToken = "oauth2-e2e-access-token"

	var tokenRequests atomic.Int32
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		tokenRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": expectedToken,
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenSrv.Close()

	var httpCollectorRequests atomic.Int32
	httpCollector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpCollectorRequests.Add(1)
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer httpCollector.Close()

	gCollector := &grpcCollector{expectedToken: expectedToken}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcSrv := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(grpcSrv, gCollector)
	colmetricpb.RegisterMetricsServiceServer(grpcSrv, &grpcMetricCollector{parent: gCollector})
	collogpb.RegisterLogsServiceServer(grpcSrv, &grpcLogCollector{parent: gCollector})
	go func() { _ = grpcSrv.Serve(lis) }()
	defer grpcSrv.Stop()

	newOAuth := func() *oauth2.Authenticator {
		a, aErr := oauth2.New(oauth2.Config{
			TokenURL:     tokenSrv.URL + "/token",
			ClientID:     "client-id",
			ClientSecret: "client-secret",
		})
		require.NoError(t, aErr)
		return a
	}

	// 1. otlptracehttp
	traceHTTPExp, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpointURL(httpCollector.URL+"/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithTraceHTTPAuthenticator(nil),
		authx.WithTraceHTTPAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, traceHTTPExp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "s"}}.Snapshots()))
	require.NoError(t, traceHTTPExp.Shutdown(ctx))

	// 2. otlptracegrpc
	traceGRPCExp, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(lis.Addr().String()),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithTraceGRPCAuthenticator(nil),
		authx.WithTraceGRPCAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, traceGRPCExp.ExportSpans(ctx, tracetest.SpanStubs{{Name: "s"}}.Snapshots()))
	require.NoError(t, traceGRPCExp.Shutdown(ctx))

	// 3. otlpmetrichttp
	metricHTTPExp, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpointURL(httpCollector.URL+"/v1/metrics"),
		otlpmetrichttp.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithMetricHTTPAuthenticator(nil),
		authx.WithMetricHTTPAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, metricHTTPExp.Export(ctx, &metricdata.ResourceMetrics{}))
	require.NoError(t, metricHTTPExp.Shutdown(ctx))

	// 4. otlpmetricgrpc
	metricGRPCExp, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(lis.Addr().String()),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithMetricGRPCAuthenticator(nil),
		authx.WithMetricGRPCAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, metricGRPCExp.Export(ctx, &metricdata.ResourceMetrics{}))
	require.NoError(t, metricGRPCExp.Shutdown(ctx))

	// 5. otlploghttp
	logHTTPExp, err := otlploghttp.New(
		ctx,
		otlploghttp.WithEndpointURL(httpCollector.URL+"/v1/logs"),
		otlploghttp.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithLogHTTPAuthenticator(nil),
		authx.WithLogHTTPAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, logHTTPExp.Export(ctx, make([]log.Record, 1)))
	require.NoError(t, logHTTPExp.Shutdown(ctx))

	// 6. otlploggrpc
	logGRPCExp, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(lis.Addr().String()),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(map[string]string{"authorization": "Bearer stale"}),
		authx.WithLogGRPCAuthenticator(nil),
		authx.WithLogGRPCAuthenticator(newOAuth()),
	)
	require.NoError(t, err)
	require.NoError(t, logGRPCExp.Export(ctx, make([]log.Record, 1)))
	require.NoError(t, logGRPCExp.Shutdown(ctx))

	assert.Equal(t, int32(3), httpCollectorRequests.Load())
	assert.Equal(t, int32(3), gCollector.requests.Load())
	assert.Equal(t, int32(6), tokenRequests.Load())
}
