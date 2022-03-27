package main

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func TestACL_ToSlice(t *testing.T) {
	acl := &ACL{false, metricsql.LabelFilter{}, ""}

	tests := []struct {
		name string
		ns   string
		want []string
		fail bool
	}{
		{
			name: "a, b",
			ns:   "a, b",
			want: []string{"a", "b", ""},
			fail: false,
		},
		{
			name: "a, , b (contains empty values)",
			ns:   "a, b",
			want: []string{"a", "b", ""},
			fail: false,
		},
		{
			name: "a",
			ns:   "a",
			want: []string{"a", ""}, //TODO: wtf?
			fail: false,
		},
		{
			name: "a b (contains spaces)", // labels should not contain spaces, thus fail
			ns:   "a b",
			fail: true,
		},
		{
			name: "(empty values)", // should never return empty values, thus fail
			ns:   "",
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := acl.toSlice(tt.ns)
			if tt.fail {
				if err == nil {
					t.Error("Expected a non-nil error, though got a nil one")
				}
			} else {
				if reflect.DeepEqual(got, tt.want) {
					t.Errorf("want %q; got %q", tt.want, got)
				}
			}
		})
	}
}

func TestACL_PrepareLF(t *testing.T) {
	acl := &ACL{false, metricsql.LabelFilter{}, ""}

	tests := []struct {
		name string
		ns   string
		want metricsql.LabelFilter
		fail bool
	}{
		{
			name: ".* (full access)",
			ns:   ".*",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "min.*, .*, stolon (full access, same as .*)",
			ns:   "min.*, .*, stolon",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "minio (only minio)",
			ns:   "minio",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "minio",
				IsRegexp:   false,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "min.* (one regexp)",
			ns:   "min.*",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "min.* (one anchored regexp)",
			ns:   "^(min.*)$",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "minio, stolon (two namespaces)",
			ns:   "minio, stolon",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "minio|stolon",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "min.*, stolon (regexp and non-regexp)",
			ns:   "min.*, stolon",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "min.*|stolon",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: ".+ (is regexp)",
			ns:   ".+",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".+",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "a,b (is a correct regexp)",
			ns:   "a,b",
			want: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "a|b",
				IsRegexp:   true,
				IsNegative: false,
			},
			fail: false,
		},
		{
			name: "[ (incorrect regexp)",
			ns:   "[",
			want: metricsql.LabelFilter{},
			fail: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := acl.PrepareLF(tt.ns)
			if tt.fail {
				if err == nil {
					t.Errorf("Expected a non-nil error, though got %s", err)
				}
			} else {
				if got != tt.want {
					t.Errorf("want %q; got %q", tt.want.AppendString(nil), got.AppendString(nil))
				}
			}
		})
	}
}

// TODO: test loadACL
