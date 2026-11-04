// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental trace API features.
//
// This package is experimental and subject to change or removal without notice.
package x // import "go.opentelemetry.io/otel/trace/x"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
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

// AttributeEnabledOption applies a configuration option to [AttributeEnabled].
type AttributeEnabledOption interface {
	applyAttributeEnabled(attributeEnabledConfig) attributeEnabledConfig
}

type attributeEnabledConfig struct {
	stability Stability
}

func (s Stability) applyAttributeEnabled(c attributeEnabledConfig) attributeEnabledConfig {
	c.stability = s
	return c
}

// SpanStartAttributeOption applies a configuration option to both Span
// creation ([trace.Tracer.Start], [Enabled]) and [AttributeEnabled].
type SpanStartAttributeOption interface {
	trace.SpanStartOption
	AttributeEnabledOption
}

type stabilityOption struct {
	trace.SpanStartOption
	s Stability
}

func (stabilityOption) Experimental() {}

func (o stabilityOption) Stability() int { return int(o.s) }

func (o stabilityOption) applyAttributeEnabled(c attributeEnabledConfig) attributeEnabledConfig {
	c.stability = o.s
	return c
}

// WithStability sets the stability level of a Span or attribute check.
//
// If not provided, the Span or attribute check defaults to StabilityStable.
//
// This option is experimental and may be changed or removed in a future release.
func WithStability(s Stability) SpanStartAttributeOption {
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

// AttributeEnabled reports whether t records an attribute with the given key
// and options (defaulting to StabilityStable if not provided).
//
// Instrumentation should call AttributeEnabled before computing or attaching a
// StabilityDevelopment attribute. It returns false if t is nil, a no-op
// Tracer, or does not support attribute enablement.
//
// This function is experimental and may be changed or removed in a future release.
func AttributeEnabled(t trace.Tracer, key attribute.Key, opts ...AttributeEnabledOption) bool {
	ae, ok := t.(interface {
		ExperimentalAttributeEnabled(key attribute.Key, stability int) bool
	})
	if !ok {
		return false
	}
	cfg := attributeEnabledConfig{stability: StabilityStable}
	for _, opt := range opts {
		if opt != nil {
			cfg = opt.applyAttributeEnabled(cfg)
		}
	}
	return ae.ExperimentalAttributeEnabled(key, int(cfg.stability))
}
