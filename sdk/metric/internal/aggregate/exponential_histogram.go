// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/internal/x"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	expoMaxScale = 20
	expoMinScale = -10

	smallestNonZeroNormalFloat64 = 0x1p-1022
)

// expoHistogramDataPoint is a single data point in an exponential histogram.
type expoHistogramDataPoint[N int64 | float64] struct {
	attrs attribute.Set
	res   FilteredExemplarReservoir[N]

	minMax atomicMinMax[N]
	sum    atomicCounter[N]

	maxSize  int
	noMinMax bool
	noSum    bool

	rescaleMu sync.Mutex
	wg        hotColdWaitGroup
	buckets   [2]expoBucketsState
	zeroCount atomic.Uint64
	scratch   []uint64

	startTime time.Time
}

type expoBucketsState struct {
	scale      atomic.Int32
	posBuckets expoBuckets
	negBuckets expoBuckets
}

func newExpoHistogramDataPoint[N int64 | float64](
	attrs attribute.Set,
	maxSize int,
	maxScale int32,
	noMinMax, noSum bool,
) *expoHistogramDataPoint[N] { // nolint:revive // we need this control flag
	dp := &expoHistogramDataPoint[N]{
		attrs:     attrs,
		maxSize:   maxSize,
		noMinMax:  noMinMax,
		noSum:     noSum,
		startTime: now(),
		scratch:   make([]uint64, maxSize),
		buckets: [2]expoBucketsState{
			{
				scale:      atomic.Int32{},
				posBuckets: expoBuckets{counts: make([]atomic.Uint64, maxSize)},
				negBuckets: expoBuckets{counts: make([]atomic.Uint64, maxSize)},
			},
			{
				scale:      atomic.Int32{},
				posBuckets: expoBuckets{counts: make([]atomic.Uint64, maxSize)},
				negBuckets: expoBuckets{counts: make([]atomic.Uint64, maxSize)},
			},
		},
	}
	dp.buckets[0].scale.Store(maxScale)
	dp.buckets[1].scale.Store(maxScale)
	return dp
}

// record adds a new measurement to the histogram. It will rescale the buckets if needed.
func (p *expoHistogramDataPoint[N]) record(v N) {
	if !p.noMinMax {
		p.minMax.Update(v)
	}
	if !p.noSum {
		p.sum.add(v)
	}

	idx := p.wg.start()
	absV := math.Abs(float64(v))

	if float64(absV) == 0.0 {
		p.zeroCount.Add(1)
		p.wg.done(idx)
		return
	}

	currentScale := p.buckets[idx].scale.Load()
	bin := p.getBin(absV, currentScale)

	bucket := &p.buckets[idx].posBuckets
	if v < 0 {
		bucket = &p.buckets[idx].negBuckets
	}

	if bucket.recordCount(bin, 1) {
		p.wg.done(idx)
		return
	}

	p.wg.done(idx)

	p.rescaleMu.Lock()

	oldHotIdx := p.wg.swapHotAndWait()
	newHotIdx := (oldHotIdx + 1) % 2

	currentScale = p.buckets[oldHotIdx].scale.Load()
	bin = p.getBin(absV, currentScale)
	bucket = &p.buckets[oldHotIdx].posBuckets
	if v < 0 {
		bucket = &p.buckets[oldHotIdx].negBuckets
	}

	bStart, bLen := unpackBounds(bucket.bounds.Load())
	scaleDelta := p.scaleChange(bin, bStart, bLen)

	if scaleDelta > 0 {
		if currentScale-scaleDelta < expoMinScale {
			otel.Handle(errors.New("exponential histogram scale underflow"))
			p.buckets[oldHotIdx].posBuckets.merge(&p.buckets[newHotIdx].posBuckets)
			p.buckets[oldHotIdx].negBuckets.merge(&p.buckets[newHotIdx].negBuckets)
			p.buckets[newHotIdx].posBuckets.reset()
			p.buckets[newHotIdx].negBuckets.reset()
			_ = p.wg.swapHotAndWait()
			p.rescaleMu.Unlock()
			return
		}
		p.buckets[oldHotIdx].posBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[oldHotIdx].negBuckets.downscale(scaleDelta, p.scratch)
		currentScale -= scaleDelta
		p.buckets[oldHotIdx].scale.Store(currentScale)
		bin = p.getBin(absV, currentScale)
	}

	bucket.recordCount(bin, 1)

	// Make oldHotIdx active again. It has the correct scale and our new bin.
	_ = p.wg.swapHotAndWait()

	// Now newHotIdx is OFFLINE. All its writes were at the old scale.
	// Downscale it if necessary.
	if scaleDelta > 0 {
		p.buckets[newHotIdx].posBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[newHotIdx].negBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[newHotIdx].scale.Store(currentScale)
	}

	// Merge the offline bucket into the active one
	for {
		scaleDelta = 0

		// Compute if we need to downscale BEFORE merge to prevent duplicate partial merges
		nStart, nLen := unpackBounds(p.buckets[newHotIdx].posBuckets.bounds.Load())
		if nLen > 0 {
			oStart, oLen := unpackBounds(p.buckets[oldHotIdx].posBuckets.bounds.Load())
			if oLen > 0 {
				low := min(nStart, oStart)
				high := max(nStart+nLen-1, oStart+oLen-1)
				delta := p.scaleChange(high, low, 1)
				if delta > scaleDelta {
					scaleDelta = delta
				}
			}
		}
		nStart, nLen = unpackBounds(p.buckets[newHotIdx].negBuckets.bounds.Load())
		if nLen > 0 {
			oStart, oLen := unpackBounds(p.buckets[oldHotIdx].negBuckets.bounds.Load())
			if oLen > 0 {
				low := min(nStart, oStart)
				high := max(nStart+nLen-1, oStart+oLen-1)
				delta := p.scaleChange(high, low, 1)
				if delta > scaleDelta {
					scaleDelta = delta
				}
			}
		}

		if scaleDelta == 0 {
			// We can safely merge because all elements are guaranteed to fit
			p.buckets[oldHotIdx].posBuckets.merge(&p.buckets[newHotIdx].posBuckets)
			p.buckets[oldHotIdx].negBuckets.merge(&p.buckets[newHotIdx].negBuckets)
			break
		}

		// We need to fully downscale oldHotIdx and newHotIdx.
		// Currently oldHotIdx is ACTIVE and newHotIdx is OFFLINE.
		// Swap so oldHotIdx is OFFLINE, and newHotIdx becomes ACTIVE.
		_ = p.wg.swapHotAndWait()

		p.buckets[oldHotIdx].posBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[oldHotIdx].negBuckets.downscale(scaleDelta, p.scratch)
		currentScale -= scaleDelta
		p.buckets[oldHotIdx].scale.Store(currentScale)

		// Swap again so newHotIdx is OFFLINE, and oldHotIdx becomes ACTIVE.
		_ = p.wg.swapHotAndWait()

		// Now we can safely downscale newHotIdx
		p.buckets[newHotIdx].posBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[newHotIdx].negBuckets.downscale(scaleDelta, p.scratch)
		p.buckets[newHotIdx].scale.Store(currentScale)
	}

	p.buckets[newHotIdx].posBuckets.reset()
	p.buckets[newHotIdx].negBuckets.reset()
	p.rescaleMu.Unlock()
}

// getBin returns the bin v should be recorded into.
func (_ *expoHistogramDataPoint[N]) getBin(v float64, scale int32) int32 {
	frac, expInt := math.Frexp(v)
	// 11-bit exponential.
	exp := int32(expInt) // nolint: gosec
	if scale <= 0 {
		// Because of the choice of fraction is always 1 power of two higher than we want.
		var correction int32 = 1
		if frac == .5 {
			// If v is an exact power of two the frac will be .5 and the exp
			// will be one higher than we want.
			correction = 2
		}
		return (exp - correction) >> (-scale)
	}
	return exp<<scale + int32(math.Log(frac)*scaleFactors[scale]) - 1
}

// scaleFactors are constants used in calculating the logarithm index. They are
// equivalent to 2^index/log(2).
var scaleFactors = [21]float64{
	math.Ldexp(math.Log2E, 0),
	math.Ldexp(math.Log2E, 1),
	math.Ldexp(math.Log2E, 2),
	math.Ldexp(math.Log2E, 3),
	math.Ldexp(math.Log2E, 4),
	math.Ldexp(math.Log2E, 5),
	math.Ldexp(math.Log2E, 6),
	math.Ldexp(math.Log2E, 7),
	math.Ldexp(math.Log2E, 8),
	math.Ldexp(math.Log2E, 9),
	math.Ldexp(math.Log2E, 10),
	math.Ldexp(math.Log2E, 11),
	math.Ldexp(math.Log2E, 12),
	math.Ldexp(math.Log2E, 13),
	math.Ldexp(math.Log2E, 14),
	math.Ldexp(math.Log2E, 15),
	math.Ldexp(math.Log2E, 16),
	math.Ldexp(math.Log2E, 17),
	math.Ldexp(math.Log2E, 18),
	math.Ldexp(math.Log2E, 19),
	math.Ldexp(math.Log2E, 20),
}

// scaleChange returns the magnitude of the scale change needed to fit bin in
// the bucket. If no scale change is needed 0 is returned.
func (p *expoHistogramDataPoint[N]) scaleChange(bin, startBin, length int32) int32 {
	if length == 0 {
		// No need to rescale if there are no buckets.
		return 0
	}

	low := int(startBin)
	high := int(bin)
	if startBin >= bin {
		low = int(bin)
		high = int(startBin) + int(length) - 1
	}

	var count int32
	for high-low >= p.maxSize {
		low >>= 1
		high >>= 1
		count++
		if count > expoMaxScale-expoMinScale {
			return count
		}
	}
	return count
}

// expoBuckets is a set of buckets in an exponential histogram.
type expoBuckets struct {
	bounds atomic.Uint64
	counts []atomic.Uint64
}

// downscale shrinks a bucket by a factor of 2*s. It will sum counts into the
// correct lower resolution bucket.
func (b *expoBuckets) downscale(delta int32, scratch []uint64) {
	bounds := b.bounds.Load()
	bStart, bLen := unpackBounds(bounds)

	if delta < 1 || bLen == 0 {
		return
	}

	maxSize := len(b.counts)
	for i := range maxSize {
		scratch[i] = 0
	}

	if bLen == 1 {
		oldIdx := (int(bStart)%maxSize + maxSize) % maxSize
		newBin := bStart >> delta
		newIdx := (int(newBin)%maxSize + maxSize) % maxSize
		if oldIdx != newIdx {
			count := b.counts[oldIdx].Swap(0)
			b.counts[newIdx].Store(count)
		}
		b.bounds.Store(packBounds(newBin, 1))
		return
	}

	for i := range bLen {
		bin := bStart + i
		idx := (int(bin)%maxSize + maxSize) % maxSize
		count := b.counts[idx].Load()
		if count > 0 {
			newBin := bin >> delta
			newIdx := (int(newBin)%maxSize + maxSize) % maxSize
			scratch[newIdx] += count
		}
	}

	newStart := bStart >> delta
	newEnd := (bStart + bLen - 1) >> delta
	newLen := newEnd - newStart + 1

	for i := range maxSize {
		b.counts[i].Store(0)
	}

	for i := range newLen {
		bin := newStart + i
		idx := (int(bin)%maxSize + maxSize) % maxSize
		b.counts[idx].Store(scratch[idx])
	}
	b.bounds.Store(packBounds(newStart, newLen))
}

func (b *expoBuckets) reset() {
	bounds := b.bounds.Swap(0)
	startBin, length := unpackBounds(bounds)
	maxSize := len(b.counts)
	for i := range length {
		bin := startBin + i
		idx := (int(bin)%maxSize + maxSize) % maxSize
		b.counts[idx].Store(0)
	}
}

func (b *expoBuckets) merge(other *expoBuckets) {
	otherStart, otherLen := unpackBounds(other.bounds.Load())
	if otherLen == 0 {
		return
	}

	maxSize := len(b.counts)
	for i := range otherLen {
		bin := otherStart + i
		idx := (int(bin)%maxSize + maxSize) % maxSize
		count := other.counts[idx].Load()
		if count > 0 {
			b.recordCount(bin, count)
		}
	}
}

// recordCount functions exactly like record but adds an explicit count value instead of incrementing by 1.
func (b *expoBuckets) recordCount(bin int32, count uint64) bool {
	maxSize := len(b.counts)
	if maxSize == 0 {
		return false
	}
	for {
		bounds := b.bounds.Load()
		bStart, bLen := unpackBounds(bounds)

		if bLen == 0 {
			newBounds := packBounds(bin, 1)
			if b.bounds.CompareAndSwap(bounds, newBounds) {
				idx := (int(bin)%maxSize + maxSize) % maxSize
				b.counts[idx].Add(count)
				return true
			}
			continue
		}

		endBin := bStart + bLen - 1

		if bin >= bStart && bin <= endBin {
			idx := (int(bin)%maxSize + maxSize) % maxSize
			b.counts[idx].Add(count)
			return true
		}

		newStart := bStart
		newEnd := endBin
		if bin < bStart {
			newStart = bin
		}
		if bin > endBin {
			newEnd = bin
		}
		newLength := newEnd - newStart + 1
		if int(newLength) > maxSize {
			return false
		}

		newBounds := packBounds(newStart, newLength)
		if b.bounds.CompareAndSwap(bounds, newBounds) {
			idx := (int(bin)%maxSize + maxSize) % maxSize
			b.counts[idx].Add(count)
			return true
		}
	}
}

// newDeltaExpoHistogram returns an Aggregator that summarizes a set of
// measurements as an exponential histogram. Each histogram is scoped by attributes
// and the aggregation cycle the measurements were made in.
func newDeltaExpoHistogram[N int64 | float64](
	maxSize, maxScale int32,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *deltaExpoHistogram[N] {
	return &deltaExpoHistogram[N]{
		noSum:    noSum,
		noMinMax: noMinMax,
		maxSize:  int(maxSize),
		maxScale: maxScale,

		newRes: r,
		values: [2]limitedSyncMap{
			{aggLimit: limit},
			{aggLimit: limit},
		},

		start: now(),
	}
}

// deltaExpoHistogram summarizes a set of measurements as an histogram with exponentially
// defined buckets.
type deltaExpoHistogram[N int64 | float64] struct {
	noSum    bool
	noMinMax bool
	maxSize  int
	maxScale int32

	newRes func(attribute.Set) FilteredExemplarReservoir[N]

	wg     hotColdWaitGroup
	values [2]limitedSyncMap

	start time.Time
}

func (e *deltaExpoHistogram[N]) measure(
	ctx context.Context,
	value N,
	fltrAttr attribute.Set,
	droppedAttr []attribute.KeyValue,
) {
	// Ignore NaN and infinity.
	if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
		return
	}

	for {
		idx := e.wg.start()
		val := e.values[idx].LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) any {
			v := newExpoHistogramDataPoint[N](attr, e.maxSize, e.maxScale, e.noMinMax, e.noSum)
			v.res = e.newRes(attr)
			return v
		}).(*expoHistogramDataPoint[N])

		val.record(value)
		val.res.Offer(ctx, value, droppedAttr)

		e.wg.done(idx)
		return
	}
}

func (e *deltaExpoHistogram[N]) collect(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.ExponentialHistogram, memory reuse is missed.
	// In that case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.ExponentialHistogram[N])
	h.Temporality = metricdata.DeltaTemporality

	oldIdx := e.wg.swapHotAndWait()

	// Values are being concurrently written while we iterate, so only use the
	// current length for capacity.
	n := e.values[oldIdx].Len()
	hDPts := reset(h.DataPoints, 0, n)

	e.values[oldIdx].Range(func(_, value any) bool {
		val := value.(*expoHistogramDataPoint[N])

		val.rescaleMu.Lock()
		activeIdx := val.wg.hotIdx()

		newPt := metricdata.ExponentialHistogramDataPoint[N]{
			Attributes:    val.attrs,
			StartTime:     e.start,
			Time:          t,
			Scale:         val.buckets[activeIdx].scale.Load(),
			ZeroCount:     val.zeroCount.Load(),
			ZeroThreshold: 0.0,
		}

		bStart, bLen := unpackBounds(val.buckets[activeIdx].posBuckets.bounds.Load())
		newPt.PositiveBucket.Offset = bStart
		newPt.PositiveBucket.Counts = buildCounts(
			val.buckets[activeIdx].posBuckets.counts,
			bStart,
			bLen,
			newPt.PositiveBucket.Counts,
		)

		nbStart, nbLen := unpackBounds(val.buckets[activeIdx].negBuckets.bounds.Load())
		newPt.NegativeBucket.Offset = nbStart
		newPt.NegativeBucket.Counts = buildCounts(
			val.buckets[activeIdx].negBuckets.counts,
			nbStart,
			nbLen,
			newPt.NegativeBucket.Counts,
		)

		totalCount := newPt.ZeroCount
		for _, c := range newPt.PositiveBucket.Counts {
			totalCount += c
		}
		for _, c := range newPt.NegativeBucket.Counts {
			totalCount += c
		}
		newPt.Count = totalCount

		if !e.noSum {
			newPt.Sum = val.sum.load()
		}
		if !e.noMinMax {
			if val.minMax.set.Load() {
				newPt.Min = metricdata.NewExtrema(val.minMax.minimum.Load())
				newPt.Max = metricdata.NewExtrema(val.minMax.maximum.Load())
			}
		}

		collectExemplars(&newPt.Exemplars, val.res.Collect)

		val.rescaleMu.Unlock()

		hDPts = append(hDPts, newPt)
		return true
	})

	e.start = t
	e.values[oldIdx].Clear()

	h.DataPoints = hDPts
	*dest = h
	return n
}

// newCumulativeExpoHistogram returns an Aggregator that summarizes a set of
// measurements as an exponential histogram. Each histogram is scoped by attributes
// and the aggregation cycle the measurements were made in.
func newCumulativeExpoHistogram[N int64 | float64](
	maxSize, maxScale int32,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *cumulativeExpoHistogram[N] {
	return &cumulativeExpoHistogram[N]{
		noSum:    noSum,
		noMinMax: noMinMax,
		maxSize:  int(maxSize),
		maxScale: maxScale,

		newRes: r,
		values: limitedSyncMap{aggLimit: limit},

		start: now(),
	}
}

// cumulativeExpoHistogram summarizes a set of measurements as an histogram with exponentially
// defined buckets.
type cumulativeExpoHistogram[N int64 | float64] struct {
	noSum    bool
	noMinMax bool
	maxSize  int
	maxScale int32

	newRes func(attribute.Set) FilteredExemplarReservoir[N]
	values limitedSyncMap

	start time.Time
}

func (e *cumulativeExpoHistogram[N]) measure(
	ctx context.Context,
	value N,
	fltrAttr attribute.Set,
	droppedAttr []attribute.KeyValue,
) {
	// Ignore NaN and infinity.
	if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
		return
	}

	val := e.values.LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) any {
		v := newExpoHistogramDataPoint[N](attr, e.maxSize, e.maxScale, e.noMinMax, e.noSum)
		v.res = e.newRes(attr)
		return v
	}).(*expoHistogramDataPoint[N])

	val.record(value)
	val.res.Offer(ctx, value, droppedAttr)
}

func (e *cumulativeExpoHistogram[N]) collect(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.ExponentialHistogram, memory reuse is missed.
	// In that case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.ExponentialHistogram[N])
	h.Temporality = metricdata.CumulativeTemporality

	// Values are being concurrently written while we iterate, so only use the
	// current length for capacity.
	hDPts := reset(h.DataPoints, 0, e.values.Len())

	perSeriesStartTimeEnabled := x.PerSeriesStartTimestamps.Enabled()

	e.values.Range(func(_, value any) bool {
		val := value.(*expoHistogramDataPoint[N])
		val.rescaleMu.Lock()
		activeIdx := val.wg.hotIdx()

		newPt := metricdata.ExponentialHistogramDataPoint[N]{
			Attributes:    val.attrs,
			StartTime:     e.start,
			Time:          t,
			Scale:         val.buckets[activeIdx].scale.Load(),
			ZeroCount:     val.zeroCount.Load(),
			ZeroThreshold: 0.0,
		}
		if perSeriesStartTimeEnabled {
			newPt.StartTime = val.startTime
		}

		bStart, bLen := unpackBounds(val.buckets[activeIdx].posBuckets.bounds.Load())
		newPt.PositiveBucket.Offset = bStart
		newPt.PositiveBucket.Counts = buildCounts(
			val.buckets[activeIdx].posBuckets.counts,
			bStart,
			bLen,
			newPt.PositiveBucket.Counts,
		)

		nbStart, nbLen := unpackBounds(val.buckets[activeIdx].negBuckets.bounds.Load())
		newPt.NegativeBucket.Offset = nbStart
		newPt.NegativeBucket.Counts = buildCounts(
			val.buckets[activeIdx].negBuckets.counts,
			nbStart,
			nbLen,
			newPt.NegativeBucket.Counts,
		)

		totalCount := newPt.ZeroCount
		for _, c := range newPt.PositiveBucket.Counts {
			totalCount += c
		}
		for _, c := range newPt.NegativeBucket.Counts {
			totalCount += c
		}
		newPt.Count = totalCount

		if !e.noSum {
			newPt.Sum = val.sum.load()
		}
		if !e.noMinMax {
			if val.minMax.set.Load() {
				newPt.Min = metricdata.NewExtrema(val.minMax.minimum.Load())
				newPt.Max = metricdata.NewExtrema(val.minMax.maximum.Load())
			}
		}

		collectExemplars(&newPt.Exemplars, val.res.Collect)

		val.rescaleMu.Unlock()

		hDPts = append(hDPts, newPt)
		return true
	})

	h.DataPoints = hDPts
	*dest = h
	return e.values.Len()
}

// packBounds packs startBin (int32) and length (int32) into a uint64.
func packBounds(startBin, length int32) uint64 {
	return (uint64(uint32(startBin)) << 32) | uint64(uint32(length)) //nolint:gosec // bitwise conversion
}

func buildCounts(counts []atomic.Uint64, startBin, length int32, reuse []uint64) []uint64 {
	if length == 0 {
		return reuse[:0]
	}
	if int(length) > cap(reuse) {
		reuse = make([]uint64, length)
	} else {
		reuse = reuse[:length]
	}
	maxSize := len(counts)
	for i := range length {
		bin := startBin + i
		idx := (int(bin)%maxSize + maxSize) % maxSize
		reuse[i] = counts[idx].Load()
	}
	// Trim trailing zeros that might have been caused by downscale operations
	for len(reuse) > 0 {
		if reuse[len(reuse)-1] != 0 {
			break
		}
		reuse = reuse[:len(reuse)-1]
	}
	return reuse
}

// unpackBounds unpacks a uint64 into startBin (int32) and length (int32).
func unpackBounds(bounds uint64) (int32, int32) {
	return int32(bounds >> 32), int32(bounds) //nolint:gosec // bitwise conversion
}
