// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package exemplar // import "go.opentelemetry.io/otel/sdk/metric/exemplar"

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// storage is an exemplar storage for [Reservoir] implementations.
type storage struct {
	// measurements are the measurements sampled.
	//
	// This does not use []metricdata.Exemplar because it potentially would
	// require an allocation for trace and span IDs in the hot path of Offer.
	measurements []atomic.Pointer[measurement]
}

func newStorage(n int) *storage {
	return &storage{measurements: make([]atomic.Pointer[measurement], n)}
}

func (r *storage) store(idx int, m *measurement) {
	old := r.measurements[idx].Swap(m)
	if old != nil {
		mPool.Put(old)
	}
}

// Collect returns all the held exemplars.
//
// The Reservoir state is preserved after this call.
func (r *storage) Collect(dest *[]Exemplar) {
	*dest = reset(*dest, len(r.measurements), len(r.measurements))
	var n int
	for i := range r.measurements {
		// For performance reasons, this iterates over measurements
		// concurrently with new measurements being written. This means we do
		// not get a point-in-time snapshot of the state of the reservoir.
		// This means that for sequential Offer calls, a later Offer call may
		// be collected and an earlier call not collected if they are written
		// to different indices.
		m := r.measurements[i].Load()
		if m == nil {
			continue
		}
		m.mux.Lock()
		for m.Idx != i {
			m.mux.Unlock()
			m = r.measurements[i].Load()
			m.mux.Lock()
		}

		m.exemplar(&(*dest)[n])
		m.mux.Unlock()
		n++
	}
	*dest = (*dest)[:n]
}

// measurement is a measurement made by a telemetry system.
type measurement struct {
	mux sync.Mutex
	// FilteredAttributes are the attributes dropped during the measurement.
	FilteredAttributes []attribute.KeyValue
	// Time is the time when the measurement was made.
	Time time.Time
	// Value is the value of the measurement.
	Value Value
	// Ctx is the active context when a measurement was made.
	Ctx context.Context

	Idx int
}

var mPool = sync.Pool{
	New: func() any {
		return &measurement{}
	},
}

// newMeasurement returns a new non-empty Measurement.
func newMeasurement(ctx context.Context, idx int, ts time.Time, v Value, droppedAttr []attribute.KeyValue) *measurement {
	m := mPool.Get().(*measurement)
	m.mux.Lock()
	defer m.mux.Unlock()
	m.FilteredAttributes = droppedAttr
	m.Time = ts
	m.Value = v
	m.Ctx = ctx
	m.Idx = idx
	return m
}

// exemplar returns m as an [Exemplar].
func (m *measurement) exemplar(dest *Exemplar) {
	dest.FilteredAttributes = m.FilteredAttributes
	dest.Time = m.Time
	dest.Value = m.Value

	sc := trace.SpanContextFromContext(m.Ctx)
	if sc.HasTraceID() {
		traceID := sc.TraceID()
		dest.TraceID = traceID[:]
	} else {
		dest.TraceID = dest.TraceID[:0]
	}

	if sc.HasSpanID() {
		spanID := sc.SpanID()
		dest.SpanID = spanID[:]
	} else {
		dest.SpanID = dest.SpanID[:0]
	}
}

func reset[T any](s []T, length, capacity int) []T {
	if cap(s) < capacity {
		return make([]T, length, capacity)
	}
	return s[:length]
}
