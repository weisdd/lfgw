package main

import (
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestApplication_modifyMetricExpr(t *testing.T) {
	logger := zerolog.New(nil)

	newFilterPlain := metricsql.LabelFilter{
		Label: "namespace",
		Value: "default",
	}
	newFilterPositiveRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "kube.*|control.*",
		IsRegexp:   true,
		IsNegative: false,
	}
	newFilterNegativeRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "kube.*|control.*",
		IsRegexp:   true,
		IsNegative: true,
	}

	tests := []struct {
		name                string
		query               string
		EnableDeduplication bool
		newFilter           metricsql.LabelFilter
		want                string
	}{
		{
			name:                "Complex example, Non-Regexp, no label",
			query:               `(histogram_quantile(0.9, rate (request_duration{job="demo"}[5m])) > 0.05 and rate(demo_api_request_duration_seconds_count{job="demo"}[5m]) > 1)`,
			EnableDeduplication: false,
			newFilter:           newFilterPlain,
			want:                `(histogram_quantile(0.9, rate(request_duration{job="demo", namespace="default"}[5m])) > 0.05) and (rate(demo_api_request_duration_seconds_count{job="demo", namespace="default"}[5m]) > 1)`,
		},
		{
			name:                "Non-Regexp, no label",
			query:               `request_duration{job="demo"}`,
			EnableDeduplication: false,
			newFilter:           newFilterPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Non-Regexp, same label name",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newFilterPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Regexp, negative, append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newFilterNegativeRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace!~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, negative, merge",
			query:               `request_duration{job="demo", namespace!~"other.*"}`,
			EnableDeduplication: false,
			newFilter:           newFilterNegativeRegexp,
			want:                `request_duration{job="demo", namespace!~"other.*|kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newFilterPositiveRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace=~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, replace",
			query:               `request_duration{job="demo", namespace=~"other.*"}`,
			EnableDeduplication: false,
			newFilter:           newFilterPositiveRegexp,
			want:                `request_duration{job="demo", namespace=~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, append",
			query:               `request_duration{job="demo", namespace=~"other.*"}`,
			EnableDeduplication: false,
			newFilter:           newFilterPositiveRegexp,
			want:                `request_duration{job="demo", namespace=~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, no changes (deduplicated)",
			query:               `request_duration{job="demo", namespace="kube-system"}`,
			EnableDeduplication: true,
			newFilter:           newFilterPositiveRegexp,
			want:                `request_duration{job="demo", namespace="kube-system"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &application{
				logger:              &logger,
				EnableDeduplication: tt.EnableDeduplication,
			}

			expr, err := metricsql.Parse(tt.query)
			if err != nil {
				t.Fatalf("%s", err)
			}
			originalExpr := metricsql.Clone(expr)

			newExpr := app.modifyMetricExpr(expr, tt.newFilter)
			if !app.equalExpr(originalExpr, expr) {
				t.Errorf("%s: The original expression got modified. Use metricsql.Clone() before modifying any expression.", tt.name)
			}

			got := string(newExpr.AppendString(nil))

			if got != tt.want {
				t.Errorf("want %s; got %s", tt.want, got)
			}
		})
	}
}

func TestShouldBeModified(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	filters := []metricsql.LabelFilter{
		{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: false,
		},
	}

	filtersRegexp := []metricsql.LabelFilter{
		{
			Label:      "namespace",
			Value:      "min.*",
			IsRegexp:   true,
			IsNegative: false,
		},
	}

	filtersDifferentLabel := []metricsql.LabelFilter{
		{
			Label:      "pod",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: false,
		},
	}

	newFilterPlain := metricsql.LabelFilter{
		Label: "namespace",
		Value: "default",
	}

	got := app.shouldBeModified(filters, newFilterPlain)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter is not a matching positive regexp")

	newFilterNegativeNonMatchingComplexRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "kube.*|control.*",
		IsRegexp:   true,
		IsNegative: true,
	}

	got = app.shouldBeModified(filters, newFilterNegativeNonMatchingComplexRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter is not a matching positive regexp")

	newFilterNegativeNonMatchingSimpleRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "ini.*",
		IsRegexp:   true,
		IsNegative: true,
	}

	got = app.shouldBeModified(filters, newFilterNegativeNonMatchingSimpleRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter is not a matching positive regexp")

	newFilterNegativeMatchingComplexRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "min.*|control.*",
		IsRegexp:   true,
		IsNegative: true,
	}

	got = app.shouldBeModified(filters, newFilterNegativeMatchingComplexRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter is not a matching positive regexp")

	newFilterPositiveNonMatchingComplexRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "kube.*|control.*",
		IsRegexp:   true,
		IsNegative: false,
	}

	got = app.shouldBeModified(filters, newFilterPositiveNonMatchingComplexRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter doesn't match original filter")

	newFilterPositiveNonMatchingSimpleRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "ini.*",
		IsRegexp:   true,
		IsNegative: true,
	}

	got = app.shouldBeModified(filters, newFilterPositiveNonMatchingSimpleRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the new filter doesn't match original filter")

	newFilterPositiveMatchingComplexRegexp := metricsql.LabelFilter{
		Label:      "namespace",
		Value:      "min.*|control.*",
		IsRegexp:   true,
		IsNegative: false,
	}

	got = app.shouldBeModified(filtersDifferentLabel, newFilterPositiveMatchingComplexRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the original filter doesn't contain the target label")

	got = app.shouldBeModified(filtersRegexp, newFilterPositiveMatchingComplexRegexp)
	assert.Equal(t, true, got, "Original expression should be modified, because the original filter is a regexp")

	// The only matching case
	got = app.shouldBeModified(filters, newFilterPositiveMatchingComplexRegexp)
	assert.Equal(t, false, got, "Original expression should NOT be modified, because the original filter is not a regexp and the new filter is a matching positive regexp")
}
