---
name: add-instrumentation
description: Add OpenTelemetry traces or metrics to Go code. Covers two cases with one discipline - self-observability for this repo's own components (exporters, processors, SDK internals) and instrumenting a library that depends on the otel-go API. Use when asked to instrument something, add spans or metrics, or extend self-observability coverage.
---

# Add Instrumentation

Add tracing or metrics to Go code using this project's own conventions.
Two modes share the ground rules below:

- **Mode A — library instrumentation**: code outside this repo (or in
  contrib) that instruments its own operations using the otel-go API.
- **Mode B — self-observability**: this repo's own components reporting
  their health, following the established `internal/observ` pattern.

Decide the mode first; it changes the package layout and gating, not the
telemetry discipline.

## Ground rules (both modes)

- **API, never SDK.** Instrumented code imports `go.opentelemetry.io/otel`
  and the `trace`/`metric` API packages only. It never constructs providers,
  never installs globals, and never depends on `go.opentelemetry.io/otel/sdk`
  outside its tests.
- **Providers come from the caller.** Accept a `TracerProvider` /
  `MeterProvider` through the component's existing option pattern and
  default to the globals (`otel.GetTracerProvider()`,
  `otel.GetMeterProvider()`). Resolve the default at construction time, not
  import time, so `SetTracerProvider` ordering does not matter.
- **Scope identifies the instrumentation.** The tracer/meter name is the
  instrumentation package's full import path, with
  `WithInstrumentationVersion` and `WithSchemaURL` set. See the constants
  block in
  `exporters/stdout/stdouttrace/internal/observ/instrumentation.go`.
- **Semantic conventions, pinned.** Use one `semconv` version per component
  (the newest vendored under `semconv/`), and take attribute keys and
  instrument definitions from it. Never hand-write a name or unit that
  semconv already defines.
- **Telemetry must not hurt the host.** No panics, no blocking, no
  unbounded growth. Instrument-creation errors are collected with
  `errors.Join` and reported (returned, or `otel.Handle`); the instrumented
  operation proceeds regardless. A nil or noop provider must be a safe
  no-op.
- **Bound cardinality.** Attribute values must come from a small known set.
  Record `semconv.ErrorType(err)` — never the error message — and never put
  user-controlled input (URLs, queries, IDs) into attribute values.
- **Near-zero cost when disabled.** Gate expensive work: check
  `span.IsRecording()` before computing span attributes and
  `instrument.Enabled(ctx)` before building measurement options. Pre-compute
  `attribute.NewSet` + `metric.WithAttributeSet` for the static attributes;
  on hot paths, pool option slices as the `observ` packages do with
  `sync.Pool`.

## Mode A — instrumenting a library

- **Spans**: name spans with low-cardinality templates (an operation name,
  not a URL or key). Start with the context you were given and pass the
  returned context down. `defer span.End()` on the same goroutine. On
  failure call both `span.RecordError(err)` and
  `span.SetStatus(codes.Error, ...)` — recording alone does not set status.
- **Metrics**: durations are `Float64Histogram` in seconds (unit `s`);
  counts are monotonic `Int64Counter`; in-flight work is an
  `Int64UpDownCounter`. Units are UCUM; instrument names follow semconv
  naming rules (dot-separated, no plural nouns like `requests.count` —
  `requests` or `request.duration`).
- **Propagation**: extract/inject only at process boundaries the library
  actually owns (an HTTP client/server, a queue producer/consumer), using
  the caller-supplied or global `TextMapPropagator`. Pure in-process
  libraries only pass `context.Context` through.

## Mode B — self-observability in this repo

Copy the shape of an existing `internal/observ` package
(`exporters/stdout/stdouttrace/internal/observ` is the smallest; the OTLP
exporters show the gRPC/HTTP variants). Specifically:

1. **Feature gate.** Experimental observability is off by default and
   enabled by `OTEL_GO_X_OBSERVABILITY` (alias
   `OTEL_GO_X_SELF_OBSERVABILITY`). The gate lives in the component's
   `internal/x` package, generated from `internal/shared/x` via gotmpl —
   regenerate rather than hand-editing. `NewInstrumentation` returns
   `(nil, nil)` when the gate is off, and all call sites tolerate the nil.
2. **Generated instruments.** Create instruments through the
   `semconv/<version>/otelconv` constructors
   (e.g. `otelconv.NewSDKExporterSpanExported(m)`), which carry the
   spec-defined name, unit, description, and bucket boundaries. If the
   metric you need has no `otelconv` constructor, that is a semconv/weaver
   question to raise, not a reason to hand-roll.
3. **Component identity.** Every instance gets
   `otel.component.type` / `otel.component.name` attributes; the name is
   `ComponentName(id)` with a per-type unique ID. Use the standardized
   component type when the spec defines one, else the Go-package-prefixed
   type name.
4. **Record pattern.** Wrap each instrumented operation start/end (see
   `ExportOp` in the stdout trace exporter): capture the start time, add
   in-flight up, and on end record in-flight down, success/failure counts
   (zero is a meaningful value — record it), and duration, attaching
   `error.type` only on the failure paths. Check `Enabled()` per instrument
   and skip all allocation when everything is disabled.

## Tests and evidence

- Traces: assert against `sdk/trace/tracetest` (`SpanRecorder` or
  `InMemoryExporter`) — span names, attributes, status, and parenting.
- Metrics: drive a `sdk/metric` `ManualReader` and compare with
  `metricdatatest.AssertEqual`, using `IgnoreTimestamp()` (and
  `IgnoreValue()` for durations). Existing `observ` test files are the
  template.
- Concurrency: instrumented paths run under `-race` in tests; add a
  concurrent-use test if the operation can be called from multiple
  goroutines.
- Cost: add a benchmark showing the disabled path allocates nothing, and a
  `benchstat` comparison if you touched an existing hot path.
- Changelog: self-observability additions are user-visible — add a
  `CHANGELOG.md` entry naming the module.
