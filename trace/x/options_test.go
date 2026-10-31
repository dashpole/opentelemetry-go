// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"testing"

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
