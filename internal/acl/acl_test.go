package acl

import (
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/stretchr/testify/assert"
)

func Test_NewACL(t *testing.T) {
	tests := []struct {
		name   string
		rawACL string
		want   ACL
		fail   bool
	}{
		{
			name:   ".* (full access)",
			rawACL: ".*",
			want: ACL{
				Fullaccess: true,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      ".*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: ".*",
			},
			fail: false,
		},
		{
			name:   "min.*, .*, stolon (implicit full access, same as .*)",
			rawACL: "min.*, .*, stolon",
			want: ACL{
				Fullaccess: true,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      ".*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: ".*",
			},
			fail: false,
		},
		{
			name:   "minio (only minio)",
			rawACL: "minio",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "minio",
					IsRegexp:   false,
					IsNegative: false,
				},
				RawACL: "minio",
			},
			fail: false,
		},
		{
			name:   "min.* (one regexp)",
			rawACL: "min.*",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "min.*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "min.*",
			},
			fail: false,
		},
		{
			name:   "min.* (one anchored regexp)",
			rawACL: "^(min.*)$",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "min.*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "min.*",
			},
			fail: false,
		},
		{
			name:   "minio, stolon (two namespaces)",
			rawACL: "minio, stolon",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "minio|stolon",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "minio, stolon",
			},
			fail: false,
		},
		{
			name:   "min.*, stolon (regexp and non-regexp)",
			rawACL: "min.*, stolon",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "min.*|stolon",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "min.*, stolon",
			},
			fail: false,
		},
		// TODO: assign special meaning to this regexp?
		{
			name:   ".+ (is a regexp)",
			rawACL: ".+",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      ".+",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: ".+",
			},
			fail: false,
		},
		{
			name:   "a,b (is a correct regexp)",
			rawACL: "a,b",
			want: ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "a|b",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "a, b",
			},
			fail: false,
		},
		{
			name:   "[ (incorrect regexp)",
			rawACL: "[",
			want:   ACL{},
			fail:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewACL(tt.rawACL)
			if tt.fail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
