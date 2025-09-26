// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"math"
	"sync/atomic"
)

// atomicSum is an efficient way of adding to a number which is either an
// int64 or float64. It is designed to be efficient when adding whole
// numbers, regardless of whether N is an int64 or float64.
//
// Inspired by the Prometheus counter implementation:
// https://github.com/prometheus/client_golang/blob/14ccb93091c00f86b85af7753100aa372d63602b/prometheus/counter.go#L108
type atomicSum[N int64 | float64] struct {
	// nFloatBits contains only the non-integer portion of the counter.
	nFloatBits atomic.Uint64
	// nInt contains only the integer portion of the counter.
	nInt atomic.Int64
}

// load returns the current value. The caller must ensure all calls to add have
// returned prior to calling load.
func (n *atomicSum[N]) load() N {
	fval := math.Float64frombits(n.nFloatBits.Load())
	ival := n.nInt.Load()
	return N(fval + float64(ival))
}

func (n *atomicSum[N]) add(value N) {
	ival := int64(value)
	// This case is where the value is an int, or if it is a whole-numbered float.
	if float64(ival) == float64(value) {
		n.nInt.Add(ival)
		return
	}

	// Value must be a float below.
	for {
		oldBits := n.nFloatBits.Load()
		newBits := math.Float64bits(math.Float64frombits(oldBits) + float64(value))
		if n.nFloatBits.CompareAndSwap(oldBits, newBits) {
			return
		}
	}
}

// // limitedSyncMap
// type limitedSyncMap[N int64 | float64] struct {
// 	startedMeasuresAndHotIdx atomic.Uint64
// 	endedMeasures            [2]atomic.Uint64
// 	values                   [2]sync.Map
// 	len                      [2]atomic.Int64

// 	newElement     func(attr attribute.Set) any
// 	measureElement func(ctx context.Context, element any, value N, droppedAttr []attribute.KeyValue)
// 	aggLimit       int
// }

// func (l *limitedSyncMap[N]) measureDelta(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
// 	// We increment h.startedMeasuresAndHotIdx so that the counter in the lower
// 	// 63 bits gets incremented. At the same time, we get the new value
// 	// back, which we can use to find the currently-hot index.
// 	hotIdx := l.startedMeasuresAndHotIdx.Add(1) >> 63
// 	// signal to collection that the measurement has completed by incrementing the ended measures.
// 	defer l.endedMeasures[hotIdx].Add(1)
// 	v := l.getOrCreateElementWithLimit(hotIdx, fltrAttr)
// 	l.measureElement(ctx, v, value, droppedAttr)
// }

// func (l *limitedSyncMap[N]) measureCumulative(ctx context.Context, value N, fltrAttr attribute.Set, droppedAttr []attribute.KeyValue) {
// 	// cumulative measurements are always made against the same sync map.
// 	hotIdx := uint64(0)
// 	v := l.getOrCreateElementWithLimit(hotIdx, fltrAttr)
// 	l.measureElement(ctx, v, value, droppedAttr)
// }

// func (l *limitedSyncMap[N]) getOrCreateElementWithLimit(hotIdx uint64, fltrAttr attribute.Set) any {
// 	attr := fltrAttr
// 	v, ok := l.values[hotIdx].Load(attr.Equivalent())
// 	if !ok {
// 		if l.aggLimit > 0 {
// 			if l.len[hotIdx].Load() >= int64(l.aggLimit-1) {
// 				attr = overflowSet
// 			}
// 		}
// 		var loaded bool
// 		v, loaded = l.values[hotIdx].LoadOrStore(attr.Equivalent(), l.newElement(attr))
// 		if !loaded {
// 			l.len[hotIdx].Add(1)
// 		}
// 	}
// 	return v
// }

// // exclusiveRangeWithReset swaps the hot and cold maps, waits for all
// // measurements to the hot map to complete, and then ranges over the elements
// // that were previously hot. This ensures Range iterates over a snapshot of the
// // hot map that only includes complete measurements, and does not block
// // measurements while that is happening.
// //
// // exclusiveRangeWithReset must not be called concurrently with itself, but can be
// // called concurrently with measure.
// //
// // This function resets the previously-hot map. For cumulative instruments, it
// // is the caller's responsibility to merge elements from within the rangeFn
// // using getOrCreateElement.
// func (l *limitedSyncMap[N]) exclusiveRangeWithReset(setup func(len int64) int, rangeFn func(key, value any) bool) {
// 	// Swap the hot and cold index while resetting the started measurements count.
// 	n := l.startedMeasuresAndHotIdx.Load()
// 	coldIdx := (^n) >> 63
// 	n = l.startedMeasuresAndHotIdx.Swap((coldIdx << 63) + 1)
// 	hotIdx := n >> 63
// 	startedCount := n & ((1 << 63) - 1)
// 	// Wait for all measurements to the previously-hot map to finish.
// 	for startedCount != l.endedMeasures[hotIdx].Load() {
// 		runtime.Gosched() // Let measurements complete.
// 	}
// 	setup(l.len[hotIdx].Load())
// 	l.values[hotIdx].Range(rangeFn)
// 	// Reset the hotIdx
// 	l.values[hotIdx].Clear()
// 	l.len[hotIdx].Store(0)
// 	l.endedMeasures[hotIdx].Store(0)
// }
