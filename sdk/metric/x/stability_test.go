// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricx "go.opentelemetry.io/otel/metric/x"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"
	"go.opentelemetry.io/otel/sdk/metric/x"
)

func TestStability2x2MatrixAllInstrumentKinds(t *testing.T) {
	type instrumentCase struct {
		name     string
		instName string
		isSync   bool
		record   func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (enabled bool, callbackCalled *atomic.Bool)
		wantData metricdata.Aggregation
	}

	cases := []instrumentCase{
		{
			name:     "Int64Counter",
			instName: "int64.counter",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Int64CounterOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Int64Counter("int64.counter", cOpts...)
				require.NoError(t, err)
				inst.Add(t.Context(), 5)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints:  []metricdata.DataPoint[int64]{{Value: 5}},
			},
		},
		{
			name:     "Float64Counter",
			instName: "float64.counter",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Float64CounterOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Float64Counter("float64.counter", cOpts...)
				require.NoError(t, err)
				inst.Add(t.Context(), 5.5)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Sum[float64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints:  []metricdata.DataPoint[float64]{{Value: 5.5}},
			},
		},
		{
			name:     "Int64UpDownCounter",
			instName: "int64.updowncounter",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Int64UpDownCounterOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Int64UpDownCounter("int64.updowncounter", cOpts...)
				require.NoError(t, err)
				inst.Add(t.Context(), -3)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: false,
				DataPoints:  []metricdata.DataPoint[int64]{{Value: -3}},
			},
		},
		{
			name:     "Float64UpDownCounter",
			instName: "float64.updowncounter",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Float64UpDownCounterOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Float64UpDownCounter("float64.updowncounter", cOpts...)
				require.NoError(t, err)
				inst.Add(t.Context(), -3.5)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Sum[float64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: false,
				DataPoints:  []metricdata.DataPoint[float64]{{Value: -3.5}},
			},
		},
		{
			name:     "Int64Histogram",
			instName: "int64.histogram",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Int64HistogramOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Int64Histogram("int64.histogram", cOpts...)
				require.NoError(t, err)
				inst.Record(t.Context(), 10)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Histogram[int64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[int64]{
					{
						Count:        1,
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Min:          metricdata.NewExtrema[int64](10),
						Max:          metricdata.NewExtrema[int64](10),
						Sum:          10,
					},
				},
			},
		},
		{
			name:     "Float64Histogram",
			instName: "float64.histogram",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Float64HistogramOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Float64Histogram("float64.histogram", cOpts...)
				require.NoError(t, err)
				inst.Record(t.Context(), 10.0)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Histogram[float64]{
				Temporality: metricdata.CumulativeTemporality,
				DataPoints: []metricdata.HistogramDataPoint[float64]{
					{
						Count:        1,
						Bounds:       []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 750, 1000, 2500, 5000, 7500, 10000},
						BucketCounts: []uint64{0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
						Min:          metricdata.NewExtrema[float64](10.0),
						Max:          metricdata.NewExtrema[float64](10.0),
						Sum:          10.0,
					},
				},
			},
		},
		{
			name:     "Int64Gauge",
			instName: "int64.gauge",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Int64GaugeOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Int64Gauge("int64.gauge", cOpts...)
				require.NoError(t, err)
				inst.Record(t.Context(), 42)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{{Value: 42}},
			},
		},
		{
			name:     "Float64Gauge",
			instName: "float64.gauge",
			isSync:   true,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var cOpts []metric.Float64GaugeOption
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				inst, err := m.Float64Gauge("float64.gauge", cOpts...)
				require.NoError(t, err)
				inst.Record(t.Context(), 42.5)
				return inst.Enabled(t.Context()), nil
			},
			wantData: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{{Value: 42.5}},
			},
		},
		{
			name:     "Int64ObservableCounter",
			instName: "int64.observable.counter",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Int64ObservableCounterOption{
					metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
						called.Store(true)
						o.Observe(7)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Int64ObservableCounter("int64.observable.counter", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints:  []metricdata.DataPoint[int64]{{Value: 7}},
			},
		},
		{
			name:     "Float64ObservableCounter",
			instName: "float64.observable.counter",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Float64ObservableCounterOption{
					metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
						called.Store(true)
						o.Observe(7.5)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Float64ObservableCounter("float64.observable.counter", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Sum[float64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: true,
				DataPoints:  []metricdata.DataPoint[float64]{{Value: 7.5}},
			},
		},
		{
			name:     "Int64ObservableUpDownCounter",
			instName: "int64.observable.updowncounter",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Int64ObservableUpDownCounterOption{
					metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
						called.Store(true)
						o.Observe(-9)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Int64ObservableUpDownCounter("int64.observable.updowncounter", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Sum[int64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: false,
				DataPoints:  []metricdata.DataPoint[int64]{{Value: -9}},
			},
		},
		{
			name:     "Float64ObservableUpDownCounter",
			instName: "float64.observable.updowncounter",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Float64ObservableUpDownCounterOption{
					metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
						called.Store(true)
						o.Observe(-9.5)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Float64ObservableUpDownCounter("float64.observable.updowncounter", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Sum[float64]{
				Temporality: metricdata.CumulativeTemporality,
				IsMonotonic: false,
				DataPoints:  []metricdata.DataPoint[float64]{{Value: -9.5}},
			},
		},
		{
			name:     "Int64ObservableGauge",
			instName: "int64.observable.gauge",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Int64ObservableGaugeOption{
					metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
						called.Store(true)
						o.Observe(99)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Int64ObservableGauge("int64.observable.gauge", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Gauge[int64]{
				DataPoints: []metricdata.DataPoint[int64]{{Value: 99}},
			},
		},
		{
			name:     "Float64ObservableGauge",
			instName: "float64.observable.gauge",
			isSync:   false,
			record: func(t *testing.T, m metric.Meter, opts ...metric.InstrumentOption) (bool, *atomic.Bool) {
				var called atomic.Bool
				cOpts := []metric.Float64ObservableGaugeOption{
					metric.WithFloat64Callback(func(_ context.Context, o metric.Float64Observer) error {
						called.Store(true)
						o.Observe(99.5)
						return nil
					}),
				}
				for _, o := range opts {
					cOpts = append(cOpts, o)
				}
				_, err := m.Float64ObservableGauge("float64.observable.gauge", cOpts...)
				require.NoError(t, err)
				return false, &called
			},
			wantData: metricdata.Gauge[float64]{
				DataPoints: []metricdata.DataPoint[float64]{{Value: 99.5}},
			},
		},
	}

	type matrixRow struct {
		name              string
		instOpts          []metric.InstrumentOption
		wantDefaultStable bool
		wantMinDev        bool
		wantViewEnabled   bool
		wantViewDisabled  bool
	}

	matrix := []matrixRow{
		{
			name:              "Stable_OptInFalse_Implicit",
			instOpts:          nil,
			wantDefaultStable: true,
			wantMinDev:        true,
			wantViewEnabled:   true,
			wantViewDisabled:  false,
		},
		{
			name:              "Stable_OptInFalse_Explicit",
			instOpts:          []metric.InstrumentOption{metricx.WithStability(metricx.StabilityStable)},
			wantDefaultStable: true,
			wantMinDev:        true,
			wantViewEnabled:   true,
			wantViewDisabled:  false,
		},
		{
			name:              "Stable_OptInTrue",
			instOpts:          []metric.InstrumentOption{metricx.WithStability(metricx.StabilityStable), metricx.WithOptIn()},
			wantDefaultStable: false,
			wantMinDev:        false,
			wantViewEnabled:   true,
			wantViewDisabled:  false,
		},
		{
			name:              "Development_OptInFalse",
			instOpts:          []metric.InstrumentOption{metricx.WithStability(metricx.StabilityDevelopment)},
			wantDefaultStable: false,
			wantMinDev:        true,
			wantViewEnabled:   true,
			wantViewDisabled:  false,
		},
		{
			name:              "Development_OptInTrue",
			instOpts:          []metric.InstrumentOption{metricx.WithStability(metricx.StabilityDevelopment), metricx.WithOptIn()},
			wantDefaultStable: false,
			wantMinDev:        false,
			wantViewEnabled:   true,
			wantViewDisabled:  false,
		},
	}

	assertResult := func(
		t *testing.T,
		tc instrumentCase,
		rdr *sdkmetric.ManualReader,
		enabled bool,
		cbCalled *atomic.Bool,
		wantEnabled bool,
	) {
		t.Helper()
		if tc.isSync {
			assert.Equal(t, wantEnabled, enabled)
		}
		var rm metricdata.ResourceMetrics
		require.NoError(t, rdr.Collect(t.Context(), &rm))
		if wantEnabled {
			require.Len(t, rm.ScopeMetrics, 1)
			require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
			metricdatatest.AssertEqual(
				t,
				metricdata.Metrics{
					Name: tc.instName,
					Data: tc.wantData,
				},
				rm.ScopeMetrics[0].Metrics[0],
				metricdatatest.IgnoreTimestamp(),
				metricdatatest.IgnoreExemplars(),
			)
		} else {
			assert.Empty(t, rm.ScopeMetrics)
		}
		if cbCalled != nil {
			assert.Equal(t, wantEnabled, cbCalled.Load())
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, row := range matrix {
				t.Run(row.name, func(t *testing.T) {
					t.Run("DefaultMinimumStability_Stable", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						assertResult(t, tc, rdr, enabled, cbCalled, row.wantDefaultStable)
					})

					t.Run("ExplicitMinimumStability_Stable", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						mp := sdkmetric.NewMeterProvider(
							sdkmetric.WithReader(rdr),
							x.WithMinimumStability(x.StabilityStable),
						)
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						assertResult(t, tc, rdr, enabled, cbCalled, row.wantDefaultStable)
					})

					t.Run("MinimumStability_Development", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						mp := sdkmetric.NewMeterProvider(
							sdkmetric.WithReader(rdr),
							x.WithMinimumStability(x.StabilityDevelopment),
						)
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						assertResult(t, tc, rdr, enabled, cbCalled, row.wantMinDev)
					})

					t.Run("MatchingView_NilEnabled", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						view := x.NewView(
							sdkmetric.Instrument{Name: tc.instName},
							x.Stream{
								Description: "custom description",
								Aggregation: sdkmetric.AggregationDefault{},
							},
						)
						mp := sdkmetric.NewMeterProvider(
							sdkmetric.WithReader(rdr),
							x.WithView(view),
						)
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						if tc.isSync {
							assert.Equal(t, row.wantDefaultStable, enabled)
						}
						var rm metricdata.ResourceMetrics
						require.NoError(t, rdr.Collect(t.Context(), &rm))
						if row.wantDefaultStable {
							require.Len(t, rm.ScopeMetrics, 1)
							require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
						} else {
							assert.Empty(t, rm.ScopeMetrics)
						}
						if cbCalled != nil {
							assert.Equal(t, row.wantDefaultStable, cbCalled.Load())
						}
					})

					t.Run("MatchingView_EnabledTrue", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						view := x.NewView(
							sdkmetric.Instrument{Name: tc.instName},
							x.Stream{Enabled: new(true)},
						)
						mp := sdkmetric.NewMeterProvider(
							sdkmetric.WithReader(rdr),
							x.WithView(view),
						)
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						assertResult(t, tc, rdr, enabled, cbCalled, row.wantViewEnabled)
					})

					t.Run("MatchingView_EnabledFalse_WithMinDev", func(t *testing.T) {
						rdr := sdkmetric.NewManualReader()
						view := x.NewView(
							sdkmetric.Instrument{Name: tc.instName},
							x.Stream{
								Aggregation: sdkmetric.AggregationDefault{},
								Enabled:     new(false),
							},
						)
						mp := sdkmetric.NewMeterProvider(
							sdkmetric.WithReader(rdr),
							x.WithMinimumStability(x.StabilityDevelopment),
							x.WithView(view),
						)
						m := mp.Meter("test")
						enabled, cbCalled := tc.record(t, m, row.instOpts...)
						assertResult(t, tc, rdr, enabled, cbCalled, row.wantViewDisabled)
					})
				})
			}
		})
	}
}

func TestDevelopmentStabilityWithRegisterCallback(t *testing.T) {
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))
	m := mp.Meter("test")

	obs, err := m.Int64ObservableGauge(
		"dev.obs",
		metricx.WithStability(metricx.StabilityDevelopment),
	)
	require.NoError(t, err)

	var called atomic.Bool
	reg, err := m.RegisterCallback(func(_ context.Context, o metric.Observer) error {
		called.Store(true)
		o.ObserveInt64(obs, 123)
		return nil
	}, obs)
	require.NoError(t, err)
	defer func() { require.NoError(t, reg.Unregister()) }()

	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(t.Context(), &rm))
	assert.Empty(t, rm.ScopeMetrics)
	assert.False(t, called.Load(), "callback should not be invoked when all registered instruments are disabled by stability")
}

func TestDevelopmentStabilityWithFinishAndMeterConfigurator(t *testing.T) {
	t.Run("WithFinish", func(t *testing.T) {
		rdr := sdkmetric.NewManualReader()
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(rdr),
			x.WithFinish(),
			x.WithMinimumStability(x.StabilityDevelopment),
		)
		t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })
		m := mp.Meter("test")

		ic, err := m.Int64Counter("dev.int64", metricx.WithStability(metricx.StabilityDevelopment))
		require.NoError(t, err)
		assert.True(t, ic.Enabled(t.Context()))
		ic.Add(t.Context(), 10, metric.WithAttributes(attribute.String("k", "v")))
		ic.(metricx.Finisher).Finish(t.Context(), attribute.String("k", "v"))

		var rm metricdata.ResourceMetrics
		require.NoError(t, rdr.Collect(t.Context(), &rm))
		require.Len(t, rm.ScopeMetrics, 1)
		require.Len(t, rm.ScopeMetrics[0].Metrics, 1)
	})

	t.Run("WithMeterConfigurator", func(t *testing.T) {
		rdr := sdkmetric.NewManualReader()
		handle := x.NewMeterConfiguratorHandle()
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(rdr),
			x.WithMeterConfigurator(handle),
			x.WithMinimumStability(x.StabilityDevelopment),
		)
		t.Cleanup(func() { _ = mp.Shutdown(t.Context()) })
		m := mp.Meter("test")

		devInst, err := m.Int64Counter("dev.counter", metricx.WithStability(metricx.StabilityDevelopment))
		require.NoError(t, err)
		assert.True(t, devInst.Enabled(t.Context()))

		devOptInInst, err := m.Int64Counter(
			"dev.optin.counter",
			metricx.WithStability(metricx.StabilityDevelopment),
			metricx.WithOptIn(),
		)
		require.NoError(t, err)
		assert.False(t, devOptInInst.Enabled(t.Context()))

		var called atomic.Bool
		_, err = m.Int64ObservableCounter(
			"dev.obs",
			metricx.WithStability(metricx.StabilityDevelopment),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				called.Store(true)
				o.Observe(1)
				return nil
			}),
		)
		require.NoError(t, err)

		handle.Set(func(instrumentation.Scope) x.MeterConfig {
			return x.NewMeterConfig(x.WithMeterEnabled(false))
		})
		assert.False(t, devInst.Enabled(t.Context()))

		var rm metricdata.ResourceMetrics
		require.NoError(t, rdr.Collect(t.Context(), &rm))
		assert.Empty(t, rm.ScopeMetrics)
		assert.False(t, called.Load())
	})
}

func TestMeterAttributeEnabled(t *testing.T) {
	ctx := t.Context()
	dev := metricx.StabilityDevelopment
	scope := instrumentation.Scope{Name: "test"}

	t.Run("NoReaders", func(t *testing.T) {
		mp := sdkmetric.NewMeterProvider(x.WithMinimumStability(x.StabilityDevelopment))
		m := mp.Meter("test")
		assert.False(t, metricx.AttributeEnabled(ctx, m, "stable.attr"))
		assert.False(t, metricx.AttributeEnabled(ctx, m, "dev.attr", dev))
	})

	for _, tc := range []struct {
		name    string
		opts    []sdkmetric.Option
		wantDev bool
	}{
		{name: "DefaultMinimumStability"},
		{name: "MinimumStabilityDevelopment", opts: []sdkmetric.Option{x.WithMinimumStability(x.StabilityDevelopment)}, wantDev: true},
		{name: "WithFinish", opts: []sdkmetric.Option{x.WithFinish()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]sdkmetric.Option{sdkmetric.WithReader(sdkmetric.NewManualReader())}, tc.opts...)
			m := sdkmetric.NewMeterProvider(opts...).Meter("test")
			assert.True(t, metricx.AttributeEnabled(ctx, m, "stable.attr"))
			assert.True(t, metricx.AttributeEnabled(ctx, m, "stable.attr", metricx.StabilityStable))
			assert.Equal(t, tc.wantDev, metricx.AttributeEnabled(ctx, m, "dev.attr", dev))
		})
	}

	t.Run("MeterConfigurator", func(t *testing.T) {
		handle := x.NewMeterConfiguratorHandle()
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			x.WithMeterConfigurator(handle),
			x.WithMinimumStability(x.StabilityDevelopment),
		)
		m := mp.Meter("test")
		assert.True(t, metricx.AttributeEnabled(ctx, m, "stable.attr"))
		assert.True(t, metricx.AttributeEnabled(ctx, m, "dev.attr", dev))

		handle.Set(func(instrumentation.Scope) x.MeterConfig {
			return x.NewMeterConfig(x.WithMeterEnabled(false))
		})
		assert.False(t, metricx.AttributeEnabled(ctx, m, "stable.attr"))
		assert.False(t, metricx.AttributeEnabled(ctx, m, "dev.attr", dev))
	})

	t.Run("ViewIncludesDevelopmentAttribute", func(t *testing.T) {
		// The View selects a single instrument of the "test" scope, but it
		// matches the "test" Meter regardless of which instruments exist.
		view := sdkmetric.NewView(
			sdkmetric.Instrument{Name: "http.server.request.duration", Scope: scope},
			sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter("http.request.method", "url.template")},
		)
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			sdkmetric.WithView(view),
		)
		m := mp.Meter("test")
		assert.True(t, metricx.AttributeEnabled(ctx, m, "url.template", dev))
		assert.False(t, metricx.AttributeEnabled(ctx, m, "http.request.body.size", dev))
		// Stable attributes are not affected by the View.
		assert.True(t, metricx.AttributeEnabled(ctx, m, "server.address"))
		// The View does not match the "other" Meter.
		assert.False(t, metricx.AttributeEnabled(ctx, mp.Meter("other"), "url.template", dev))
	})

	t.Run("ViewIncludeOverrides", func(t *testing.T) {
		include := attribute.NewAllowKeysFilter("url.template")
		for _, tc := range []struct {
			name      string
			view      sdkmetric.Option
			configure func(*x.MeterConfiguratorHandle)
			want      bool
		}{
			{
				name: "ScopeLessView",
				view: sdkmetric.WithView(sdkmetric.NewView(sdkmetric.Instrument{Name: "*"}, sdkmetric.Stream{AttributeFilter: include})),
				want: true,
			},
			{
				name: "ScopeVersionMismatch",
				view: sdkmetric.WithView(sdkmetric.NewView(
					sdkmetric.Instrument{Scope: instrumentation.Scope{Name: "test", Version: "v2"}},
					sdkmetric.Stream{AttributeFilter: include},
				)),
			},
			{
				name: "CustomView",
				view: sdkmetric.WithView(func(i sdkmetric.Instrument) (sdkmetric.Stream, bool) {
					return sdkmetric.Stream{AttributeFilter: include}, i.Scope == scope
				}),
				want: true,
			},
			{
				name: "DroppedView",
				view: sdkmetric.WithView(sdkmetric.NewView(
					sdkmetric.Instrument{Scope: scope},
					sdkmetric.Stream{AttributeFilter: include, Aggregation: sdkmetric.AggregationDrop{}},
				)),
			},
			{
				name: "DisabledView",
				view: x.WithView(x.NewView(sdkmetric.Instrument{Scope: scope}, x.Stream{AttributeFilter: include, Enabled: new(false)})),
			},
			{
				name: "DisabledMeter",
				view: sdkmetric.WithView(sdkmetric.NewView(sdkmetric.Instrument{Scope: scope}, sdkmetric.Stream{AttributeFilter: include})),
				configure: func(h *x.MeterConfiguratorHandle) {
					h.Set(func(instrumentation.Scope) x.MeterConfig {
						return x.NewMeterConfig(x.WithMeterEnabled(false))
					})
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				handle := x.NewMeterConfiguratorHandle()
				mp := sdkmetric.NewMeterProvider(
					sdkmetric.WithReader(sdkmetric.NewManualReader()),
					tc.view,
					x.WithMeterConfigurator(handle),
				)
				m := mp.Meter("test")
				if tc.configure != nil {
					tc.configure(handle)
				}
				assert.Equal(t, tc.want, metricx.AttributeEnabled(ctx, m, "url.template", dev))
			})
		}
	})

	t.Run("MultipleReaders", func(t *testing.T) {
		// The View applies to the second Reader only; that suffices.
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			sdkmetric.WithView(sdkmetric.NewView(
				sdkmetric.Instrument{Scope: scope},
				sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter("url.template")},
			)),
		)
		assert.True(t, metricx.AttributeEnabled(ctx, mp.Meter("test"), "url.template", dev))
	})

	t.Run("Concurrent", func(t *testing.T) {
		handle := x.NewMeterConfiguratorHandle()
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			sdkmetric.WithView(sdkmetric.NewView(
				sdkmetric.Instrument{Scope: scope},
				sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter("url.template")},
			)),
			x.WithMeterConfigurator(handle),
		)
		m := mp.Meter("test")
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				for range 100 {
					_ = metricx.AttributeEnabled(ctx, m, "url.template", dev)
				}
			})
		}
		wg.Go(func() {
			handle.Set(func(instrumentation.Scope) x.MeterConfig {
				return x.NewMeterConfig(x.WithMeterEnabled(false))
			})
		})
		wg.Wait()
		assert.False(t, metricx.AttributeEnabled(ctx, m, "url.template", dev))
	})

	t.Run("DenyFilterDoesNotIncludeDevelopmentAttribute", func(t *testing.T) {
		view := sdkmetric.NewView(
			sdkmetric.Instrument{Scope: scope},
			sdkmetric.Stream{AttributeFilter: attribute.NewDenyKeysFilter("http.request.body.size")},
		)
		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewManualReader()),
			sdkmetric.WithView(view),
		)
		assert.False(t, metricx.AttributeEnabled(ctx, mp.Meter("test"), "url.template", dev))
	})
}
