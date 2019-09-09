// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multi_test

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/testgraph"
)

func directedBuilder(nodes []graph.Node, edges []testgraph.WeightedLine, _, _ float64) (g graph.Graph, n []graph.Node, e []testgraph.Edge, s, a float64, ok bool) {
	seen := set.NewNodes()
	dg := multi.NewDirectedGraph()
	for _, n := range nodes {
		seen.Add(n)
		dg.AddNode(n)
	}
	for _, edge := range edges {
		f := dg.Node(edge.From().ID())
		if f == nil {
			f = edge.From()
		}
		t := dg.Node(edge.To().ID())
		if t == nil {
			t = edge.To()
		}
		cl := multi.Line{F: f, T: t, UID: edge.ID()}
		seen.Add(cl.F)
		seen.Add(cl.T)
		e = append(e, cl)
		dg.SetLine(cl)
	}
	if len(seen) != 0 {
		n = make([]graph.Node, 0, len(seen))
	}
	for _, sn := range seen {
		n = append(n, sn)
	}
	return dg, n, e, math.NaN(), math.NaN(), true
}

func TestDirected(t *testing.T) {
	t.Run("EdgeExistence", func(t *testing.T) {
		testgraph.EdgeExistence(t, directedBuilder)
	})
	t.Run("LineExistence", func(t *testing.T) {
		testgraph.LineExistence(t, directedBuilder, true)
	})
	t.Run("NodeExistence", func(t *testing.T) {
		testgraph.NodeExistence(t, directedBuilder)
	})
	t.Run("ReturnAdjacentNodes", func(t *testing.T) {
		testgraph.ReturnAdjacentNodes(t, directedBuilder, true)
	})
	t.Run("ReturnAllLines", func(t *testing.T) {
		testgraph.ReturnAllLines(t, directedBuilder, true)
	})
	t.Run("ReturnAllNodes", func(t *testing.T) {
		testgraph.ReturnAllNodes(t, directedBuilder, true)
	})
	t.Run("ReturnNodeSlice", func(t *testing.T) {
		testgraph.ReturnNodeSlice(t, directedBuilder, true)
	})

	t.Run("AddNodes", func(t *testing.T) {
		testgraph.AddNodes(t, multi.NewDirectedGraph(), 100)
	})
	t.Run("AddArbitraryNodes", func(t *testing.T) {
		testgraph.AddArbitraryNodes(t,
			multi.NewDirectedGraph(),
			testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return multi.Node(id) }),
		)
	})
	t.Run("RemoveNodes", func(t *testing.T) {
		g := multi.NewDirectedGraph()
		it := testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return multi.Node(id) })
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
				d--
				g.SetLine(g.NewLine(u, v))
			}
		}
		testgraph.RemoveNodes(t, g)
	})
	t.Run("AddLines", func(t *testing.T) {
		testgraph.AddLines(t, 100,
			multi.NewDirectedGraph(),
			func(id int64) graph.Node { return multi.Node(id) },
			true, // Can update nodes.
		)
	})
	t.Run("RemoveLines", func(t *testing.T) {
		g := multi.NewDirectedGraph()
		it := testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return multi.Node(id) })
		for it.Next() {
			g.AddNode(it.Node())
		}
		it.Reset()
		var lines []graph.Line
		rnd := rand.New(rand.NewSource(1))
		for it.Next() {
			u := it.Node()
			d := rnd.Intn(5)
			vit := g.Nodes()
			for d >= 0 && vit.Next() {
				v := vit.Node()
				d--
				l := g.NewLine(u, v)
				g.SetLine(l)
				lines = append(lines, l)
			}
		}
		rnd.Shuffle(len(lines), func(i, j int) {
			lines[i], lines[j] = lines[j], lines[i]
		})
		testgraph.RemoveLines(t, g, iterator.NewOrderedLines(lines))
	})
}

// Tests Issue #27
func TestEdgeOvercounting(t *testing.T) {
	g := generateDummyGraph()

	if neigh := graph.NodesOf(g.From(int64(2))); len(neigh) != 2 {
		t.Errorf("Node 2 has incorrect number of neighbors got neighbors %v (count %d), expected 2 neighbors {0,1}", neigh, len(neigh))
	}
}

func generateDummyGraph() *multi.DirectedGraph {
	nodes := [4]struct{ srcID, targetID int }{
		{2, 1},
		{1, 0},
		{2, 0},
		{0, 2},
	}

	g := multi.NewDirectedGraph()

	for i, n := range nodes {
		g.SetLine(multi.Line{F: multi.Node(n.srcID), T: multi.Node(n.targetID), UID: int64(i)})
	}

	return g
}

// Test for issue #123 https://github.com/gonum/graph/issues/123
func TestIssue123DirectedGraph(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	g := multi.NewDirectedGraph()

	n0 := g.NewNode()
	g.AddNode(n0)

	n1 := g.NewNode()
	g.AddNode(n1)

	g.RemoveNode(n0.ID())

	n2 := g.NewNode()
	g.AddNode(n2)
}
