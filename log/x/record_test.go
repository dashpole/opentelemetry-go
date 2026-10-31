// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/log/x"
)

type mockStabilityLogger struct {
	log.Logger
	lastEmitStability    int
	lastEnabledStability int
	enabled              bool
}

func (m *mockStabilityLogger) ExperimentalEmit(_ context.Context, _ log.Record, stability int) {
	m.lastEmitStability = stability
}

func (m *mockStabilityLogger) ExperimentalEnabled(_ context.Context, _ log.EnabledParameters, stability int) bool {
	m.lastEnabledStability = stability
	return m.enabled
}

func TestRecordAndEnabledParameters(t *testing.T) {
	ctx := context.Background()

	// Record methods and Clone preserve embedded log.Record fields and Stability.
	var r x.Record
	if r.Stability() != x.StabilityStable {
		t.Fatalf("got default Stability %d, want %d", r.Stability(), x.StabilityStable)
	}
	r.SetEventName("dev.event")
	r.SetSeverity(log.SeverityInfo)
	r.SetStability(x.StabilityDevelopment)
	r.AddAttributes(attribute.String("k", "v"))

	cloned := r.Clone()
	cloned.SetStability(x.StabilityStable)
	if r.Stability() != x.StabilityDevelopment {
		t.Fatalf("mutating clone changed original stability: got %d", r.Stability())
	}
	if cloned.EventName() != "dev.event" || cloned.Severity() != log.SeverityInfo || cloned.AttributesLen() != 1 {
		t.Fatalf("clone did not preserve embedded Record fields: %+v", cloned)
	}

	// Nil logger should not panic and should return false for Enabled.
	x.Emit(ctx, nil, r)
	if x.Enabled(ctx, nil, x.EnabledParameters{}) {
		t.Fatal("expected Enabled(ctx, nil, ...) to be false")
	}

	// Noop logger fallback.
	noopLogger := noop.NewLoggerProvider().Logger("test")
	x.Emit(ctx, noopLogger, r)
	if x.Enabled(ctx, noopLogger, x.EnabledParameters{Stability: x.StabilityDevelopment}) {
		t.Fatal("expected Enabled on noop.Logger to be false")
	}

	// ExperimentalEmit / ExperimentalEnabled dispatch.
	mock := &mockStabilityLogger{enabled: true}
	x.Emit(ctx, mock, r)
	if mock.lastEmitStability != int(x.StabilityDevelopment) {
		t.Fatalf("got Emit stability %d, want %d", mock.lastEmitStability, x.StabilityDevelopment)
	}
	if !x.Enabled(ctx, mock, x.EnabledParameters{Severity: log.SeverityInfo, EventName: "dev.event", Stability: x.StabilityDevelopment}) {
		t.Fatal("expected Enabled on mockStabilityLogger to be true")
	}
	if mock.lastEnabledStability != int(x.StabilityDevelopment) {
		t.Fatalf("got Enabled stability %d, want %d", mock.lastEnabledStability, x.StabilityDevelopment)
	}
}
