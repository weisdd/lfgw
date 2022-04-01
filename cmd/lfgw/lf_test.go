package main

import (
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestApplication_modifyMetricExpr(t *testing.T) {
	logger := zerolog.New(nil)

	newACLPlain := ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label: "namespace",
			Value: "default",
		},
		RawACL: "default",
	}

	newACLPositiveRegexp := ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "kube.*|control.*",
			IsRegexp:   true,
			IsNegative: false,
		},
		RawACL: "kube.*, control.*",
	}

	// Technically, it's not really possible to create such ACL, but better to keep an eye on it anyway
	newACLNegativeRegexp := ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "kube.*|control.*",
			IsRegexp:   true,
			IsNegative: true,
		},
		RawACL: "kube.*, control.*",
	}

	tests := []struct {
		name                string
		query               string
		EnableDeduplication bool
		newFilter           ACL
		want                string
	}{
		{
			name:                "Complex example, Non-Regexp, no label",
			query:               `(histogram_quantile(0.9, rate (request_duration{job="demo"}[5m])) > 0.05 and rate(demo_api_request_duration_seconds_count{job="demo"}[5m]) > 1)`,
			EnableDeduplication: false,
			newFilter:           newACLPlain,
			want:                `(histogram_quantile(0.9, rate(request_duration{job="demo", namespace="default"}[5m])) > 0.05) and (rate(demo_api_request_duration_seconds_count{job="demo", namespace="default"}[5m]) > 1)`,
		},
		{
			name:                "Non-Regexp, no label, append",
			query:               `request_duration{job="demo"}`,
			EnableDeduplication: false,
			newFilter:           newACLPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Non-Regexp, same label name, replace",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newACLPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Regexp, negative, append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newACLNegativeRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace!~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, negative, merge",
			query:               `request_duration{job="demo", namespace!~"other.*"}`,
			EnableDeduplication: false,
			newFilter:           newACLNegativeRegexp,
			want:                `request_duration{job="demo", namespace!~"other.*|kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			newFilter:           newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace=~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, replace",
			query:               `request_duration{job="demo", namespace=~"other.*"}`,
			EnableDeduplication: false,
			newFilter:           newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace=~"kube.*|control.*"}`,
		},
		{
			name:                "Regexp, positive, no changes (deduplicated)",
			query:               `request_duration{job="demo", namespace="kube-system"}`,
			EnableDeduplication: true,
			newFilter:           newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace="kube-system"}`,
		},
		{
			name:                "Regexp, positive, append (not deduplicated)",
			query:               `request_duration{job="demo", namespace="default"}`,
			EnableDeduplication: true,
			newFilter:           newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace="default", namespace=~"kube.*|control.*"}`,
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
			assert.Equal(t, tt.want, got)
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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*, control.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters do not contain the target label")
	})

	t.Run("Original filter is a regexp and not a subfilter of the new ACL", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "mini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*, control.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filter is a regexp and not a subfilter of the new ACL")
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
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label: "namespace",
				Value: "default",
			},
			RawACL: "default",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative non-matching complex regexp", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "kube.*|control.*",
				IsRegexp:   true,
				IsNegative: true,
			},
			RawACL: "kube.*, control.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative non-matching simple regexp", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "ini.*",
				IsRegexp:   true,
				IsNegative: true,
			},
			RawACL: "ini.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Negative matching complex regexp", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: true,
			},
			RawACL: "min.*, control.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter is not a matching positive regexp")
	})

	t.Run("Positive non-matching complex regexp", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "kube.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "kube.*, control.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter doesn't match original filter")
	})

	t.Run("Positive non-matching simple regexp", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "ini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "ini.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the new filter doesn't match original filter")
	})

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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "mini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "mini.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters contain regexp filters, which are not subfilters of the new filter")
	})

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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "mini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "mini.*",
		}

		want := false
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters contain the same label multiple times (regexp, non-regexp), and the original regexp is not a subfilter of the new filter")
	})

	// Matching cases

	t.Run("Original filter is not a regexp, new filter matches", func(t *testing.T) {
		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*, control.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*, control.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter is a fake positive regexp (it doesn't contain any special characters, should have been a non-regexp expression, e.g. namespace=~\"kube-system\") and the new filter is a matching positive regexp")
	})

	t.Run("Repeating filters, new filter matches", func(t *testing.T) {
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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "mini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "mini.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter contains the same non-regexp label multiple times and the new filter matches")
	})

	t.Run("Original filters are a mix of a fake regexp and a non-regexp filters and the new filter matches", func(t *testing.T) {
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

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "mini.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "mini.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because original filters contain a mix of a fake regexp and a non-regexp filters (basically, they're equal in results)")
	})

	t.Run("Original filter and the new filter contain the same regexp", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because original filter and the new filter contain the same regexp")
	})

	t.Run("The new filter gives full access", func(t *testing.T) {
		acl := ACL{
			Fullaccess: true,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: ".*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the new filter gives full access")
	})

	t.Run("Original filter is a regexp and a subfilter of the new ACL", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "min.*, control.*",
		}

		want := true
		got := app.shouldNotBeModified(filters, acl)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter is a regexp subfilter of the ACL")
	})
}
