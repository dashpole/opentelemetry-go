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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// NewFilter returns an Aggregator that wraps an agg with an attribute
// filtering function. Both pre-computed non-pre-computed Aggregators can be
// passed for agg. An appropriate Aggregator will be returned for the detected
// type.
func NewFilter[N int64 | float64](agg Aggregator[N], fn attribute.Filter) Aggregator[N] {
	if fn == nil {
		return agg
	}
	if fa, ok := agg.(precomputeAggregator[N]); ok {
		return newPrecomputedFilter(fa, fn)
	}
	return newFilter(agg, fn)
}

// filter wraps an aggregator with an attribute filter. All recorded
// measurements will have their attributes filtered before they are passed to
// the underlying aggregator's Aggregate method.
//
// This should not be used to wrap a pre-computed Aggregator. Use a
// precomputedFilter instead.
type filter[N int64 | float64] struct {
	filter     attribute.Filter
	aggregator Aggregator[N]

	sync.Mutex
	seen map[attribute.Set]attribute.Set
}

// newFilter returns an filter Aggregator that wraps agg with the attribute
// filter fn.
//
// This should not be used to wrap a pre-computed Aggregator. Use a
// precomputedFilter instead.
func newFilter[N int64 | float64](agg Aggregator[N], fn attribute.Filter) *filter[N] {
	return &filter[N]{
		filter:     fn,
		aggregator: agg,
		seen:       make(map[attribute.Set]attribute.Set),
	}
}

// Aggregate records the measurement, scoped by attr, and aggregates it
// into an aggregation.
func (f *filter[N]) Aggregate(measurement N, attr attribute.Set) {
	// TODO (#3006): drop stale attributes from seen.
	f.Lock()
	defer f.Unlock()
	fAttr, ok := f.seen[attr]
	if !ok {
		fAttr, _ = attr.Filter(f.filter)
		f.seen[attr] = fAttr
	}
	f.aggregator.Aggregate(measurement, fAttr)
}

// Aggregation returns an Aggregation, for all the aggregated
// measurements made and ends an aggregation cycle.
func (f *filter[N]) Aggregation() metricdata.Aggregation {
	return f.aggregator.Aggregation()
}

// precomputedFilter is an aggregator that applies attribute filter when
// Aggregating for pre-computed Aggregations. The pre-computed Aggregations
// need to operate normally when no attribute filtering is done (for sums this
// means setting the value), but when attribute filtering is done it needs to
// be added to any set value.
type precomputedFilter[N int64 | float64] struct {
	filter     attribute.Filter
	aggregator precomputeAggregator[N]

	sync.Mutex
	seen map[attribute.Set]attribute.Set
}

// newPrecomputedFilter returns a precomputedFilter Aggregator that wraps agg
// with the attribute filter fn.
//
// This should not be used to wrap a non-pre-computed Aggregator. Use a
// precomputedFilter instead.
func newPrecomputedFilter[N int64 | float64](agg precomputeAggregator[N], fn attribute.Filter) *precomputedFilter[N] {
	return &precomputedFilter[N]{
		filter:     fn,
		aggregator: agg,
		seen:       make(map[attribute.Set]attribute.Set),
	}
}

// Aggregate records the measurement, scoped by attr, and aggregates it
// into an aggregation.
func (f *precomputedFilter[N]) Aggregate(measurement N, attr attribute.Set) {
	// TODO (#3006): drop stale attributes from seen.
	f.Lock()
	defer f.Unlock()
	fAttr, ok := f.seen[attr]
	if !ok {
		fAttr, _ = attr.Filter(f.filter)
		f.seen[attr] = fAttr
	}
	f.aggregator.aggregateFiltered(measurement, fAttr)
}

// Aggregation returns an Aggregation, for all the aggregated
// measurements made and ends an aggregation cycle.
func (f *precomputedFilter[N]) Aggregation() metricdata.Aggregation {
	return f.aggregator.Aggregation()
}
