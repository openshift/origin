package ebpf

import (
	"testing"
)

func TestCollectionABI(t *testing.T) {
	cabi := &CollectionABI{
		Maps: map[string]*MapABI{
			"a": {
				Type:       ArrayOfMaps,
				KeySize:    4,
				ValueSize:  2,
				MaxEntries: 3,
				InnerMap: &MapABI{
					Type: Array,
				},
			},
		},
		Programs: map[string]*ProgramABI{
			"1": {Type: SocketFilter},
		},
	}

	if err := cabi.CheckSpec(abiFixtureCollectionSpec()); err != nil {
		t.Error("ABI check found error:", err)
	}

	cs := abiFixtureCollectionSpec()
	delete(cs.Maps, "a")
	if err := cabi.CheckSpec(cs); err == nil {
		t.Error("Did not detect missing map")
	}

	cs = abiFixtureCollectionSpec()
	delete(cs.Programs, "1")
	if err := cabi.CheckSpec(cs); err == nil {
		t.Error("Did not detect missing program")
	}

	if err := cabi.Check(abiFixtureCollection()); err != nil {
		t.Error("ABI check found error:", err)

	}

	coll := abiFixtureCollection()
	coll.Maps["a"].abi.KeySize = 12
	if err := cabi.Check(coll); err == nil {
		t.Error("Check not check map ABI")
	}

	delete(coll.Maps, "a")
	if err := cabi.Check(coll); err == nil {
		t.Error("Check did not detect missing map")
	}

	coll = abiFixtureCollection()
	coll.Programs["1"].abi.Type = TracePoint
	if err := cabi.Check(coll); err == nil {
		t.Error("Did not check program ABI")
	}

	delete(coll.Programs, "1")
	if err := cabi.Check(coll); err == nil {
		t.Error("Check did not detect missing program")
	}
}

func TestMapABI(t *testing.T) {
	mabi := &MapABI{
		Type:       ArrayOfMaps,
		KeySize:    4,
		ValueSize:  2,
		MaxEntries: 3,
		InnerMap: &MapABI{
			Type: Array,
		},
	}

	if err := mabi.Check(abiFixtureMap()); err != nil {
		t.Error("ABI check found error:", err)
	}

	fm := abiFixtureMap()
	fm.abi.Type = Hash
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect incorrect type")
	}

	fm = abiFixtureMap()
	fm.abi.KeySize = 3
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect incorrect key size")
	}

	fm = abiFixtureMap()
	fm.abi.ValueSize = 23
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect incorrect value size")
	}

	fm = abiFixtureMap()
	fm.abi.MaxEntries = 23
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect incorrect max entries")
	}

	fm = abiFixtureMap()
	mabi.InnerMap.Type = Hash
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect incorrect inner map type")
	}

	fm = abiFixtureMap()
	mabi.InnerMap = nil
	if err := mabi.Check(fm); err == nil {
		t.Error("Did not detect missing inner map ABI")
	}
}

func TestProgramABI(t *testing.T) {
	fabi := &ProgramABI{Type: SocketFilter}

	if err := fabi.Check(abiFixtureProgram()); err != nil {
		t.Error("ABI check found error:", err)
	}

	fp := abiFixtureProgram()
	fp.abi.Type = TracePoint
	if err := fabi.Check(fp); err == nil {
		t.Error("Did not detect incorrect type")
	}
}

func abiFixtureCollectionSpec() *CollectionSpec {
	return &CollectionSpec{
		Maps: map[string]*MapSpec{
			"a": abiFixtureMapSpec(),
		},
		Programs: map[string]*ProgramSpec{
			"1": abiFixtureProgramSpec(),
		},
	}
}

func abiFixtureCollection() *Collection {
	return &Collection{
		Maps: map[string]*Map{
			"a": abiFixtureMap(),
		},
		Programs: map[string]*Program{
			"1": abiFixtureProgram(),
		},
	}
}

func abiFixtureMapSpec() *MapSpec {
	return &MapSpec{
		Type:       ArrayOfMaps,
		KeySize:    4,
		ValueSize:  2,
		MaxEntries: 3,
		InnerMap: &MapSpec{
			Type:    Array,
			KeySize: 2,
		},
	}
}

func abiFixtureMap() *Map {
	return &Map{
		abi: *newMapABIFromSpec(abiFixtureMapSpec()),
	}
}

func abiFixtureProgramSpec() *ProgramSpec {
	return &ProgramSpec{
		Type: SocketFilter,
	}
}

func abiFixtureProgram() *Program {
	return &Program{
		abi: *newProgramABIFromSpec(abiFixtureProgramSpec()),
	}
}

func ExampleCollectionABI() {
	abi := CollectionABI{
		Maps: map[string]*MapABI{
			"a": {
				Type: Array,
				// Members which aren't specified are not checked
			},
			// Use an empty ABI if you just want to make sure
			// something is present.
			"b": {},
		},
		Programs: map[string]*ProgramABI{
			"1": {Type: XDP},
		},
	}

	spec, err := LoadCollectionSpec("from-somewhere.elf")
	if err != nil {
		panic(err)
	}

	// CheckSpec only makes sure that all entries of the ABI
	// are present. It doesn't check whether the ABI is correct.
	// See below for how to do that.
	if err := abi.CheckSpec(spec); err != nil {
		panic(err)
	}

	coll, err := NewCollection(spec)
	if err != nil {
		panic(err)
	}

	// Check finally compares the ABI and the collection, and
	// makes sure that they match.
	if err := abi.Check(coll); err != nil {
		panic(err)
	}
}
