// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package global

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	lognoop "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// attributeEnabler enables attributes with a stability of at least Stable (0).
type attributeEnabler struct{}

func (attributeEnabler) ExperimentalAttributeEnabled(_ context.Context, _ attribute.Key, stability int) bool {
	return stability >= 0
}

type attributeEnabledTracer struct {
	tracenoop.Tracer
	attributeEnabler
}

type attributeEnabledMeter struct {
	metricnoop.Meter
	attributeEnabler
}

type attributeEnabledMeterProvider struct{ metricnoop.MeterProvider }

func (attributeEnabledMeterProvider) Meter(string, ...metric.MeterOption) metric.Meter {
	return attributeEnabledMeter{}
}

type attributeEnabledLogger struct {
	lognoop.Logger
	attributeEnabler
}

type attributeEnabledLoggerProvider struct{ lognoop.LoggerProvider }

func (attributeEnabledLoggerProvider) Logger(string, ...log.LoggerOption) log.Logger {
	return attributeEnabledLogger{}
}

func TestTracerAttributeEnabled(t *testing.T) {
	ctx := t.Context()
	tp := &tracerProvider{}
	tr := tp.Tracer("test").(*tracer)
	assert.False(t, tr.ExperimentalAttributeEnabled(ctx, "attr", 0), "no delegate")

	tp.setDelegate(fnTracerProvider{
		tracer: func(string, ...trace.TracerOption) trace.Tracer { return attributeEnabledTracer{} },
	})
	assert.True(t, tr.ExperimentalAttributeEnabled(ctx, "attr", 0))
	assert.False(t, tr.ExperimentalAttributeEnabled(ctx, "attr", -1))
}

func TestMeterAttributeEnabled(t *testing.T) {
	ctx := t.Context()
	mp := &meterProvider{}
	m := mp.Meter("test").(*meter)
	assert.False(t, m.ExperimentalAttributeEnabled(ctx, "attr", 0), "no delegate")

	mp.setDelegate(attributeEnabledMeterProvider{})
	assert.True(t, m.ExperimentalAttributeEnabled(ctx, "attr", 0))
	assert.False(t, m.ExperimentalAttributeEnabled(ctx, "attr", -1))
}

func TestLoggerAttributeEnabled(t *testing.T) {
	ctx := t.Context()
	lp := &loggerProvider{}
	l := lp.Logger("test").(*logger)
	assert.False(t, l.ExperimentalAttributeEnabled(ctx, "attr", 0), "no delegate")

	lp.setDelegate(attributeEnabledLoggerProvider{})
	assert.True(t, l.ExperimentalAttributeEnabled(ctx, "attr", 0))
	assert.False(t, l.ExperimentalAttributeEnabled(ctx, "attr", -1))
}

func TestTracerEnabledNoopDelegate(t *testing.T) {
	tp := &tracerProvider{}
	tr := tp.Tracer("test").(*tracer)
	assert.False(t, tr.Enabled(t.Context()), "no delegate")

	tp.setDelegate(tracenoop.NewTracerProvider())
	assert.False(t, tr.Enabled(t.Context()), "noop delegate")
}
