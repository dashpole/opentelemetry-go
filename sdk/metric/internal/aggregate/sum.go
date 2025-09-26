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

	values sync.Map
	len    atomic.Int64
}

func newValueMap[N int64 | float64](limit int, r func(attribute.Set) FilteredExemplarReservoir[N]) *valueMap[N] {
	return &valueMap[N]{
		newRes:   r,
		aggLimit: limit,
	}
}

func (s *valueMap[N]) measure(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
	attr := fltrAttr
	v, ok := s.values.Load(attr.Equivalent())
	if !ok {
		if s.aggLimit > 0 {
			if s.len.Load() >= int64(s.aggLimit-1) {
				attr = overflowSet
			}
		}
		var loaded bool
		v, loaded = s.values.LoadOrStore(attr.Equivalent(), &sumValue[N]{
			res:   s.newRes(attr),
			attrs: attr,
		})
		if !loaded {
			s.len.Add(1)
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

	dPts := reset(sData.DataPoints, 0, int(s.len.Load()))

	var i int
	s.values.Range(func(key, _ any) bool {
		s.len.Add(-1)
		// Delete the key once it is read to ensure we don't report stale
		// values.
		value, _ := s.values.LoadAndDelete(key)
		val := value.(*sumValue[N])
		newPt := metricdata.DataPoint[N]{
			Attributes: val.attrs,
			StartTime:  s.start,
			Time:       t,
			Value:      val.n.load(),
		}
		collectExemplars(&newPt.Exemplars, val.res.Collect)
		dPts = append(dPts, newPt)
		i++
		return true
	})
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return int(i)
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

	dPts := reset(sData.DataPoints, 0, int(s.len.Load()))

	var i int
	s.values.Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		newPt := metricdata.DataPoint[N]{
			Attributes: val.attrs,
			StartTime:  s.start,
			Time:       t,
			Value:      val.n.load(),
		}
		collectExemplars(&newPt.Exemplars, val.res.Collect)
		dPts = append(dPts, newPt)
		i++
		return true
	})

	sData.DataPoints = dPts
	*dest = sData

	return i
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

	dPts := reset(sData.DataPoints, 0, int(s.len.Load()))

	var i int
	s.values.Range(func(key, _ any) bool {
		s.len.Add(-1)
		// Delete the key once it is read to ensure we don't report stale
		// values.
		value, _ := s.values.LoadAndDelete(key)
		val := value.(*sumValue[N])
		newPt := metricdata.DataPoint[N]{
			Attributes: val.attrs,
			StartTime:  s.start,
			Time:       t,
			Value:      val.n.load(),
		}
		collectExemplars(&newPt.Exemplars, val.res.Collect)
		dPts = append(dPts, newPt)
		i++
		return true
	})
	s.reported = newReported
	// The delta collection cycle resets.
	s.start = t

	sData.DataPoints = dPts
	*dest = sData

	return i
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

	dPts := reset(sData.DataPoints, 0, int(s.len.Load()))

	var i int
	s.values.Range(func(key, value any) bool {
		val := value.(*sumValue[N])
		newPt := metricdata.DataPoint[N]{
			Attributes: val.attrs,
			StartTime:  s.start,
			Time:       t,
			Value:      val.n.load(),
		}
		collectExemplars(&newPt.Exemplars, val.res.Collect)
		dPts = append(dPts, newPt)
		i++
		return true
	})

	sData.DataPoints = dPts
	*dest = sData

	return i
}
