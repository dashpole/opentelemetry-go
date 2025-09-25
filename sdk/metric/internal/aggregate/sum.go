// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type sumValue[N int64 | float64] struct {
	n     atomicSum[N]
	res   FilteredExemplarReservoir[N]
	attrs attribute.Set
}

func (s *sumValue[N]) measure(ctx context.Context, value N, droppedAttr []attribute.KeyValue) {
	s.res.Offer(ctx, value, droppedAttr)
	s.n.add(value)
}

// valueMap is the storage for sums.
type valueMap[N int64 | float64] struct {
	newRes   func(attribute.Set) FilteredExemplarReservoir[N]
	aggLimit int

	hot    atomic.Bool
	values [2]sync.Map
	len    [2]atomic.Int64
}

func newValueMap[N int64 | float64](limit int, r func(attribute.Set) FilteredExemplarReservoir[N]) *valueMap[N] {
	return &valueMap[N]{
		newRes:   r,
		aggLimit: limit,
	}
}

func (s *valueMap[N]) hotIdx() int {
	if s.hot.Load() {
		return 1
	}
	return 0
}

func (s *valueMap[N]) measure(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
	attr := fltrAttr
	hotIdx := s.hotIdx()
	v, ok := s.values[hotIdx].Load(attr.Equivalent())
	if !ok {
		if s.aggLimit > 0 {
			if s.len[hotIdx].Load() >= int64(s.aggLimit-1) {
				attr = overflowSet
			}
		}
		var loaded bool
		v, loaded = s.values[hotIdx].LoadOrStore(attr.Equivalent(), &sumValue[N]{
			res:   s.newRes(attr),
			attrs: attr,
		})
		if !loaded {
			s.len[hotIdx].Add(1)
		}
	}
	v.(*sumValue[N]).measure(ctx, value, droppedAttr)
}

// newSum returns an aggregator that summarizes a set of measurements as their
// arithmetic sum. Each sum is scoped by attributes and the aggregation cycle
// the measurements were made in.
func newSum[N int64 | float64](monotonic bool, limit int, r func(attribute.Set) FilteredExemplarReservoir[N]) *sum[N] {
	return &sum[N]{
		valueMap:  newValueMap[N](limit, r),
		monotonic: monotonic,
		start:     now(),
	}
}

// sum summarizes a set of measurements made as their arithmetic sum.
type sum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time
}

func (s *sum[N]) delta(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	n := int(s.len[s.hotIdx()].Load())
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[s.hotIdx()].Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		i++
		return true
	})
	// TODO
	// Do not report stale values.
	// clear(s.values)
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return int(n)
}

func (s *sum[N]) cumulative(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	n := int(s.len[s.hotIdx()].Load())
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[s.hotIdx()].Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		i++
		return true
	})

	sData.DataPoints = dPts
	*dest = sData

	return n
}

// newPrecomputedSum returns an aggregator that summarizes a set of
// observations as their arithmetic sum. Each sum is scoped by attributes and
// the aggregation cycle the measurements were made in.
func newPrecomputedSum[N int64 | float64](
	monotonic bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *precomputedSum[N] {
	return &precomputedSum[N]{
		valueMap:  newValueMap[N](limit, r),
		monotonic: monotonic,
		start:     now(),
	}
}

// precomputedSum summarizes a set of observations as their arithmetic sum.
type precomputedSum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time

	reported map[attribute.Distinct]N
}

func (s *precomputedSum[N]) delta(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()
	newReported := make(map[attribute.Distinct]N)

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.DeltaTemporality
	sData.IsMonotonic = s.monotonic

	n := int(s.len[s.hotIdx()].Load())
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[s.hotIdx()].Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		i++
		return true
	})
	// Unused attribute sets do not report.
	// clear(s.values)
	s.reported = newReported
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return n
}

func (s *precomputedSum[N]) cumulative(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.Sum, memory reuse is missed. In that case,
	// use the zero-value sData and hope for better alignment next cycle.
	sData, _ := (*dest).(metricdata.Sum[N])
	sData.Temporality = metricdata.CumulativeTemporality
	sData.IsMonotonic = s.monotonic

	n := int(s.len[s.hotIdx()].Load())
	dPts := reset(sData.DataPoints, n, n)

	var i int
	s.values[s.hotIdx()].Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		dPts[i].Attributes = val.attrs
		dPts[i].StartTime = s.start
		dPts[i].Time = t
		dPts[i].Value = val.n.load()
		collectExemplars(&dPts[i].Exemplars, val.res.Collect)
		i++
		return true
	})

	sData.DataPoints = dPts
	*dest = sData

	return n
}
