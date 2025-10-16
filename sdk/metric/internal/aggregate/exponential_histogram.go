// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

const (
	expoMaxScale = 20
	expoMinScale = -10
)

// expoHistogramDataPoint is a single data point in an exponential histogram.
type expoHistogramDataPoint[N int64 | float64] struct {
	attrs attribute.Set
	res   FilteredExemplarReservoir[N]

	count     atomic.Uint64
	minMax    atomicMinMax[N]
	sum       atomicCounter[N]
	zeroCount atomic.Uint64

	noMinMax bool
	noSum    bool

	posBuckets hotColdExpoBuckets
	negBuckets hotColdExpoBuckets
}

func newExpoHistogramDataPoint[N int64 | float64](
	attrs attribute.Set,
	maxSize int32,
	maxScale int32,
	noMinMax, noSum bool,
) *expoHistogramDataPoint[N] { // nolint:revive // we need this control flag
	return &expoHistogramDataPoint[N]{
		attrs:    attrs,
		noMinMax: noMinMax,
		noSum:    noSum,
		posBuckets: hotColdExpoBuckets{
			buckets: [2]expoBuckets{
				{
					bucketRange: atomicLimitedRange{maxSize: maxSize},
					scale:       maxScale,
				},
				{
					bucketRange: atomicLimitedRange{maxSize: maxSize},
					scale:       maxScale,
				},
			},
		},
		negBuckets: hotColdExpoBuckets{
			buckets: [2]expoBuckets{
				{
					bucketRange: atomicLimitedRange{maxSize: maxSize},
					scale:       maxScale,
				},
				{
					bucketRange: atomicLimitedRange{maxSize: maxSize},
					scale:       maxScale,
				},
			},
		},
	}
}

// record adds a new measurement to the histogram. It will rescale the buckets if needed.
func (p *expoHistogramDataPoint[N]) record(v N) {
	fmt.Printf("record %v\n", v)
	p.count.Add(1)

	if !p.noMinMax {
		p.minMax.observe(v)
	}
	if !p.noSum {
		p.sum.add(v)
	}

	absV := math.Abs(float64(v))

	if float64(absV) == 0.0 {
		p.zeroCount.Add(1)
		return
	}

	hcBucket := &p.posBuckets
	if v < 0 {
		hcBucket = &p.negBuckets
	}

	hotIdx := hcBucket.hcwg.start()

	fmt.Printf("scale %v\n", hcBucket.buckets[hotIdx].scale)
	bin := hcBucket.buckets[hotIdx].getBin(absV)
	fmt.Printf("bin %v\n", bin)
	if hcBucket.buckets[hotIdx].tryRecord(bin) {
		start, end := hcBucket.buckets[hotIdx].bucketRange.Load()
		fmt.Printf("start and end after success start %v, end %v\n", start, end)
		hcBucket.hcwg.done(hotIdx)
		return
	}
	start, end := hcBucket.buckets[hotIdx].bucketRange.Load()
	fmt.Printf("start and end after failed start %v, end %v\n", start, end)
	// Even though we haven't recorded our measurement, we need to mark our
	// attempt completed so we don't deadlock when trying to aquire the lock.
	hcBucket.hcwg.done(hotIdx)

	// slow path: rescale required
	hcBucket.rescaleMux.Lock()
	defer hcBucket.rescaleMux.Unlock()
	// Now that we have the lock, mark our measure as completed so we can
	// swap hot + cold.

	// If the new bin would make the counts larger than maxScale, we need to
	// downscale current measurements.
	if scaleDelta := hcBucket.buckets[hotIdx].scaleChange(bin); scaleDelta > 0 {
		fmt.Sprintf("scaleDelta %v to fit bin %v\n", scaleDelta, bin)
		if hcBucket.buckets[hotIdx].scale-scaleDelta < expoMinScale {
			// With a scale of -10 there is only two buckets for the whole range of float64 values.
			// This can only happen if there is a max size of 1.
			otel.Handle(errors.New("exponential histogram scale underflow"))
			return
		}
		coldIdx := (hotIdx + 1) % 2
		// set the scale and start/end for the cold buckets to the new scale
		// prior to swapping it to hot.
		hcBucket.buckets[coldIdx].scale = hcBucket.buckets[hotIdx].scale - scaleDelta
		hotStart, hotEnd := hcBucket.buckets[hotIdx].bucketRange.Load()
		hcBucket.buckets[coldIdx].bucketRange.Store(hotStart, hotEnd)
		hcBucket.buckets[coldIdx].downscale(scaleDelta)
		// record our point prior to swapping hot and cold to ensure our
		// measurement fits, and that a different measurement doesn't steal our
		// rescale.
		bin := hcBucket.buckets[coldIdx].getBin(absV)
		if !hcBucket.buckets[coldIdx].tryRecord(bin) {
			start, end := hcBucket.buckets[coldIdx].bucketRange.Load()
			panic(fmt.Sprintf("this should not happen. start %v, end %v, hotStart %v, hotEnd %v bin %v", start, end, hotStart, hotEnd, bin))
			otel.Handle(errors.New("exponential resize failed"))
			return
		}

		hcBucket.hcwg.swapHotAndWait()
		// hotIdx is now cold after the swap. Downscale it so it can be merged.
		hcBucket.buckets[hotIdx].downscale(scaleDelta)
		// Merge old buckets into new (hot) buckets and clear old values.
		hcBucket.buckets[hotIdx].bucketCounts.Range(func(key, value any) bool {
			hcBucket.buckets[coldIdx].incrementBucket(key.(int32), value.(*atomic.Uint64).Load())
			// TODO: use LoadAndDelete with a sync.Pool to avoid allocations.
			hcBucket.buckets[hotIdx].bucketCounts.Delete(key)
			return true
		})
	}
}

// getBin returns the bin v should be recorded into.
func (p *expoBuckets) getBin(v float64) int32 {
	frac, expInt := math.Frexp(v)
	// 11-bit exponential.
	exp := int32(expInt) // nolint: gosec
	if p.scale <= 0 {
		// Because of the choice of fraction is always 1 power of two higher than we want.
		var correction int32 = 1
		if frac == .5 {
			// If v is an exact power of two the frac will be .5 and the exp
			// will be one higher than we want.
			correction = 2
		}
		return (exp - correction) >> (-p.scale)
	}
	return exp<<p.scale + int32(math.Log(frac)*scaleFactors[p.scale]) - 1
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

// expoBuckets is a set of buckets in an exponential histogram.
type expoBuckets struct {
	// bucketRange tracks the lowest and highest bucket
	bucketRange atomicLimitedRange
	// this is a map[int32]*atomic.Uint64
	bucketCounts sync.Map
	scale        int32
}

type hotColdExpoBuckets struct {
	buckets [2]expoBuckets
	hcwg    hotColdWaitGroup
	// rescaleMux ensures only one rescale occurs at once.
	rescaleMux sync.Mutex
}

// scaleChange returns the magnitude of the scale change needed to fit bin in
// the bucket. If no scale change is needed 0 is returned.
func (p *expoBuckets) scaleChange(bin int32) int32 {
	low, high := p.bucketRange.Load()
	if low == high {
		// No need to rescale if there are no buckets.
		return 0
	}

	low = min(low, bin)
	high = max(high, bin+1)

	var count int32
	for high-low >= p.bucketRange.maxSize {
		low >>= 1
		high >>= 1
		count++
		if count > expoMaxScale-expoMinScale {
			return count
		}
	}
	return count
}

// tryRecord increments the count for the given bin, and expands the buckets if
// it can. It returns true if it was able to expand the buckets to include bin.
func (b *expoBuckets) tryRecord(bin int32) bool {
	if !b.bucketRange.Add(bin) {
		return false
	}

	// Increment the bucket
	b.incrementBucket(bin, 1)
	return true
}

func (b *expoBuckets) incrementBucket(idx int32, inc uint64) {
	val, loaded := b.bucketCounts.Load(idx)
	if !loaded {
		var a atomic.Uint64
		val, _ = b.bucketCounts.LoadOrStore(idx, &a)
	}
	val.(*atomic.Uint64).Add(inc)
}

func (b *expoBuckets) loadBucket(idx int32) uint64 {
	loaded, ok := b.bucketCounts.Load(idx)
	if !ok {
		return 0
	}
	return loaded.(*atomic.Uint64).Load()
}

func (b *expoBuckets) storeBucket(idx int32, val uint64) {
	loaded, ok := b.bucketCounts.Load(idx)
	if ok {
		loaded.(*atomic.Uint64).Store(val)
		return
	}
	var a atomic.Uint64
	loaded, _ = b.bucketCounts.LoadOrStore(idx, &a)
	loaded.(*atomic.Uint64).Store(val)
}

func (b *expoBuckets) resize(start, end int32) {
	oldStart, oldEnd := b.bucketRange.Load()
	b.bucketRange.Store(start, end)
	for i := oldStart; i < oldEnd; i++ {
		if i >= start || i < end {
			// Keep, since this is within the new range.
			continue
		}
		b.bucketCounts.Delete(i)
	}
}

func (b *expoBuckets) copyInto(into *[]uint64) {
	start, end := b.bucketRange.Load()
	length := int(end - start)
	*into = reset(*into, length, length)
	for i := 0; i < length; i++ {
		(*into)[i] = b.loadBucket(int32(i))
	}
}

// downscale shrinks a bucket by a factor of 2*s. It will sum counts into the
// correct lower resolution bucket.
func (b *expoBuckets) downscale(delta int32) {
	// Example
	// delta = 2
	// Original offset: -6
	// Counts: [ 3,  1,  2,  3,  4,  5, 6, 7, 8, 9, 10]
	// bins:    -6  -5, -4, -3, -2, -1, 0, 1, 2, 3, 4
	// new bins:-2, -2, -1, -1, -1, -1, 0, 0, 0, 0, 1
	// new Offset: -2
	// new Counts: [4, 14, 30, 10]

	start, end := b.bucketRange.Load()
	length := end - start
	if length <= 1 || delta < 1 {
		start >>= delta
		b.bucketRange.Store(start, end)
		return
	}

	steps := int32(1) << delta
	offset := int32(start) % steps
	offset = (offset + steps) % steps // to make offset positive
	for i := int32(1); i < length; i++ {
		idx := i + offset
		if idx%steps == 0 {
			b.storeBucket(idx/steps, b.loadBucket(i))
			continue
		}
		b.incrementBucket(idx/steps, b.loadBucket(i))
	}

	end = (length-1+offset)/steps + 1
	start >>= delta
	b.resize(start, end)
	b.bucketRange.Store(start, end)
}

// newDeltaExponentialHistogram returns an Aggregator that summarizes a set of
// measurements as a delta exponential histogram. Each histogram is scoped by
// attributes and the aggregation cycle the measurements were made in.
func newDeltaExponentialHistogram[N int64 | float64](
	maxSize, maxScale int32,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *deltaExpoHistogram[N] {
	return &deltaExpoHistogram[N]{
		noSum:    noSum,
		noMinMax: noMinMax,
		maxSize:  maxSize,
		maxScale: maxScale,

		newRes: r,
		hotColdValMap: [2]limitedSyncMap{
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
	maxSize  int32
	maxScale int32

	newRes        func(attribute.Set) FilteredExemplarReservoir[N]
	hcwg          hotColdWaitGroup
	hotColdValMap [2]limitedSyncMap

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

	hotIdx := e.hcwg.start()
	defer e.hcwg.done(hotIdx)
	v := e.hotColdValMap[hotIdx].LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) any {
		hPt := newExpoHistogramDataPoint[N](attr, e.maxSize, e.maxScale, e.noMinMax, e.noSum)
		hPt.res = e.newRes(attr)
		return hPt
	}).(*expoHistogramDataPoint[N])
	v.record(value)
	v.res.Offer(ctx, value, droppedAttr)
}

func (e *deltaExpoHistogram[N]) collect(
	dest *metricdata.Aggregation, //nolint:gocritic // The pointer is needed for the ComputeAggregation interface
) int {
	t := now()

	// If *dest is not a metricdata.ExponentialHistogram, memory reuse is missed.
	// In that case, use the zero-value h and hope for better alignment next cycle.
	h, _ := (*dest).(metricdata.ExponentialHistogram[N])
	h.Temporality = metricdata.DeltaTemporality

	// delta always clears values on collection
	readIdx := e.hcwg.swapHotAndWait()

	// The len will not change while we iterate over values, since we waited
	// for all writes to finish to the cold values and len.
	n := e.hotColdValMap[readIdx].Len()
	hDPts := reset(h.DataPoints, n, n)

	var i int
	e.hotColdValMap[readIdx].Range(func(_, value any) bool {
		val := value.(*expoHistogramDataPoint[N])
		hDPts[i].Attributes = val.attrs
		hDPts[i].StartTime = e.start
		hDPts[i].Time = t
		hDPts[i].Count = val.count.Load()
		hDPts[i].ZeroCount = val.zeroCount.Load()
		hDPts[i].ZeroThreshold = 0.0

		if !e.noSum {
			hDPts[i].Sum = val.sum.load()
		}
		if !e.noMinMax {
			if minimum, maximum, ok := val.minMax.load(); ok {
				hDPts[i].Min = metricdata.NewExtrema(minimum)
				hDPts[i].Max = metricdata.NewExtrema(maximum)
			}
		}

		collectExemplars(&hDPts[i].Exemplars, val.res.Collect)

		// aquire exclusive access to the cold pos and negative bucket counts.
		val.posBuckets.rescaleMux.Lock()
		defer val.posBuckets.rescaleMux.Unlock()
		val.negBuckets.rescaleMux.Lock()
		defer val.negBuckets.rescaleMux.Unlock()
		posReadIdx := val.posBuckets.hcwg.swapHotAndWait()
		negReadIdx := val.negBuckets.hcwg.swapHotAndWait()
		// To allow lockless writing, we track the positive and negative scale
		// separately. To re-unify the scale value, we adopt the minimum of the
		// positive and negative scale, and then downscale the higher of the
		// two.
		scale := min(val.posBuckets.buckets[posReadIdx].scale, val.negBuckets.buckets[negReadIdx].scale)
		hDPts[i].Scale = scale

		scaleChange := val.posBuckets.buckets[posReadIdx].scale - scale
		if scaleChange > 0 {
			val.posBuckets.buckets[posReadIdx].downscale(scaleChange)
		}
		scaleChange = val.posBuckets.buckets[negReadIdx].scale - scale
		if scaleChange > 0 {
			val.posBuckets.buckets[negReadIdx].downscale(scaleChange)
		}

		start, end := val.posBuckets.buckets[posReadIdx].bucketRange.Load()
		hDPts[i].PositiveBucket.Offset = int32(start)
		hDPts[i].PositiveBucket.Counts = reset(
			hDPts[i].PositiveBucket.Counts,
			int(end-start),
			int(end-start),
		)
		val.posBuckets.buckets[posReadIdx].copyInto(&hDPts[i].PositiveBucket.Counts)

		start, end = val.posBuckets.buckets[negReadIdx].bucketRange.Load()
		hDPts[i].NegativeBucket.Offset = int32(start)
		hDPts[i].NegativeBucket.Counts = reset(
			hDPts[i].NegativeBucket.Counts,
			int(end-start),
			int(end-start),
		)
		val.posBuckets.buckets[negReadIdx].copyInto(&hDPts[i].NegativeBucket.Counts)

		i++
		return true
	})
	// Unused attribute sets do not report.
	e.hotColdValMap[readIdx].Clear()

	e.start = t
	h.DataPoints = hDPts
	*dest = h
	return n
}

// newCumulativeExponentialHistogram returns an Aggregator that summarizes a
// set of measurements as a cumulative exponential histogram. Each histogram is
// scoped by attributes and the aggregation cycle the measurements were made
// in.
func newCumulativeExponentialHistogram[N int64 | float64](
	maxSize, maxScale int32,
	noMinMax, noSum bool,
	limit int,
	r func(attribute.Set) FilteredExemplarReservoir[N],
) *cumulativeExpoHistogram[N] {
	return &cumulativeExpoHistogram[N]{
		noSum:    noSum,
		noMinMax: noMinMax,
		maxSize:  maxSize,
		maxScale: maxScale,

		newRes: r,
		values: limitedSyncMap{aggLimit: limit},

		start: now(),
	}
}

// cumulativeExpoHistogram summarizes a set of measurements as an cumulative
// histogram with exponentially defined buckets.
type cumulativeExpoHistogram[N int64 | float64] struct {
	noSum    bool
	noMinMax bool
	maxSize  int32
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

	v := e.values.LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) any {
		hPt := newExpoHistogramDataPoint[N](attr, e.maxSize, e.maxScale, e.noMinMax, e.noSum)
		hPt.res = e.newRes(attr)
		return hPt
	}).(*expoHistogramDataPoint[N])
	v.record(value)
	v.res.Offer(ctx, value, droppedAttr)
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

	var i int
	e.values.Range(func(_, value any) bool {
		val := value.(*expoHistogramDataPoint[N])
		newPt := metricdata.ExponentialHistogramDataPoint[N]{
			Attributes:    val.attrs,
			StartTime:     e.start,
			Time:          t,
			Count:         val.count.Load(),
			ZeroCount:     val.zeroCount.Load(),
			ZeroThreshold: 0.0,
		}

		if !e.noSum {
			newPt.Sum = val.sum.load()
		}
		if !e.noMinMax {
			if minimum, maximum, ok := val.minMax.load(); ok {
				newPt.Min = metricdata.NewExtrema(minimum)
				newPt.Max = metricdata.NewExtrema(maximum)
			}
		}

		collectExemplars(&newPt.Exemplars, val.res.Collect)

		// aquire exclusive access to the cold pos and negative bucket counts.
		val.posBuckets.rescaleMux.Lock()
		defer val.posBuckets.rescaleMux.Unlock()
		val.negBuckets.rescaleMux.Lock()
		defer val.negBuckets.rescaleMux.Unlock()
		posReadIdx := val.posBuckets.hcwg.swapHotAndWait()
		negReadIdx := val.negBuckets.hcwg.swapHotAndWait()
		// To allow lockless writing, we track the positive and negative scale
		// separately. To re-unify the scale value, we adopt the minimum of the
		// positive and negative scale, and then downscale the higher of the
		// two.
		scale := min(val.posBuckets.buckets[posReadIdx].scale, val.negBuckets.buckets[negReadIdx].scale)
		newPt.Scale = scale

		scaleChange := val.posBuckets.buckets[posReadIdx].scale - scale
		if scaleChange > 0 {
			val.posBuckets.buckets[posReadIdx].downscale(scaleChange)
		}
		scaleChange = val.posBuckets.buckets[negReadIdx].scale - scale
		if scaleChange > 0 {
			val.posBuckets.buckets[negReadIdx].downscale(scaleChange)
		}

		start, end := val.posBuckets.buckets[posReadIdx].bucketRange.Load()
		newPt.PositiveBucket.Offset = int32(start)
		newPt.PositiveBucket.Counts = reset(
			newPt.PositiveBucket.Counts,
			int(end-start),
			int(end-start),
		)
		val.posBuckets.buckets[posReadIdx].copyInto(&newPt.PositiveBucket.Counts)

		start, end = val.posBuckets.buckets[negReadIdx].bucketRange.Load()
		newPt.NegativeBucket.Offset = int32(start)
		newPt.NegativeBucket.Counts = reset(
			newPt.NegativeBucket.Counts,
			int(end-start),
			int(end-start),
		)
		val.posBuckets.buckets[negReadIdx].copyInto(&newPt.NegativeBucket.Counts)
		hDPts = append(hDPts, newPt)

		i++
		// TODO (#3006): This will use an unbounded amount of memory if there
		// are unbounded number of attribute sets being aggregated. Attribute
		// sets that become "stale" need to be forgotten so this will not
		// overload the system.
		return true
	})

	h.DataPoints = hDPts
	*dest = h
	return i
}
