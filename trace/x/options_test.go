// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.opentelemetry.io/otel/trace/x"
)

func TestWithStability(t *testing.T) {
	if x.StabilityDevelopment >= x.StabilityStable {
		t.Fatalf("expected StabilityDevelopment (%d) < StabilityStable (%d)", x.StabilityDevelopment, x.StabilityStable)
	}
	var zero x.Stability
	if zero != x.StabilityStable {
		t.Fatalf("expected zero value of Stability to be StabilityStable (%d), got %d", x.StabilityStable, zero)
	}

	for _, tc := range []struct {
		name string
		s    x.Stability
		want int
	}{
		{name: "Development", s: x.StabilityDevelopment, want: -1},
		{name: "Stable", s: x.StabilityStable, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opt := x.WithStability(tc.s)
			if _, ok := opt.(interface{ Experimental() }); !ok {
				t.Fatal("WithStability does not implement Experimental()")
			}
			sOpt, ok := opt.(interface{ Stability() int })
			if !ok {
				t.Fatal("WithStability does not implement Stability() int")
			}
			if got := sOpt.Stability(); got != tc.want {
				t.Fatalf("Stability() = %d, want %d", got, tc.want)
			}
			// Verify trace.NewSpanStartConfig ignores experimental options without panicking.
			_ = trace.NewSpanStartConfig(opt)
		})
	}
}

type mockEnabledTracer struct {
	trace.Tracer
	enabled bool
}

func (m mockEnabledTracer) Enabled(context.Context, ...trace.SpanStartOption) bool {
	return m.enabled
}

func TestEnabled(t *testing.T) {
	ctx := context.Background()
	if x.Enabled(ctx, nil) {
		t.Fatal("expected Enabled(ctx, nil) to be false")
	}
	if x.Enabled(ctx, noop.NewTracerProvider().Tracer("test")) {
		t.Fatal("expected Enabled on noop.Tracer to be false")
	}
	if !x.Enabled(ctx, mockEnabledTracer{enabled: true}) {
		t.Fatal("expected Enabled on mockEnabledTracer{enabled: true} to be true")
	}
	if x.Enabled(ctx, mockEnabledTracer{enabled: false}) {
		t.Fatal("expected Enabled on mockEnabledTracer{enabled: false} to be false")
	}
}

type mockAttributeEnabledTracer struct {
	trace.Tracer
	gotKey       attribute.Key
	gotStability int
}

func (m *mockAttributeEnabledTracer) ExperimentalAttributeEnabled(_ context.Context, key attribute.Key, stability int) bool {
	m.gotKey, m.gotStability = key, stability
	return stability >= 0
}

func TestAttributeEnabled(t *testing.T) {
	ctx := t.Context()
	if x.AttributeEnabled(ctx, nil, "k") {
		t.Fatal("expected AttributeEnabled(ctx, nil, \"k\") to be false")
	}
	if x.AttributeEnabled(ctx, noop.NewTracerProvider().Tracer("test"), "k") {
		t.Fatal("expected AttributeEnabled on a no-op Tracer to be false")
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
			m := &mockAttributeEnabledTracer{}
			got := x.AttributeEnabled(ctx, m, "attr", tc.opts...)
			if want := tc.want == x.StabilityStable; got != want {
				t.Fatalf("AttributeEnabled() = %t, want %t", got, want)
			}
			if m.gotKey != "attr" || m.gotStability != int(tc.want) {
				t.Fatalf("got key=%q stability=%d, want key=\"attr\" stability=%d", m.gotKey, m.gotStability, tc.want)
			}
		})
	}
}
