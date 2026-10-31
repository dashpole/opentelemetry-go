// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental trace API features.
//
// This package is experimental and subject to change or removal without notice.
package x // import "go.opentelemetry.io/otel/trace/x"

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Stability represents the stability level of telemetry emitted by instrumentation.
//
// The ordering is StabilityDevelopment < StabilityStable.
type Stability int

const (
	// StabilityDevelopment indicates telemetry whose shape, name, or attributes
	// are experimental and subject to incompatible change.
	StabilityDevelopment Stability = -1
	// StabilityStable indicates telemetry whose shape and semantics are stable.
	// This is the default stability level when none is specified.
	StabilityStable Stability = 0
)

type stabilityOption struct {
	trace.SpanStartOption
	s Stability
}

func (stabilityOption) Experimental() {}

func (o stabilityOption) Stability() int { return int(o.s) }

// WithStability sets the stability level of a Span.
//
// If not provided, the Span defaults to StabilityStable.
//
// This option is experimental and may be changed or removed in a future release.
func WithStability(s Stability) trace.SpanStartOption {
	return stabilityOption{s: s}
}

// EnabledTracer is an experimental extension interface for trace.Tracer
// implementations that support checking whether a Span would be recorded.
type EnabledTracer interface {
	Enabled(ctx context.Context, opts ...trace.SpanStartOption) bool
}

// Enabled reports whether t is enabled for the given context and options.
//
// If t implements EnabledTracer, Enabled delegates to t.Enabled(ctx, opts...).
// Otherwise, it returns false if t is nil or a noop.Tracer, and true otherwise.
func Enabled(ctx context.Context, t trace.Tracer, opts ...trace.SpanStartOption) bool {
	if t == nil {
		return false
	}
	if et, ok := t.(EnabledTracer); ok {
		return et.Enabled(ctx, opts...)
	}
	return t != noop.Tracer{}
}
