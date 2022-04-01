package main

import (
	"os"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/stretchr/testify/assert"
)

func TestACL_ToSlice(t *testing.T) {
	acl := &ACL{}

	t.Run("a, b", func(t *testing.T) {
		want := []string{"a", "b"}
		got, err := acl.toSlice("a, b")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a, , b (contains empty values)", func(t *testing.T) {
		want := []string{"a", "b"}
		got, err := acl.toSlice("a, , b")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a", func(t *testing.T) {
		want := []string{"a"}
		got, err := acl.toSlice("a")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a b (contains spaces)", func(t *testing.T) {
		_, err := acl.toSlice("a b")
		assert.NotNil(t, err)
	})

	t.Run("(empty values)", func(t *testing.T) {
		_, err := acl.toSlice("")
		assert.NotNil(t, err)
	})
}

func TestACL_PrepareACL(t *testing.T) {
	acl := &ACL{false, metricsql.LabelFilter{}, ""}

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
			name:   "min.*, .*, stolon (full access, same as .*)",
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
			name:   ".+ (is regexp)",
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
			got, err := acl.PrepareACL(tt.rawACL)
			if tt.fail {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestACL_LoadACL(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    ACLMap
	}{
		{
			name:    "admin",
			content: "admin: .*",
			want: ACLMap{
				"admin": &ACL{
					Fullaccess: true,
					LabelFilter: metricsql.LabelFilter{
						Label:      "namespace",
						Value:      ".*",
						IsRegexp:   true,
						IsNegative: false,
					},
					RawACL: ".*",
				},
			},
		},
		{
			name:    "implicit-admin",
			content: `implicit-admin: ku.*, .*, min.*`,
			want: ACLMap{
				"implicit-admin": &ACL{
					Fullaccess: true,
					LabelFilter: metricsql.LabelFilter{
						Label:      "namespace",
						Value:      ".*",
						IsRegexp:   true,
						IsNegative: false,
					},
					RawACL: ".*",
				},
			},
		},
		{
			name:    "multiple-values",
			content: "multiple-values: ku.*, min.*",
			want: ACLMap{
				"multiple-values": &ACL{
					Fullaccess: false,
					LabelFilter: metricsql.LabelFilter{
						Label:      "namespace",
						Value:      "ku.*|min.*",
						IsRegexp:   true,
						IsNegative: false,
					},
					RawACL: "ku.*, min.*",
				},
			},
		},
		{
			name:    "single-value",
			content: "single-value: default",
			want: ACLMap{
				"single-value": &ACL{
					Fullaccess: false,
					LabelFilter: metricsql.LabelFilter{
						Label:      "namespace",
						Value:      "default",
						IsRegexp:   false,
						IsNegative: false,
					},
					RawACL: "default",
				},
			},
		},
	}

	f, err := os.CreateTemp("", "acl-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	app := &application{
		ACLPath: f.Name(),
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveACLToFile(t, f, tt.content)
			got, err := app.loadACL()
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("incorrect ACL", func(t *testing.T) {
		saveACLToFile(t, f, "test-role:")
		_, err := app.loadACL()
		assert.NotNil(t, err)

		saveACLToFile(t, f, "test-role: a b")
		_, err = app.loadACL()
		assert.NotNil(t, err)
	})

	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// saveACLToFile writes given content to a file (existing data is deleted)
func saveACLToFile(t testing.TB, f *os.File, content string) {
	t.Helper()
	if err := f.Truncate(0); err != nil {
		f.Close()
		t.Fatal(err)
	}

	if _, err := f.Seek(0, 0); err != nil {
		f.Close()
		t.Fatal(err)
	}

	if _, err := f.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatal(err)
	}
}

func TestApplication_rolesToRawACL(t *testing.T) {
	app := &application{
		ACLMap: ACLMap{
			"admin": &ACL{
				Fullaccess: true,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      ".*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: ".*",
			},
			"multiple-values": &ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "ku.*|min.*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "ku.*, min.*",
			},
			"single-value": &ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "default",
					IsRegexp:   false,
					IsNegative: false,
				},
				RawACL: "default",
			},
		},
	}

	t.Run("0 roles", func(t *testing.T) {
		roles := []string{}
		_, err := app.rolesToRawACL(roles)
		assert.NotNil(t, err)
	})

	t.Run("0 known roles", func(t *testing.T) {
		roles := []string{"unknown-role"}
		_, err := app.rolesToRawACL(roles)
		assert.NotNil(t, err)
	})

	t.Run("1 known role", func(t *testing.T) {
		roles := []string{"multiple-values"}
		got, err := app.rolesToRawACL(roles)

		want := "ku.*, min.*"
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple known roles", func(t *testing.T) {
		roles := []string{"multiple-values", "single-value"}
		got, err := app.rolesToRawACL(roles)

		want := "ku.*, min.*, default"
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Empty rawACL", func(t *testing.T) {
		app := &application{
			ACLMap: ACLMap{
				"empty-acl": &ACL{},
			},
		}
		roles := []string{"empty-acl"}

		_, err := app.rolesToRawACL(roles)
		assert.NotNil(t, err)
	})
}

func TestApplication_GetACL(t *testing.T) {
	app := &application{
		ACLMap: ACLMap{
			"admin": &ACL{
				Fullaccess: true,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      ".*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: ".*",
			},
			"multiple-values": &ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "ku.*|min.*",
					IsRegexp:   true,
					IsNegative: false,
				},
				RawACL: "ku.*, min.*",
			},
			"single-value": &ACL{
				Fullaccess: false,
				LabelFilter: metricsql.LabelFilter{
					Label:      "namespace",
					Value:      "default",
					IsRegexp:   false,
					IsNegative: false,
				},
				RawACL: "default",
			},
		},
	}

	t.Run("0 roles", func(t *testing.T) {
		roles := []string{}
		_, err := app.getACL(roles)
		assert.NotNil(t, err)
	})

	t.Run("0 known roles", func(t *testing.T) {
		roles := []string{"unknown-role"}
		_, err := app.getACL(roles)
		assert.NotNil(t, err)
	})

	t.Run("1 role", func(t *testing.T) {
		roles := []string{"single-value"}
		want := *app.ACLMap["single-value"]
		got, err := app.getACL(roles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, full access", func(t *testing.T) {
		roles := []string{"admin", "multiple-values"}
		want := *app.ACLMap["admin"]
		got, err := app.getACL(roles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, no full access", func(t *testing.T) {
		roles := []string{"single-value", "multiple-values", "unknown-role"}
		knownRoles := []string{"single-value", "multiple-values"}

		rawACL, err := app.rolesToRawACL(knownRoles)
		assert.Nil(t, err)

		want := ACL{
			Fullaccess: false,
			RawACL:     rawACL,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "default|ku.*|min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
		}

		got, err := app.getACL(roles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})
}
