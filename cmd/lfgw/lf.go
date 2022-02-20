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
func (app *application) appendOrMergeRegexpLF(filters []metricsql.LabelFilter, newFilter metricsql.LabelFilter) []metricsql.LabelFilter {
	newFilters := make([]metricsql.LabelFilter, 0, cap(filters)+1)

	// Helps to determine whether we found a similar regexp (only with
	// different value)
	foundMatch := false

	for _, filter := range filters {
		// Merge label filter's value
		if filter.Label == newFilter.Label && filter.IsRegexp && filter.IsNegative == newFilter.IsNegative {
			foundMatch = true
			// Merge only negative regexps, because merge for positive regexp will expose data
			if filter.Value != "" && filter.IsNegative {
				filter.Value = fmt.Sprintf("%s|%s", filter.Value, newFilter.Value)
			} else {
				filter.Value = newFilter.Value
			}
		}
		newFilters = append(newFilters, filter)
	}

	if !foundMatch {
		newFilters = append(newFilters, newFilter)
	}
	return newFilters
}

// modifyMetricExpr walks through the query and modifies only metricsql.Expr based on the supplied label filter.
func (app *application) modifyMetricExpr(query string, newFilter metricsql.LabelFilter) (string, error) {
	expr, err := metricsql.Parse(query)
	if err != nil {
		return "", err
	}
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
	metricsql.VisitAll(expr, modifyLabelFilter)

	app.debugLog.Printf("Rewrote query %s to query %s", query, expr.AppendString(nil))

	return string(expr.AppendString(nil)), nil
}

// optimizeMetricExpr optimizes expressions. More details: https://pkg.go.dev/github.com/VictoriaMetrics/metricsql#Optimize
func (app *application) optimizeMetricExpr(query string) (string, error) {
	if !app.OptimizeExpressions {
		return query, nil
	}

	expr, err := metricsql.Parse(query)
	if err != nil {
		return "", err
	}

	newExpr := metricsql.Optimize(expr)

	if !app.equalExpr(expr, newExpr) {
		app.debugLog.Printf("Optimized query %s to query %s", query, newExpr.AppendString(nil))
	}

	return string(newExpr.AppendString(nil)), nil
}

// equalExpr says whether two expressions are equal
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
					newVal, err := app.modifyMetricExpr(v, lf)
					if err != nil {
						return "", err
					}

					newVal, err = app.optimizeMetricExpr(newVal)
					if err != nil {
						return "", err
					}

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
