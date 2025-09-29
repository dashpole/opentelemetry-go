// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"math"
	"runtime"
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

// locklessDataPoint allows writers to update datapoints without any locking,
// while allowing readers to get a consistent snapshot where only completed
// measure() calls are included. It prioritizes fast and lockless writes over
// read performance since measurements are usually in the application's hot
// path.
//
// This is accomplished by keeping a "hot" and a "cold" data point, where
// measurements are made on the "hot" data point and data is read from the cold
// one. Prior to reading, the reader atomically switches the hot bit, resets
// the started count, and then waits for the ended count to be equal to the
// started count before reading.
type locklessDataPoint[N int64 | float64] struct {
	// startedWritesAndHotIdx contains a 63-bit counter in the lower bits,
	// and a 1 bit hot index to denote which of the two data-points new
	// measurements to write to. These are contained together so that read()
	// can atomically swap the hot bit, reset the started writes to zero, and
	// read the number writes that were started prior to the hot bit being
	// swapped.
	startedWritesAndHotIdx atomic.Uint64
	// endedMeasures is the number of writes that have completed to each
	// dataPoint.
	endedWrites [2]atomic.Uint64
	dataPoint   [2]*atomicSum[N]
}

// startWrite returns the data point that should be written to, and the hot
// index. The caller must call endWrite on the hot index after it finishes its
// write operation. startWrite is safe to call concurrently with other methods.
func (l *locklessDataPoint[N]) startWrite() (*atomicSum[N], int) {
	// We increment h.startedMeasuresAndHotIdx so that the counter in the lower
	// 63 bits gets incremented. At the same time, we get the new value
	// back, which we can use to find the currently-hot index.
	hotIdx := l.startedWritesAndHotIdx.Add(1) >> 63
	return l.dataPoint[hotIdx], int(hotIdx)
}

// endWrite signals to the reader that a write has fully completed.
// endWrite is safe to call concurrently.
func (l *locklessDataPoint[N]) endWrite(hotIdx int) {
	l.endedWrites[hotIdx].Add(1)
}

// read swaps the hot bit, waits for all writes to complete, and then returns
// the now-cold data point. It is the caller's responsibility to reset the
// data-point back to its zero value, and to ensure read is not called
// concurrently.
func (l *locklessDataPoint[N]) read() *atomicSum[N] {
	n := l.startedWritesAndHotIdx.Load()
	coldIdx := (^n) >> 63
	// Swap the hot and cold index while resetting the started measurements
	// count to zero.
	n = l.startedWritesAndHotIdx.Swap((coldIdx << 63) + 1)
	hotIdx := n >> 63
	startedCount := n & ((1 << 63) - 1)
	// Wait for all measurements to the previously-hot map to finish.
	for startedCount != l.endedWrites[hotIdx].Load() {
		runtime.Gosched() // Let measurements complete.
	}
	// reset the number of ended measures
	l.endedWrites[hotIdx].Store(0)
	return l.dataPoint[hotIdx]
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
