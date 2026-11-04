# Experimental Log SDK

[![PkgGoDev](https://pkg.go.dev/badge/go.opentelemetry.io/otel/sdk/log/x)](https://pkg.go.dev/go.opentelemetry.io/otel/sdk/log/x)

This package contains experimental SDK-level features for the OpenTelemetry log SDK.
These types are currently under development and not part of the stable API.
They may be changed in backwards-incompatible ways, or removed entirely.

## Features

- [Instrumentation Stability](#instrumentation-stability)

### Instrumentation Stability

`WithMinimumStability` configures the minimum stability level (`StabilityStable` by default, or `StabilityDevelopment`) for log `Record`s and attributes on a `LoggerProvider`, working alongside `go.opentelemetry.io/otel/log/x.Record`, `go.opentelemetry.io/otel/log/x.Emit`, `go.opentelemetry.io/otel/log/x.Enabled`, and `go.opentelemetry.io/otel/log/x.AttributeEnabled`.

- Records emitted with a stability level less than `minimum_stability` (such as `StabilityDevelopment` when `minimum_stability` is `StabilityStable`) are dropped before invoking any `Processor`.
- `AttributeEnabled(ctx, logger, key, opts...)` returns `false` when there are no registered `Processor`s, after `Shutdown`, or when `stability` (defaulting to `StabilityStable`) is less than `minimum_stability`.
- Setting `WithMinimumStability(StabilityDevelopment)` enables both `StabilityStable` and `StabilityDevelopment` log `Record`s and attributes.
