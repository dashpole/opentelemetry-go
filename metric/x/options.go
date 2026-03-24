// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x // import "go.opentelemetry.io/otel/metric/x"

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/internal/x"
	"go.opentelemetry.io/otel/metric"
)

type defaultAttributesOption struct {
	metric.InstrumentOption
	x.ExperimentalOption

	keys []attribute.Key
}

func (o defaultAttributesOption) DefaultAttributes() []attribute.Key {
	return o.keys
}

// WithDefaultAttributes returns a metric.InstrumentOption that specifies default attribute keys.
func WithDefaultAttributes(keys ...attribute.Key) metric.InstrumentOption {
	return defaultAttributesOption{keys: keys}
}
