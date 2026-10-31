// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental metric options and extension interfaces.
package x

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type defaultAttributesOption struct {
	metric.InstrumentOption
	keys []attribute.Key
}

// Experimental prevents the API from panicking when the option is used.
func (defaultAttributesOption) Experimental() {}

func (o defaultAttributesOption) AllowedKeys() []attribute.Key {
	return o.keys
}

// WithDefaultAttributes returns a metric.InstrumentOption that specifies default attribute keys.
// The implementation should treat keys that are not included in the passed keys as opt-in, and they should be filtered out by default.
// Users of [go.opentelemetry.io/otel/sdk/metric] can enable these attributes by using the AttributeFilter of a View to include them.
func WithDefaultAttributes(keys ...attribute.Key) metric.InstrumentOption {
	return defaultAttributesOption{keys: keys}
}

type optInOption struct {
	metric.InstrumentOption
}

// Experimental prevents the API from panicking when the option is used.
func (optInOption) Experimental() {}

func (optInOption) OptIn() bool {
	return true
}

// WithOptIn returns a [metric.InstrumentOption] that marks an instrument as
// opt-in (disabled by default).
//
// The implementation should use the drop aggregation for the instrument by
// default unless it is explicitly enabled by a View.
func WithOptIn() metric.InstrumentOption {
	return optInOption{}
}

// Stability represents the maturity level of telemetry.
type Stability int

const (
	// StabilityDevelopment indicates experimental telemetry that may change
	// or be removed in minor versions of a stable instrumentation library.
	StabilityDevelopment Stability = -1
	// StabilityStable indicates telemetry covered by backward-compatibility
	// guarantees.
	StabilityStable Stability = 0
)

type stabilityOption struct {
	metric.InstrumentOption
	s Stability
}

// Experimental prevents the API from panicking when the option is used.
func (stabilityOption) Experimental() {}

func (o stabilityOption) Stability() int {
	return int(o.s)
}

// WithStability returns a [metric.InstrumentOption] that sets the stability
// advisory parameter of an instrument.
//
// By default, instruments have [StabilityStable]. Instruments configured with
// [StabilityDevelopment] use the drop aggregation by default when the
// MeterProvider's minimum stability is [StabilityStable], unless explicitly
// enabled by a View or when the MeterProvider's minimum stability is set to
// [StabilityDevelopment].
func WithStability(s Stability) metric.InstrumentOption {
	return stabilityOption{s: s}
}

// Settable is an optional interface that Options can implement
// to allow reuse without additional allocations.
//
// Example usage with sync.Pool:
//
//	var optionPool = sync.Pool{
//		New: func() any {
//			return metric.WithAttributeSet(*attribute.EmptySet())
//		},
//	}
//
//	func record(ctx context.Context, counter metric.Int64Counter, set attribute.Set) {
//		opt := optionPool.Get().(metric.MeasurementOption)
//		defer optionPool.Put(opt)
//
//		if r, ok := opt.(x.Settable[attribute.Set]); ok {
//			r.Set(set)
//		} else {
//			opt = metric.WithAttributeSet(set)
//		}
//		counter.Add(ctx, 1, opt)
//	}
//
// WARNING: It is the user's responsibility to ensure that the option is not
// concurrently set while being passed to the API or used by another goroutine.
type Settable[T any] interface {
	Set(T)
}
