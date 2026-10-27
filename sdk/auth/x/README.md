# Experimental Authentication SDK

<!--
Adding/uncommenting the following after initial PR merge:
[![PkgGoDev](https://pkg.go.dev/badge/go.opentelemetry.io/otel/sdk/auth/x)](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/auth/x)
-->

This package contains experimental SDK-level authentication features and OTLP exporter options.
These types are currently under development and not part of the stable API.
They may be changed in backwards-incompatible ways, or removed entirely.

## Features

- [Transport-Level Authenticators](#transport-level-authenticators)
- [OAuth 2.0 Client Credentials (`oauth2`)](#oauth-20-client-credentials-oauth2)
- [OTLP Exporter Authentication Options](#otlp-exporter-authentication-options)

### Transport-Level Authenticators

- `HTTPAuthenticator` wraps an `http.RoundTripper` (`RoundTripper(base http.RoundTripper) (http.RoundTripper, error)`) and provides an idempotent `Shutdown(ctx context.Context) error` method, matching the OpenTelemetry Collector's `extensionauth.HTTPClient` pattern.
- `GRPCAuthenticator` provides `credentials.PerRPCCredentials` (`PerRPCCredentials() (credentials.PerRPCCredentials, error)`) and an idempotent `Shutdown(ctx context.Context) error` method, matching the OpenTelemetry Collector's `extensionauth.GRPCClient` pattern.
- `Authenticator` embeds both `HTTPAuthenticator` and `GRPCAuthenticator` (`NewHeaders` adapts a per-request header callback into an `Authenticator`).

### OAuth 2.0 Client Credentials (`oauth2`)

Subpackage `go.opentelemetry.io/otel/sdk/auth/x/oauth2` implements the OAuth 2.0 Client Credentials grant (`RFC 6749 §4.4`) with automatic token caching and pre-expiry refresh, satisfying `x.Authenticator` (`x.HTTPAuthenticator` and `x.GRPCAuthenticator`).

### OTLP Exporter Authentication Options

- `WithTraceHTTPAuthenticator` (`otlptracehttp.Option`)
- `WithTraceGRPCAuthenticator` (`otlptracegrpc.Option`)
- `WithMetricHTTPAuthenticator` (`otlpmetrichttp.Option`)
- `WithMetricGRPCAuthenticator` (`otlpmetricgrpc.Option`)
- `WithLogHTTPAuthenticator` (`otlploghttp.Option`)
- `WithLogGRPCAuthenticator` (`otlploggrpc.Option`)

## Compatibility and Stability

Experimental features do not fall within the scope of the OpenTelemetry Go versioning and stability [policy](../../../VERSIONING.md).
These features may be removed or modified in successive version releases, including patch versions.
