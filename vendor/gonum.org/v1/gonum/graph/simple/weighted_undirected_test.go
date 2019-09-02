// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple_test

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/testgraph"
)

func weightedUndirectedBuilder(nodes []graph.Node, edges []testgraph.WeightedLine, self, absent float64) (g graph.Graph, n []graph.Node, e []testgraph.Edge, s, a float64, ok bool) {
	seen := set.NewNodes()
	ug := simple.NewWeightedUndirectedGraph(self, absent)
	for _, n := range nodes {
		seen.Add(n)
		ug.AddNode(n)
	}
	for _, edge := range edges {
		if edge.From().ID() == edge.To().ID() {
			continue
		}
		f := ug.Node(edge.From().ID())
		if f == nil {
			f = edge.From()
		}
		t := ug.Node(edge.To().ID())
		if t == nil {
			t = edge.To()
		}
		ce := simple.WeightedEdge{F: f, T: t, W: edge.Weight()}
		seen.Add(ce.F)
		seen.Add(ce.T)
		e = append(e, ce)
		ug.SetWeightedEdge(ce)
	}
	if len(e) == 0 && len(edges) != 0 {
		return nil, nil, nil, math.NaN(), math.NaN(), false
	}
	if len(seen) != 0 {
		n = make([]graph.Node, 0, len(seen))
	}
	for _, sn := range seen {
		n = append(n, sn)
	}
	return ug, n, e, self, absent, true
}

func TestWeightedUndirected(t *testing.T) {
	t.Run("EdgeExistence", func(t *testing.T) {
		testgraph.EdgeExistence(t, weightedUndirectedBuilder)
	})
	t.Run("NodeExistence", func(t *testing.T) {
		testgraph.NodeExistence(t, weightedUndirectedBuilder)
	})
	t.Run("ReturnAdjacentNodes", func(t *testing.T) {
		testgraph.ReturnAdjacentNodes(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllEdges", func(t *testing.T) {
		testgraph.ReturnAllEdges(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllNodes", func(t *testing.T) {
		testgraph.ReturnAllNodes(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllWeightedEdges", func(t *testing.T) {
		testgraph.ReturnAllWeightedEdges(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnEdgeSlice", func(t *testing.T) {
		testgraph.ReturnEdgeSlice(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnWeightedEdgeSlice", func(t *testing.T) {
		testgraph.ReturnWeightedEdgeSlice(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnNodeSlice", func(t *testing.T) {
		testgraph.ReturnNodeSlice(t, weightedUndirectedBuilder, true)
	})
	t.Run("Weight", func(t *testing.T) {
		testgraph.Weight(t, weightedUndirectedBuilder)
	})

	t.Run("AddNodes", func(t *testing.T) {
		testgraph.AddNodes(t, simple.NewWeightedUndirectedGraph(1, 0), 100)
	})
	t.Run("AddArbitraryNodes", func(t *testing.T) {
		testgraph.AddArbitraryNodes(t,
			simple.NewWeightedUndirectedGraph(1, 0),
			testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return simple.Node(id) }),
		)
	})
	t.Run("AddWeightedEdges", func(t *testing.T) {
		testgraph.AddWeightedEdges(t, 100,
			simple.NewWeightedUndirectedGraph(1, 0),
			0.5,
			func(id int64) graph.Node { return simple.Node(id) },
			false, // Cannot set self-loops.
			true,  // Can update nodes.
		)
	})
	t.Run("NoLoopAddWeightedEdges", func(t *testing.T) {
		testgraph.NoLoopAddWeightedEdges(t, 100,
			simple.NewWeightedUndirectedGraph(1, 0),
			0.5,
			func(id int64) graph.Node { return simple.Node(id) },
		)
	})
	t.Run("RemoveNodes", func(t *testing.T) {
		g := simple.NewWeightedUndirectedGraph(1, 0)
		it := testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return simple.Node(id) })
		for it.Next() {
			g.AddNode(it.Node())
		}
		it.Reset()
		rnd := rand.New(rand.NewSource(1))
		for it.Next() {
			u := it.Node()
			d := rnd.Intn(5)
			vit := g.Nodes()
			for d >= 0 && vit.Next() {
				v := vit.Node()
				if v.ID() == u.ID() {
					continue
				}
				d--
				g.SetWeightedEdge(g.NewWeightedEdge(u, v, 1))
			}
		}
		testgraph.RemoveNodes(t, g)
	})
	t.Run("RemoveEdges", func(t *testing.T) {
		g := simple.NewWeightedUndirectedGraph(1, 0)
		it := testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return simple.Node(id) })
		for it.Next() {
			g.AddNode(it.Node())
		}
		it.Reset()
		rnd := rand.New(rand.NewSource(1))
		for it.Next() {
			u := it.Node()
			d := rnd.Intn(5)
			vit := g.Nodes()
			for d >= 0 && vit.Next() {
				v := vit.Node()
				if v.ID() == u.ID() {
					continue
				}
				d--
				g.SetWeightedEdge(g.NewWeightedEdge(u, v, 1))
			}
		}
		testgraph.RemoveEdges(t, g, g.Edges())
	})
}

func TestAssertWeightedMutableNotDirected(t *testing.T) {
	var g graph.UndirectedWeightedBuilder = simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	if _, ok := g.(graph.Directed); ok {
		t.Fatal("Graph is directed, but a MutableGraph cannot safely be directed!")
	}
}

func TestWeightedMaxID(t *testing.T) {
	g := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	nodes := make(map[graph.Node]struct{})
	for i := simple.Node(0); i < 3; i++ {
		g.AddNode(i)
		nodes[i] = struct{}{}
	}
	g.RemoveNode(int64(0))
	delete(nodes, simple.Node(0))
	g.RemoveNode(int64(2))
	delete(nodes, simple.Node(2))
	n := g.NewNode()
	g.AddNode(n)
	if g.Node(n.ID()) == nil {
		t.Error("added node does not exist in graph")
	}
	if _, exists := nodes[n]; exists {
		t.Errorf("Created already existing node id: %v", n.ID())
	}
}

// Test for issue #123 https://github.com/gonum/graph/issues/123
func TestIssue123WeightedUndirectedGraph(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	g := simple.NewWeightedUndirectedGraph(0, math.Inf(1))

	n0 := g.NewNode()
	g.AddNode(n0)

	n1 := g.NewNode()
	g.AddNode(n1)

	g.RemoveNode(n0.ID())

	n2 := g.NewNode()
	g.AddNode(n2)
}
