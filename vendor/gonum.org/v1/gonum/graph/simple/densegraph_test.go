// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"math"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
)

var (
	directedMatrix = (*DirectedMatrix)(nil)

	_ graph.Graph            = directedMatrix
	_ graph.Directed         = directedMatrix
	_ graph.WeightedDirected = directedMatrix

	undirectedMatrix = (*UndirectedMatrix)(nil)

	_ graph.Graph              = undirectedMatrix
	_ graph.Undirected         = undirectedMatrix
	_ graph.WeightedUndirected = undirectedMatrix
)

func TestBasicDenseImpassable(t *testing.T) {
	dg := NewUndirectedMatrix(5, math.Inf(1), 0, math.Inf(1))
	if dg == nil {
		t.Fatal("Directed graph could not be made")
	}

	for i := 0; i < 5; i++ {
		if !dg.Has(int64(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(int64(i)); degree != 0 {
			t.Errorf("Node in impassable graph has a neighbor. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(int64(i)) {
			t.Errorf("Node exists that shouldn't: %d", i)
		}
	}
}

func TestBasicDensePassable(t *testing.T) {
	dg := NewUndirectedMatrix(5, 1, 0, math.Inf(1))
	if dg == nil {
		t.Fatal("Directed graph could not be made")
	}

	for i := 0; i < 5; i++ {
		if !dg.Has(int64(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(int64(i)); degree != 4 {
			t.Errorf("Node in passable graph missing neighbors. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(int64(i)) {
			t.Errorf("Node exists that shouldn't: %d", i)
		}
	}
}

func TestDirectedDenseAddRemove(t *testing.T) {
	dg := NewDirectedMatrix(10, math.Inf(1), 0, math.Inf(1))
	dg.SetWeightedEdge(WeightedEdge{F: Node(0), T: Node(2), W: 1})

	if neighbors := dg.From(int64(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.Edge(int64(0), int64(2)) == nil {
		t.Errorf("Adding edge didn't create successor")
	}

	dg.RemoveEdge(int64(0), int64(2))

	if neighbors := dg.From(int64(0)); len(neighbors) != 0 || dg.Edge(int64(0), int64(2)) != nil {
		t.Errorf("Removing edge didn't properly remove successor")
	}

	if neighbors := dg.To(int64(2)); len(neighbors) != 0 || dg.Edge(int64(0), int64(2)) != nil {
		t.Errorf("Removing directed edge wrongly kept predecessor")
	}

	dg.SetWeightedEdge(WeightedEdge{F: Node(0), T: Node(2), W: 2})
	// I figure we've torture tested From/To at this point
	// so we'll just use the bool functions now
	if dg.Edge(int64(0), int64(2)) == nil {
		t.Fatal("Adding directed edge didn't change successor back")
	}
	c1, _ := dg.Weight(int64(2), int64(0))
	c2, _ := dg.Weight(int64(0), int64(2))
	if c1 == c2 {
		t.Error("Adding directed edge affected cost in undirected manner")
	}
}

func TestUndirectedDenseAddRemove(t *testing.T) {
	dg := NewUndirectedMatrix(10, math.Inf(1), 0, math.Inf(1))
	dg.SetEdge(Edge{F: Node(0), T: Node(2)})

	if neighbors := dg.From(int64(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.EdgeBetween(int64(0), int64(2)) == nil {
		t.Errorf("Couldn't add neighbor")
	}

	if neighbors := dg.From(int64(2)); len(neighbors) != 1 || neighbors[0].ID() != 0 ||
		dg.EdgeBetween(int64(2), int64(0)) == nil {
		t.Errorf("Adding an undirected neighbor didn't add it reciprocally")
	}
}

func TestDenseLists(t *testing.T) {
	dg := NewDirectedMatrix(15, 1, 0, math.Inf(1))
	nodes := dg.Nodes()

	if len(nodes) != 15 {
		t.Fatalf("Wrong number of nodes")
	}

	sort.Sort(ordered.ByID(nodes))

	for i, node := range dg.Nodes() {
		if int64(i) != node.ID() {
			t.Errorf("Node list doesn't return properly id'd nodes")
		}
	}

	edges := dg.Edges()
	if len(edges) != 15*14 {
		t.Errorf("Improper number of edges for passable dense graph")
	}

	dg.RemoveEdge(int64(12), int64(11))
	edges = dg.Edges()
	if len(edges) != (15*14)-1 {
		t.Errorf("Removing edge didn't affect edge listing properly")
	}
}
