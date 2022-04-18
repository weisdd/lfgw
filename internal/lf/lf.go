package lf

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/weisdd/lfgw/internal/acl"
)

type QueryModifier struct {
	ACL                 acl.ACL
	EnableDeduplication bool
	OptimizeExpressions bool
}

// GetModifiedEncodedURLValues rewrites GET/POST "query" and "match" parameters to filter out metrics.
func (qm *QueryModifier) GetModifiedEncodedURLValues(params url.Values) (string, error) {
	newParams := url.Values{}

	// TODO: rewrite?
	for k, vv := range params {
		switch k {
		case "query", "match[]":
			for _, v := range vv {
				{
					expr, err := metricsql.Parse(v)
					if err != nil {
						return "", err
					}

					expr = qm.modifyMetricExpr(expr)
					if qm.OptimizeExpressions {
						expr = metricsql.Optimize(expr)
					}

					newVal := string(expr.AppendString(nil))
					newParams.Add(k, newVal)
				}
			}
		default:
			for _, v := range vv {
				newParams.Add(k, v)
			}
		}
	}

	return newParams.Encode(), nil
}

// modifyMetricExpr walks through the query and modifies only metricsql.Expr based on the supplied acl with label filter.
func (qm *QueryModifier) modifyMetricExpr(expr metricsql.Expr) metricsql.Expr {
	newExpr := metricsql.Clone(expr)

	// We cannot pass any extra parameters, so we need to use a closure
	// to say which label filter to add
	modifyLabelFilter := func(expr metricsql.Expr) {
		if me, ok := expr.(*metricsql.MetricExpr); ok {
			if qm.ACL.LabelFilter.IsRegexp {
				if !qm.EnableDeduplication || !qm.shouldNotBeModified(me.LabelFilters) {
					me.LabelFilters = appendOrMergeRegexpLF(me.LabelFilters, qm.ACL.LabelFilter)
				}
			} else {
				me.LabelFilters = replaceLFByName(me.LabelFilters, qm.ACL.LabelFilter)
			}
		}
	}

	// Update label filters
	metricsql.VisitAll(newExpr, modifyLabelFilter)

	return newExpr
}

// TODO: simplify description
// shouldNotBeModified helps to understand whether the original label filters have to be modified. The function returns false if any of the original filters do not match expectations described further. It returns true if [the list of original filters contains either a fake positive regexp (no special symbols, e.g. namespace=~"kube-system") or a non-regexp filter] and [acl.LabelFilter is a matching positive regexp]. Also, if original filter is a subfilter of the new filter or has the same value; if acl gives full access. Target label is taken from the acl.LabelFilter.
func (qm *QueryModifier) shouldNotBeModified(filters []metricsql.LabelFilter) bool {
	if qm.ACL.Fullaccess {
		return true
	}

	seen := 0
	seenUnmodified := 0

	// TODO: move to a map? Might not be worth doing as filters of the same type are unlikely
	rawSubACLs := strings.Split(qm.ACL.RawACL, ", ")
	newLF := qm.ACL.LabelFilter

	for _, filter := range filters {
		if filter.Label == newLF.Label && newLF.IsRegexp && !newLF.IsNegative {
			seen++

			// Target: non-regexps or fake regexps
			if !filter.IsRegexp || isFakePositiveRegexp(filter) {
				// Prometheus treats all regexp queries as anchored, whereas our raw regexp doesn't have them. So, we should take anchored values.
				re, err := metricsql.CompileRegexpAnchored(newLF.Value)
				// There shouldn't be any errors, though, just in case, better to skip deduplication
				if err == nil && re.MatchString(filter.Value) {
					seenUnmodified++
					continue
				}
			}

			// Target: both are positive regexps, filter is a subfilter of the newLF or has the same value
			if filter.IsRegexp && !filter.IsNegative {
				for _, rawSubACL := range rawSubACLs {
					if filter.Value == rawSubACL {
						seenUnmodified++
						continue
					}
				}
			}
		}
	}

	return seen > 0 && seen == seenUnmodified
}

// appendOrMergeRegexpLF appends label filter or merges its value in case it's a regexp with the same label name and of the same type (negative / positive).
func appendOrMergeRegexpLF(filters []metricsql.LabelFilter, newFilter metricsql.LabelFilter) []metricsql.LabelFilter {
	newFilters := make([]metricsql.LabelFilter, 0, cap(filters)+1)

	// In case we merge original filter value with newFilter, we'd like to skip adding newFilter to the resulting set.
	skipAddingNewFilter := false

	for _, filter := range filters {
		// Inspect label filters with the target name
		if filter.Label == newFilter.Label {
			// Inspect regexp filters of the same type (negative, positive)
			if filter.IsRegexp && newFilter.IsRegexp && filter.IsNegative == newFilter.IsNegative {
				skipAddingNewFilter = true
				// Merge only negative regexps, because merge for positive regexp will expose data
				if filter.Value != "" && filter.IsNegative {
					filter.Value = fmt.Sprintf("%s|%s", filter.Value, newFilter.Value)
				} else {
					filter.Value = newFilter.Value
				}
			}
		}
		newFilters = append(newFilters, filter)
	}

	if !skipAddingNewFilter {
		newFilters = append(newFilters, newFilter)
	}
	return newFilters
}

// replaceLFByName drops all label filters with the matching name and then appends the supplied filter.
func replaceLFByName(filters []metricsql.LabelFilter, newFilter metricsql.LabelFilter) []metricsql.LabelFilter {
	newFilters := make([]metricsql.LabelFilter, 0, cap(filters)+1)

	// Drop all label filters with the matching name
	for _, filter := range filters {
		if filter.Label != newFilter.Label {
			newFilters = append(newFilters, filter)
		}
	}

	newFilters = append(newFilters, newFilter)
	return newFilters
}

// isFakePositiveRegexp returns true if the given filter is a positive regexp that doesn't contain special symbols, e.g. namespace=~"kube-system"
func isFakePositiveRegexp(filter metricsql.LabelFilter) bool {
	if filter.IsRegexp && !filter.IsNegative {
		if !strings.ContainsAny(filter.Value, acl.RegexpSymbols) {
			return true
		}
	}

	return false
}
