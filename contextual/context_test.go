// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextual

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
)

func TestContextWithAttributes(t *testing.T) {
	ctx := context.Background()

	attrs1 := attribute.NewSet(attribute.String("k1", "v1"))
	ctx = ContextWithAttributes(ctx, attrs1)

	ctxAttrs := AttributesFromContext(ctx)
	
	// By default, goes to Exemplar
	assert.Equal(t, 1, ctxAttrs.Exemplar.Len())
	assert.Equal(t, 0, ctxAttrs.Metric.Len())
	
	v, ok := ctxAttrs.Exemplar.Value("k1")
	assert.True(t, ok)
	assert.Equal(t, "v1", v.AsString())

	// Add with AddToMetrics(true)
	attrs2 := attribute.NewSet(attribute.String("k2", "v2"))
	ctx = ContextWithAttributes(ctx, attrs2, WithAddToMetrics(true))

	ctxAttrs = AttributesFromContext(ctx)
	assert.Equal(t, 1, ctxAttrs.Exemplar.Len())
	assert.Equal(t, 1, ctxAttrs.Metric.Len())
	
	v, ok = ctxAttrs.Metric.Value("k2")
	assert.True(t, ok)
	assert.Equal(t, "v2", v.AsString())
}

func TestContextWithAttributesOverride(t *testing.T) {
	ctx := context.Background()

	attrs1 := attribute.NewSet(attribute.String("k1", "v1"))
	ctx = ContextWithAttributes(ctx, attrs1, WithAddToMetrics(true))

	// Override k1, but with AddToMetrics=false (default)
	attrs2 := attribute.NewSet(attribute.String("k1", "v2"))
	ctx = ContextWithAttributes(ctx, attrs2)

	ctxAttrs := AttributesFromContext(ctx)
	
	// Child wins, and it was AddToMetrics=false, so it should be in Exemplar only!
	assert.Equal(t, 1, ctxAttrs.Exemplar.Len())
	assert.Equal(t, 0, ctxAttrs.Metric.Len())
	
	v, ok := ctxAttrs.Exemplar.Value("k1")
	assert.True(t, ok)
	assert.Equal(t, "v2", v.AsString())
}

func TestContextAttributesAll(t *testing.T) {
	ctx := context.Background()

	attrs1 := attribute.NewSet(attribute.String("k1", "v1"))
	ctx = ContextWithAttributes(ctx, attrs1)

	attrs2 := attribute.NewSet(attribute.String("k2", "v2"))
	ctx = ContextWithAttributes(ctx, attrs2, WithAddToMetrics(true))

	ctxAttrs := AttributesFromContext(ctx)
	all := ctxAttrs.All()
	
	assert.Equal(t, 2, len(all))
	// Since it is sorted, k1 should be first, k2 second (alphabetical).
	assert.Equal(t, attribute.Key("k1"), all[0].Key)
	assert.Equal(t, "v1", all[0].Value.AsString())
	assert.Equal(t, attribute.Key("k2"), all[1].Key)
	assert.Equal(t, "v2", all[1].Value.AsString())

}
