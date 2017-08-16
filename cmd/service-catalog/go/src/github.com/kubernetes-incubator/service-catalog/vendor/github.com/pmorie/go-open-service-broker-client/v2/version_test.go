package v2

import (
	"testing"
)

func TestAtLeast(t *testing.T) {
	v2_12 := Version2_12()
	v2_11 := Version2_11()

	if !v2_12.AtLeast(v2_11) {
		t.Error("Expected 2.12 >= 2.11")
	}

	if v2_11.AtLeast(v2_12) {
		t.Error("Expected 2.11 < 2.12")
	}
}
