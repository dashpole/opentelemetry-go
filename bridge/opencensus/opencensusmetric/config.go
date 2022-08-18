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
	"go.opentelemetry.io/otel/sdk/resource"
)

// Config contains configuration options for a Producer.
type Config struct {
	res *resource.Resource
}

// Option applies an option to a Config.
type Option interface {
	apply(Config) Config
}

type optionFunc func(Config) Config

func (fn optionFunc) apply(cfg Config) Config {
	return fn(cfg)
}

// WithResource sets the resource for the bridge.
func WithResource(res *resource.Resource) Option {
	return optionFunc(func(cfg Config) Config {
		cfg.res = res
		return cfg
	})
}

// newConfig applies all the options to a returned Config.
func newConfig(options []Option) Config {
	var cfg Config
	for _, option := range options {
		cfg = option.apply(cfg)
	}
	if cfg.res == nil {
		cfg.res = resource.Default()
	}
	return cfg
}
