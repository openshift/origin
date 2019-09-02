// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multi_test

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/testgraph"
)

func weightedUndirectedBuilder(nodes []graph.Node, edges []testgraph.WeightedLine, self, absent float64) (g graph.Graph, n []graph.Node, e []testgraph.Edge, s, a float64, ok bool) {
	seen := set.NewNodes()
	ug := multi.NewWeightedUndirectedGraph()
	ug.EdgeWeightFunc = func(l graph.WeightedLines) float64 {
		if l.Len() == 0 {
			return absent
		}
		var w float64
		for l.Next() {
			w += l.WeightedLine().Weight()
		}
		l.Reset()
		return w
	}
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
		cl := multi.WeightedLine{F: f, T: t, UID: edge.ID(), W: edge.Weight()}
		seen.Add(cl.F)
		seen.Add(cl.T)
		e = append(e, cl)
		ug.SetWeightedLine(cl)
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
	t.Run("LineExistence", func(t *testing.T) {
		testgraph.LineExistence(t, directedBuilder, true)
	})
	t.Run("NodeExistence", func(t *testing.T) {
		testgraph.NodeExistence(t, weightedUndirectedBuilder)
	})
	t.Run("ReturnAdjacentNodes", func(t *testing.T) {
		testgraph.ReturnAdjacentNodes(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllLines", func(t *testing.T) {
		testgraph.ReturnAllLines(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllNodes", func(t *testing.T) {
		testgraph.ReturnAllNodes(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnAllWeightedLines", func(t *testing.T) {
		testgraph.ReturnAllWeightedLines(t, weightedUndirectedBuilder, true)
	})
	t.Run("ReturnNodeSlice", func(t *testing.T) {
		testgraph.ReturnNodeSlice(t, weightedUndirectedBuilder, true)
	})
	t.Run("Weight", func(t *testing.T) {
		testgraph.Weight(t, weightedUndirectedBuilder)
	})

	t.Run("AddNodes", func(t *testing.T) {
		testgraph.AddNodes(t, multi.NewWeightedUndirectedGraph(), 100)
	})
	t.Run("AddArbitraryNodes", func(t *testing.T) {
		testgraph.AddArbitraryNodes(t,
			multi.NewWeightedUndirectedGraph(),
			testgraph.NewRandomNodes(100, 1, func(id int64) graph.Node { return multi.Node(id) }),
		)
	})
	t.Run("RemoveNodes", func(t *testing.T) {
		g := multi.NewWeightedUndirectedGraph()
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
				g.SetWeightedLine(g.NewWeightedLine(u, v, 1))
			}
		}
		testgraph.RemoveNodes(t, g)
	})
	t.Run("AddWeightedLines", func(t *testing.T) {
		testgraph.AddWeightedLines(t, 100,
			multi.NewWeightedUndirectedGraph(),
			0.5,
			func(id int64) graph.Node { return multi.Node(id) },
			true, // Can update nodes.
		)
	})
	t.Run("RemoveLines", func(t *testing.T) {
		g := multi.NewWeightedUndirectedGraph()
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
				l := g.NewWeightedLine(u, v, 1)
				g.SetWeightedLine(l)
				lines = append(lines, l)
			}
		}
		rnd.Shuffle(len(lines), func(i, j int) {
			lines[i], lines[j] = lines[j], lines[i]
		})
		testgraph.RemoveLines(t, g, iterator.NewOrderedLines(lines))
	})
}

func TestWeightedMaxID(t *testing.T) {
	g := multi.NewWeightedUndirectedGraph()
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
func TestIssue123WeightedUndirectedGraph(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	g := multi.NewWeightedUndirectedGraph()

	n0 := g.NewNode()
	g.AddNode(n0)

	n1 := g.NewNode()
	g.AddNode(n1)

	g.RemoveNode(n0.ID())

	n2 := g.NewNode()
	g.AddNode(n2)
}
