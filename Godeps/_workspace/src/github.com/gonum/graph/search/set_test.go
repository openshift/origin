// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package search

import (
	"testing"

	"github.com/gonum/graph"
)

type node int

func (n node) ID() int { return int(n) }

// count reports the number of elements stored in the set.
func (s Set) count() int {
	return len(s)
}

// remove delete the specified element from the set.
func (s Set) remove(n graph.Node) {
	delete(s, n.ID())
}

// TestSame tests the assumption that pointer equality via unsafe conversion
// of a map[int]struct{} to uintptr is a valid test for perfect identity between
// set values. If any of the tests in TestSame fail, the package is broken and same
// must be reimplemented to conform to the runtime map implementation. The relevant
// code to look at (at least for gc) is in runtime/hashmap.{h,goc}.
func TestSame(t *testing.T) {
	var (
		a = make(Set)
		b = make(Set)
		c = a
	)

	if same(a, b) {
		t.Error("Independently created sets test as same")
	}
	if !same(a, c) {
		t.Error("Set copy and original test as not same.")
	}
	a.add(node(1))
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
	s := make(Set)
	if s == nil {
		t.Fatal("Set cannot be created successfully")
	}

	if s.count() != 0 {
		t.Error("Set somehow contains new elements upon creation")
	}

	s.add(node(1))
	s.add(node(3))
	s.add(node(5))

	if s.count() != 3 {
		t.Error("Incorrect number of set elements after adding")
	}

	if !s.has(node(1)) || !s.has(node(3)) || !s.has(node(5)) {
		t.Error("Set doesn't contain element that was added")
	}

	s.add(node(1))

	if s.count() > 3 {
		t.Error("Set double-adds element (element not unique)")
	} else if s.count() < 3 {
		t.Error("Set double-add lowered len")
	}

	if !s.has(node(1)) {
		t.Error("Set doesn't contain double-added element")
	}

	if !s.has(node(3)) || !s.has(node(5)) {
		t.Error("Set removes element on double-add")
	}

	for e, n := range s {
		if e != n.ID() {
			t.Error("Element ID did not match key: %d != %d", e, n.ID())
		}
	}
}

func TestRemove(t *testing.T) {
	s := make(Set)

	s.add(node(1))
	s.add(node(3))
	s.add(node(5))

	s.remove(node(1))

	if s.count() != 2 {
		t.Error("Incorrect number of set elements after removing an element")
	}

	if s.has(node(1)) {
		t.Error("Element present after removal")
	}

	if !s.has(node(3)) || !s.has(node(5)) {
		t.Error("Set remove removed wrong element")
	}

	s.remove(node(1))

	if s.count() != 2 || s.has(node(1)) {
		t.Error("Double set remove does something strange")
	}

	s.add(node(1))

	if s.count() != 3 || !s.has(node(1)) {
		t.Error("Cannot add element after removal")
	}
}

func TestClear(t *testing.T) {
	s := make(Set)

	s.add(node(8))
	s.add(node(9))
	s.add(node(10))

	s = clear(s)

	if s.count() != 0 {
		t.Error("Clear did not properly reset set to size 0")
	}
}

func TestSelfEqual(t *testing.T) {
	s := make(Set)

	if !equal(s, s) {
		t.Error("Set is not equal to itself")
	}

	s.add(node(1))

	if !equal(s, s) {
		t.Error("Set ceases self equality after adding element")
	}
}

func TestEqual(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)

	if !equal(s1, s2) {
		t.Error("Two different empty sets not equal")
	}

	s1.add(node(1))
	if equal(s1, s2) {
		t.Error("Two different sets with different elements not equal")
	}

	s2.add(node(1))
	if !equal(s1, s2) {
		t.Error("Two sets with same element not equal")
	}
}

func TestCopy(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)

	s1.add(node(1))
	s1.add(node(2))
	s1.add(node(3))

	s2.copy(s1)

	if !equal(s1, s2) {
		t.Fatalf("Two sets not equal after copy")
	}

	s2.remove(node(1))

	if equal(s1, s2) {
		t.Errorf("Mutating one set mutated another after copy")
	}
}

func TestSelfCopy(t *testing.T) {
	s1 := make(Set)

	s1.add(node(1))
	s1.add(node(2))

	s1.copy(s1)

	if s1.count() != 2 {
		t.Error("Something strange happened when copying into self")
	}
}

func TestUnionSame(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(1))
	s1.add(node(2))

	s2.add(node(1))
	s2.add(node(2))

	s3.union(s1, s2)

	if s3.count() != 2 {
		t.Error("Union of same sets yields set with wrong len")
	}

	if !s3.has(node(1)) || !s3.has(node(2)) {
		t.Error("Union of same sets yields wrong elements")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestUnionDiff(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(1))
	s1.add(node(2))

	s2.add(node(3))

	s3.union(s1, s2)

	if s3.count() != 3 {
		t.Error("Union of different sets yields set with wrong len")
	}

	if !s3.has(node(1)) || !s3.has(node(2)) || !s3.has(node(3)) {
		t.Error("Union of different sets yields set with wrong elements")
	}

	if s1.has(node(3)) || !s1.has(node(2)) || !s1.has(node(1)) || s1.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !s2.has(node(3)) || s2.has(node(1)) || s2.has(node(2)) || s2.count() != 1 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestUnionOverlapping(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(1))
	s1.add(node(2))

	s2.add(node(2))
	s2.add(node(3))

	s3.union(s1, s2)

	if s3.count() != 3 {
		t.Error("Union of overlapping sets yields set with wrong len")
	}

	if !s3.has(node(1)) || !s3.has(node(2)) || !s3.has(node(3)) {
		t.Error("Union of overlapping sets yields set with wrong elements")
	}

	if s1.has(node(3)) || !s1.has(node(2)) || !s1.has(node(1)) || s1.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 1)")
	}

	if !s2.has(node(3)) || s2.has(node(1)) || !s2.has(node(2)) || s2.count() != 2 {
		t.Error("Union of sets mutates non-destination set (argument 2)")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectSame(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(2))
	s1.add(node(3))

	s2.add(node(2))
	s2.add(node(3))

	s3.intersect(s1, s2)

	if card := s3.count(); card != 2 {
		t.Errorf("Intersection of identical sets yields set of wrong len %d", card)
	}

	if !s3.has(node(2)) || !s3.has(node(3)) {
		t.Error("Intersection of identical sets yields set of wrong elements")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectDiff(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(2))
	s1.add(node(3))

	s2.add(node(1))
	s2.add(node(4))

	s3.intersect(s1, s2)

	if card := s3.count(); card != 0 {
		t.Errorf("Intersection of different yields non-empty set %d", card)
	}

	if !s1.has(node(2)) || !s1.has(node(3)) || s1.has(node(1)) || s1.has(node(4)) || s1.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if s2.has(node(2)) || s2.has(node(3)) || !s2.has(node(1)) || !s2.has(node(4)) || s2.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}

func TestIntersectOverlapping(t *testing.T) {
	s1 := make(Set)
	s2 := make(Set)
	s3 := make(Set)

	s1.add(node(2))
	s1.add(node(3))

	s2.add(node(3))
	s2.add(node(4))

	s3.intersect(s1, s2)

	if card := s3.count(); card != 1 {
		t.Errorf("Intersection of overlapping sets yields set of incorrect len %d", card)
	}

	if !s3.has(node(3)) {
		t.Errorf("Intersection of overlapping sets yields set with wrong element")
	}

	if !s1.has(node(2)) || !s1.has(node(3)) || s1.has(node(4)) || s1.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	if s2.has(node(2)) || !s2.has(node(3)) || !s2.has(node(4)) || s2.count() != 2 {
		t.Error("Intersection of sets mutates non-destination set (argument 1)")
	}

	for i, s := range []Set{s1, s2, s3} {
		for e, n := range s {
			if e != n.ID() {
				t.Error("Element ID did not match key in s%d: %d != %d", i+1, e, n.ID())
			}
		}
	}
}
