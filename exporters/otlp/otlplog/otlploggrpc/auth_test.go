// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlploggrpc_test

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collogpb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/sdk/log"
)

type testAuthOption struct {
	otlploggrpc.Option
	headers  func(context.Context) (map[string]string, error)
	shutdown func(context.Context) error
}

func (testAuthOption) Experimental() {}

func (o testAuthOption) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return testPerRPCCreds{headers: o.headers}, nil
}

type testPerRPCCreds struct {
	headers func(context.Context) (map[string]string, error)
}

func (c testPerRPCCreds) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	if c.headers != nil {
		return c.headers(ctx)
	}
	return nil, nil
}

func (testPerRPCCreds) RequireTransportSecurity() bool { return false }

func (o testAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	if o.shutdown != nil {
		return o.shutdown(ctx)
	}
	return nil
}

type noopExperimentalOption struct {
	otlploggrpc.Option
}

func (noopExperimentalOption) Experimental() {}

type authLogCollector struct {
	collogpb.UnimplementedLogsServiceServer
	expectedToken        *atomic.Value
	requests             atomic.Int32
	receivedAuthHeader   atomic.Value
	receivedStaticHeader atomic.Value
	receivedMixedCaseHdr atomic.Value
}

func (s *authLogCollector) Export(ctx context.Context, _ *collogpb.ExportLogsServiceRequest) (*collogpb.ExportLogsServiceResponse, error) {
	s.requests.Add(1)
	md, _ := metadata.FromIncomingContext(ctx)
	var authVal string
	if vals := md.Get("authorization"); len(vals) > 0 {
		authVal = vals[0]
	}
	s.receivedAuthHeader.Store(authVal)
	if vals := md.Get("x-custom-static"); len(vals) > 0 {
		s.receivedStaticHeader.Store(vals[0])
	}
	if vals := md.Get("x-mixed-case-auth"); len(vals) > 0 {
		s.receivedMixedCaseHdr.Store(vals[0])
	}

	expected := "Bearer " + s.expectedToken.Load().(string)
	if authVal != expected {
		return nil, status.Error(codes.Unauthenticated, "invalid or missing bearer token")
	}
	return &collogpb.ExportLogsServiceResponse{}, nil
}

func TestAuthenticatorEndToEndAndHeaderPrecedence(t *testing.T) {
	ctx := context.Background()

	var currentToken atomic.Value
	currentToken.Store("initial-token")

	collector := &authLogCollector{expectedToken: &currentToken}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	grpcSrv := grpc.NewServer()
	collogpb.RegisterLogsServiceServer(grpcSrv, collector)
	go func() { _ = grpcSrv.Serve(lis) }()
	defer grpcSrv.Stop()

	var shutdownCount atomic.Int32
	authOpt := testAuthOption{
		headers: func(context.Context) (map[string]string, error) {
			return map[string]string{
				"Authorization":     "Bearer " + currentToken.Load().(string),
				"X-Mixed-Case-Auth": "normalized-value",
			}, nil
		},
		shutdown: func(context.Context) error {
			shutdownCount.Add(1)
			return nil
		},
	}

	exp, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(lis.Addr().String()),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithHeaders(map[string]string{
			"authorization":   "Bearer stale-static-token",
			"x-custom-static": "preserved-static-value",
		}),
		noopExperimentalOption{},
		authOpt,
	)
	require.NoError(t, err)

	records := make([]log.Record, 1)

	require.NoError(t, exp.Export(ctx, records))
	assert.Equal(t, int32(1), collector.requests.Load())
	assert.Equal(t, "Bearer initial-token", collector.receivedAuthHeader.Load())
	assert.Equal(t, "preserved-static-value", collector.receivedStaticHeader.Load())
	assert.Equal(t, "normalized-value", collector.receivedMixedCaseHdr.Load())

	currentToken.Store("refreshed-token")
	require.NoError(t, exp.Export(ctx, records))
	assert.Equal(t, int32(2), collector.requests.Load())
	assert.Equal(t, "Bearer refreshed-token", collector.receivedAuthHeader.Load())

	require.NoError(t, exp.Shutdown(ctx))
	assert.Equal(t, int32(1), shutdownCount.Load())

	// Verify shared export timeout budget.
	beforeCalls := collector.requests.Load()
	slowAuthExp, err := otlploggrpc.New(
		ctx,
		otlploggrpc.WithEndpoint(lis.Addr().String()),
		otlploggrpc.WithInsecure(),
		otlploggrpc.WithTimeout(20*time.Millisecond),
		otlploggrpc.WithRetry(otlploggrpc.RetryConfig{Enabled: false}),
		testAuthOption{
			headers: func(reqCtx context.Context) (map[string]string, error) {
				<-reqCtx.Done()
				return map[string]string{"Authorization": "Bearer late"}, nil
			},
		},
	)
	require.NoError(t, err)
	defer func() { _ = slowAuthExp.Shutdown(ctx) }()

	err = slowAuthExp.Export(ctx, records)
	require.Error(t, err)
	assert.Equal(t, beforeCalls, collector.requests.Load())
}
