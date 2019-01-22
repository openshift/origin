// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "testing"

type node int64

func (n node) ID() int64 { return int64(n) }

// count reports the number of elements stored in the node set.
func (s Nodes) count() int {
	return len(s)
}

// TestSame tests the assumption that pointer equality via unsafe conversion
// of a map[int]struct{} to uintptr is a valid test for perfect identity between
// set values. If any of the tests in TestSame fail, the package is broken and same
// must be reimplemented to conform to the runtime map implementation. The relevant
// code to look at (at least for gc) is in runtime/hashmap.{h,goc}.
func TestSame(t *testing.T) {
	var (
		a = make(Nodes)
		b = make(Nodes)
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

func TestAdd(t *testing.T) {
	s := make(Nodes)
	if s == nil {
		t.Fatal("Set cannot be created successfully")
	}

	if s.count() != 0 {
		t.Error("Set somehow contains new elements upon creation")
	}

	s.Add(node(1))
	s.Add(node(3))
	s.Add(node(5))

	if s.count() != 3 {
		t.Error("Incorrect number of set elements after adding")
	}

	if !s.Has(node(1)) || !s.Has(node(3)) || !s.Has(node(5)) {
		t.Error("Set doesn't contain element that was added")
	}

	s.Add(node(1))

	if s.count() > 3 {
		t.Error("Set double-adds element (element not unique)")
	} else if s.count() < 3 {
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

func TestRemove(t *testing.T) {
	s := make(Nodes)

	s.Add(node(1))
	s.Add(node(3))
	s.Add(node(5))

	s.Remove(node(1))

	if s.count() != 2 {
		t.Error("Incorrect number of set elements after removing an element")
	}

	if s.Has(node(1)) {
		t.Error("Element present after removal")
	}

	if !s.Has(node(3)) || !s.Has(node(5)) {
		t.Error("Set remove removed wrong element")
	}

	s.Remove(node(1))

	if s.count() != 2 || s.Has(node(1)) {
		t.Error("Double set remove does something strange")
	}

	s.Add(node(1))

	if s.count() != 3 || !s.Has(node(1)) {
		t.Error("Cannot add element after removal")
	}
}

func TestClear(t *testing.T) {
	s := make(Nodes)

	s.Add(node(8))
	s.Add(node(9))
	s.Add(node(10))

	s.clear()

	if s.count() != 0 {
		t.Error("clear did not properly reset set to size 0")
	}
}

func TestSelfEqual(t *testing.T) {
	s := make(Nodes)

	if !Equal(s, s) {
		t.Error("Set is not equal to itself")
	}

	s.Add(node(1))

	if !Equal(s, s) {
		t.Error("Set ceases self equality after adding element")
	}
}

func TestEqual(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)

	if !Equal(a, b) {
		t.Error("Two different empty sets not equal")
	}

	a.Add(node(1))
	if Equal(a, b) {
		t.Error("Two different sets with different elements not equal")
	}

	b.Add(node(1))
	if !Equal(a, b) {
		t.Error("Two sets with same element not equal")
	}
}

func TestCopy(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)

	a.Add(node(1))
	a.Add(node(2))
	a.Add(node(3))

	b.Copy(a)

	if !Equal(a, b) {
		t.Fatalf("Two sets not equal after copy")
	}

	b.Remove(node(1))

	if Equal(a, b) {
		t.Errorf("Mutating one set mutated another after copy")
	}
}

func TestSelfCopy(t *testing.T) {
	a := make(Nodes)

	a.Add(node(1))
	a.Add(node(2))

	a.Copy(a)

	if a.count() != 2 {
		t.Error("Something strange happened when copying into self")
	}
}

func TestUnionSame(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(1))
	b.Add(node(2))

	c.Union(a, b)

	if c.count() != 2 {
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

func TestUnionDiff(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(3))

	c.Union(a, b)

	if c.count() != 3 {
		t.Error("Union of different sets yields set with wrong len")
	}

	if !c.Has(node(1)) || !c.Has(node(2)) || !c.Has(node(3)) {
		t.Error("Union of different sets yields set with wrong elements")
	}

	if a.Has(node(3)) || !a.Has(node(2)) || !a.Has(node(1)) || a.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !b.Has(node(3)) || b.Has(node(1)) || b.Has(node(2)) || b.count() != 1 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestUnionOverlapping(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(1))
	a.Add(node(2))

	b.Add(node(2))
	b.Add(node(3))

	c.Union(a, b)

	if c.count() != 3 {
		t.Error("Union of overlapping sets yields set with wrong len")
	}

	if !c.Has(node(1)) || !c.Has(node(2)) || !c.Has(node(3)) {
		t.Error("Union of overlapping sets yields set with wrong elements")
	}

	if a.Has(node(3)) || !a.Has(node(2)) || !a.Has(node(1)) || a.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !b.Has(node(3)) || b.Has(node(1)) || !b.Has(node(2)) || b.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Nodes{a, b, c} {
		for e, n := range s {
			if e != n.ID() {
				t.Errorf("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectSame(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(2))
	b.Add(node(3))

	c.Intersect(a, b)

	if card := c.count(); card != 2 {
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

func TestIntersectDiff(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(1))
	b.Add(node(4))

	c.Intersect(a, b)

	if card := c.count(); card != 0 {
		t.Errorf("Intersection of different yields non-empty set %d", card)
	}

	if !a.Has(node(2)) || !a.Has(node(3)) || a.Has(node(1)) || a.Has(node(4)) || a.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if b.Has(node(2)) || b.Has(node(3)) || !b.Has(node(1)) || !b.Has(node(4)) || b.count() != 2 {
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

func TestIntersectOverlapping(t *testing.T) {
	a := make(Nodes)
	b := make(Nodes)
	c := make(Nodes)

	a.Add(node(2))
	a.Add(node(3))

	b.Add(node(3))
	b.Add(node(4))

	c.Intersect(a, b)

	if card := c.count(); card != 1 {
		t.Errorf("Intersection of overlapping sets yields set of incorrect len %d", card)
	}

	if !c.Has(node(3)) {
		t.Errorf("Intersection of overlapping sets yields set with wrong element")
	}

	if !a.Has(node(2)) || !a.Has(node(3)) || a.Has(node(4)) || a.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if b.Has(node(2)) || !b.Has(node(3)) || !b.Has(node(4)) || b.count() != 2 {
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
