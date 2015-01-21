package netutils

import (
	"testing"
)

func TestAllocateIP(t *testing.T) {
	ipa, err := NewIPAllocator("10.1.2.0/24")
	if err != nil {
		t.Fatal("Failed to initialize IP allocator: %v", err)
	}
	t.Log(ipa.GetIP())
}
