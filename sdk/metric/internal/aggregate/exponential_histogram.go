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
	expoHistogramPointCounters[N]

	attrs attribute.Set
	res   FilteredExemplarReservoir[N]

	noMinMax bool
	noSum    bool
	maxScale int32
}

func newExpoHistogramDataPoint[N int64 | float64](
	attrs attribute.Set,
	maxSize int,
	maxScale int32,
	noMinMax, noSum bool,
) *expoHistogramDataPoint[N] { // nolint:revive // we need this control flag
	return &expoHistogramDataPoint[N]{
		attrs:                      attrs,
		noMinMax:                   noMinMax,
		noSum:                      noSum,
		expoHistogramPointCounters: newExpoHistogramPointCounters[N](maxSize, maxScale),
	}
}

// hotColdExpoHistogramPoint a hot and cold exponential histogram points, used
// in cumulative aggregations.
type hotColdExpoHistogramPoint[N int64 | float64] struct {
	hcwg         hotColdWaitGroup
	hotColdPoint [2]expoHistogramPointCounters[N]

	attrs attribute.Set
	res   FilteredExemplarReservoir[N]

	noMinMax bool
	noSum    bool
	maxScale int32
}

func newHotColdExpoHistogramDataPoint[N int64 | float64](
	attrs attribute.Set,
	maxSize int,
	maxScale int32,
	noMinMax, noSum bool,
) *hotColdExpoHistogramPoint[N] { // nolint:revive // we need this control flag
	return &hotColdExpoHistogramPoint[N]{
		attrs:    attrs,
		noMinMax: noMinMax,
		noSum:    noSum,
		maxScale: maxScale,
		hotColdPoint: [2]expoHistogramPointCounters[N]{
			newExpoHistogramPointCounters[N](maxSize, maxScale),
			newExpoHistogramPointCounters[N](maxSize, maxScale),
		},
	}
}

// record adds a new measurement to the histogram. It will rescale the buckets if needed.
func (p *expoHistogramPointCounters[N]) record(v N, noMinMax, noSum bool) { // nolint:revive // we need this control flag
	absV := math.Abs(float64(v))
	if absV == 2 {
		fmt.Printf("STARTS HERE!!!!!!!!!!!!!!!!!!!!!!!!!!!\n")
	}
	if float64(absV) == 0.0 {
		p.zeroCount.Add(1)
		return
	}
	bucket := &p.posBuckets
	if v < 0 {
		bucket = &p.negBuckets
		fmt.Printf("DOING NEGATIVE BUCKETS\n")
	}
	if !bucket.tryFastRecord(absV) {
		if !bucket.record(absV) {
			// We failed to record for an unrecoverable reason.
			return
		} else {
			fmt.Printf("SLOW PATH FOR %v\n", v)
		}
	} else {
		fmt.Printf("FAST PATH FOR %v\n", v)
	}
	if !noMinMax {
		p.minMax.Update(v)
	}
	if !noSum {
		p.sum.add(v)
	}
}

// expoHistogramPointCounters contains only the atomic counter data, and is
// used by both expoHistogramDataPoint and hotColdExpoHistogramPoint.
type expoHistogramPointCounters[N int64 | float64] struct {
	minMax    atomicMinMax[N]
	sum       atomicCounter[N]
	zeroCount atomic.Uint64

	posBuckets hotColdExpoBuckets
	negBuckets hotColdExpoBuckets
}

func newExpoHistogramPointCounters[N int64 | float64](
	maxSize int,
	maxScale int32) expoHistogramPointCounters[N] {
	return expoHistogramPointCounters[N]{
		posBuckets: newHotColdExpoBuckets(maxSize, maxScale),
		negBuckets: newHotColdExpoBuckets(maxSize, maxScale),
	}
}

func (e *expoHistogramPointCounters[N]) reset(maxScale int32) {
	e.sum.reset()
	e.zeroCount.Store(0)
	e.posBuckets.reset(maxScale)
	e.negBuckets.reset(maxScale)
}

func (e *expoHistogramPointCounters[N]) loadInto(into *metricdata.ExponentialHistogramDataPoint[N], noMinMax, noSum bool) {
	into.ZeroCount = e.zeroCount.Load()
	if !noSum {
		into.Sum = e.sum.load()
	}
	if !noMinMax && e.minMax.set.Load() {
		into.Min = metricdata.NewExtrema(e.minMax.minimum.Load())
		into.Max = metricdata.NewExtrema(e.minMax.maximum.Load())
	}
	// Lock to ensure no rescale happens while we read values.
	e.posBuckets.rescaleMux.Lock()
	defer e.posBuckets.rescaleMux.Unlock()
	e.negBuckets.rescaleMux.Lock()
	defer e.negBuckets.rescaleMux.Unlock()
	fmt.Println("BEFORE")
	e.negBuckets.print()
	into.Scale = e.posBuckets.unifyScale(&e.negBuckets)
	fmt.Println("AFTER")
	e.negBuckets.print()

	posCount, posOffset := e.posBuckets.loadCountsAndOffset(&into.PositiveBucket.Counts)
	into.PositiveBucket.Offset = posOffset

	negCount, negOffset := e.negBuckets.loadCountsAndOffset(&into.NegativeBucket.Counts)
	into.NegativeBucket.Offset = negOffset

	into.Count = posCount + negCount + into.ZeroCount

}

// mergeInto merges this set of histogram counter data into another,
// and resets the state of this set of counters. This is used by
// hotColdHistogramPoint to ensure that the cumulative counters continue to
// accumulate after being read.
func (p *expoHistogramPointCounters[N]) mergeInto( // nolint:revive // Intentional internal control flag
	into *expoHistogramPointCounters[N],
	noMinMax, noSum bool,
) {
	// Do not reset min or max because cumulative min and max only ever grow
	// smaller or larger respectively.
	if !noMinMax && p.minMax.set.Load() {
		into.minMax.Update(p.minMax.minimum.Load())
		into.minMax.Update(p.minMax.maximum.Load())
	}
	if !noSum {
		into.sum.add(p.sum.load())
	}
	p.posBuckets.mergeInto(&into.posBuckets)
	p.negBuckets.mergeInto(&into.negBuckets)
}

type hotColdExpoBuckets struct {
	rescaleMux     sync.Mutex
	hcwg           hotColdWaitGroup
	hotColdBuckets [2]expoBuckets

	maxScale int32
}

func newHotColdExpoBuckets(maxSize int, maxScale int32) hotColdExpoBuckets {
	return hotColdExpoBuckets{
		hotColdBuckets: [2]expoBuckets{
			newExpoBuckets(maxSize, maxScale),
			newExpoBuckets(maxSize, maxScale),
		},
		maxScale: maxScale,
	}
}

func (b *hotColdExpoBuckets) tryFastRecord(v float64) bool {
	hotIdx := b.hcwg.start()
	defer b.hcwg.done(hotIdx)
	return b.hotColdBuckets[hotIdx].recordBucket(b.hotColdBuckets[hotIdx].getBin(v))
}

func (b *hotColdExpoBuckets) record(v float64) bool {
	b.rescaleMux.Lock()
	defer b.rescaleMux.Unlock()

	// Hot may have been swapped while we were waiting for the lock.
	// We don't use p.hcwg.start() because we already hold the lock, and would
	// deadlock when waiting for writes to complete.
	hotIdx := b.hcwg.loadHot()
	hotBucket := &b.hotColdBuckets[hotIdx]

	// Try recording again in-case it was resized while we were waiting, and to
	// ensure the bucket range doesn't change.
	bin := hotBucket.getBin(v)
	if hotBucket.recordBucket(hotBucket.getBin(v)) {
		fmt.Printf("Resized while we waited for lock PATH for %v\n", v)
		return true
	}

	// Since recordBucket failed above, we know we need a scale change.
	scaleDelta := hotBucket.scaleChange(bin)
	if hotBucket.scale-scaleDelta < expoMinScale {
		// With a scale of -10 there is only two buckets for the whole range of float64 values.
		// This can only happen if there is a max size of 1.
		otel.Handle(errors.New("exponential histogram scale underflow"))
		return false
	}
	// Copy scale and min/max to cold
	coldIdx := (hotIdx + 1) % 2
	coldBucket := &b.hotColdBuckets[coldIdx]
	coldBucket.scale = hotBucket.scale
	startBin, endBin := hotBucket.startAndEnd.Load()
	coldBucket.startAndEnd.Store(startBin, endBin)
	// Downscale cold to the new scale
	coldBucket.downscale(scaleDelta)
	// Expand the cold prior to swapping to hot to ensure our measurement fits.
	bin = coldBucket.getBin(v)
	coldBucket.resizeToInclude(bin)

	b.hcwg.swapHotAndWait()
	// Now that hot has become cold, downscale it, and merge it into the new hot buckets.
	hotBucket.downscale(scaleDelta)
	hotBucket.mergeInto(coldBucket)
	hotBucket.reset(b.maxScale)

	return coldBucket.recordBucket(bin)
}

func (b *hotColdExpoBuckets) mergeInto(into *hotColdExpoBuckets) {
	b.hotColdBuckets[b.hcwg.loadHot()].mergeInto(&into.hotColdBuckets[into.hcwg.loadHot()])
}

func (b *hotColdExpoBuckets) print() {
	b.hotColdBuckets[b.hcwg.loadHot()].print()
}

func (b *hotColdExpoBuckets) reset(maxScale int32) {
	b.hotColdBuckets[0].reset(maxScale)
	b.hotColdBuckets[1].reset(maxScale)
}

// lock must already be held
func (b *hotColdExpoBuckets) unifyScale(other *hotColdExpoBuckets) int32 {
	bHotIdx := b.hcwg.loadHot()
	bScale := b.hotColdBuckets[bHotIdx].scale
	otherHotIdx := other.hcwg.loadHot()
	otherScale := other.hotColdBuckets[otherHotIdx].scale
	if bScale < otherScale {
		other.downscale(otherScale-bScale, otherHotIdx)
	} else if bScale > otherScale {
		b.downscale(bScale-otherScale, bHotIdx)
	}
	return min(bScale, otherScale)
}

// downscale force-downscales the bucket. It is assumed that the new scale is valid.
func (b *hotColdExpoBuckets) downscale(delta int32, hotIdx uint64) {
	fmt.Printf("downscale(%v, %v)\n", delta, hotIdx)
	// Copy scale and min/max to cold
	coldIdx := (hotIdx + 1) % 2
	coldBucket := &b.hotColdBuckets[coldIdx]
	hotBucket := &b.hotColdBuckets[hotIdx]
	coldBucket.scale = hotBucket.scale
	startBin, endBin := hotBucket.startAndEnd.Load()
	fmt.Printf("downscale start %v, end %v loaded from hot\n", startBin, endBin)

	coldBucket.startAndEnd.Store(startBin, endBin)
	// Downscale cold to the new scale
	coldBucket.downscale(delta)
	startBin, endBin = coldBucket.startAndEnd.Load()
	fmt.Printf("downscale COLD start %v, end %v after downscale\n", startBin, endBin)

	b.hcwg.swapHotAndWait()
	// Now that hot has become cold, downscale it, and merge it into the new hot buckets.
	hotBucket.downscale(delta)
	hotBucket.mergeInto(coldBucket)
	hotBucket.reset(b.maxScale)
	startBin, endBin = coldBucket.startAndEnd.Load()
	fmt.Printf("downscale COLD start %v, end %v after hot merged Into\n", startBin, endBin)
}

func (b *hotColdExpoBuckets) loadCountsAndOffset(buckets *[]uint64) (uint64, int32) {
	return b.hotColdBuckets[b.hcwg.loadHot()].loadCountsAndOffset(buckets)
}

// expoBuckets is a set of buckets in an exponential histogram.
type expoBuckets struct {
	scale       int32
	startAndEnd atomicLimitedRange
	counts      []atomic.Uint64
}

func newExpoBuckets(maxSize int, maxScale int32) expoBuckets {
	return expoBuckets{
		scale:       maxScale,
		counts:      make([]atomic.Uint64, maxSize),
		startAndEnd: atomicLimitedRange{maxSize: int32(maxSize)},
	}
}

// getIdx returns the index into counts for the provided bin.
func (e *expoBuckets) getIdx(bin int32) int {
	newBin := int(bin) % len(e.counts)
	return (newBin + len(e.counts)) % len(e.counts)
}

func (e *expoBuckets) reset(maxScale int32) {
	e.scale = maxScale
	e.startAndEnd.Store(0, 0)
	for i := range e.counts {
		e.counts[i].Store(0)
	}
}

func (e *expoBuckets) print() {
	buckets := []uint64{}
	count, offset := e.loadCountsAndOffset(&buckets)
	fmt.Printf("expoBuckets offset %v, buckets %+v, count %v\n", offset, buckets, count)
}

func (e *expoBuckets) loadCountsAndOffset(into *[]uint64) (uint64, int32) {
	// TODO (#3047): Making copies for bounds and counts incurs a large
	// memory allocation footprint. Alternatives should be explored.
	start, end := e.startAndEnd.Load()
	length := int(end - start)
	counts := reset(*into, length, length)
	count := uint64(0)
	eIdx := start
	for i := range length {
		val := e.counts[e.getIdx(eIdx)].Load()
		counts[i] = val
		count += val
		eIdx++
	}
	*into = counts
	return count, start
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

// scaleChange returns the magnitude of the scale change needed to fit bin in
// the bucket. If no scale change is needed 0 is returned.
func (b *expoBuckets) scaleChange(bin int32) int32 {
	startBin, endBin := b.startAndEnd.Load()
	if startBin == endBin {
		// No need to rescale if there are no buckets.
		return 0
	}

	lastBin := endBin - 1
	if bin < startBin {
		startBin = bin
	} else if bin > lastBin {
		lastBin = bin
	}

	var count int32
	for lastBin-startBin >= int32(len(b.counts)) {
		startBin >>= 1
		lastBin >>= 1
		count++
		if count > expoMaxScale-expoMinScale {
			return count
		}
	}
	return count
}

// recordBucket returns true if the bucket was incremented, or false if a downscale is required to
func (b *expoBuckets) recordBucket(bin int32) bool {
	fmt.Printf("recordBucket(%v) START\n", bin)
	if b.startAndEnd.Add(bin) {
		b.counts[b.getIdx(bin)].Add(1)
		startBin, endBin := b.startAndEnd.Load()
		fmt.Printf("recordBucket(%v) END. start %v, end %v\n", bin, startBin, endBin)
		return true
	}
	return false
}

func (b *expoBuckets) validate() {
	startBin, endBin := b.startAndEnd.Load()
	if endBin-startBin > int32(len(b.counts)) {
		fmt.Printf("inconsistent start %v end %v len %v\n", startBin, endBin, len(b.counts))
	}
}

// downscale shrinks a bucket by a factor of 2*s. It will sum counts into the
// correct lower resolution bucket.
func (b *expoBuckets) downscale(delta int32) {
	b.scale -= delta
	// Example
	// delta = 2
	// Original offset: -6
	// Counts: [ 3,  1,  2,  3,  4,  5, 6, 7, 8, 9, 10]
	// bins:    -6  -5, -4, -3, -2, -1, 0, 1, 2, 3, 4
	// new bins:-2, -2, -1, -1, -1, -1, 0, 0, 0, 0, 1
	// new Offset: -2
	// new Counts: [4, 14, 30, 10]

	startBin, endBin := b.startAndEnd.Load()
	length := endBin - startBin
	if length <= 1 || delta < 1 {
		newStartBin := startBin >> delta
		newEndBin := newStartBin + length
		b.startAndEnd.Store(newStartBin, newEndBin)
		b.validate()
		// Shift all elements left by the change in start position
		startShift := b.getIdx(startBin - newStartBin)
		b.counts = append(b.counts[startShift:], b.counts[:startShift]...)

		// Clear all elements that are outside of our start to end range
		for i := newEndBin; i < newStartBin+int32(len(b.counts)); i++ {
			b.counts[b.getIdx(i)].Store(0)
		}
		fmt.Printf("downscale SHORT downsize %v startBin %v, endBin %v, counts %v\n", delta, newStartBin, newEndBin, b.counts)
		return
	}

	steps := int32(1) << delta
	offset := startBin % steps
	offset = (offset + steps) % steps // to make offset positive
	newLen := (length-1+offset)/steps + 1
	newStartBin := startBin >> delta
	newEndBin := newStartBin + newLen
	startShift := b.getIdx(startBin - newStartBin)

	fmt.Printf("downscale START downsize %v startBin %v, newStartBin %v, shift %v, endBin %v, newEndBin %v, steps %v, offset %v, counts %v\n", delta, startBin, newStartBin, startShift, endBin, newEndBin, steps, offset, b.counts)
	for i := startBin + 1; i < endBin; i++ {
		newIdx := b.getIdx(int32(math.Floor(float64(i)/float64(steps))) + int32(startShift))
		if i%steps == 0 {
			b.counts[newIdx].Store(b.counts[b.getIdx(i)].Load())
			fmt.Printf("downscale SET idx %v counts %+v\n", b.getIdx(i/steps+int32(startShift)), b.counts)
			continue
		}
		b.counts[newIdx].Add(b.counts[b.getIdx(i)].Load())
		fmt.Printf("downscale ADD startBin %v, endBin %v, i %v i/steps %v idx %v, getIdx %v counts %+v\n", startBin, endBin, i, i/steps, i/steps+int32(startShift), b.getIdx(i/steps+int32(startShift)), b.counts)
		if startBin == -524288 {
			panic("FOO")
		}
	}
	fmt.Printf("downscale END Merge downsize %v startBin %v, endBin %v, counts %v\n", delta, startBin, endBin, b.counts)

	fmt.Printf("downscale LONG downsize %v startBin %v, endBin %v, counts %v\n", delta, newStartBin, newEndBin, b.counts)
	b.startAndEnd.Store(newStartBin, newEndBin)
	b.validate()
	// Shift all elements left by the change in start position
	fmt.Printf("downscale SHIFT startBin %v newStartBin %v shift %v\n", startBin, newStartBin, startShift)
	b.counts = append(b.counts[startShift:], b.counts[:startShift]...)

	fmt.Printf("downscale AfterShiFT downsize %v startBin %v, endBin %v, counts %v\n", delta, newStartBin, newEndBin, b.counts)
	// Clear all elements that are outside of our start to end range
	for i := newEndBin; i < newStartBin+int32(len(b.counts)); i++ {
		b.counts[b.getIdx(i)].Store(0)
	}
	fmt.Printf("downscale FINAL downsize %v startBin %v, endBin %v, counts %v\n", delta, newStartBin, newEndBin, b.counts)
}

func (b *expoBuckets) resizeToInclude(bin int32) {
	startBin, endBin := b.startAndEnd.Load()
	if startBin == endBin {
		startBin = bin
		endBin = bin + 1
		b.startAndEnd.Store(startBin, endBin)
		b.validate()
		fmt.Printf("resizeToInclude AAAAA bin %v start %v , end %v\n", bin, startBin, endBin)
	} else if bin < startBin {
		b.startAndEnd.Store(bin, endBin)
		b.validate()
		fmt.Printf("resizeToInclude CCCCC bin %v start %v , end %v\n", bin, startBin, endBin)
	} else if bin >= endBin {
		b.startAndEnd.Store(startBin, bin+1)
		b.validate()
		fmt.Printf("resizeToInclude DDDDDD bin %v start %v , end %v\n", bin, startBin, endBin)
	} else {
		fmt.Printf("resizeToInclude BBBBBB bin %v start %v , end %v\n", bin, startBin, endBin)
	}
}

// mergeInto merges this expoBuckets into another, and resets the state
// of the expoBuckets. This is used to ensure that the cumulative counters
// continue to accumulate after being read. It returns the scale change that
// was applied to the input buckets.
func (b *expoBuckets) mergeInto(into *expoBuckets) {
	// Rescale both to the same scale
	scaleDelta := into.scale - b.scale
	if scaleDelta > 0 {
		into.downscale(scaleDelta)
	} else if scaleDelta < 0 {
		b.downscale(-scaleDelta)
	}
	if into.scale != b.scale {
		panic("scale not equal when merging")
	}

	startBin, endBin := b.startAndEnd.Load()
	if startBin != endBin {
		into.resizeToInclude(startBin)
		into.resizeToInclude(endBin - 1)
	}
	scaleDelta = into.scaleChange(endBin - 1)
	if scaleDelta > 0 {
		// Merging buckets required a scale change to the positive buckets to
		// fit within the max scale. Update scale and scale down the negative
		// buckets to match.
		b.downscale(scaleDelta)
		into.downscale(scaleDelta)
	}
	// At this point, into as been expanded to be a superset of b.
	// Now we finally increment buckets.
	bStartBin, _ := b.startAndEnd.Load()
	intoStartBin, _ := b.startAndEnd.Load()
	startBinDelta := bStartBin - intoStartBin
	for i := range b.counts {
		into.counts[i+int(startBinDelta)].Add(b.counts[i].Load())
	}
	fmt.Printf("MERGE into counts after merge %+v\n", into.counts)
	b.validate()
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
		maxSize:  int(maxSize),
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
	maxSize  int
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
	v.record(value, e.noMinMax, e.noSum)
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
		hDPts[i].ZeroThreshold = 0.0

		val.loadInto(&hDPts[i], e.noMinMax, e.noSum)
		collectExemplars(&hDPts[i].Exemplars, val.res.Collect)

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
		maxSize:  int(maxSize),
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

	v := e.values.LoadOrStoreAttr(fltrAttr, func(attr attribute.Set) any {
		hPt := newHotColdExpoHistogramDataPoint[N](attr, e.maxSize, e.maxScale, e.noMinMax, e.noSum)
		hPt.res = e.newRes(attr)
		return hPt
	}).(*hotColdExpoHistogramPoint[N])

	hotIdx := v.hcwg.start()
	defer v.hcwg.done(hotIdx)
	v.hotColdPoint[hotIdx].record(value, e.noMinMax, e.noSum)
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
		val := value.(*hotColdExpoHistogramPoint[N])
		readIdx := val.hcwg.swapHotAndWait()
		newPt := metricdata.ExponentialHistogramDataPoint[N]{
			Attributes:    val.attrs,
			StartTime:     e.start,
			Time:          t,
			ZeroThreshold: 0.0,
		}

		val.hotColdPoint[readIdx].loadInto(&newPt, e.noMinMax, e.noSum)
		// Once we've read the point, merge it back into the hot histogram
		// point since it is cumulative.
		hotIdx := (readIdx + 1) % 2
		val.hotColdPoint[readIdx].mergeInto(&val.hotColdPoint[hotIdx], e.noMinMax, e.noSum)
		val.hotColdPoint[readIdx].reset(val.maxScale)

		collectExemplars(&newPt.Exemplars, val.res.Collect)
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
