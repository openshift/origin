package internal

import (
	"bytes"
	"io"
	"testing"
)

func TestDiscardZero(t *testing.T) {
	_, err := io.Copy(DiscardZeroes{}, bytes.NewReader([]byte{0, 0, 0}))
	if err != nil {
		t.Error("Returned an error even though input was zero:", err)
	}

	_, err = io.Copy(DiscardZeroes{}, bytes.NewReader([]byte{1}))
	if err == nil {
		t.Error("No error even though input is non-zero")
	}
}
