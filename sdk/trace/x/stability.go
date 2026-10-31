// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x // import "go.opentelemetry.io/otel/sdk/trace/x"

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracex "go.opentelemetry.io/otel/trace/x"
)

// Stability represents the stability level of telemetry emitted by instrumentation.
type Stability = tracex.Stability

const (
	// StabilityDevelopment indicates telemetry whose shape, name, or attributes
	// are experimental and subject to incompatible change.
	StabilityDevelopment = tracex.StabilityDevelopment
	// StabilityStable indicates telemetry whose shape and semantics are stable.
	// This is the default stability level when none is specified.
	StabilityStable = tracex.StabilityStable
)

// WithStability sets the stability level of a Span.
//
// If not provided, the Span defaults to StabilityStable.
//
// This option is experimental and may be changed or removed in a future release.
var WithStability = tracex.WithStability

// EnabledTracer is an experimental extension interface for trace.Tracer
// implementations that support checking whether a Span would be recorded.
type EnabledTracer = tracex.EnabledTracer

// Enabled reports whether t is enabled for the given context and options.
var Enabled = tracex.Enabled

type minimumStabilityOption struct {
	sdktrace.TracerProviderOption
	s Stability
}

func (minimumStabilityOption) Experimental() {}

func (o minimumStabilityOption) MinimumStability() int { return int(o.s) }

// WithMinimumStability configures the minimum stability level for Spans on a
// TracerProvider.
//
// Spans with a stability level less than s (e.g. StabilityDevelopment when
// minimum stability is StabilityStable) return a non-recording Span without
// invoking the IDGenerator, Sampler, or any SpanProcessor, while preserving
// any parent SpanContext in the returned Context.
//
// By default, if WithMinimumStability is not provided, a TracerProvider uses
// StabilityStable.
//
// This option is experimental and may be changed or removed in a future release.
func WithMinimumStability(s Stability) sdktrace.TracerProviderOption {
	return minimumStabilityOption{s: s}
}
