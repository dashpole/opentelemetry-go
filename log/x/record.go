// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental log API features.
//
// This package is experimental and subject to change or removal without notice.
package x // import "go.opentelemetry.io/otel/log/x"

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
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

// Record represents a log record, extending [log.Record] with experimental
// [Stability] configuration.
//
// Once stabilized in go.opentelemetry.io/otel/log, [Record.Stability] and
// [Record.SetStability] will be methods directly on [log.Record].
type Record struct {
	log.Record
	stability Stability
}

// Stability returns the [Stability] level of the log record.
func (r *Record) Stability() Stability {
	return r.stability
}

// SetStability sets the [Stability] level of the log record.
func (r *Record) SetStability(s Stability) {
	r.stability = s
}

// Clone returns a copy of the record with no shared state.
// The original record and the clone can both be modified without interfering
// with each other.
func (r *Record) Clone() Record {
	return Record{
		Record:    r.Record.Clone(),
		stability: r.stability,
	}
}

// EnabledParameters represents the payload for [Enabled], extending
// [log.EnabledParameters] with experimental [Stability] configuration.
//
// Once stabilized in go.opentelemetry.io/otel/log, Stability will be a field
// directly on [log.EnabledParameters].
type EnabledParameters struct {
	Severity  log.Severity
	EventName string
	Stability Stability
}

// Emit emits r on l, respecting r.Stability() if supported by l.
func Emit(ctx context.Context, l log.Logger, r Record) {
	if l == nil {
		return
	}
	if sl, ok := l.(interface {
		ExperimentalEmit(ctx context.Context, r log.Record, stability int)
	}); ok {
		sl.ExperimentalEmit(ctx, r.Record, int(r.stability))
		return
	}
	l.Emit(ctx, r.Record)
}

// Enabled reports whether l emits for the given context and param, respecting
// param.Stability if supported by l.
func Enabled(ctx context.Context, l log.Logger, param EnabledParameters) bool {
	if l == nil {
		return false
	}
	p := log.EnabledParameters{
		Severity:  param.Severity,
		EventName: param.EventName,
	}
	if sl, ok := l.(interface {
		ExperimentalEnabled(ctx context.Context, param log.EnabledParameters, stability int) bool
	}); ok {
		return sl.ExperimentalEnabled(ctx, p, int(param.Stability))
	}
	return l.Enabled(ctx, p)
}

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

type stabilityOption struct {
	s Stability
}

func (o stabilityOption) applyAttributeEnabled(c attributeEnabledConfig) attributeEnabledConfig {
	c.stability = o.s
	return c
}

// WithStability sets the [Stability] level for [AttributeEnabled].
//
// If not provided, [AttributeEnabled] defaults to [StabilityStable].
//
// This option is experimental and may be changed or removed in a future release.
func WithStability(s Stability) AttributeEnabledOption {
	return stabilityOption{s: s}
}

// AttributeEnabled reports whether l records an attribute with the given
// context, key, and options (defaulting to StabilityStable if not provided).
//
// Instrumentation should call AttributeEnabled before computing or attaching a
// StabilityDevelopment attribute. It returns false if l is nil, a no-op
// Logger, or does not support attribute enablement.
//
// This function is experimental and may be changed or removed in a future release.
func AttributeEnabled(ctx context.Context, l log.Logger, key attribute.Key, opts ...AttributeEnabledOption) bool {
	ae, ok := l.(interface {
		ExperimentalAttributeEnabled(ctx context.Context, key attribute.Key, stability int) bool
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
	return ae.ExperimentalAttributeEnabled(ctx, key, int(cfg.stability))
}
