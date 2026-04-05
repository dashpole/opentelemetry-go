// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextual

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
)

type contextKey struct{}

var key = contextKey{}

type config struct {
	addToMetrics bool
}

// ContextAttributes contains context-scoped attributes separated by their target.
type ContextAttributes struct {
	Metric   attribute.Set
	Exemplar attribute.Set
}

// All returns all context-scoped attributes combined as a slice.
func (ca ContextAttributes) All() []attribute.KeyValue {
	var kvs []attribute.KeyValue
	kvs = append(kvs, ca.Metric.ToSlice()...)
	kvs = append(kvs, ca.Exemplar.ToSlice()...)
	set := attribute.NewSet(kvs...)
	return set.ToSlice()
}



// Option applies a configuration option.
type Option interface {
	apply(config) config
}

type optionFunc func(config) config

func (f optionFunc) apply(c config) config {
	return f(c)
}

// WithAddToMetrics specifies whether the attributes should be added to metrics.
// By default, they are not added to metrics (only to exemplars).
func WithAddToMetrics(b bool) Option {
	return optionFunc(func(c config) config {
		c.addToMetrics = b
		return c
	})
}

type contextData struct {
	attrs  attribute.Set
	config config
	parent *contextData
}

// ContextWithAttributes returns a new context with the provided attributes attached.
// The attributes are additive to any existing attributes in the context.
func ContextWithAttributes(ctx context.Context, attrs attribute.Set, opts ...Option) context.Context {
	var c config
	for _, opt := range opts {
		c = opt.apply(c)
	}

	parentData, _ := ctx.Value(key).(*contextData)

	data := &contextData{
		attrs:  attrs,
		config: c,
		parent: parentData,
	}

	return context.WithValue(ctx, key, data)
}

// AttributesFromContext returns the context-scoped attributes separated by their target.
// Attributes added closer to the current context override those added further up the chain.
func AttributesFromContext(ctx context.Context) ContextAttributes {
	data, ok := ctx.Value(key).(*contextData)
	if !ok {
		return ContextAttributes{
			Metric:   attribute.NewSet(),
			Exemplar: attribute.NewSet(),
		}
	}

	// Build chain from child to parent (child is chain[0]).
	var chain []*contextData
	curr := data
	for curr != nil {
		chain = append(chain, curr)
		curr = curr.parent
	}

	seen := make(map[attribute.Key]bool)
	var metricKVs []attribute.KeyValue
	var exemplarKVs []attribute.KeyValue

	// Iterate from child to parent.
	// The first time we see a key, it is the child-most value, so it wins.
	for _, d := range chain {
		for _, kv := range d.attrs.ToSlice() {
			if !seen[kv.Key] {
				seen[kv.Key] = true
				if d.config.addToMetrics {
					metricKVs = append(metricKVs, kv)
				} else {
					exemplarKVs = append(exemplarKVs, kv)
				}
			}
		}
	}

	return ContextAttributes{
		Metric:   attribute.NewSet(metricKVs...),
		Exemplar: attribute.NewSet(exemplarKVs...),
	}
}

