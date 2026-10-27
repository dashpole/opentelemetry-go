// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc/credentials"
)

// HTTPAuthenticator authenticates outgoing HTTP telemetry export requests by
// wrapping the exporter's underlying [http.RoundTripper] (matching the
// OpenTelemetry Collector's extensionauth.HTTPClient pattern).
//
// Implementations of HTTPAuthenticator MUST adhere to the following
// requirements:
//
//  1. Concurrency safety: All methods and returned [http.RoundTripper]
//     instances MUST be safe for concurrent invocation across multiple
//     goroutines.
//  2. Request immutability: Returned [http.RoundTripper] implementations MUST
//     NOT mutate the incoming [http.Request] or its header map in place; clone
//     the request via [http.Request.Clone] before modifying headers.
//  3. Timeout and cancellation adherence: Implementations MUST honor any
//     cancellation, deadline, or timeout signal on the request's
//     [context.Context] when performing out-of-band credential acquisition or
//     token refresh.
//  4. Credential redaction: Implementations MUST NOT log sensitive credential
//     values, client secrets, or dynamic authentication header values in
//     diagnostic output or error messages.
type HTTPAuthenticator interface {
	// RoundTripper wraps base with an [http.RoundTripper] that attaches dynamic
	// client credentials to each outgoing HTTP export request attempt.
	RoundTripper(base http.RoundTripper) (http.RoundTripper, error)

	// Shutdown releases any background resources held by the authenticator
	// (such as idle HTTP connections used for token exchange). Shutdown MUST
	// be idempotent and safe to call multiple times.
	Shutdown(ctx context.Context) error
}

// GRPCAuthenticator authenticates outgoing gRPC telemetry export RPCs by
// providing [credentials.PerRPCCredentials] (matching the OpenTelemetry
// Collector's extensionauth.GRPCClient pattern).
//
// Implementations of GRPCAuthenticator MUST adhere to the following
// requirements:
//
//  1. Concurrency safety: All methods and returned
//     [credentials.PerRPCCredentials] instances MUST be safe for concurrent
//     invocation across multiple goroutines.
//  2. Timeout and cancellation adherence: Implementations MUST honor any
//     cancellation, deadline, or timeout signal on the provided
//     [context.Context] when performing out-of-band credential acquisition or
//     token refresh in GetRequestMetadata.
//  3. Credential redaction: Implementations MUST NOT log sensitive credential
//     values, client secrets, or dynamic authentication metadata values in
//     diagnostic output or error messages.
type GRPCAuthenticator interface {
	// PerRPCCredentials returns a [credentials.PerRPCCredentials] that attaches
	// dynamic per-RPC metadata entries to each outgoing gRPC export attempt.
	PerRPCCredentials() (credentials.PerRPCCredentials, error)

	// Shutdown releases any background resources held by the authenticator
	// (such as idle HTTP connections used for token exchange). Shutdown MUST
	// be idempotent and safe to call multiple times.
	Shutdown(ctx context.Context) error
}

// Authenticator is a combined transport-level authenticator capable of
// authenticating both HTTP and gRPC OTLP telemetry exporters.
type Authenticator interface {
	HTTPAuthenticator
	GRPCAuthenticator
}

// NewHeaders returns an [Authenticator] that invokes fn on each HTTP
// [http.RoundTripper.RoundTrip] or gRPC
// [credentials.PerRPCCredentials.GetRequestMetadata] call to resolve dynamic
// headers and has a no-op Shutdown.
//
// If fn is nil, no dynamic headers are added.
func NewHeaders(fn func(context.Context) (map[string]string, error)) Authenticator {
	return headersAuthenticator{fn: fn}
}

type headersAuthenticator struct {
	fn func(context.Context) (map[string]string, error)
}

func (h headersAuthenticator) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	return &headersRoundTripper{
		base: base,
		fn:   h.fn,
	}, nil
}

func (h headersAuthenticator) PerRPCCredentials() (credentials.PerRPCCredentials, error) {
	return headersPerRPCCredentials(h), nil
}

func (headersAuthenticator) Shutdown(context.Context) error {
	return nil
}

type headersRoundTripper struct {
	base http.RoundTripper
	fn   func(context.Context) (map[string]string, error)
}

func (rt *headersRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.fn == nil {
		return rt.base.RoundTrip(req)
	}
	hdrs, err := rt.fn(req.Context())
	if err != nil {
		return nil, err
	}
	if len(hdrs) == 0 {
		return rt.base.RoundTrip(req)
	}
	cloned := req.Clone(req.Context())
	ApplyHTTPHeaders(cloned.Header, hdrs)
	return rt.base.RoundTrip(cloned)
}

type headersPerRPCCredentials struct {
	fn func(context.Context) (map[string]string, error)
}

func (c headersPerRPCCredentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	if c.fn == nil {
		return nil, nil
	}
	return c.fn(ctx)
}

func (headersPerRPCCredentials) RequireTransportSecurity() bool {
	return false
}

// MergeHeaders returns a new map containing static headers merged with dynamic
// headers.
//
// Header keys are compared case-insensitively: when a key in dynamic matches a
// key in static (ignoring ASCII case), the static entry is replaced by the
// dynamic key and value. Neither input map is mutated.
func MergeHeaders(static, dynamic map[string]string) map[string]string {
	if len(static) == 0 && len(dynamic) == 0 {
		return nil
	}
	out := make(map[string]string, len(static)+len(dynamic))
	for k, v := range static {
		out[k] = v
	}
	for dk, dv := range dynamic {
		for sk := range out {
			if strings.EqualFold(sk, dk) {
				delete(out, sk)
			}
		}
		out[dk] = dv
	}
	return out
}

// ApplyHTTPHeaders merges dynamic headers into dst, overriding any existing
// header entries with matching keys (case-insensitively via [http.Header.Set]).
func ApplyHTTPHeaders(dst http.Header, dynamic map[string]string) {
	if dst == nil || len(dynamic) == 0 {
		return
	}
	for k, v := range dynamic {
		for existingKey := range dst {
			if strings.EqualFold(existingKey, k) {
				delete(dst, existingKey)
			}
		}
		dst.Set(k, v)
	}
}
