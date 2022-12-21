// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal // import "go.opentelemetry.io/otel/sdk/metric/internal"

import (
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// valueMap is the storage for sums.
type valueMap[N int64 | float64] struct {
	sync.Mutex
	values map[attribute.Set]N
}

func newValueMap[N int64 | float64]() *valueMap[N] {
	return &valueMap[N]{values: make(map[attribute.Set]N)}
}

func (s *valueMap[N]) Aggregate(value N, attr attribute.Set) {
	s.Lock()
	s.values[attr] += value
	s.Unlock()
}

// NewDeltaSum returns an Aggregator that summarizes a set of measurements as
// their arithmetic sum. Each sum is scoped by attributes and the aggregation
// cycle the measurements were made in.
//
// The monotonic value is used to communicate the produced Aggregation is
// monotonic or not. The returned Aggregator does not make any guarantees this
// value is accurate. It is up to the caller to ensure it.
//
// Each aggregation cycle is treated independently. When the returned
// Aggregator's Aggregation method is called it will reset all sums to zero.
func NewDeltaSum[N int64 | float64](monotonic bool) Aggregator[N] {
	return newDeltaSum[N](monotonic)
}

func newDeltaSum[N int64 | float64](monotonic bool) *deltaSum[N] {
	return &deltaSum[N]{
		valueMap:  newValueMap[N](),
		monotonic: monotonic,
		start:     now(),
	}
}

// deltaSum summarizes a set of measurements made in a single aggregation
// cycle as their arithmetic sum.
type deltaSum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time
}

func (s *deltaSum[N]) Aggregation() metricdata.Aggregation {
	s.Lock()
	defer s.Unlock()

	if len(s.values) == 0 {
		return nil
	}

	t := now()
	out := metricdata.Sum[N]{
		Temporality: metricdata.DeltaTemporality,
		IsMonotonic: s.monotonic,
		DataPoints:  make([]metricdata.DataPoint[N], 0, len(s.values)),
	}
	for attr, value := range s.values {
		out.DataPoints = append(out.DataPoints, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      value,
		})
		// Unused attribute sets do not report.
		delete(s.values, attr)
	}
	// The delta collection cycle resets.
	s.start = t
	return out
}

// NewCumulativeSum returns an Aggregator that summarizes a set of
// measurements as their arithmetic sum. Each sum is scoped by attributes and
// the aggregation cycle the measurements were made in.
//
// The monotonic value is used to communicate the produced Aggregation is
// monotonic or not. The returned Aggregator does not make any guarantees this
// value is accurate. It is up to the caller to ensure it.
//
// Each aggregation cycle is treated independently. When the returned
// Aggregator's Aggregation method is called it will reset all sums to zero.
func NewCumulativeSum[N int64 | float64](monotonic bool) Aggregator[N] {
	return newCumulativeSum[N](monotonic)
}

func newCumulativeSum[N int64 | float64](monotonic bool) *cumulativeSum[N] {
	return &cumulativeSum[N]{
		valueMap:  newValueMap[N](),
		monotonic: monotonic,
		start:     now(),
	}
}

// cumulativeSum summarizes a set of measurements made over all aggregation
// cycles as their arithmetic sum.
type cumulativeSum[N int64 | float64] struct {
	*valueMap[N]

	monotonic bool
	start     time.Time
}

func (s *cumulativeSum[N]) Aggregation() metricdata.Aggregation {
	s.Lock()
	defer s.Unlock()

	if len(s.values) == 0 {
		return nil
	}

	t := now()
	out := metricdata.Sum[N]{
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: s.monotonic,
		DataPoints:  make([]metricdata.DataPoint[N], 0, len(s.values)),
	}
	for attr, value := range s.values {
		out.DataPoints = append(out.DataPoints, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      value,
		})
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}
	return out
}

type precomputedValue[N int64 | float64] struct {
	// measured is the value directly measured.
	measured N
	// filtered is the sum of values from spatially aggregations.
	filtered N
}

// valueMap is the storage for precomputed sums.
type precomputedMap[N int64 | float64] struct {
	sync.Mutex
	values map[attribute.Set]precomputedValue[N]
}

func newPrecomputedMap[N int64 | float64]() *precomputedMap[N] {
	return &precomputedMap[N]{
		values: make(map[attribute.Set]precomputedValue[N]),
	}
}

// Aggregate records value as a cumulative sum for attr.
func (s *precomputedMap[N]) Aggregate(value N, attr attribute.Set) {
	s.Lock()
	v := s.values[attr]
	v.measured = value
	s.values[attr] = v
	s.Unlock()
}

// filtered records value with spatially re-aggregated attrs.
func (s *precomputedMap[N]) filtered(value N, attr attribute.Set) { // nolint: unused  // Used to agg filtered.
	s.Lock()
	v := s.values[attr]
	v.filtered += value
	s.values[attr] = v
	s.Unlock()
}

// NewPrecomputedDeltaSum returns an Aggregator that summarizes a set of
// measurements as their pre-computed arithmetic sum. Each sum is scoped by
// attributes and the aggregation cycle the measurements were made in.
//
// The monotonic value is used to communicate the produced Aggregation is
// monotonic or not. The returned Aggregator does not make any guarantees this
// value is accurate. It is up to the caller to ensure it.
//
// The output Aggregation will report recorded values as delta temporality. It
// is up to the caller to ensure this is accurate.
func NewPrecomputedDeltaSum[N int64 | float64](monotonic bool) Aggregator[N] {
	return &precomputedDeltaSum[N]{
		precomputedMap: newPrecomputedMap[N](),
		reported:       make(map[attribute.Set]N),
		monotonic:      monotonic,
		start:          now(),
	}
}

// precomputedDeltaSum summarizes a set of measurements recorded over all
// aggregation cycles as the delta arithmetic sum.
type precomputedDeltaSum[N int64 | float64] struct {
	*precomputedMap[N]

	reported map[attribute.Set]N

	monotonic bool
	start     time.Time
}

func (s *precomputedDeltaSum[N]) Aggregation() metricdata.Aggregation {
	s.Lock()
	defer s.Unlock()

	if len(s.values) == 0 {
		return nil
	}

	t := now()
	out := metricdata.Sum[N]{
		Temporality: metricdata.DeltaTemporality,
		IsMonotonic: s.monotonic,
		DataPoints:  make([]metricdata.DataPoint[N], 0, len(s.values)),
	}
	for attr, value := range s.values {
		v := value.measured + value.filtered
		delta := v - s.reported[attr]
		out.DataPoints = append(out.DataPoints, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      delta,
		})
		if delta != 0 {
			s.reported[attr] = v
		}
		value.filtered = N(0)
		s.values[attr] = value
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}
	// The delta collection cycle resets.
	s.start = t
	return out
}

// NewPrecomputedCumulativeSum returns an Aggregator that summarizes a set of
// measurements as their pre-computed arithmetic sum. Each sum is scoped by
// attributes and the aggregation cycle the measurements were made in.
//
// The monotonic value is used to communicate the produced Aggregation is
// monotonic or not. The returned Aggregator does not make any guarantees this
// value is accurate. It is up to the caller to ensure it.
//
// The output Aggregation will report recorded values as cumulative
// temporality. It is up to the caller to ensure this is accurate.
func NewPrecomputedCumulativeSum[N int64 | float64](monotonic bool) Aggregator[N] {
	return &precomputedCumulativeSum[N]{
		precomputedMap: newPrecomputedMap[N](),
		monotonic:      monotonic,
		start:          now(),
	}
}

// precomputedCumulativeSum summarizes a set of measurements recorded over all
// aggregation cycles directly as the cumulative arithmetic sum.
type precomputedCumulativeSum[N int64 | float64] struct {
	*precomputedMap[N]

	monotonic bool
	start     time.Time
}

func (s *precomputedCumulativeSum[N]) Aggregation() metricdata.Aggregation {
	s.Lock()
	defer s.Unlock()

	if len(s.values) == 0 {
		return nil
	}

	t := now()
	out := metricdata.Sum[N]{
		Temporality: metricdata.CumulativeTemporality,
		IsMonotonic: s.monotonic,
		DataPoints:  make([]metricdata.DataPoint[N], 0, len(s.values)),
	}
	for attr, value := range s.values {
		out.DataPoints = append(out.DataPoints, metricdata.DataPoint[N]{
			Attributes: attr,
			StartTime:  s.start,
			Time:       t,
			Value:      value.measured + value.filtered,
		})
		value.filtered = N(0)
		s.values[attr] = value
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
	}
	return out
}
