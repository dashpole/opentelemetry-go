// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x // import "go.opentelemetry.io/otel/metric/x"

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type defaultAttributesOption struct {
	metric.InstrumentOption
	keys []attribute.Key
}

func (o defaultAttributesOption) allowedKeys() []attribute.Key {
	return o.keys
}

// WithDefaultAttributes returns a metric.InstrumentOption that specifies default attribute keys.
func WithDefaultAttributes(keys ...attribute.Key) metric.InstrumentOption {
	return defaultAttributesOption{keys: keys}
}

type Int64CounterConfig struct {
	metric.Int64CounterConfig
	allowedKeys []attribute.Key
}

func (c Int64CounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64UpDownCounterConfig struct {
	metric.Int64UpDownCounterConfig
	allowedKeys []attribute.Key
}

func (c Int64UpDownCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64HistogramConfig struct {
	metric.Int64HistogramConfig
	allowedKeys []attribute.Key
}

func (c Int64HistogramConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64GaugeConfig struct {
	metric.Int64GaugeConfig
	allowedKeys []attribute.Key
}

func (c Int64GaugeConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64CounterConfig struct {
	metric.Float64CounterConfig
	allowedKeys []attribute.Key
}

func (c Float64CounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64UpDownCounterConfig struct {
	metric.Float64UpDownCounterConfig
	allowedKeys []attribute.Key
}

func (c Float64UpDownCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64HistogramConfig struct {
	metric.Float64HistogramConfig
	allowedKeys []attribute.Key
}

func (c Float64HistogramConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64GaugeConfig struct {
	metric.Float64GaugeConfig
	allowedKeys []attribute.Key
}

func (c Float64GaugeConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64ObservableCounterConfig struct {
	metric.Int64ObservableCounterConfig
	allowedKeys []attribute.Key
}

func (c Int64ObservableCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64ObservableUpDownCounterConfig struct {
	metric.Int64ObservableUpDownCounterConfig
	allowedKeys []attribute.Key
}

func (c Int64ObservableUpDownCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Int64ObservableGaugeConfig struct {
	metric.Int64ObservableGaugeConfig
	allowedKeys []attribute.Key
}

func (c Int64ObservableGaugeConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64ObservableCounterConfig struct {
	metric.Float64ObservableCounterConfig
	allowedKeys []attribute.Key
}

func (c Float64ObservableCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64ObservableUpDownCounterConfig struct {
	metric.Float64ObservableUpDownCounterConfig
	allowedKeys []attribute.Key
}

func (c Float64ObservableUpDownCounterConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

type Float64ObservableGaugeConfig struct {
	metric.Float64ObservableGaugeConfig
	allowedKeys []attribute.Key
}

func (c Float64ObservableGaugeConfig) DefaultAttributes() []attribute.Key {
	return c.allowedKeys
}

func NewInt64CounterConfig(opts ...metric.Int64CounterOption) Int64CounterConfig {
	var stableOpts []metric.Int64CounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64CounterConfig(stableOpts...)
	return Int64CounterConfig{Int64CounterConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64UpDownCounterConfig(opts ...metric.Int64UpDownCounterOption) Int64UpDownCounterConfig {
	var stableOpts []metric.Int64UpDownCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64UpDownCounterConfig(stableOpts...)
	return Int64UpDownCounterConfig{Int64UpDownCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64HistogramConfig(opts ...metric.Int64HistogramOption) Int64HistogramConfig {
	var stableOpts []metric.Int64HistogramOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64HistogramConfig(stableOpts...)
	return Int64HistogramConfig{Int64HistogramConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64GaugeConfig(opts ...metric.Int64GaugeOption) Int64GaugeConfig {
	var stableOpts []metric.Int64GaugeOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64GaugeConfig(stableOpts...)
	return Int64GaugeConfig{Int64GaugeConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64CounterConfig(opts ...metric.Float64CounterOption) Float64CounterConfig {
	var stableOpts []metric.Float64CounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64CounterConfig(stableOpts...)
	return Float64CounterConfig{Float64CounterConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64UpDownCounterConfig(opts ...metric.Float64UpDownCounterOption) Float64UpDownCounterConfig {
	var stableOpts []metric.Float64UpDownCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64UpDownCounterConfig(stableOpts...)
	return Float64UpDownCounterConfig{Float64UpDownCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64HistogramConfig(opts ...metric.Float64HistogramOption) Float64HistogramConfig {
	var stableOpts []metric.Float64HistogramOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64HistogramConfig(stableOpts...)
	return Float64HistogramConfig{Float64HistogramConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64GaugeConfig(opts ...metric.Float64GaugeOption) Float64GaugeConfig {
	var stableOpts []metric.Float64GaugeOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64GaugeConfig(stableOpts...)
	return Float64GaugeConfig{Float64GaugeConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64ObservableCounterConfig(opts ...metric.Int64ObservableCounterOption) Int64ObservableCounterConfig {
	var stableOpts []metric.Int64ObservableCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64ObservableCounterConfig(stableOpts...)
	return Int64ObservableCounterConfig{Int64ObservableCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64ObservableUpDownCounterConfig(opts ...metric.Int64ObservableUpDownCounterOption) Int64ObservableUpDownCounterConfig {
	var stableOpts []metric.Int64ObservableUpDownCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64ObservableUpDownCounterConfig(stableOpts...)
	return Int64ObservableUpDownCounterConfig{Int64ObservableUpDownCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewInt64ObservableGaugeConfig(opts ...metric.Int64ObservableGaugeOption) Int64ObservableGaugeConfig {
	var stableOpts []metric.Int64ObservableGaugeOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewInt64ObservableGaugeConfig(stableOpts...)
	return Int64ObservableGaugeConfig{Int64ObservableGaugeConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64ObservableCounterConfig(opts ...metric.Float64ObservableCounterOption) Float64ObservableCounterConfig {
	var stableOpts []metric.Float64ObservableCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64ObservableCounterConfig(stableOpts...)
	return Float64ObservableCounterConfig{Float64ObservableCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64ObservableUpDownCounterConfig(opts ...metric.Float64ObservableUpDownCounterOption) Float64ObservableUpDownCounterConfig {
	var stableOpts []metric.Float64ObservableUpDownCounterOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64ObservableUpDownCounterConfig(stableOpts...)
	return Float64ObservableUpDownCounterConfig{Float64ObservableUpDownCounterConfig: stable, allowedKeys: allowedKeys}
}

func NewFloat64ObservableGaugeConfig(opts ...metric.Float64ObservableGaugeOption) Float64ObservableGaugeConfig {
	var stableOpts []metric.Float64ObservableGaugeOption
	var allowedKeys []attribute.Key
	for _, o := range opts {
		if exp, ok := o.(interface{ allowedKeys() []attribute.Key }); ok {
			allowedKeys = append(allowedKeys, exp.allowedKeys()...)
		} else {
			stableOpts = append(stableOpts, o)
		}
	}
	stable := metric.NewFloat64ObservableGaugeConfig(stableOpts...)
	return Float64ObservableGaugeConfig{Float64ObservableGaugeConfig: stable, allowedKeys: allowedKeys}
}
