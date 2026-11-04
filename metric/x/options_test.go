// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"slices"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/metric/x"
)

func TestWithOptIn(t *testing.T) {
	opt := x.WithOptIn()
	if opt == nil {
		t.Fatal("WithOptIn() returned nil")
	}

	exp, ok := opt.(interface{ Experimental() })
	if !ok {
		t.Fatal("WithOptIn() does not implement Experimental()")
	}
	exp.Experimental()

	optIn, ok := opt.(interface{ OptIn() bool })
	if !ok {
		t.Fatal("WithOptIn() does not implement OptIn() bool")
	}
	if !optIn.OptIn() {
		t.Fatal("OptIn() returned false, want true")
	}

	// Ensure all instrument config constructors accept WithOptIn without panicking.
	_ = metric.NewInt64CounterConfig(opt)
	_ = metric.NewInt64UpDownCounterConfig(opt)
	_ = metric.NewInt64HistogramConfig(opt)
	_ = metric.NewInt64GaugeConfig(opt)
	_ = metric.NewInt64ObservableCounterConfig(opt)
	_ = metric.NewInt64ObservableUpDownCounterConfig(opt)
	_ = metric.NewInt64ObservableGaugeConfig(opt)
	_ = metric.NewFloat64CounterConfig(opt)
	_ = metric.NewFloat64UpDownCounterConfig(opt)
	_ = metric.NewFloat64HistogramConfig(opt)
	_ = metric.NewFloat64GaugeConfig(opt)
	_ = metric.NewFloat64ObservableCounterConfig(opt)
	_ = metric.NewFloat64ObservableUpDownCounterConfig(opt)
	_ = metric.NewFloat64ObservableGaugeConfig(opt)
}

func TestWithDefaultAttributes(t *testing.T) {
	k := attribute.Key("k")
	opt := x.WithDefaultAttributes(k)
	if opt == nil {
		t.Fatal("WithDefaultAttributes() returned nil")
	}

	allowed, ok := opt.(interface{ AllowedKeys() []attribute.Key })
	if !ok {
		t.Fatal("WithDefaultAttributes() does not implement AllowedKeys() []attribute.Key")
	}
	if got, want := allowed.AllowedKeys(), []attribute.Key{k}; !slices.Equal(got, want) {
		t.Fatalf("AllowedKeys() = %v, want %v", got, want)
	}
}

func TestWithStability(t *testing.T) {
	var zero x.Stability
	if zero != x.StabilityStable {
		t.Fatalf("zero value of Stability = %v, want StabilityStable (%v)", zero, x.StabilityStable)
	}
	if x.StabilityDevelopment >= x.StabilityStable {
		t.Fatalf("StabilityDevelopment (%v) >= StabilityStable (%v), want <", x.StabilityDevelopment, x.StabilityStable)
	}

	opt := x.WithStability(x.StabilityDevelopment)
	if opt == nil {
		t.Fatal("WithStability() returned nil")
	}

	exp, ok := opt.(interface{ Experimental() })
	if !ok {
		t.Fatal("WithStability() does not implement Experimental()")
	}
	exp.Experimental()

	stab, ok := opt.(interface{ Stability() int })
	if !ok {
		t.Fatal("WithStability() does not implement Stability() int")
	}
	if got, want := stab.Stability(), int(x.StabilityDevelopment); got != want {
		t.Fatalf("Stability() = %d, want %d", got, want)
	}

	// Ensure all instrument config constructors accept WithStability without panicking.
	_ = metric.NewInt64CounterConfig(opt)
	_ = metric.NewInt64UpDownCounterConfig(opt)
	_ = metric.NewInt64HistogramConfig(opt)
	_ = metric.NewInt64GaugeConfig(opt)
	_ = metric.NewInt64ObservableCounterConfig(opt)
	_ = metric.NewInt64ObservableUpDownCounterConfig(opt)
	_ = metric.NewInt64ObservableGaugeConfig(opt)
	_ = metric.NewFloat64CounterConfig(opt)
	_ = metric.NewFloat64UpDownCounterConfig(opt)
	_ = metric.NewFloat64HistogramConfig(opt)
	_ = metric.NewFloat64GaugeConfig(opt)
	_ = metric.NewFloat64ObservableCounterConfig(opt)
	_ = metric.NewFloat64ObservableUpDownCounterConfig(opt)
	_ = metric.NewFloat64ObservableGaugeConfig(opt)
}

type mockAttributeEnabledMeter struct {
	metric.Meter
	gotKey       attribute.Key
	gotStability int
}

func (m *mockAttributeEnabledMeter) ExperimentalAttributeEnabled(key attribute.Key, stability int) bool {
	m.gotKey, m.gotStability = key, stability
	return stability >= 0
}

func TestAttributeEnabled(t *testing.T) {
	if x.AttributeEnabled(nil, "k") {
		t.Fatal("expected AttributeEnabled(nil, \"k\") to be false")
	}
	if x.AttributeEnabled(noop.NewMeterProvider().Meter("test"), "k") {
		t.Fatal("expected AttributeEnabled on a no-op Meter to be false")
	}

	for _, tc := range []struct {
		name string
		opts []x.AttributeEnabledOption
		want x.Stability
	}{
		{name: "Default", want: x.StabilityStable},
		{name: "Stable", opts: []x.AttributeEnabledOption{x.StabilityStable}, want: x.StabilityStable},
		{name: "Development", opts: []x.AttributeEnabledOption{x.StabilityDevelopment}, want: x.StabilityDevelopment},
		{name: "WithStability(Development)", opts: []x.AttributeEnabledOption{x.WithStability(x.StabilityDevelopment)}, want: x.StabilityDevelopment},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mockAttributeEnabledMeter{}
			got := x.AttributeEnabled(m, "attr", tc.opts...)
			if want := tc.want == x.StabilityStable; got != want {
				t.Fatalf("AttributeEnabled() = %t, want %t", got, want)
			}
			if m.gotKey != "attr" || m.gotStability != int(tc.want) {
				t.Fatalf("got key=%q stability=%d, want key=\"attr\" stability=%d", m.gotKey, m.gotStability, tc.want)
			}
		})
	}
}
