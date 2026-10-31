// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	sdktracex "go.opentelemetry.io/otel/sdk/trace/x"
	"go.opentelemetry.io/otel/trace"
	tracex "go.opentelemetry.io/otel/trace/x"
)

type countingIDGenerator struct {
	newIDsCalls    atomic.Int64
	newSpanIDCalls atomic.Int64
}

func (g *countingIDGenerator) NewIDs(context.Context) (trace.TraceID, trace.SpanID) {
	g.newIDsCalls.Add(1)
	return trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
}

func (g *countingIDGenerator) NewSpanID(context.Context, trace.TraceID) trace.SpanID {
	g.newSpanIDCalls.Add(1)
	return trace.SpanID{8, 7, 6, 5, 4, 3, 2, 1}
}

type countingSampler struct {
	calls atomic.Int64
}

func (s *countingSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	s.calls.Add(1)
	return sdktrace.AlwaysSample().ShouldSample(p)
}

func (*countingSampler) Description() string { return "countingSampler" }

func TestTracerMinimumStability(t *testing.T) {
	for _, tc := range []struct {
		name               string
		providerOpts       []sdktrace.TracerProviderOption
		wantStableRecorded bool
		wantDevRecorded    bool
	}{
		{
			name:               "DefaultProvider_MinimumStabilityStable",
			providerOpts:       nil,
			wantStableRecorded: true,
			wantDevRecorded:    false,
		},
		{
			name:               "ExplicitMinimumStabilityStable",
			providerOpts:       []sdktrace.TracerProviderOption{sdktracex.WithMinimumStability(sdktracex.StabilityStable)},
			wantStableRecorded: true,
			wantDevRecorded:    false,
		},
		{
			name:               "ExplicitMinimumStabilityDevelopment",
			providerOpts:       []sdktrace.TracerProviderOption{sdktracex.WithMinimumStability(sdktracex.StabilityDevelopment)},
			wantStableRecorded: true,
			wantDevRecorded:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			sr := tracetest.NewSpanRecorder()
			idGen := &countingIDGenerator{}
			sampler := &countingSampler{}

			opts := append([]sdktrace.TracerProviderOption{
				sdktrace.WithSpanProcessor(sr),
				sdktrace.WithIDGenerator(idGen),
				sdktrace.WithSampler(sampler),
			}, tc.providerOpts...)
			tp := sdktrace.NewTracerProvider(opts...)
			t.Cleanup(func() { _ = tp.Shutdown(ctx) })

			tr := tp.Tracer("test-tracer")

			// Check Enabled for default (unspecified), explicit Stable, and Development.
			assert.Equal(t, tc.wantStableRecorded, tracex.Enabled(ctx, tr))
			assert.Equal(t, tc.wantStableRecorded, tracex.Enabled(ctx, tr, tracex.WithStability(tracex.StabilityStable)))
			assert.Equal(t, tc.wantDevRecorded, tracex.Enabled(ctx, tr, tracex.WithStability(tracex.StabilityDevelopment)))

			// 1. Default span (no WithStability option -> StabilityStable)
			parentCtx, defaultSpan := tr.Start(ctx, "default-span")
			assert.Equal(t, tc.wantStableRecorded, defaultSpan.IsRecording())

			// 2. Explicit Stable span
			_, stableSpan := tr.Start(parentCtx, "stable-span", tracex.WithStability(tracex.StabilityStable))
			assert.Equal(t, tc.wantStableRecorded, stableSpan.IsRecording())
			stableSpan.End()

			idCallsBeforeDev := idGen.newIDsCalls.Load() + idGen.newSpanIDCalls.Load()
			samplerCallsBeforeDev := sampler.calls.Load()

			// 3. Development span as child of parentCtx
			devCtx, devSpan := tr.Start(parentCtx, "dev-span", tracex.WithStability(tracex.StabilityDevelopment))
			assert.Equal(t, tc.wantDevRecorded, devSpan.IsRecording())
			if !tc.wantDevRecorded {
				// Must not invoke IDGenerator or Sampler when suppressed by minimum_stability.
				assert.Equal(t, idCallsBeforeDev, idGen.newIDsCalls.Load()+idGen.newSpanIDCalls.Load(), "IDGenerator should not be invoked for suppressed Development span")
				assert.Equal(t, samplerCallsBeforeDev, sampler.calls.Load(), "Sampler should not be invoked for suppressed Development span")
				// Must preserve parent SpanContext in returned Context.
				assert.Equal(t, defaultSpan.SpanContext(), trace.SpanContextFromContext(devCtx), "parent SpanContext must be preserved in returned Context")
				assert.Equal(t, defaultSpan.SpanContext(), devSpan.SpanContext(), "non-recording Span must carry parent SpanContext")
			}

			// 4. Child Stable span under suppressed Development span must inherit parentCtx's SpanContext.
			_, childAfterDevSpan := tr.Start(devCtx, "child-after-dev", tracex.WithStability(tracex.StabilityStable))
			assert.Equal(t, tc.wantStableRecorded, childAfterDevSpan.IsRecording())
			childAfterDevSpan.End()

			devSpan.End()
			defaultSpan.End()

			ended := sr.Ended()
			var endedNames []string
			for _, s := range ended {
				endedNames = append(endedNames, s.Name())
			}

			if tc.wantDevRecorded {
				assert.ElementsMatch(t, []string{"default-span", "stable-span", "dev-span", "child-after-dev"}, endedNames)
			} else {
				assert.ElementsMatch(t, []string{"default-span", "stable-span", "child-after-dev"}, endedNames)
				// Verify child-after-dev has defaultSpan as parent, and defaultSpan has ChildSpanCount == 2 (not 3).
				for _, s := range ended {
					switch s.Name() {
					case "child-after-dev":
						assert.Equal(t, defaultSpan.SpanContext().SpanID(), s.Parent().SpanID())
					case "default-span":
						assert.Equal(t, 1, s.ChildSpanCount(), "suppressed Development span must not increment parent ChildSpanCount")
					}
				}
			}
		})
	}
}

func TestTracerEnabledWithoutProcessorsOrAfterShutdown(t *testing.T) {
	ctx := t.Context()

	// TracerProvider with no SpanProcessors: Enabled is false.
	tpNoProc := sdktrace.NewTracerProvider()
	trNoProc := tpNoProc.Tracer("no-proc")
	assert.False(t, tracex.Enabled(ctx, trNoProc))

	// TracerProvider with a SpanProcessor: Enabled is true until Shutdown.
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	tr := tp.Tracer("with-proc")
	assert.True(t, tracex.Enabled(ctx, tr))

	require.NoError(t, tp.Shutdown(ctx))
	assert.False(t, tracex.Enabled(ctx, tr))
}
