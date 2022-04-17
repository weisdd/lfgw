package main

import (
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/weisdd/lfgw/internal/acl"
)

func TestApplication_modifyMetricExpr(t *testing.T) {
	logger := zerolog.New(nil)

	newACLPlain := acl.ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "default",
			IsRegexp:   false,
			IsNegative: false,
		},
		RawACL: "default",
	}

	newACLPositiveRegexp := acl.ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|stolon",
			IsRegexp:   true,
			IsNegative: false,
		},
		RawACL: "min.*, stolon",
	}

	// Technically, it's not really possible to create such ACL, but better to keep an eye on it anyway
	newACLNegativeRegexp := acl.ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "min.*|stolon",
			IsRegexp:   true,
			IsNegative: true,
		},
		RawACL: "min.*, stolon",
	}

	tests := []struct {
		name                string
		query               string
		EnableDeduplication bool
		acl                 acl.ACL
		want                string
	}{
		{
			name:                "Complex example, Non-Regexp, no label; append",
			query:               `(histogram_quantile(0.9, rate (request_duration{job="demo"}[5m])) > 0.05 and rate(demo_api_request_duration_seconds_count{job="demo"}[5m]) > 1)`,
			EnableDeduplication: false,
			acl:                 newACLPlain,
			want:                `(histogram_quantile(0.9, rate(request_duration{job="demo", namespace="default"}[5m])) > 0.05) and (rate(demo_api_request_duration_seconds_count{job="demo", namespace="default"}[5m]) > 1)`,
		},
		{
			name:                "Non-Regexp, no label; append",
			query:               `request_duration{job="demo"}`,
			EnableDeduplication: false,
			acl:                 newACLPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Non-Regexp, same label name; replace",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			acl:                 newACLPlain,
			want:                `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:                "Regexp, negative; append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			acl:                 newACLNegativeRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace!~"min.*|stolon"}`,
		},
		{
			name:                "Regexp, negative; merge",
			query:               `request_duration{job="demo", namespace!~"other.*"}`,
			EnableDeduplication: false,
			acl:                 newACLNegativeRegexp,
			want:                `request_duration{job="demo", namespace!~"other.*|min.*|stolon"}`,
		},
		{
			name:                "Regexp, positive; append",
			query:               `request_duration{job="demo", namespace="other"}`,
			EnableDeduplication: false,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace="other", namespace=~"min.*|stolon"}`,
		},
		{
			name:                "Regexp, positive; replace",
			query:               `request_duration{job="demo", namespace=~"other.*"}`,
			EnableDeduplication: false,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace=~"min.*|stolon"}`,
		},
		{
			name:                "Regexp, positive; append (not deduplicated)",
			query:               `request_duration{job="demo", namespace="default"}`,
			EnableDeduplication: true,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{job="demo", namespace="default", namespace=~"min.*|stolon"}`,
		},
		// Examples from readme, deduplication is enabled
		{
			name:                "Original filter is a non-regexp, matches policy (deduplicated)",
			query:               `request_duration{namespace="minio"}`,
			EnableDeduplication: true,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace="minio"}`,
		},
		{
			name:                "Original filter is a fake regexp (deduplicated)",
			query:               `request_duration{namespace=~"minio"}`,
			EnableDeduplication: true,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace=~"minio"}`,
		},
		{
			name:                "Original filter is a subfilter of the policy (deduplicated)",
			query:               `request_duration{namespace=~"min.*"}`,
			EnableDeduplication: true,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace=~"min.*"}`,
		},
		// Same examples, deduplication is disabled
		{
			name:                "Original filter is a non-regexp, matches policy, but deduplication is disabled; append",
			query:               `request_duration{namespace="minio"}`,
			EnableDeduplication: false,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace="minio", namespace=~"min.*|stolon"}`,
		},
		{
			name:                "Original filter is a fake regexp, but deduplication is disabled; append",
			query:               `request_duration{namespace=~"minio"}`,
			EnableDeduplication: false,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace=~"min.*|stolon"}`,
		},
		{
			name:                "Original filter is a subfilter of the policy, but deduplication is disabled; replace",
			query:               `request_duration{namespace=~"min.*"}`,
			EnableDeduplication: false,
			acl:                 newACLPositiveRegexp,
			want:                `request_duration{namespace=~"min.*|stolon"}`,
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

			newExpr := app.modifyMetricExpr(expr, tt.acl)
			assert.Equal(t, originalExpr, expr, "The original expression got modified. Use metricsql.Clone() before modifying any expression.")

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

	t.Run("Not a regexp", func(t *testing.T) {
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
			IsRegexp:   true,
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

		acl := acl.ACL{
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

		acl := acl.ACL{
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
		acl := acl.ACL{
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
		acl := acl.ACL{
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
		acl := acl.ACL{
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
		acl := acl.ACL{
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
		acl := acl.ACL{
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
		acl := acl.ACL{
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

	t.Run("Repeating regexp filters (not subfilters)", func(t *testing.T) {
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

		acl := acl.ACL{
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

	t.Run("Multiple regexp filters, one of which is not a subfilter of the new filter", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "contro.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := acl.ACL{
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
		assert.Equal(t, want, got, "Original expression should be modified, because the original filters are regexps, one of which is not a subfilter of the new filter")
	})

	t.Run("Mix of a non-matching regexp and a non-regexp filters", func(t *testing.T) {
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

		acl := acl.ACL{
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
		assert.Equal(t, want, got, "Original expression should be modified, because amongst the original filters with the same label (regexp, non-regexp) there is a regexp, which is not a subfilter of the new filter")
	})

	// Matching cases

	t.Run("Original filter is not a regexp, new filter matches", func(t *testing.T) {
		acl := acl.ACL{
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

		acl := acl.ACL{
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

	t.Run("Original filter is a regexp and a subfilter of the new ACL", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := acl.ACL{
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

	t.Run("Multiple regexp filters, new filter matches (subfilters)", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			{
				Label:      "namespace",
				Value:      "control.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		acl := acl.ACL{
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
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filters are subfilters of the new filter")
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

		acl := acl.ACL{
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
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the original filter contains the same non-regexp label filter multiple times and the new filter matches")
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

		acl := acl.ACL{
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

		acl := acl.ACL{
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
		acl := acl.ACL{
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
}
