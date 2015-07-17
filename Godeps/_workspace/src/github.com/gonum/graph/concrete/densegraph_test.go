// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concrete_test

import (
	"math"
	"sort"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

var (
	_ graph.Graph    = (*concrete.UndirectedDenseGraph)(nil)
	_ graph.Directed = (*concrete.DirectedDenseGraph)(nil)
)

func TestBasicDenseImpassable(t *testing.T) {
	dg := concrete.NewUndirectedDenseGraph(5, false, math.Inf(1))
	if dg == nil {
		t.Fatal("Directed graph could not be made")
	}

	for i := 0; i < 5; i++ {
		if !dg.Has(concrete.Node(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(concrete.Node(i)); degree != 0 {
			t.Errorf("Node in impassable graph has a neighbor. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(concrete.Node(i)) {
			t.Errorf("Node exists that shouldn't: %d", i)
		}
	}
}

func TestBasicDensePassable(t *testing.T) {
	dg := concrete.NewUndirectedDenseGraph(5, true, math.Inf(1))
	if dg == nil {
		t.Fatal("Directed graph could not be made")
	}

	for i := 0; i < 5; i++ {
		if !dg.Has(concrete.Node(i)) {
			t.Errorf("Node that should exist doesn't: %d", i)
		}

		if degree := dg.Degree(concrete.Node(i)); degree != 4 {
			t.Errorf("Node in passable graph missing neighbors. Node: %d Degree: %d", i, degree)
		}
	}

	for i := 5; i < 10; i++ {
		if dg.Has(concrete.Node(i)) {
			t.Errorf("Node exists that shouldn't: %d", i)
		}
	}
}

func TestDirectedDenseAddRemove(t *testing.T) {
	dg := concrete.NewDirectedDenseGraph(10, false, math.Inf(1))
	dg.SetEdgeWeight(concrete.Edge{concrete.Node(0), concrete.Node(2)}, 1)

	if neighbors := dg.From(concrete.Node(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.Edge(concrete.Node(0), concrete.Node(2)) == nil {
		t.Errorf("Adding edge didn't create successor")
	}

	dg.RemoveEdge(concrete.Edge{concrete.Node(0), concrete.Node(2)})

	if neighbors := dg.From(concrete.Node(0)); len(neighbors) != 0 || dg.Edge(concrete.Node(0), concrete.Node(2)) != nil {
		t.Errorf("Removing edge didn't properly remove successor")
	}

	if neighbors := dg.To(concrete.Node(2)); len(neighbors) != 0 || dg.Edge(concrete.Node(0), concrete.Node(2)) != nil {
		t.Errorf("Removing directed edge wrongly kept predecessor")
	}

	dg.SetEdgeWeight(concrete.Edge{concrete.Node(0), concrete.Node(2)}, 2)
	// I figure we've torture tested From/To at this point
	// so we'll just use the bool functions now
	if dg.Edge(concrete.Node(0), concrete.Node(2)) == nil {
		t.Error("Adding directed edge didn't change successor back")
	} else if c1, c2 := dg.Weight(concrete.Edge{concrete.Node(2), concrete.Node(0)}), dg.Weight(concrete.Edge{concrete.Node(0), concrete.Node(2)}); math.Abs(c1-c2) < .000001 {
		t.Error("Adding directed edge affected cost in undirected manner")
	}
}

func TestUndirectedDenseAddRemove(t *testing.T) {
	dg := concrete.NewUndirectedDenseGraph(10, false, math.Inf(1))
	dg.SetEdgeWeight(concrete.Edge{concrete.Node(0), concrete.Node(2)}, 1)

	if neighbors := dg.From(concrete.Node(0)); len(neighbors) != 1 || neighbors[0].ID() != 2 ||
		dg.EdgeBetween(concrete.Node(0), concrete.Node(2)) == nil {
		t.Errorf("Couldn't add neighbor")
	}

	if neighbors := dg.From(concrete.Node(2)); len(neighbors) != 1 || neighbors[0].ID() != 0 ||
		dg.EdgeBetween(concrete.Node(2), concrete.Node(0)) == nil {
		t.Errorf("Adding an undirected neighbor didn't add it reciprocally")
	}
}

type nodeSorter []graph.Node

func (ns nodeSorter) Len() int {
	return len(ns)
}

func (ns nodeSorter) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}

func (ns nodeSorter) Less(i, j int) bool {
	return ns[i].ID() < ns[j].ID()
}

func TestDenseLists(t *testing.T) {
	dg := concrete.NewDirectedDenseGraph(15, true, math.Inf(1))
	nodes := nodeSorter(dg.Nodes())

	if len(nodes) != 15 {
		t.Fatalf("Wrong number of nodes")
	}

	sort.Sort(nodes)

	for i, node := range dg.Nodes() {
		if i != node.ID() {
			t.Errorf("Node list doesn't return properly id'd nodes")
		}
	}

	edges := dg.Edges()
	if len(edges) != 15*14 {
		t.Errorf("Improper number of edges for passable dense graph")
	}

	dg.RemoveEdge(concrete.Edge{concrete.Node(12), concrete.Node(11)})
	edges = dg.Edges()
	if len(edges) != (15*14)-1 {
		t.Errorf("Removing edge didn't affect edge listing properly")
	}
}
