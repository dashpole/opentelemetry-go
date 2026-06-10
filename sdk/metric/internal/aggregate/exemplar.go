// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregate // import "go.opentelemetry.io/otel/sdk/metric/internal/aggregate"

import (
	"sync"

	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var exemplarPool = sync.Pool{
	New: func() any { return new([]exemplar.Exemplar) },
}

func collectExemplars[N int64 | float64](out *[]metricdata.Exemplar[N], f func(*[]exemplar.Exemplar)) {
	dest := exemplarPool.Get().(*[]exemplar.Exemplar)
	defer func() {
		for i := range *dest {
			(*dest)[i].FilteredAttributes = nil
			(*dest)[i].TraceID = (*dest)[i].TraceID[:0]
			(*dest)[i].SpanID = (*dest)[i].SpanID[:0]
		}
		*dest = (*dest)[:0]
		exemplarPool.Put(dest)
	}()

	*dest = reset(*dest, len(*out), cap(*out))

	f(dest)

	*out = reset(*out, len(*dest), len(*dest))
	for i, e := range *dest {
		(*out)[i].FilteredAttributes = e.FilteredAttributes
		(*out)[i].Time = e.Time

		if cap((*out)[i].TraceID) >= len(e.TraceID) {
			(*out)[i].TraceID = (*out)[i].TraceID[:len(e.TraceID)]
		} else {
			(*out)[i].TraceID = make([]byte, len(e.TraceID))
		}
		copy((*out)[i].TraceID, e.TraceID)

		if cap((*out)[i].SpanID) >= len(e.SpanID) {
			(*out)[i].SpanID = (*out)[i].SpanID[:len(e.SpanID)]
		} else {
			(*out)[i].SpanID = make([]byte, len(e.SpanID))
		}
		copy((*out)[i].SpanID, e.SpanID)

		switch e.Value.Type() {
		case exemplar.Int64ValueType:
			(*out)[i].Value = N(e.Value.Int64())
		case exemplar.Float64ValueType:
			(*out)[i].Value = N(e.Value.Float64())
		}
	}
}
