package ebpf

import (
	"testing"

	"github.com/cilium/ebpf/internal/testutils"
)

func TestMapABIEqual(t *testing.T) {
	abi := &MapABI{
		Type:       Array,
		KeySize:    4,
		ValueSize:  2,
		MaxEntries: 3,
		Flags:      1,
	}

	if !abi.Equal(abi) {
		t.Error("Equal returns true when comparing an ABI to itself")
	}

	if abi.Equal(&MapABI{}) {
		t.Error("Equal returns true for different ABIs")
	}
}

func TestMapABIFromProc(t *testing.T) {
	hash, err := NewMap(&MapSpec{
		Type:       Hash,
		KeySize:    4,
		ValueSize:  5,
		MaxEntries: 2,
		Flags:      0x1, // BPF_F_NO_PREALLOC
	})
	if err != nil {
		t.Fatal(err)
	}
	defer hash.Close()

	abi, err := newMapABIFromProc(hash.fd)
	if err != nil {
		t.Fatal("Can't get map ABI:", err)
	}

	if abi.Type != Hash {
		t.Error("Expected Hash, got", abi.Type)
	}

	if abi.KeySize != 4 {
		t.Error("Expected KeySize of 4, got", abi.KeySize)
	}

	if abi.ValueSize != 5 {
		t.Error("Expected ValueSize of 5, got", abi.ValueSize)
	}

	if abi.MaxEntries != 2 {
		t.Error("Expected MaxEntries of 2, got", abi.MaxEntries)
	}

	if abi.Flags != 1 {
		t.Error("Expected Flags to be 1, got", abi.Flags)
	}

	nested, err := NewMap(&MapSpec{
		Type:       ArrayOfMaps,
		KeySize:    4,
		MaxEntries: 2,
		InnerMap: &MapSpec{
			Type:       Array,
			KeySize:    4,
			ValueSize:  4,
			MaxEntries: 2,
		},
	})
	testutils.SkipIfNotSupported(t, err)
	if err != nil {
		t.Fatal(err)
	}
	defer nested.Close()

	_, err = newMapABIFromProc(nested.fd)
	if err != nil {
		t.Fatal("Can't get nested map ABI from /proc:", err)
	}
}

func TestProgramABI(t *testing.T) {
	abi := &ProgramABI{Type: SocketFilter}

	if !abi.Equal(abi) {
		t.Error("Equal returns true when comparing an ABI to itself")
	}

	if abi.Equal(&ProgramABI{}) {
		t.Error("Equal returns true for different ABIs")
	}
}
