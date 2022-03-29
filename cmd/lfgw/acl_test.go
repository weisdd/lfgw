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
					RawACL: "ku.*, .*, min.*",
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

func TestApplication_GetUserRoles(t *testing.T) {
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

	t.Run("0 known roles", func(t *testing.T) {
		oidcRoles := []string{"unknown-role"}

		_, err := app.getUserRoles(oidcRoles)
		assert.NotNil(t, err)
	})

	t.Run("1 known role", func(t *testing.T) {
		oidcRoles := []string{"single-value", "uknown-role"}
		want := []string{"single-value"}

		got, err := app.getUserRoles(oidcRoles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)

	})

	t.Run("multiple known roles", func(t *testing.T) {
		oidcRoles := []string{"single-value", "multiple-values"}
		want := []string{"single-value", "multiple-values"}

		got, err := app.getUserRoles(oidcRoles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})
}

func TestApplication_GetLF(t *testing.T) {
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
		_, err := app.getLF(roles)
		assert.NotNil(t, err)
	})

	t.Run("1 role", func(t *testing.T) {
		roles := []string{"single-value"}
		want := app.ACLMap["single-value"].LabelFilter
		got, err := app.getLF(roles)
		assert.Nil(t, err)
		assert.Equal(t, got, want)
	})

	t.Run("multiple roles, full access", func(t *testing.T) {
		roles := []string{"admin", "multiple-values"}
		want := app.ACLMap["admin"].LabelFilter
		got, err := app.getLF(roles)
		assert.Nil(t, err)
		assert.Equal(t, got, want)
	})

	t.Run("multiple roles, no full access", func(t *testing.T) {
		// TODO: maybe shouldn't test this here
		roles := []string{"single-value", "multiple-values", "unknown-role"}
		roles, err := app.getUserRoles(roles)
		assert.Nil(t, err)

		acl := app.ACLMap["single-value"]
		want, err := acl.PrepareLF(app.rolesToRawACL(roles))
		assert.Nil(t, err)

		got, err := app.getLF(roles)
		assert.Nil(t, err)
		assert.Equal(t, got, want)
	})
}
