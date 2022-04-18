package querymodifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestACL_ToSlice(t *testing.T) {
	t.Run("a, b", func(t *testing.T) {
		want := []string{"a", "b"}
		got, err := toSlice("a, b")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a, , b (contains empty values)", func(t *testing.T) {
		want := []string{"a", "b"}
		got, err := toSlice("a, , b")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a", func(t *testing.T) {
		want := []string{"a"}
		got, err := toSlice("a")
		assert.Nil(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("a b (contains spaces)", func(t *testing.T) {
		_, err := toSlice("a b")
		assert.NotNil(t, err)
	})

	t.Run("(empty values)", func(t *testing.T) {
		_, err := toSlice("")
		assert.NotNil(t, err)
	})
}
