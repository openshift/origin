// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dfa

import (
	"fmt"
	"maps"
	"slices"
)

// A Semilattice describes a bounded semilattice over Elem.
// That is, a partial order over values of type Elem, with a binary
// Merge operator and an identity element.
//
// This is typically implemented by a stateless type, and acts as a factory for
// lattice elements.
type Semilattice[Elem any] interface {
	// Ident returns the identity element of this lattice, that is the unit of
	// the Merge operation.
	Ident() Elem

	// Equals returns whether a and b are the same element.
	Equals(a, b Elem) bool

	// Merge combines two lattice values, such as the two possible values of a
	// variable at the end of an if/else statement.
	//
	// Merge must satisfy the following identities, where we use ∧ for Merge, =
	// for Equals, and 𝟏 for Ident:
	//
	// - Associativity: x ∧ (y ∧ z) = (x ∧ y) ∧ z
	// - Commutativity: x ∧ y = y ∧ x
	// - Idempotency: x ∧ x = x
	// - Identity: x ∧ 𝟏 = x
	Merge(a, b Elem) Elem
}

// A MapLattice implements [Semilattice][map[Key]Elem]. The values in the map
// are themselves defined by [Semilattice] L.
//
// Any elements missing from the map are implicitly L's identity element, and
// L's identity element never appears as a value in the map.
//
// For densely numbered keys, consider using [DenseMapLattice] instead.
type MapLattice[Key comparable, Elem any, L Semilattice[Elem]] struct {
	l L
}

func (m MapLattice[Key, Elem, L]) Ident() map[Key]Elem {
	return nil
}

func (m MapLattice[Key, Elem, L]) Equals(a, b map[Key]Elem) bool {
	return maps.EqualFunc(a, b, m.l.Equals)
}

func (m MapLattice[Key, Elem, L]) Merge(a, b map[Key]Elem) map[Key]Elem {
	if len(a) == 0 {
		return b
	} else if len(b) == 0 {
		return a
	}

	// We need to consider the union of keys in a and b.
	out := make(map[Key]Elem)
	id := m.l.Ident()
	for k, av := range a {
		bv, ok := b[k]
		if !ok {
			// Because Merge(x, Ident()) == x, we can skip calling L.Merge.
			out[k] = av
			continue
		}

		w := m.l.Merge(av, bv)
		if m.l.Equals(w, id) {
			// In a semilattice, Merge(x, y) = Ident is only possible when x ==
			// Ident and y == Ident.
			panic(fmt.Sprintf(
				"%T is not a semilattice: Merge(%v, %v) returned Ident for non-Ident arguments",
				m.l, av, bv))
		}
		out[k] = w
	}
	// We considered keys that are only in a, and in both a and b. Now we just
	// need to handle keys that are only in b.
	for k, v2 := range b {
		if _, ok := a[k]; !ok {
			out[k] = v2
		}
	}

	return out
}

// A DenseMapLattice implements [Semilattice][[]Elem]. It is like a [MapLattice]
// that is indexed by integers. The values in the map are themselves defined by
// [Semilattice] L.
//
// Unlike [MapLattice], L's identity element may appear as a value in the map,
// to allow for gaps in the numbering of keys when the identity element is
// Elem's zero value.
type DenseMapLattice[Elem any, L Semilattice[Elem]] struct {
	l L
}

func (s DenseMapLattice[Elem, L]) Ident() []Elem {
	return nil
}

func (s DenseMapLattice[Elem, L]) Equals(a, b []Elem) bool {
	nmin := min(len(a), len(b))
	ident := s.l.Ident()

	// Check that up to nmin, all elements in a and b match. If one of a or b
	// is longer, then its tail nmin:nmax must only contain identity elements.
	return slices.EqualFunc(a[:nmin], b[:nmin], s.l.Equals) &&
		!slices.ContainsFunc(a[nmin:], func(e Elem) bool {
			return !s.l.Equals(e, ident)
		}) &&
		!slices.ContainsFunc(b[nmin:], func(e Elem) bool {
			return !s.l.Equals(e, ident)
		})
}

func (s DenseMapLattice[Elem, L]) Merge(a, b []Elem) []Elem {
	if len(a) == 0 {
		return b
	} else if len(b) == 0 {
		return a
	}
	out := make([]Elem, max(len(a), len(b)))
	for k := range max(len(a), len(b)) {
		av := s.l.Ident()
		bv := s.l.Ident()
		if k < len(a) {
			av = a[k]
		}
		if k < len(b) {
			bv = b[k]
		}
		out[k] = s.l.Merge(av, bv)
	}
	return out
}
