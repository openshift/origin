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

func undirectedBuilder(nodes []graph.Node, edges []testgraph.WeightedLine, _, _ float64) (g graph.Graph, n []graph.Node, e []testgraph.Edge, s, a float64, ok bool) {
	seen := set.NewNodes()
	ug := multi.NewUndirectedGraph()
	for _, n := range nodes {
		seen.Add(n)
		ug.AddNode(n)
	}
	for _, edge := range edges {
		f := ug.Node(edge.From().ID())
		if f == nil {
			f = edge.From()
		}
		t := ug.Node(edge.To().ID())
		if t == nil {
			t = edge.To()
		}
		ce := multi.Line{F: f, T: t, UID: edge.ID()}
		seen.Add(ce.F)
		seen.Add(ce.T)
		e = append(e, ce)
		ug.SetLine(ce)
	}
	if len(seen) != 0 {
		n = make([]graph.Node, 0, len(seen))
	}
	for _, sn := range seen {
		n = append(n, sn)
	}
	return ug, n, e, math.NaN(), math.NaN(), true
}

func TestUndirected(t *testing.T) {
	t.Run("EdgeExistence", func(t *testing.T) {
		testgraph.EdgeExistence(t, undirectedBuilder)
	})
	t.Run("LineExistence", func(t *testing.T) {
		testgraph.LineExistence(t, directedBuilder, true)
	})
	t.Run("NodeExistence", func(t *testing.T) {
		testgraph.NodeExistence(t, undirectedBuilder)
	})
	t.Run("ReturnAdjacentNodes", func(t *testing.T) {
		testgraph.ReturnAdjacentNodes(t, undirectedBuilder, true)
	})
	t.Run("ReturnAllLines", func(t *testing.T) {
		testgraph.ReturnAllLines(t, undirectedBuilder, true)
	})
	t.Run("ReturnAllNodes", func(t *testing.T) {
		testgraph.ReturnAllNodes(t, undirectedBuilder, true)
	})
	t.Run("ReturnNodeSlice", func(t *testing.T) {
		testgraph.ReturnNodeSlice(t, undirectedBuilder, true)
	})

	t.Run("AddNodes", func(t *testing.T) {
		testgraph.AddNodes(t, multi.NewUndirectedGraph(), 100)
	})
	t.Run("AddArbitraryNodes", func(t *testing.T) {
		testgraph.AddArbitraryNodes(t,
			multi.NewUndirectedGraph(),
			testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return multi.Node(id) }),
		)
	})
	t.Run("RemoveNodes", func(t *testing.T) {
		g := multi.NewUndirectedGraph()
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
			multi.NewUndirectedGraph(),
			func(id int64) graph.Node { return multi.Node(id) },
			true, // Can update nodes.
		)
	})
	t.Run("RemoveLines", func(t *testing.T) {
		g := multi.NewUndirectedGraph()
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

func TestMaxID(t *testing.T) {
	g := multi.NewUndirectedGraph()
	nodes := make(map[graph.Node]struct{})
	for i := multi.Node(0); i < 3; i++ {
		g.AddNode(i)
		nodes[i] = struct{}{}
	}
	g.RemoveNode(int64(0))
	delete(nodes, multi.Node(0))
	g.RemoveNode(int64(2))
	delete(nodes, multi.Node(2))
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
func TestIssue123UndirectedGraph(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	g := multi.NewUndirectedGraph()

	n0 := g.NewNode()
	g.AddNode(n0)

	n1 := g.NewNode()
	g.AddNode(n1)

	g.RemoveNode(n0.ID())

	n2 := g.NewNode()
	g.AddNode(n2)
}
