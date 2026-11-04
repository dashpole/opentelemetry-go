// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x contains experimental metric options and extension interfaces.
package x

import (
	"context"

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

// InstrumentAttributeOption applies a configuration option to both instrument
// creation and [AttributeEnabled].
type InstrumentAttributeOption interface {
	metric.InstrumentOption
	AttributeEnabledOption
}

type stabilityOption struct {
	metric.InstrumentOption
	s Stability
}

// Experimental prevents the API from panicking when the option is used.
func (stabilityOption) Experimental() {}

func (o stabilityOption) Stability() int {
	return int(o.s)
}

func (o stabilityOption) applyAttributeEnabled(c attributeEnabledConfig) attributeEnabledConfig {
	c.stability = o.s
	return c
}

// WithStability returns an [InstrumentAttributeOption] that sets the stability
// advisory parameter of an instrument or an [AttributeEnabled] check.
//
// By default, instruments have [StabilityStable]. Instruments configured with
// [StabilityDevelopment] use the drop aggregation by default when the
// MeterProvider's minimum stability is [StabilityStable], unless explicitly
// enabled by a View or when the MeterProvider's minimum stability is set to
// [StabilityDevelopment].
func WithStability(s Stability) InstrumentAttributeOption {
	return stabilityOption{s: s}
}

// AttributeEnabled reports whether m records an attribute with the given
// context, key, and options (defaulting to StabilityStable if not provided).
//
// Instrumentation should call AttributeEnabled before computing or attaching a
// StabilityDevelopment attribute. It returns false if m is nil, a no-op
// Meter, or does not support attribute enablement.
//
// This function is experimental and may be changed or removed in a future release.
func AttributeEnabled(ctx context.Context, m metric.Meter, key attribute.Key, opts ...AttributeEnabledOption) bool {
	ae, ok := m.(interface {
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
