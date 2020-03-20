// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import (
	"reflect"
	"testing"
)

type node int64

func (n node) ID() int64 { return int64(n) }

// TestSame tests the assumption that pointer equality via unsafe conversion
// of a map[int]struct{} to uintptr is a valid test for perfect identity between
// set values. If any of the tests in TestSame fail, the package is broken and same
// must be reimplemented to conform to the runtime map implementation. The relevant
// code to look at (at least for gc) is the hmap type in runtime/map.go.

func TestSameInt64s(t *testing.T) {
	var (
		a = make(Int64s)
		b = make(Int64s)
		c = a
	)

	if int64sSame(a, b) {
		t.Error("Independently created sets test as same")
	}
	if !int64sSame(a, c) {
		t.Error("Set copy and original test as not same.")
	}
	a.Add(1)
	if !int64sSame(a, c) {
		t.Error("Set copy and original test as not same after addition.")
	}
	if !int64sSame(nil, nil) {
		t.Error("nil sets test as not same.")
	}
	if int64sSame(b, nil) {
		t.Error("nil and empty sets test as same.")
	}
}

func TestAddInt64s(t *testing.T) {
	s := make(Int64s)
	if s == nil {
		t.Fatal("Set cannot be created successfully")
	}

	if s.Count() != 0 {
		t.Error("Set somehow contains new elements upon creation")
	}

	s.Add(1)
	s.Add(3)
	s.Add(5)

	if s.Count() != 3 {
		t.Error("Incorrect number of set elements after adding")
	}

	if !s.Has(1) || !s.Has(3) || !s.Has(5) {
		t.Error("Set doesn't contain element that was added")
	}

	s.Add(1)

	if s.Count() > 3 {
		t.Error("Set double-adds element (element not unique)")
	} else if s.Count() < 3 {
		t.Error("Set double-add lowered len")
	}

	if !s.Has(1) {
		t.Error("Set doesn't contain double-added element")
	}

	if !s.Has(3) || !s.Has(5) {
		t.Error("Set removes element on double-add")
	}
}

func TestRemoveInt64s(t *testing.T) {
	s := make(Int64s)

	s.Add(1)
	s.Add(3)
	s.Add(5)

	s.Remove(1)

	if s.Count() != 2 {
		t.Error("Incorrect number of set elements after removing an element")
	}

	if s.Has(1) {
		t.Error("Element present after removal")
	}

	if !s.Has(3) || !s.Has(5) {
		t.Error("Set remove removed wrong element")
	}

	s.Remove(1)

	if s.Count() != 2 || s.Has(1) {
		t.Error("Double set remove does something strange")
	}

	s.Add(1)

	if s.Count() != 3 || !s.Has(1) {
		t.Error("Cannot add element after removal")
	}
}

func TestSelfEqualInt64s(t *testing.T) {
	s := make(Int64s)

	if !Int64sEqual(s, s) {
		t.Error("Set is not equal to itself")
	}

	s.Add(1)

	if !Int64sEqual(s, s) {
		t.Error("Set ceases self equality after adding element")
	}
}

func TestEqualInt64s(t *testing.T) {
	a := make(Int64s)
	b := make(Int64s)

	if !Int64sEqual(a, b) {
		t.Error("Two different empty sets not equal")
	}

	a.Add(1)
	if Int64sEqual(a, b) {
		t.Error("Two different sets with different sizes equal")
	}

	b.Add(1)
	if !Int64sEqual(a, b) {
		t.Error("Two sets with same element not equal")
	}

	b.Remove(1)
	b.Add(2)
	if Int64sEqual(a, b) {
		t.Error("Two different sets with different elements equal")
	}
}

func TestSameInts(t *testing.T) {
	var (
		a = make(Ints)
		b = make(Ints)
		c = a
	)

	if intsSame(a, b) {
		t.Error("Independently created sets test as same")
	}
	if !intsSame(a, c) {
		t.Error("Set copy and original test as not same.")
	}
	a.Add(1)
	if !intsSame(a, c) {
		t.Error("Set copy and original test as not same after addition.")
	}
	if !intsSame(nil, nil) {
		t.Error("nil sets test as not same.")
	}
	if intsSame(b, nil) {
		t.Error("nil and empty sets test as same.")
	}
}

func TestAddInts(t *testing.T) {
	s := make(Ints)
	if s == nil {
		t.Fatal("Set cannot be created successfully")
	}

	if s.Count() != 0 {
		t.Error("Set somehow contains new elements upon creation")
	}

	s.Add(1)
	s.Add(3)
	s.Add(5)

	if s.Count() != 3 {
		t.Error("Incorrect number of set elements after adding")
	}

	if !s.Has(1) || !s.Has(3) || !s.Has(5) {
		t.Error("Set doesn't contain element that was added")
	}

	s.Add(1)

	if s.Count() > 3 {
		t.Error("Set double-adds element (element not unique)")
	} else if s.Count() < 3 {
		t.Error("Set double-add lowered len")
	}

	if !s.Has(1) {
		t.Error("Set doesn't contain double-added element")
	}

	if !s.Has(3) || !s.Has(5) {
		t.Error("Set removes element on double-add")
	}
}

func TestRemoveInts(t *testing.T) {
	s := make(Ints)

	s.Add(1)
	s.Add(3)
	s.Add(5)

	s.Remove(1)

	if s.Count() != 2 {
		t.Error("Incorrect number of set elements after removing an element")
	}

	if s.Has(1) {
		t.Error("Element present after removal")
	}

	if !s.Has(3) || !s.Has(5) {
		t.Error("Set remove removed wrong element")
	}

	s.Remove(1)

	if s.Count() != 2 || s.Has(1) {
		t.Error("Double set remove does something strange")
	}

	s.Add(1)

	if s.Count() != 3 || !s.Has(1) {
		t.Error("Cannot add element after removal")
	}
}

func TestSelfEqualInts(t *testing.T) {
	s := make(Ints)

	if !IntsEqual(s, s) {
		t.Error("Set is not equal to itself")
	}

	s.Add(1)

	if !IntsEqual(s, s) {
		t.Error("Set ceases self equality after adding element")
	}
}

func TestEqualInts(t *testing.T) {
	a := make(Ints)
	b := make(Ints)

	if !IntsEqual(a, b) {
		t.Error("Two different empty sets not equal")
	}

	a.Add(1)
	if IntsEqual(a, b) {
		t.Error("Two different sets with different sizes equal")
	}

	b.Add(1)
	if !IntsEqual(a, b) {
		t.Error("Two sets with same element not equal")
	}

	b.Remove(1)
	b.Add(2)
	if IntsEqual(a, b) {
		t.Error("Two different sets with different elements equal")
	}
}

func TestSameNodes(t *testing.T) {
	var (
		a = NewNodes()
		b = NewNodes()
		c = a
	)

	if same(a, b) {
		t.Error("Independently created sets test as same")
	}
	if !same(a, c) {
		t.Error("Set copy and original test as not same.")
	}
	a.Add(node(1))
	if !same(a, c) {
		t.Error("Set copy and original test as not same after addition.")
	}
	if !same(nil, nil) {
		t.Error("nil sets test as not same.")
	}
	if same(b, nil) {
		t.Error("nil and empty sets test as same.")
	}
}

func TestAddNodes(t *testing.T) {
	s := NewNodes()
	if s == nil {
		t.Fatal("Set cannot be created successfully")
	}

	if s.Count() != 0 {
		t.Error("Set somehow contains new elements upon creation")
	}

	s.Add(node(1))
	s.Add(node(3))
	s.Add(node(5))

	if s.Count() != 3 {
		t.Error("Incorrect number of set elements after adding")
	}

	if !s.Has(node(1)) || !s.Has(node(3)) || !s.Has(node(5)) {
		t.Error("Set doesn't contain element that was added")
	}

	s.Add(node(1))

	if s.Count() > 3 {
		t.Error("Set double-adds element (element not unique)")
	} else if s.Count() < 3 {
		t.Error("Set double-add lowered len")
	}

	if !s.Has(node(1)) {
		t.Error("Set doesn't contain double-added element")
	}

	if !s.Has(node(3)) || !s.Has(node(5)) {
		t.Error("Set removes element on double-add")
	}

	for e, n := range s {
		if e != n.ID() {
			t.Errorf("Element ID did not match key: %d != %d", e, n.ID())
		}
	}
}

func TestRemoveNodes(t *testing.T) {
	s := NewNodes()

	s.Add(node(1))
	s.Add(node(3))
	s.Add(node(5))

	s.Remove(node(1))

	if s.Count() != 2 {
		t.Error("Incorrect number of set elements after removing an element")
	}

	if s.Has(node(1)) {
		t.Error("Element present after removal")
	}

	if !s.Has(node(3)) || !s.Has(node(5)) {
		t.Error("Set remove removed wrong element")
	}

	s.Remove(node(1))

	if s.Count() != 2 || s.Has(node(1)) {
		t.Error("Double set remove does something strange")
	}

	s.Add(node(1))

	if s.Count() != 3 || !s.Has(node(1)) {
		t.Error("Cannot add element after removal")
	}
}

func TestSelfEqualNodes(t *testing.T) {
	s := NewNodes()

	if !Equal(s, s) {
		t.Error("Set is not equal to itself")
	}

	s.Add(node(1))

	if !Equal(s, s) {
		t.Error("Set ceases self equality after adding element")
	}
}

func TestEqualNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	if !Equal(a, b) {
		t.Error("Two different empty sets not equal")
	}

	a.Add(node(1))
	if Equal(a, b) {
		t.Error("Two different sets with different sizes equal")
	}

	b.Add(node(1))
	if !Equal(a, b) {
		t.Error("Two sets with same element not equal")
	}

	b.Remove(node(1))
	b.Add(node(2))
	if Equal(a, b) {
		t.Error("Two different sets with different elements equal")
	}
}

func TestCopyNodes(t *testing.T) {
	a := NewNodes()

	a.Add(node(1))
	a.Add(node(2))
	a.Add(node(3))

	b := CloneNodes(a)

	if !Equal(a, b) {
		t.Fatalf("Two sets not equal after copy: %v != %v", a, b)
	}

	b.Remove(node(1))

	if Equal(a, b) {
		t.Errorf("Mutating one set mutated another after copy: %v == %v", a, b)
	}
}

func TestUnionSameNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(1))
	b.Add(node(2))

	c := UnionOfNodes(a, b)

	if c.Count() != 2 {
		t.Error("Union of same sets yields set with wrong len")
	}

	if !c.Has(node(1)) || !c.Has(node(2)) {
		t.Error("Union of same sets yields wrong elements")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestUnionDiffNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(3))

	c := UnionOfNodes(a, b)

	if c.Count() != 3 {
		t.Error("Union of different sets yields set with wrong len")
	}

	if !c.Has(node(1)) || !c.Has(node(2)) || !c.Has(node(3)) {
		t.Error("Union of different sets yields set with wrong elements")
	}

	if a.Has(node(3)) || !a.Has(node(2)) || !a.Has(node(1)) || a.Count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !b.Has(node(3)) || b.Has(node(1)) || b.Has(node(2)) || b.Count() != 1 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}

	c = UnionOfNodes(a, a)
	if !reflect.DeepEqual(c, a) {
		t.Errorf("Union of equal sets not equal to sets: %v != %v", c, a)
	}
}

func TestUnionOverlappingNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(2))
	b.Add(node(3))

	c := UnionOfNodes(a, b)

	if c.Count() != 3 {
		t.Error("Union of overlapping sets yields set with wrong len")
	}

	if !c.Has(node(1)) || !c.Has(node(2)) || !c.Has(node(3)) {
		t.Error("Union of overlapping sets yields set with wrong elements")
	}

	if a.Has(node(3)) || !a.Has(node(2)) || !a.Has(node(1)) || a.Count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !b.Has(node(3)) || b.Has(node(1)) || !b.Has(node(2)) || b.Count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}

	c = IntersectionOfNodes(a, a)
	if !reflect.DeepEqual(c, a) {
		t.Errorf("Intersection of equal sets not equal to sets: %v != %v", c, a)
	}
}

func TestIntersectSameNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(2))
	b.Add(node(3))

	c := IntersectionOfNodes(a, b)

	if card := c.Count(); card != 2 {
		t.Errorf("Intersection of identical sets yields set of wrong len %d", card)
	}

	if !c.Has(node(2)) || !c.Has(node(3)) {
		t.Error("Intersection of identical sets yields set of wrong elements")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectDiffNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(1))
	b.Add(node(4))

	c := IntersectionOfNodes(a, b)

	if card := c.Count(); card != 0 {
		t.Errorf("Intersection of different yields non-empty set %d", card)
	}

	if !a.Has(node(2)) || !a.Has(node(3)) || a.Has(node(1)) || a.Has(node(4)) || a.Count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if b.Has(node(2)) || b.Has(node(3)) || !b.Has(node(1)) || !b.Has(node(4)) || b.Count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectOverlappingNodes(t *testing.T) {
	a := NewNodes()
	b := NewNodes()

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(3))
	b.Add(node(4))

	c := IntersectionOfNodes(a, b)

	if card := c.Count(); card != 1 {
		t.Errorf("Intersection of overlapping sets yields set of incorrect len %d", card)
	}

	if !c.Has(node(3)) {
		t.Errorf("Intersection of overlapping sets yields set with wrong element")
	}

	if !a.Has(node(2)) || !a.Has(node(3)) || a.Has(node(4)) || a.Count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if b.Has(node(2)) || !b.Has(node(3)) || !b.Has(node(4)) || b.Count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}

	c = IntersectionOfNodes(c, a)
	want := Nodes{3: node(3)}
	if !reflect.DeepEqual(c, want) {
		t.Errorf("Intersection of sets with dst equal to a not equal: %v != %v", c, want)
	}
	c = IntersectionOfNodes(a, c)
	if !reflect.DeepEqual(c, want) {
		t.Errorf("Intersection of sets with dst equal to a not equal: %v != %v", c, want)
	}
}
