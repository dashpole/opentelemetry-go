// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"errors"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/sdk/instrumentation"
)

var (
	errMultiInst = errors.New("name replacement for multiple instruments")
	errEmptyView = errors.New("no criteria provided for view")

	emptyView = func(Instrument) (Stream, bool) { return Stream{}, false }
)

// View is an override to the default behavior of the SDK. It defines how data
// should be collected for certain instruments. It returns true and the exact
// Stream to use for matching Instruments. Otherwise, if the view does not
// match, false is returned.
//
// Note that attributes filtered out by a View may still appear on Exemplars,
// because Exemplars are recorded with the dropped measurement attributes
// when View attribute filtering is applied.
type View func(Instrument) (Stream, bool)

// NewView returns a View that applies the Stream mask for all instruments that
// match criteria. The returned View will only apply mask if all non-zero-value
// fields of criteria match the corresponding Instrument passed to the view. If
// no criteria are provided, all field of criteria are their zero-values, a
// view that matches no instruments is returned. If you need to match a
// zero-value field, create a View directly.
//
// The Name field of criteria supports wildcard pattern matching. The "*"
// wildcard is recognized as matching zero or more characters, and "?" is
// recognized as matching exactly one character. For example, a pattern of "*"
// matches all instrument names.
//
// The Stream mask only applies updates for non-zero-value fields. By default,
// the Instrument the View matches against will be use for the Name,
// Description, and Unit of the returned Stream and no Aggregation or
// AttributeFilter are set. All non-zero-value fields of mask are used instead
// of the default. If you need to zero out a Stream field returned from a
// View, create a View directly.
func NewView(criteria Instrument, mask Stream) View {
	if criteria.IsEmpty() {
		global.Error(
			errEmptyView, "dropping view",
			"mask", mask,
		)
		return emptyView
	}

	var matchFunc func(Instrument) bool
	if strings.ContainsAny(criteria.Name, "*?") {
		if mask.Name != "" {
			global.Error(
				errMultiInst, "dropping view",
				"criteria", criteria,
				"mask", mask,
			)
			return emptyView
		}

		// Handle branching here in NewView instead of criteria.matches so
		// criteria.matches remains inlinable for the simple case.
		pattern := regexp.QuoteMeta(criteria.Name)
		pattern = "^" + pattern + "$"
		pattern = strings.ReplaceAll(pattern, `\?`, ".")
		pattern = strings.ReplaceAll(pattern, `\*`, ".*")
		re := regexp.MustCompile(pattern)
		matchFunc = func(i Instrument) bool {
			return re.MatchString(i.Name) &&
				criteria.matchesDescription(i) &&
				criteria.matchesKind(i) &&
				criteria.matchesUnit(i) &&
				criteria.matchesScope(i)
		}
	} else {
		matchFunc = criteria.matches
	}

	var agg Aggregation
	if mask.Aggregation != nil {
		agg = mask.Aggregation.copy()
		if err := agg.err(); err != nil {
			global.Error(
				err, "not using aggregation with view",
				"criteria", criteria,
				"mask", mask,
			)
			agg = nil
		}
	}

	return func(i Instrument) (Stream, bool) {
		if matchFunc(i) || (i.Kind == instrumentKindScope && criteria.matchesScope(i)) {
			return Stream{
				Name:                              nonZero(mask.Name, i.Name),
				Description:                       nonZero(mask.Description, i.Description),
				Unit:                              nonZero(mask.Unit, i.Unit),
				Aggregation:                       agg,
				AttributeFilter:                   mask.AttributeFilter,
				ExemplarReservoirProviderSelector: mask.ExemplarReservoirProviderSelector,
				enabled:                           mask.enabled,
			}, true
		}
		return Stream{}, false
	}
}

// nonZero returns v if it is non-zero-valued, otherwise alt.
func nonZero[T comparable](v, alt T) T {
	var zero T
	if v != zero {
		return v
	}
	return alt
}

// includeFilters returns the allow-list AttributeFilters of the active Views
// of p that match scope.
//
// Each View is called with an Instrument with an undefined Kind and only scope
// set. A View created by NewView matches it if its criteria's Scope matches,
// regardless of its other criteria. An AttributeFilter is an allow-list if it
// drops the empty (invalid) key.
func (p *pipeline) includeFilters(scope instrumentation.Scope) []attribute.Filter {
	probe := Instrument{Kind: instrumentKindScope, Scope: scope}
	empty := attribute.Key("").String("")
	var filters []attribute.Filter
	for _, v := range p.views {
		s, ok := v(probe)
		if !ok || s.AttributeFilter == nil || s.AttributeFilter(empty) || (s.enabled != nil && !*s.enabled) {
			continue
		}
		if _, drop := s.Aggregation.(AggregationDrop); drop {
			continue
		}
		filters = append(filters, s.AttributeFilter)
	}
	return filters
}
