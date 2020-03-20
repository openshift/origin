// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iterator_test

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

type edge struct{ f, t int }

func (e edge) From() graph.Node         { return simple.Node(e.f) }
func (e edge) To() graph.Node           { return simple.Node(e.t) }
func (e edge) ReversedEdge() graph.Edge { return edge{f: e.t, t: e.f} }

var orderedEdgesTests = []struct {
	edges []graph.Edge
}{
	{edges: nil},
	{edges: []graph.Edge{edge{f: 1, t: 2}}},
	{edges: []graph.Edge{edge{f: 1, t: 2}, edge{f: 2, t: 3}, edge{f: 3, t: 4}, edge{f: 4, t: 5}}},
	{edges: []graph.Edge{edge{f: 5, t: 4}, edge{f: 4, t: 3}, edge{f: 3, t: 2}, edge{f: 2, t: 1}}},
}

func TestOrderedEdgesIterate(t *testing.T) {
	for _, test := range orderedEdgesTests {
		it := iterator.NewOrderedEdges(test.edges)
		for i := 0; i < 2; i++ {
			if it.Len() != len(test.edges) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.edges))
			}
			var got []graph.Edge
			for it.Next() {
				got = append(got, it.Edge())
			}
			want := test.edges
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

func TestOrderedEdgesSlice(t *testing.T) {
	for _, test := range orderedEdgesTests {
		it := iterator.NewOrderedEdges(test.edges)
		for i := 0; i < 2; i++ {
			got := it.EdgeSlice()
			want := test.edges
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

type weightedEdge struct {
	f, t int
	w    float64
}

func (e weightedEdge) From() graph.Node         { return simple.Node(e.f) }
func (e weightedEdge) To() graph.Node           { return simple.Node(e.t) }
func (e weightedEdge) ReversedEdge() graph.Edge { e.f, e.t = e.t, e.f; return e }
func (e weightedEdge) Weight() float64          { return e.w }

var orderedWeightedEdgesTests = []struct {
	edges []graph.WeightedEdge
}{
	{edges: nil},
	{edges: []graph.WeightedEdge{weightedEdge{f: 1, t: 2, w: 1}}},
	{edges: []graph.WeightedEdge{weightedEdge{f: 1, t: 2, w: 1}, weightedEdge{f: 2, t: 3, w: 2}, weightedEdge{f: 3, t: 4, w: 3}, weightedEdge{f: 4, t: 5, w: 4}}},
	{edges: []graph.WeightedEdge{weightedEdge{f: 5, t: 4, w: 4}, weightedEdge{f: 4, t: 3, w: 3}, weightedEdge{f: 3, t: 2, w: 2}, weightedEdge{f: 2, t: 1, w: 1}}},
}

func TestOrderedWeightedEdgesIterate(t *testing.T) {
	for _, test := range orderedWeightedEdgesTests {
		it := iterator.NewOrderedWeightedEdges(test.edges)
		for i := 0; i < 2; i++ {
			if it.Len() != len(test.edges) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.edges))
			}
			var got []graph.WeightedEdge
			for it.Next() {
				got = append(got, it.WeightedEdge())
			}
			want := test.edges
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

func TestOrderedWeightedEdgesSlice(t *testing.T) {
	for _, test := range orderedWeightedEdgesTests {
		it := iterator.NewOrderedWeightedEdges(test.edges)
		for i := 0; i < 2; i++ {
			got := it.WeightedEdgeSlice()
			want := test.edges
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}
