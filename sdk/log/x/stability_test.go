// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/log"
	logx "go.opentelemetry.io/otel/log/x"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdklogx "go.opentelemetry.io/otel/sdk/log/x"
)

type recordingProcessor struct {
	sdklog.Processor
	records      []sdklog.Record
	enabledCalls atomic.Int64
}

func (p *recordingProcessor) OnEmit(_ context.Context, r *sdklog.Record) error {
	p.records = append(p.records, r.Clone())
	return nil
}

func (p *recordingProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool {
	p.enabledCalls.Add(1)
	return true
}

func (*recordingProcessor) Shutdown(context.Context) error   { return nil }
func (*recordingProcessor) ForceFlush(context.Context) error { return nil }

func TestLoggerMinimumStability(t *testing.T) {
	for _, tc := range []struct {
		name               string
		providerOpts       []sdklog.LoggerProviderOption
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
			providerOpts:       []sdklog.LoggerProviderOption{sdklogx.WithMinimumStability(sdklogx.StabilityStable)},
			wantStableRecorded: true,
			wantDevRecorded:    false,
		},
		{
			name:               "ExplicitMinimumStabilityDevelopment",
			providerOpts:       []sdklog.LoggerProviderOption{sdklogx.WithMinimumStability(sdklogx.StabilityDevelopment)},
			wantStableRecorded: true,
			wantDevRecorded:    true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			proc := &recordingProcessor{}

			opts := append([]sdklog.LoggerProviderOption{
				sdklog.WithProcessor(proc),
			}, tc.providerOpts...)
			lp := sdklog.NewLoggerProvider(opts...)
			t.Cleanup(func() { _ = lp.Shutdown(ctx) })

			logger := lp.Logger("test-logger")

			// Check Enabled for standard Logger.Enabled, logx.Enabled (default), Stable, and Development.
			assert.Equal(t, tc.wantStableRecorded, logger.Enabled(ctx, log.EnabledParameters{}))
			assert.Equal(t, tc.wantStableRecorded, logx.Enabled(ctx, logger, logx.EnabledParameters{}))
			assert.Equal(t, tc.wantStableRecorded, logx.Enabled(ctx, logger, logx.EnabledParameters{Stability: logx.StabilityStable}))

			enabledCallsBeforeDev := proc.enabledCalls.Load()
			assert.Equal(t, tc.wantDevRecorded, logx.Enabled(ctx, logger, logx.EnabledParameters{Stability: logx.StabilityDevelopment}))
			if !tc.wantDevRecorded {
				assert.Equal(t, enabledCallsBeforeDev, proc.enabledCalls.Load(), "Processor.Enabled must not be invoked when suppressed by minimum_stability")
			}

			// Emit default (via Logger.Emit), default (via logx.Emit), explicit Stable, and Development records.
			var rDefault log.Record
			rDefault.SetEventName("default-event")
			logger.Emit(ctx, rDefault)

			var rLogxDefault logx.Record
			rLogxDefault.SetEventName("logx-default-event")
			logx.Emit(ctx, logger, rLogxDefault)

			var rStable logx.Record
			rStable.SetEventName("stable-event")
			rStable.SetStability(logx.StabilityStable)
			logx.Emit(ctx, logger, rStable)

			var rDev logx.Record
			rDev.SetEventName("dev-event")
			rDev.SetStability(logx.StabilityDevelopment)
			logx.Emit(ctx, logger, rDev)

			var gotEvents []string
			for _, rec := range proc.records {
				gotEvents = append(gotEvents, rec.EventName())
			}

			if tc.wantDevRecorded {
				require.Equal(t, []string{"default-event", "logx-default-event", "stable-event", "dev-event"}, gotEvents)
			} else {
				require.Equal(t, []string{"default-event", "logx-default-event", "stable-event"}, gotEvents)
			}
		})
	}
}
