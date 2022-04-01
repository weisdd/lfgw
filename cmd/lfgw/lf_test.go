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

func TestApplication_isFakePositiveRegexp(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	t.Run("Not regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: false,
		}

		want := false
		got := app.isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})

	t.Run("Fake positive regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})

	t.Run("Fake negative regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: true,
		}

		want := false
		got := app.isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})

	t.Run("Real positive regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})

	t.Run("Real negative regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*",
			IsRegexp:   true,
			IsNegative: true,
		}

		want := false
		got := app.isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})
}

func TestApplication_shouldNotBeModified(t *testing.T) {
	logger := zerolog.New(nil)
	app := &application{
		logger: &logger,
	}

	t.Run("Original filters do not contain target label", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "pod",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters do not contain the target label")
	})

	t.Run("Original filter is a regexp", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filter is a regexp")
	})

	filters := []metricsql.LabelFilter{
		{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: false,
		},
	}

	t.Run("Not a regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label: "namespace",
			Value: "default",
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative non-matching complex regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "kube.*|control.*",
			IsRegexp:   true,
			IsNegative: true,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative non-matching simple regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "ini.*",
			IsRegexp:   true,
			IsNegative: true,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative matching complex regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|control.*",
			IsRegexp:   true,
			IsNegative: true,
		}
		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Positive non-matching complex regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "kube.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter doesn't match original filter")
	})

	t.Run("Positive non-matching simple regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "ini.*",
			IsRegexp:   true,
			IsNegative: true,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter doesn't match original filter")
	})

	// TODO: adjust test?
	t.Run("Repeating regexp filters", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "mini.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters contain regexp filters")
	})

	// TODO: adjust test?
	t.Run("Mix of regexp and non-regexp filters", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "mini.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := false
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters contain the same label multiple times (regexp, non-regexp)")
	})

	// TODO: move more to matching cases
	// Matching cases

	t.Run("Original filter is not a regexp, new filter matches", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter is not a regexp and the new filter is a matching positive regexp")
	})

	t.Run("Original filter is a fake positive regexp (no special symbols), new filter matches", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter is a fake positive regexp (it doesn't contain any special characters, should have been a non-regexp expression, e.g. namespace=~\"kube-system\") and the new filter is a matching positive regexp")
	})

	t.Run("Repeating filters", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "mini.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter contains the same matching non-regexp label multiple times")
	})

	t.Run("Original filters are a mix of a fake regexp and a non-regexp filters, new filter matches", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "mini.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because original filters contain a mix of a fake regexp and a non-regexp filters (basically, they're equal in results)")
	})

	t.Run("Original filter are a new filter contain the same regexp", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*",
			IsRegexp:   true,
			IsNegative: false,
		}

		want := true
		got := app.shouldNotBeModified(filters, newFilter)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because original filter and a new filter contain the same regexp")
	})
}
