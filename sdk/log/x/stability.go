// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental log SDK features.
//
// This package is experimental and subject to change or removal without notice.
package x // import "go.opentelemetry.io/otel/sdk/log/x"

import (
	logx "go.opentelemetry.io/otel/log/x"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// Stability represents the stability level of telemetry emitted by instrumentation.
type Stability = logx.Stability

const (
	// StabilityDevelopment indicates telemetry whose shape, name, or attributes
	// are experimental and subject to incompatible change.
	StabilityDevelopment = logx.StabilityDevelopment
	// StabilityStable indicates telemetry whose shape and semantics are stable.
	// This is the default stability level when none is specified.
	StabilityStable = logx.StabilityStable
)

// Record represents a log record, extending [go.opentelemetry.io/otel/log.Record]
// with experimental [Stability] configuration.
type Record = logx.Record

// EnabledParameters represents the payload for [Enabled], extending
// [go.opentelemetry.io/otel/log.EnabledParameters] with experimental [Stability]
// configuration.
type EnabledParameters = logx.EnabledParameters

// Emit emits r on l, respecting r.Stability() if supported by l.
var Emit = logx.Emit

// Enabled reports whether l emits for the given context and param, respecting
// param.Stability if supported by l.
var Enabled = logx.Enabled

type minimumStabilityOption struct {
	sdklog.LoggerProviderOption
	s Stability
}

func (minimumStabilityOption) Experimental() {}

func (o minimumStabilityOption) MinimumStability() int { return int(o.s) }

// WithMinimumStability configures the minimum stability level for log Records
// on a LoggerProvider.
//
// Records with a stability level less than s (e.g. StabilityDevelopment when
// minimum stability is StabilityStable) are dropped without invoking any
// Processor, and Enabled returns false.
//
// By default, if WithMinimumStability is not provided, a LoggerProvider uses
// StabilityStable.
//
// This option is experimental and may be changed or removed in a future release.
func WithMinimumStability(s Stability) sdklog.LoggerProviderOption {
	return minimumStabilityOption{s: s}
}
