// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x

import (
	"context"
	"net/http"

	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

type traceHTTPAuthOption struct {
	// nil embed; skip guard in asHTTPOptions prevents applyHTTPOption from being called.
	otlptracehttp.Option
	auth HTTPAuthenticator
}

func (traceHTTPAuthOption) Experimental() {}

func (o traceHTTPAuthOption) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return o.auth.RoundTripper(base)
}

func (o traceHTTPAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopTraceHTTPOption struct {
	otlptracehttp.Option
}

func (noopTraceHTTPOption) Experimental() {}

// WithTraceHTTPAuthenticator returns an [otlptracehttp.Option] that configures
// an [HTTPAuthenticator] on an OTLP HTTP trace exporter.
//
// Passing a nil HTTPAuthenticator is a no-op.
func WithTraceHTTPAuthenticator(a HTTPAuthenticator) otlptracehttp.Option {
	if a == nil {
		return noopTraceHTTPOption{}
	}
	return traceHTTPAuthOption{auth: a}
}

type traceGRPCAuthOption struct {
	// nil embed; skip guard in asGRPCOptions prevents applyGRPCOption from being called.
	otlptracegrpc.Option
	auth GRPCAuthenticator
}

func (traceGRPCAuthOption) Experimental() {}

func (o traceGRPCAuthOption) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return o.auth.PerRPCCredentials()
}

func (o traceGRPCAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopTraceGRPCOption struct {
	otlptracegrpc.Option
}

func (noopTraceGRPCOption) Experimental() {}

// WithTraceGRPCAuthenticator returns an [otlptracegrpc.Option] that configures
// a [GRPCAuthenticator] on an OTLP gRPC trace exporter.
//
// Passing a nil GRPCAuthenticator is a no-op.
func WithTraceGRPCAuthenticator(a GRPCAuthenticator) otlptracegrpc.Option {
	if a == nil {
		return noopTraceGRPCOption{}
	}
	return traceGRPCAuthOption{auth: a}
}

type metricHTTPAuthOption struct {
	// nil embed; skip guard in asHTTPOptions prevents applyHTTPOption from being called.
	otlpmetrichttp.Option
	auth HTTPAuthenticator
}

func (metricHTTPAuthOption) Experimental() {}

func (o metricHTTPAuthOption) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return o.auth.RoundTripper(base)
}

func (o metricHTTPAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopMetricHTTPOption struct {
	otlpmetrichttp.Option
}

func (noopMetricHTTPOption) Experimental() {}

// WithMetricHTTPAuthenticator returns an [otlpmetrichttp.Option] that configures
// an [HTTPAuthenticator] on an OTLP HTTP metric exporter.
//
// Passing a nil HTTPAuthenticator is a no-op.
func WithMetricHTTPAuthenticator(a HTTPAuthenticator) otlpmetrichttp.Option {
	if a == nil {
		return noopMetricHTTPOption{}
	}
	return metricHTTPAuthOption{auth: a}
}

type metricGRPCAuthOption struct {
	// nil embed; skip guard in asGRPCOptions prevents applyGRPCOption from being called.
	otlpmetricgrpc.Option
	auth GRPCAuthenticator
}

func (metricGRPCAuthOption) Experimental() {}

func (o metricGRPCAuthOption) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return o.auth.PerRPCCredentials()
}

func (o metricGRPCAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopMetricGRPCOption struct {
	otlpmetricgrpc.Option
}

func (noopMetricGRPCOption) Experimental() {}

// WithMetricGRPCAuthenticator returns an [otlpmetricgrpc.Option] that configures
// a [GRPCAuthenticator] on an OTLP gRPC metric exporter.
//
// Passing a nil GRPCAuthenticator is a no-op.
func WithMetricGRPCAuthenticator(a GRPCAuthenticator) otlpmetricgrpc.Option {
	if a == nil {
		return noopMetricGRPCOption{}
	}
	return metricGRPCAuthOption{auth: a}
}

type logHTTPAuthOption struct {
	// nil embed; skip guard in newConfig prevents applyHTTPOption from being called.
	otlploghttp.Option
	auth HTTPAuthenticator
}

func (logHTTPAuthOption) Experimental() {}

func (o logHTTPAuthOption) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return o.auth.RoundTripper(base)
}

func (o logHTTPAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopLogHTTPOption struct {
	otlploghttp.Option
}

func (noopLogHTTPOption) Experimental() {}

// WithLogHTTPAuthenticator returns an [otlploghttp.Option] that configures
// an [HTTPAuthenticator] on an OTLP HTTP log exporter.
//
// Passing a nil HTTPAuthenticator is a no-op.
func WithLogHTTPAuthenticator(a HTTPAuthenticator) otlploghttp.Option {
	if a == nil {
		return noopLogHTTPOption{}
	}
	return logHTTPAuthOption{auth: a}
}

type logGRPCAuthOption struct {
	// nil embed; skip guard in newConfig prevents applyOption from being called.
	otlploggrpc.Option
	auth GRPCAuthenticator
}

func (logGRPCAuthOption) Experimental() {}

func (o logGRPCAuthOption) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return o.auth.PerRPCCredentials()
}

func (o logGRPCAuthOption) AuthenticatorShutdown(ctx context.Context) error {
	return o.auth.Shutdown(ctx)
}

type noopLogGRPCOption struct {
	otlploggrpc.Option
}

func (noopLogGRPCOption) Experimental() {}

// WithLogGRPCAuthenticator returns an [otlploggrpc.Option] that configures
// a [GRPCAuthenticator] on an OTLP gRPC log exporter.
//
// Passing a nil GRPCAuthenticator is a no-op.
func WithLogGRPCAuthenticator(a GRPCAuthenticator) otlploggrpc.Option {
	if a == nil {
		return noopLogGRPCOption{}
	}
	return logGRPCAuthOption{auth: a}
}
