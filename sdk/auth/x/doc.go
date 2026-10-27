// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package x provides experimental exporter authentication contracts and OTLP
// exporter options for acquiring, caching, refreshing, and attaching dynamic
// client credentials to outgoing telemetry export requests.
//
// While the OpenTelemetry Protocol (OTLP) exporters are the primary built-in
// consumers of [Authenticator], this package is decoupled from OTLP protocol
// packages so that contrib authentication plugins and non-OTLP HTTP or gRPC
// exporters can share a single contract.
//
// These types are currently under development and not part of the stable v1 API.
package x // import "go.opentelemetry.io/otel/sdk/auth/x"
