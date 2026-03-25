// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package x // import "go.opentelemetry.io/otel/metric/internal/x"

// ExperimentalOption is an interface that can be used to signal to API and SDK
// configuration that an option is defined in an experimental module.
type ExperimentalOption interface {
	experimentalOption()
}
