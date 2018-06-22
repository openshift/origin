// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"math"
	"sort"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/internal/ordered"
)

var (
	_ graph.Graph    = (*UndirectedMatrix)(nil)
	_ graph.Directed = (*DirectedMatrix)(nil)
)

func TestBasicDenseImpassable(t *testing.T) {
	dg := NewUndirectedMatrix(5, math.Inf(1), 0, math.Inf(1))
	if dg == nil {
		t.Fatal("Directed graph could not be made")
	}

	for i := 0; i < 5; i++ {
		if !dg.Has(Node(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(Node(i)); degree != 0 {
			t.Errorf("Node in impassable graph has a neighbor. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(Node(i)) {
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
		if !dg.Has(Node(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(Node(i)); degree != 4 {
			t.Errorf("Node in passable graph missing neighbors. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(Node(i)) {
			t.Errorf("Node exists that shouldn't: %d", i)
		}
	}
}

func TestDirectedDenseAddRemove(t *testing.T) {
	dg := NewDirectedMatrix(10, math.Inf(1), 0, math.Inf(1))
	dg.SetEdge(Edge{F: Node(0), T: Node(2), W: 1})

	if neighbors := dg.From(Node(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.Edge(Node(0), Node(2)) == nil {
		t.Errorf("Adding edge didn't create successor")
	}

	dg.RemoveEdge(Edge{F: Node(0), T: Node(2)})

	if neighbors := dg.From(Node(0)); len(neighbors) != 0 || dg.Edge(Node(0), Node(2)) != nil {
		t.Errorf("Removing edge didn't properly remove successor")
	}

	if neighbors := dg.To(Node(2)); len(neighbors) != 0 || dg.Edge(Node(0), Node(2)) != nil {
		t.Errorf("Removing directed edge wrongly kept predecessor")
	}

	dg.SetEdge(Edge{F: Node(0), T: Node(2), W: 2})
	// I figure we've torture tested From/To at this point
	// so we'll just use the bool functions now
	if dg.Edge(Node(0), Node(2)) == nil {
		t.Fatal("Adding directed edge didn't change successor back")
	}
	c1, _ := dg.Weight(Node(2), Node(0))
	c2, _ := dg.Weight(Node(0), Node(2))
	if c1 == c2 {
		t.Error("Adding directed edge affected cost in undirected manner")
	}
}

func TestUndirectedDenseAddRemove(t *testing.T) {
	dg := NewUndirectedMatrix(10, math.Inf(1), 0, math.Inf(1))
	dg.SetEdge(Edge{F: Node(0), T: Node(2)})

	if neighbors := dg.From(Node(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.EdgeBetween(Node(0), Node(2)) == nil {
		t.Errorf("Couldn't add neighbor")
	}

	if neighbors := dg.From(Node(2)); len(neighbors) != 1 || neighbors[0].ID() != 0 ||
		dg.EdgeBetween(Node(2), Node(0)) == nil {
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
		if i != node.ID() {
			t.Errorf("Node list doesn't return properly id'd nodes")
		}
	}

	edges := dg.Edges()
	if len(edges) != 15*14 {
		t.Errorf("Improper number of edges for passable dense graph")
	}

	dg.RemoveEdge(Edge{F: Node(12), T: Node(11)})
	edges = dg.Edges()
	if len(edges) != (15*14)-1 {
		t.Errorf("Removing edge didn't affect edge listing properly")
	}
}
