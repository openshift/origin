// +build output

package test

import (
	"testing"
)

func TestFoo(t *testing.T) {
	t.Run("panic", func(t *testing.T) {
		panic("here")
	})
	t.Run("pass", func(t *testing.T) {
	})
	t.Run("skip", func(t *testing.T) {
		t.Skip("skipped")
	})
}
