package ebpf

import (
	"testing"

	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/internal/testutils"
)

func TestCollectionSpecNotModified(t *testing.T) {
	cs := CollectionSpec{
		Maps: map[string]*MapSpec{
			"my-map": {
				Type:       Array,
				KeySize:    4,
				ValueSize:  4,
				MaxEntries: 1,
			},
		},
		Programs: map[string]*ProgramSpec{
			"test": {
				Type: SocketFilter,
				Instructions: asm.Instructions{
					asm.LoadImm(asm.R1, 0, asm.DWord),
					asm.LoadImm(asm.R0, 0, asm.DWord),
					asm.Return(),
				},
				License: "MIT",
			},
		},
	}

	cs.Programs["test"].Instructions[0].Reference = "my-map"

	coll, err := NewCollection(&cs)
	if err != nil {
		t.Fatal(err)
	}
	coll.Close()

	if cs.Programs["test"].Instructions[0].Constant != 0 {
		t.Error("Creating a collection modifies input spec")
	}
}

func TestCollectionSpecCopy(t *testing.T) {
	cs := &CollectionSpec{
		Maps: map[string]*MapSpec{
			"my-map": {
				Type:       Array,
				KeySize:    4,
				ValueSize:  4,
				MaxEntries: 1,
			},
		},
		Programs: map[string]*ProgramSpec{
			"test": {
				Type: SocketFilter,
				Instructions: asm.Instructions{
					asm.LoadMapPtr(asm.R1, 0),
					asm.LoadImm(asm.R0, 0, asm.DWord),
					asm.Return(),
				},
				License: "MIT",
			},
		},
	}
	cpy := cs.Copy()

	if cpy == cs {
		t.Error("Copy returned the same pointner")
	}

	if cpy.Maps["my-map"] == cs.Maps["my-map"] {
		t.Error("Copy returned same Maps")
	}

	if cpy.Programs["test"] == cs.Programs["test"] {
		t.Error("Copy returned same Programs")
	}
}

func TestCollectionSpecRewriteMaps(t *testing.T) {
	insns := asm.Instructions{
		// R1 map
		asm.LoadMapPtr(asm.R1, 0),
		// R2 key
		asm.Mov.Reg(asm.R2, asm.R10),
		asm.Add.Imm(asm.R2, -4),
		asm.StoreImm(asm.R2, 0, 0, asm.Word),
		// Lookup map[0]
		asm.FnMapLookupElem.Call(),
		asm.JEq.Imm(asm.R0, 0, "ret"),
		asm.LoadMem(asm.R0, asm.R0, 0, asm.Word),
		asm.Return().Sym("ret"),
	}
	insns[0].Reference = "test-map"

	cs := &CollectionSpec{
		Maps: map[string]*MapSpec{
			"test-map": {
				Type:       Array,
				KeySize:    4,
				ValueSize:  4,
				MaxEntries: 1,
			},
		},
		Programs: map[string]*ProgramSpec{
			"test-prog": {
				Type:         SocketFilter,
				Instructions: insns,
				License:      "MIT",
			},
		},
	}

	// Override the map with another one
	newMap, err := NewMap(cs.Maps["test-map"])
	if err != nil {
		t.Fatal(err)
	}
	defer newMap.Close()

	err = newMap.Put(uint32(0), uint32(2))
	if err != nil {
		t.Fatal(err)
	}

	err = cs.RewriteMaps(map[string]*Map{
		"test-map": newMap,
	})
	if err != nil {
		t.Fatal(err)
	}

	if cs.Maps["test-map"] != nil {
		t.Error("RewriteMaps doesn't remove map from CollectionSpec.Maps")
	}

	coll, err := NewCollection(cs)
	if err != nil {
		t.Fatal(err)
	}
	defer coll.Close()

	ret, _, err := coll.Programs["test-prog"].Test(make([]byte, 14))
	testutils.SkipIfNotSupported(t, err)
	if err != nil {
		t.Fatal(err)
	}

	if ret != 2 {
		t.Fatal("new / override map not used")
	}
}
