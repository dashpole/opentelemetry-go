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

package opencensusmetric // import "go.opentelemetry.io/otel/bridge/opencensus/opencensusmetric"

import (
	"context"

	ocmetricdata "go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricproducer"

	"go.opentelemetry.io/otel/bridge/opencensus/opencensusmetric/internal"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	// instrumentationName is the name of this instrumentation package.
	instrumentationName = "go.opentelemetry.io/otel/bridge/opencensus/opencensusmetric"
)

// producer is a producer which provides metrics collected using OpenCensus
// instrumentation.
type producer struct {
	res     *resource.Resource
	scope   instrumentation.Scope
	manager *metricproducer.Manager
}

// NewProducer returns a producer which can be invoked to collect metrics.
func NewProducer(opts ...Option) metric.Producer {
	cfg := newConfig(opts)
	return &producer{
		res:     cfg.res,
		scope:   instrumentation.Scope{Name: instrumentationName, Version: SemVersion()},
		manager: metricproducer.GlobalManager(),
	}
}

// Produce gathers all metrics from the OpenCensus in-memory state.
func (p *producer) Produce(context.Context) (metricdata.ResourceMetrics, error) {
	producers := p.manager.GetAll()
	data := []*ocmetricdata.Metric{}
	for _, ocProducer := range producers {
		data = append(data, ocProducer.Read()...)
	}
	otelMetrics, err := internal.ConvertMetrics(data)
	return metricdata.ResourceMetrics{
		Resource: p.res,
		ScopeMetrics: []metricdata.ScopeMetrics{
			{
				Scope:   p.scope,
				Metrics: otelMetrics,
			},
		},
	}, err
}
