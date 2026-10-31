// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental log API features.
//
// This package is experimental and subject to change or removal without notice.
package x // import "go.opentelemetry.io/otel/log/x"

import (
	"context"

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
