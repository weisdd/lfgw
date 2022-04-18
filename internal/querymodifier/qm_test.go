package querymodifier

import (
	"net/url"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/stretchr/testify/assert"
)

func TestQueryModifier_GetModifiedEncodedURLValues(t *testing.T) {
	// TODO: test that the original URL.Values haven't changed
	t.Run("No matching parameters", func(t *testing.T) {
		params := url.Values{
			"random": []string{"randomvalue"},
		}

		acl, err := NewACL("minio")
		if err != nil {
			t.Fatal(err)
		}

		qm := QueryModifier{
			ACL:                 acl,
			EnableDeduplication: false,
			OptimizeExpressions: false,
		}

		want := params.Encode()
		got, err := qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("query and match[]", func(t *testing.T) {
		query := `request_duration{job="demo", namespace="other"}`

		params := url.Values{
			"query":   []string{query},
			"match[]": []string{query},
		}

		newQuery := `request_duration{job="demo", namespace="minio"}`
		newParams := url.Values{
			"query":   []string{newQuery},
			"match[]": []string{newQuery},
		}

		acl, err := NewACL("minio")
		if err != nil {
			t.Fatal(err)
		}

		qm := QueryModifier{
			ACL:                 acl,
			EnableDeduplication: false,
			OptimizeExpressions: false,
		}
		want := newParams.Encode()
		got, err := qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Deduplicate", func(t *testing.T) {
		query := `request_duration{job="demo", namespace=~"minio"}`

		params := url.Values{
			"query":   []string{query},
			"match[]": []string{query},
		}

		newQueryDeduplicated := `request_duration{job="demo", namespace=~"minio"}`
		newParamsDeduplicated := url.Values{
			"query":   []string{newQueryDeduplicated},
			"match[]": []string{newQueryDeduplicated},
		}

		newQueryNotDeduplicated := `request_duration{job="demo", namespace=~"mini.*"}`
		newParamsNotDeduplicated := url.Values{
			"query":   []string{newQueryNotDeduplicated},
			"match[]": []string{newQueryNotDeduplicated},
		}

		acl, err := NewACL("mini.*")
		if err != nil {
			t.Fatal(err)
		}

		qm := QueryModifier{
			ACL:                 acl,
			EnableDeduplication: false,
			OptimizeExpressions: false,
		}

		qm.EnableDeduplication = true
		want := newParamsDeduplicated.Encode()
		got, err := qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)

		qm.EnableDeduplication = false
		want = newParamsNotDeduplicated.Encode()
		got, err = qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Optimize", func(t *testing.T) {
		// Example is taken from https://github.com/VictoriaMetrics/metricsql/blob/50340b1c7e599295deafc510f5cb833de0669c20/optimizer_test.go#L149
		query := `foo AND bar{baz="aa"}`

		params := url.Values{
			"query":   []string{query},
			"match[]": []string{query},
		}

		newQueryOptimized := `foo{baz="aa", namespace="minio"} and bar{baz="aa", namespace="minio"}`
		newParamsOptimized := url.Values{
			"query":   []string{newQueryOptimized},
			"match[]": []string{newQueryOptimized},
		}

		newQueryNotOptimized := `foo{namespace="minio"} and bar{baz="aa", namespace="minio"}`
		newParamsNotOptimized := url.Values{
			"query":   []string{newQueryNotOptimized},
			"match[]": []string{newQueryNotOptimized},
		}

		acl, err := NewACL("minio")
		if err != nil {
			t.Fatal(err)
		}

		qm := QueryModifier{
			ACL:                 acl,
			EnableDeduplication: false,
			OptimizeExpressions: false,
		}

		qm.OptimizeExpressions = true
		want := newParamsOptimized.Encode()
		got, err := qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)

		qm.OptimizeExpressions = false
		want = newParamsNotOptimized.Encode()
		got, err = qm.GetModifiedEncodedURLValues(params)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})
}

func TestQueryModifier_modifyMetricExpr(t *testing.T) {
	newACLPlain := ACL{
		Fullaccess: false,
		LabelFilter: metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "default",
			IsRegexp:   false,
			IsNegative: false,
		},
		RawACL: "default",
	}

	newACLPositiveRegexp := ACL{
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
	newACLNegativeRegexp := ACL{
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
		acl                 ACL
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
			qm := QueryModifier{
				ACL:                 tt.acl,
				EnableDeduplication: tt.EnableDeduplication,
				OptimizeExpressions: true,
			}

			expr, err := metricsql.Parse(tt.query)
			if err != nil {
				t.Fatalf("%s", err)
			}
			originalExpr := metricsql.Clone(expr)

			newExpr := qm.modifyMetricExpr(expr)
			assert.Equal(t, originalExpr, expr, "The original expression got modified. Use metricsql.Clone() before modifying any expression.")

			got := string(newExpr.AppendString(nil))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestQueryModifier_shouldNotBeModified(t *testing.T) {
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := false
		got := qm.shouldNotBeModified(filters)
		assert.Equal(t, want, got, "Original expression should be modified, because amongst the original filters with the same label (regexp, non-regexp) there is a regexp, which is not a subfilter of the new filter")
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
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

		qm := QueryModifier{
			ACL: acl,
		}

		want := true
		got := qm.shouldNotBeModified(filters)
		assert.Equal(t, want, got, "Original expression should NOT be modified, because the new filter gives full access")
	})
}

func Test_appendOrMergeRegexpLF(t *testing.T) {
	t.Run("Non-Regexp LF", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label: "namespace",
			Value: "ReplacedValue",
		}

		filters := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialVal.*",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValu.*",
				IsRegexp:   true,
				IsNegative: true,
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialVal.*",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValu.*",
				IsRegexp:   true,
				IsNegative: true,
			},
			// The function doesn't take into account non-regexp LFs, that's why it's added
			{
				Label: "namespace",
				Value: "ReplacedValue",
			},
		}
		got := appendOrMergeRegexpLF(filters, newFilter)
		assert.Equal(t, want, got)
	})

	t.Run("Positive Regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:    "namespace",
			Value:    "ReplacedValue",
			IsRegexp: true,
		}

		filters := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialVal.*",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValu.*",
				IsRegexp:   true,
				IsNegative: true,
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			// Only positive regexp should get modified (replaced)
			{
				Label:    "namespace",
				Value:    "ReplacedValue",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValu.*",
				IsRegexp:   true,
				IsNegative: true,
			},
		}
		got := appendOrMergeRegexpLF(filters, newFilter)
		assert.Equal(t, want, got)
	})

	t.Run("Negative Regexp", func(t *testing.T) {
		newFilter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "MergedValue",
			IsRegexp:   true,
			IsNegative: true,
		}

		filters := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialVal.*",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValu.*",
				IsRegexp:   true,
				IsNegative: true,
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialVal.*",
				IsRegexp: true,
			},
			// Only negative regexp should get modified (Merged)
			{
				Label:      "namespace",
				Value:      "InitialValu.*|MergedValue",
				IsRegexp:   true,
				IsNegative: true,
			},
		}
		got := appendOrMergeRegexpLF(filters, newFilter)
		assert.Equal(t, want, got)
	})

}

func Test_replaceLFByName(t *testing.T) {
	newFilter := metricsql.LabelFilter{
		Label: "namespace",
		Value: "ReplacedValue",
	}

	t.Run("No matching label", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
			{
				Label: "namespace",
				Value: "ReplacedValue",
			},
		}
		got := replaceLFByName(filters, newFilter)
		assert.Equal(t, want, got)
	})

	t.Run("1 matching label", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
			{
				Label: "namespace",
				Value: "InitialValue",
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
			{
				Label: "namespace",
				Value: "ReplacedValue",
			},
		}
		got := replaceLFByName(filters, newFilter)
		assert.Equal(t, want, got)

	})

	t.Run("many matching labels", func(t *testing.T) {
		filters := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
			{
				Label: "namespace",
				Value: "InitialValue",
			},
			{
				Label:    "namespace",
				Value:    "InitialValu.*",
				IsRegexp: true,
			},
			{
				Label:      "namespace",
				Value:      "InitialValue.*",
				IsRegexp:   true,
				IsNegative: true,
			},
		}

		want := []metricsql.LabelFilter{
			{
				Label: "job",
				Value: "InitialValue",
			},
			{
				Label: "namespace",
				Value: "ReplacedValue",
			},
		}
		got := replaceLFByName(filters, newFilter)
		assert.Equal(t, want, got)
	})
}

func Test_isFakePositiveRegexp(t *testing.T) {
	t.Run("Not a regexp", func(t *testing.T) {
		filter := metricsql.LabelFilter{
			Label:      "namespace",
			Value:      "minio",
			IsRegexp:   false,
			IsNegative: false,
		}

		want := false
		got := isFakePositiveRegexp(filter)
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
		got := isFakePositiveRegexp(filter)
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
		got := isFakePositiveRegexp(filter)
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
		got := isFakePositiveRegexp(filter)
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
		got := isFakePositiveRegexp(filter)
		assert.Equal(t, want, got)
	})
}
