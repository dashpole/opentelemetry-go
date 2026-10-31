// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x

import (
	metricx "go.opentelemetry.io/otel/metric/x"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Stability represents the maturity level of telemetry.
type Stability = metricx.Stability

const (
	// StabilityDevelopment indicates experimental telemetry that may change
	// or be removed in minor versions of a stable instrumentation library.
	StabilityDevelopment = metricx.StabilityDevelopment
	// StabilityStable indicates telemetry covered by backward-compatibility
	// guarantees.
	StabilityStable = metricx.StabilityStable
)

type minimumStabilityOption struct {
	sdkmetric.Option
	s Stability
}

// Experimental prevents the SDK from applying the embedded stable option.
func (minimumStabilityOption) Experimental() {}

func (o minimumStabilityOption) MinimumStability() int {
	return int(o.s)
}

// WithMinimumStability returns an [sdkmetric.Option] that configures the
// minimum [Stability] level of instruments enabled by default on a
// [sdkmetric.MeterProvider].
//
// If this option is not provided, [StabilityStable] is used.
func WithMinimumStability(s Stability) sdkmetric.Option {
	return minimumStabilityOption{s: s}
}
