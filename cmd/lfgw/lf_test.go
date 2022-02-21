package main

import (
	"io"
	"log"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func TestApplication_modifyMetricExpr(t *testing.T) {
	app := &application{
		errorLog: log.New(io.Discard, "", 0),
		infoLog:  log.New(io.Discard, "", 0),
		debugLog: log.New(io.Discard, "", 0),
	}

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
		name      string
		query     string
		newFilter metricsql.LabelFilter
		want      string
	}{
		{
			name:      "Complex example, Non-Regexp, no label",
			query:     `(histogram_quantile(0.9, rate (request_duration{job="demo"}[5m])) > 0.05 and rate(demo_api_request_duration_seconds_count{job="demo"}[5m]) > 1)`,
			newFilter: newFilterPlain,
			want:      `(histogram_quantile(0.9, rate(request_duration{job="demo", namespace="default"}[5m])) > 0.05) and (rate(demo_api_request_duration_seconds_count{job="demo", namespace="default"}[5m]) > 1)`,
		},
		{
			name:      "Non-Regexp, no label",
			query:     `request_duration{job="demo"}`,
			newFilter: newFilterPlain,
			want:      `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:      "Non-Regexp, same label name",
			query:     `request_duration{job="demo", namespace="other"}`,
			newFilter: newFilterPlain,
			want:      `request_duration{job="demo", namespace="default"}`,
		},
		{
			name:      "Regexp, negative, append",
			query:     `request_duration{job="demo", namespace="other"}`,
			newFilter: newFilterNegativeRegexp,
			want:      `request_duration{job="demo", namespace="other", namespace!~"kube.*|control.*"}`,
		},
		{
			name:      "Regexp, negative, merge",
			query:     `request_duration{job="demo", namespace!~"other.*"}`,
			newFilter: newFilterNegativeRegexp,
			want:      `request_duration{job="demo", namespace!~"other.*|kube.*|control.*"}`,
		},
		{
			name:      "Regexp, positive, append",
			query:     `request_duration{job="demo", namespace="other"}`,
			newFilter: newFilterPositiveRegexp,
			want:      `request_duration{job="demo", namespace="other", namespace=~"kube.*|control.*"}`,
		},
		{
			name:      "Regexp, positive, replace",
			query:     `request_duration{job="demo", namespace=~"other.*"}`,
			newFilter: newFilterPositiveRegexp,
			want:      `request_duration{job="demo", namespace=~"kube.*|control.*"}`,
		},
		{
			name:      "Regexp, positive, append",
			query:     `request_duration{job="demo", namespace=~"other.*"}`,
			newFilter: newFilterPositiveRegexp,
			want:      `request_duration{job="demo", namespace=~"kube.*|control.*"}`,
		},
		// TODO: more mixed tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := metricsql.Parse(tt.query)
			if err != nil {
				t.Fatalf("%s", err)
			}

			newExpr := app.modifyMetricExpr(expr, tt.newFilter)
			if app.equalExpr(expr, newExpr) {
				t.Error("The original expression got modified. Use metricsql.Clone() before modifying any expression.")
			}

			got := string(newExpr.AppendString(nil))

			if got != tt.want {
				t.Errorf("want %s; got %s", tt.want, got)
			}
		})
	}
}
