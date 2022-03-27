package main

import (
	"fmt"
	"net/url"

	"github.com/VictoriaMetrics/metricsql"
)

// replaceLFByName drops all label filters with the matching name and then appends the supplied filter.
func (app *application) replaceLFByName(filters []metricsql.LabelFilter, newFilter metricsql.LabelFilter) []metricsql.LabelFilter {
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

// appendOrMergeRegexpLF appends label filter or merges its value in case it's a regexp with the same name
// and of the same type (negative / positive).
// appendOrMergeRegexpLF appends label filter or merges its value in case it's a regexp with the same name and of the same type (negative / positive).
func (app *application) appendOrMergeRegexpLF(filters []metricsql.LabelFilter, newFilter metricsql.LabelFilter) []metricsql.LabelFilter {
	newFilters := make([]metricsql.LabelFilter, 0, cap(filters)+1)

	// In case we merge original filter value with newFilter, we'd like to skip adding newFilter to the resulting set.
	skipAddingNewFilter := false

	for _, filter := range filters {
		// Inspect label filters with the targe name
		if filter.Label == newFilter.Label {
			// Inspect regexp filters of the same type (negative, positive)
			if filter.IsRegexp && filter.IsNegative == newFilter.IsNegative {
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

// modifyMetricExpr walks through the query and modifies only metricsql.Expr based on the supplied label filter.
func (app *application) modifyMetricExpr(expr metricsql.Expr, newFilter metricsql.LabelFilter) metricsql.Expr {
	newExpr := metricsql.Clone(expr)

	// We cannot pass any extra parameters, so we need to use a closure
	// to say which label filter to add
	modifyLabelFilter := func(expr metricsql.Expr) {
		if me, ok := expr.(*metricsql.MetricExpr); ok {
			if newFilter.IsRegexp {
				me.LabelFilters = app.appendOrMergeRegexpLF(me.LabelFilters, newFilter)
			} else {
				me.LabelFilters = app.replaceLFByName(me.LabelFilters, newFilter)
			}
		}
	}

	// Update label filters
	metricsql.VisitAll(newExpr, modifyLabelFilter)

	// TODO: log somehow?
	// app.logger.Debug().Caller().
	// 	Msgf("Rewrote query %s to query %s", expr.AppendString(nil), newExpr.AppendString(nil))

	return newExpr
}

// optimizeMetricExpr optimizes expressions. More details: https://pkg.go.dev/github.com/VictoriaMetrics/metricsql#Optimize
func (app *application) optimizeMetricExpr(expr metricsql.Expr) metricsql.Expr {
	newExpr := metricsql.Optimize(expr)

	// TODO: log somehow?
	// if !app.equalExpr(expr, newExpr) {
	// 	app.logger.Debug().Caller().
	// 		Msgf("Optimized query %s to query %s", expr.AppendString(nil), newExpr.AppendString(nil))
	// }

	return newExpr
}

// equalExpr says whether two expressions are equal.
func (app *application) equalExpr(expr1 metricsql.Expr, expr2 metricsql.Expr) bool {
	return string(expr1.AppendString(nil)) == string(expr2.AppendString(nil))
}

// prepareQueryParams rewrites GET/POST "query" and "match" parameters to filter out metrics.
func (app *application) prepareQueryParams(params *url.Values, lf metricsql.LabelFilter) (string, error) {
	newParams := &url.Values{}

	for k, vv := range *params {
		switch k {
		case "query", "match[]":
			for _, v := range vv {
				{
					expr, err := metricsql.Parse(v)
					if err != nil {
						return "", err
					}

					expr = app.modifyMetricExpr(expr, lf)
					if app.OptimizeExpressions {
						expr = app.optimizeMetricExpr(expr)
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
