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

**The godocs are canonical for API facts.** This skill cites them and adds
only what has no godoc home: repo-internal procedure, and spec-level rules
the API docs do not yet state. If this skill and a doc comment disagree,
the doc comment wins — and if you rely on an API fact stated only here,
treat that as a godoc gap worth fixing upstream.

## Ground rules (both modes)

- **API, never SDK.** Instrumented code imports `go.opentelemetry.io/otel`
  and the `trace`/`metric` API packages only. It never constructs providers,
  never installs globals, and never depends on `go.opentelemetry.io/otel/sdk`
  outside its tests.
- **Providers, scope name, and version: follow the package docs.** The
  canonical pattern — accept a `TracerProvider`/`MeterProvider`, default to
  the global at construction time, name the tracer/meter by the
  instrumentation package's import path with `WithInstrumentationVersion` —
  is written out with code in `trace/doc.go` and `metric/doc.go`. Also set
  `WithSchemaURL` to the semconv version in use.
- **Semantic conventions, pinned.** Use one `semconv` version per component
  (the newest vendored under `semconv/`), and take attribute keys and
  instrument definitions from it. Never hand-write a name or unit that
  semconv already defines.
- **Telemetry must not hurt the host.** No panics, no blocking, no
  unbounded growth. Instrument-creation errors are collected with
  `errors.Join` and reported (returned, or `otel.Handle`); the instrumented
  operation proceeds regardless. A nil or noop provider must be a safe
  no-op.
- **Bound cardinality** (spec rule, not yet in the API godocs). Attribute
  values must come from a small known set. Record `semconv.ErrorType(err)`
  — never the error message — and never put user-controlled input (URLs,
  queries, IDs) into attribute values.
- **Near-zero cost when disabled.** Gate expensive work: check
  `span.IsRecording()` before computing span attributes and
  `instrument.Enabled(ctx)` before building measurement options. Pre-compute
  `attribute.NewSet` + `metric.WithAttributeSet` for the static attributes;
  on hot paths, pool option slices as the `observ` packages do with
  `sync.Pool`.

## Mode A — instrumenting a library

- **Spans**: name spans with low-cardinality templates (an operation name,
  not a URL or key; spec rule, not yet in the godocs). On failure call both
  `RecordError` and `SetStatus` — per `RecordError`'s own doc comment, it
  records an exception event and does not change the span status. The
  lifecycle basics (start from the caller's context, pass the returned
  context down, `defer span.End()`) are shown in `trace/doc.go`.
- **Metrics**: pick the instrument by following the compiled examples in
  `metric/example_test.go` (one per instrument type). Units are UCUM codes
  (see `metric.WithUnit`). Spec conventions the godocs do not yet state:
  durations are `Float64Histogram` in seconds (unit `s`), and instrument
  names are dot-separated with no plural nouns — `requests` or
  `request.duration`, never `requests.count`.
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
