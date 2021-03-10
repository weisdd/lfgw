package main

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func TestACL_ToSlice(t *testing.T) {
	acl := &ACL{false, metricsql.LabelFilter{}}

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
					t.Errorf("Expected a non-nil error, though got %s", err)
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
	acl := &ACL{false, metricsql.LabelFilter{}}

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
				Value:      "^(a|b)$",
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
// // PrepareLF Returns a label filter based on rule definitions (non-regexp for one namespace, regexp - for many)
// func (a ACL) PrepareLF(ns string) (metricsql.LabelFilter, error) {
// 	var lf = metricsql.LabelFilter{
// 		Label:      "namespace",
// 		IsNegative: false,
// 	}

// 	if ns == ".*" {
// 		lf.Value = ns
// 		lf.IsRegexp = true
// 	}

// 	buffer, err := a.toSlice(ns)
// 	if err != nil {
// 		return metricsql.LabelFilter{}, err
// 	}

// 	if len(buffer) == 1 {
// 		lf.Value = buffer[0]
// 		if strings.ContainsAny(buffer[0], `.+*?^$()[]{}|\`) {
// 			lf.IsRegexp = true
// 		}
// 	} else {
// 		lf.Value = fmt.Sprintf("^(%s)$", strings.Join(buffer, "|"))
// 		lf.IsRegexp = true
// 	}

// 	if lf.IsRegexp {
// 		_, err := regexp.Compile(lf.Value)
// 		if err != nil {
// 			return metricsql.LabelFilter{}, fmt.Errorf("%s in %q (converted from %q)", err, lf.Value, ns)
// 		}
// 	}

// 	return lf, nil
// }
