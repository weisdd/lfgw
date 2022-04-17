package acl

import (
	"os"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/stretchr/testify/assert"
)

func TestACL_rolesToRawACL(t *testing.T) {
	a := ACLs{
		"admin": ACL{
			Fullaccess: true,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: ".*",
		},
		"multiple-values": ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "ku.*|min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "ku.*, min.*",
		},
		"single-value": ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "default",
				IsRegexp:   false,
				IsNegative: false,
			},
			RawACL: "default",
		},
	}

	t.Run("0 roles", func(t *testing.T) {
		roles := []string{}
		_, err := a.rolesToRawACL(roles)
		assert.NotNil(t, err)
	})

	t.Run("1 known role", func(t *testing.T) {
		roles := []string{"multiple-values"}
		want := "ku.*, min.*"

		got, err := a.rolesToRawACL(roles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple known roles", func(t *testing.T) {
		roles := []string{"multiple-values", "single-value"}
		want := "ku.*, min.*, default"

		got, err := a.rolesToRawACL(roles)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("Empty rawACL", func(t *testing.T) {
		a := ACLs{
			"empty-acl": ACL{},
		}

		roles := []string{"empty-acl"}

		_, err := a.rolesToRawACL(roles)
		assert.NotNil(t, err)
	})
}

func TestACL_GetUserACL(t *testing.T) {
	a := ACLs{
		"admin": ACL{
			Fullaccess: true,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: ".*",
		},
		"multiple-values": ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "ku.*|min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "ku.*, min.*",
		},
		"single-value": ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "default",
				IsRegexp:   false,
				IsNegative: false,
			},
			RawACL: "default",
		},
	}

	// Assumed roles disabled
	t.Run("0 roles", func(t *testing.T) {
		roles := []string{}
		_, err := a.GetUserACL(roles, false)
		assert.NotNil(t, err)
	})

	t.Run("0 known roles", func(t *testing.T) {
		roles := []string{"unknown-role"}
		_, err := a.GetUserACL(roles, false)
		assert.NotNil(t, err)
	})

	t.Run("1 role", func(t *testing.T) {
		roles := []string{"single-value"}
		want := a["single-value"]
		got, err := a.GetUserACL(roles, false)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, full access", func(t *testing.T) {
		roles := []string{"admin", "multiple-values"}
		want := a["admin"]
		got, err := a.GetUserACL(roles, false)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, 1 is unknown, no full access", func(t *testing.T) {
		roles := []string{"single-value", "multiple-values", "unknown-role"}
		knownRoles := []string{"single-value", "multiple-values"}

		rawACL, err := a.rolesToRawACL(knownRoles)
		assert.Nil(t, err)

		want := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "default|ku.*|min.*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: rawACL,
		}

		got, err := a.GetUserACL(roles, false)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	// Assumed roles enabled
	t.Run("0 known roles, 1 is unknown (assumed roles enabled)", func(t *testing.T) {
		roles := []string{"unknown-role"}

		want := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "unknown-role",
				IsRegexp:   false,
				IsNegative: false,
			},
			RawACL: "unknown-role",
		}

		got, err := a.GetUserACL(roles, true)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, 1 is unknown (assumed roles enabled)", func(t *testing.T) {
		roles := []string{"multiple-values", "single-value", "unknown-role"}

		want := ACL{
			Fullaccess: false,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      "ku.*|min.*|default|unknown-role",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: "ku.*, min.*, default, unknown-role",
		}

		got, err := a.GetUserACL(roles, true)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("multiple roles, 1 is unknown, 1 gives full access (assumed roles enabled)", func(t *testing.T) {
		roles := []string{"multiple-values", "admin", "unknown-role"}

		want := ACL{
			Fullaccess: true,
			LabelFilter: metricsql.LabelFilter{
				Label:      "namespace",
				Value:      ".*",
				IsRegexp:   true,
				IsNegative: false,
			},
			RawACL: ".*",
		}

		got, err := a.GetUserACL(roles, true)
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})
}

func TestACL_NewACLsFromFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    ACLs
	}{
		{
			name:    "admin",
			content: "admin: .*",
			want: ACLs{
				"admin": ACL{
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
			want: ACLs{
				"implicit-admin": ACL{
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
			want: ACLs{
				"multiple-values": ACL{
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
			want: ACLs{
				"single-value": ACL{
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			saveACLToFile(t, f, tt.content)
			got, err := NewACLsFromFile(f.Name())
			assert.Nil(t, err)
			assert.Equal(t, tt.want, got)
		})
	}

	t.Run("incorrect ACL", func(t *testing.T) {
		saveACLToFile(t, f, "test-role:")
		_, err := NewACLsFromFile(f.Name())
		assert.NotNil(t, err)

		saveACLToFile(t, f, "test-role: a b")
		_, err = NewACLsFromFile(f.Name())
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
