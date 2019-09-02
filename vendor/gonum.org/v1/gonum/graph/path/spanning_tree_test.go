// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

func init() {
	for _, test := range spanningTreeTests {
		var w float64
		for _, e := range test.treeEdges {
			w += e.W
		}
		if w != test.want {
			panic(fmt.Sprintf("bad test: %s weight mismatch: %v != %v", test.name, w, test.want))
		}
	}
}

type spanningGraph interface {
	graph.WeightedBuilder
	graph.WeightedUndirected
	WeightedEdges() graph.WeightedEdges
}

var spanningTreeTests = []struct {
	name      string
	graph     func() spanningGraph
	edges     []simple.WeightedEdge
	want      float64
	treeEdges []simple.WeightedEdge
}{
	{
		name:  "Empty",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		want:  0,
	},
	{
		// https://upload.wikimedia.org/wikipedia/commons/f/f7/Prim%27s_algorithm.svg
		// Modified to make edge weights unique; A--B is increased to 2.5 otherwise
		// to prevent the alternative solution being found.
		name:  "Prim WP figure 1",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('B'), W: 2.5},
			{F: simple.Node('A'), T: simple.Node('D'), W: 1},
			{F: simple.Node('B'), T: simple.Node('D'), W: 2},
			{F: simple.Node('C'), T: simple.Node('D'), W: 3},
		},

		want: 6,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('D'), W: 1},
			{F: simple.Node('B'), T: simple.Node('D'), W: 2},
			{F: simple.Node('C'), T: simple.Node('D'), W: 3},
		},
	},
	{
		// https://upload.wikimedia.org/wikipedia/commons/5/5c/MST_kruskal_en.gif
		name:  "Kruskal WP figure 1",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('a'), T: simple.Node('b'), W: 3},
			{F: simple.Node('a'), T: simple.Node('e'), W: 1},
			{F: simple.Node('b'), T: simple.Node('c'), W: 5},
			{F: simple.Node('b'), T: simple.Node('e'), W: 4},
			{F: simple.Node('c'), T: simple.Node('d'), W: 2},
			{F: simple.Node('c'), T: simple.Node('e'), W: 6},
			{F: simple.Node('d'), T: simple.Node('e'), W: 7},
		},

		want: 11,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node('a'), T: simple.Node('b'), W: 3},
			{F: simple.Node('a'), T: simple.Node('e'), W: 1},
			{F: simple.Node('b'), T: simple.Node('c'), W: 5},
			{F: simple.Node('c'), T: simple.Node('d'), W: 2},
		},
	},
	{
		// https://upload.wikimedia.org/wikipedia/commons/8/87/Kruskal_Algorithm_6.svg
		name:  "Kruskal WP example",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('B'), W: 7},
			{F: simple.Node('A'), T: simple.Node('D'), W: 5},
			{F: simple.Node('B'), T: simple.Node('C'), W: 8},
			{F: simple.Node('B'), T: simple.Node('D'), W: 9},
			{F: simple.Node('B'), T: simple.Node('E'), W: 7},
			{F: simple.Node('C'), T: simple.Node('E'), W: 5},
			{F: simple.Node('D'), T: simple.Node('E'), W: 15},
			{F: simple.Node('D'), T: simple.Node('F'), W: 6},
			{F: simple.Node('E'), T: simple.Node('F'), W: 8},
			{F: simple.Node('E'), T: simple.Node('G'), W: 9},
			{F: simple.Node('F'), T: simple.Node('G'), W: 11},
		},

		want: 39,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('B'), W: 7},
			{F: simple.Node('A'), T: simple.Node('D'), W: 5},
			{F: simple.Node('B'), T: simple.Node('E'), W: 7},
			{F: simple.Node('C'), T: simple.Node('E'), W: 5},
			{F: simple.Node('D'), T: simple.Node('F'), W: 6},
			{F: simple.Node('E'), T: simple.Node('G'), W: 9},
		},
	},
	{
		// https://upload.wikimedia.org/wikipedia/commons/2/2e/Boruvka%27s_algorithm_%28Sollin%27s_algorithm%29_Anim.gif
		name:  "Borůvka WP example",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('B'), W: 13},
			{F: simple.Node('A'), T: simple.Node('C'), W: 6},
			{F: simple.Node('B'), T: simple.Node('C'), W: 7},
			{F: simple.Node('B'), T: simple.Node('D'), W: 1},
			{F: simple.Node('C'), T: simple.Node('D'), W: 14},
			{F: simple.Node('C'), T: simple.Node('E'), W: 8},
			{F: simple.Node('C'), T: simple.Node('H'), W: 20},
			{F: simple.Node('D'), T: simple.Node('E'), W: 9},
			{F: simple.Node('D'), T: simple.Node('F'), W: 3},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('E'), T: simple.Node('J'), W: 18},
			{F: simple.Node('G'), T: simple.Node('H'), W: 15},
			{F: simple.Node('G'), T: simple.Node('I'), W: 5},
			{F: simple.Node('G'), T: simple.Node('J'), W: 19},
			{F: simple.Node('G'), T: simple.Node('K'), W: 10},
			{F: simple.Node('H'), T: simple.Node('J'), W: 17},
			{F: simple.Node('I'), T: simple.Node('K'), W: 11},
			{F: simple.Node('J'), T: simple.Node('K'), W: 16},
			{F: simple.Node('J'), T: simple.Node('L'), W: 4},
			{F: simple.Node('K'), T: simple.Node('L'), W: 12},
		},

		want: 83,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('C'), W: 6},
			{F: simple.Node('B'), T: simple.Node('C'), W: 7},
			{F: simple.Node('B'), T: simple.Node('D'), W: 1},
			{F: simple.Node('D'), T: simple.Node('F'), W: 3},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('E'), T: simple.Node('J'), W: 18},
			{F: simple.Node('G'), T: simple.Node('H'), W: 15},
			{F: simple.Node('G'), T: simple.Node('I'), W: 5},
			{F: simple.Node('G'), T: simple.Node('K'), W: 10},
			{F: simple.Node('J'), T: simple.Node('L'), W: 4},
			{F: simple.Node('K'), T: simple.Node('L'), W: 12},
		},
	},
	{
		// https://upload.wikimedia.org/wikipedia/commons/d/d2/Minimum_spanning_tree.svg
		// Nodes labelled row major.
		name:  "Minimum Spanning Tree WP figure 1",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(1), T: simple.Node(2), W: 4},
			{F: simple.Node(1), T: simple.Node(3), W: 1},
			{F: simple.Node(1), T: simple.Node(4), W: 4},
			{F: simple.Node(2), T: simple.Node(3), W: 5},
			{F: simple.Node(2), T: simple.Node(5), W: 9},
			{F: simple.Node(2), T: simple.Node(6), W: 9},
			{F: simple.Node(2), T: simple.Node(8), W: 7},
			{F: simple.Node(3), T: simple.Node(4), W: 3},
			{F: simple.Node(3), T: simple.Node(8), W: 9},
			{F: simple.Node(4), T: simple.Node(8), W: 10},
			{F: simple.Node(4), T: simple.Node(10), W: 18},
			{F: simple.Node(5), T: simple.Node(6), W: 2},
			{F: simple.Node(5), T: simple.Node(7), W: 4},
			{F: simple.Node(5), T: simple.Node(9), W: 6},
			{F: simple.Node(6), T: simple.Node(7), W: 2},
			{F: simple.Node(6), T: simple.Node(8), W: 8},
			{F: simple.Node(7), T: simple.Node(8), W: 9},
			{F: simple.Node(7), T: simple.Node(9), W: 3},
			{F: simple.Node(7), T: simple.Node(10), W: 9},
			{F: simple.Node(8), T: simple.Node(10), W: 8},
			{F: simple.Node(9), T: simple.Node(10), W: 9},
		},

		want: 38,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node(1), T: simple.Node(2), W: 4},
			{F: simple.Node(1), T: simple.Node(3), W: 1},
			{F: simple.Node(2), T: simple.Node(8), W: 7},
			{F: simple.Node(3), T: simple.Node(4), W: 3},
			{F: simple.Node(5), T: simple.Node(6), W: 2},
			{F: simple.Node(6), T: simple.Node(7), W: 2},
			{F: simple.Node(6), T: simple.Node(8), W: 8},
			{F: simple.Node(7), T: simple.Node(9), W: 3},
			{F: simple.Node(8), T: simple.Node(10), W: 8},
		},
	},

	{
		// https://upload.wikimedia.org/wikipedia/commons/2/2e/Boruvka%27s_algorithm_%28Sollin%27s_algorithm%29_Anim.gif
		// but with C--H and E--J cut.
		name:  "Borůvka WP example cut",
		graph: func() spanningGraph { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('B'), W: 13},
			{F: simple.Node('A'), T: simple.Node('C'), W: 6},
			{F: simple.Node('B'), T: simple.Node('C'), W: 7},
			{F: simple.Node('B'), T: simple.Node('D'), W: 1},
			{F: simple.Node('C'), T: simple.Node('D'), W: 14},
			{F: simple.Node('C'), T: simple.Node('E'), W: 8},
			{F: simple.Node('D'), T: simple.Node('E'), W: 9},
			{F: simple.Node('D'), T: simple.Node('F'), W: 3},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('G'), T: simple.Node('H'), W: 15},
			{F: simple.Node('G'), T: simple.Node('I'), W: 5},
			{F: simple.Node('G'), T: simple.Node('J'), W: 19},
			{F: simple.Node('G'), T: simple.Node('K'), W: 10},
			{F: simple.Node('H'), T: simple.Node('J'), W: 17},
			{F: simple.Node('I'), T: simple.Node('K'), W: 11},
			{F: simple.Node('J'), T: simple.Node('K'), W: 16},
			{F: simple.Node('J'), T: simple.Node('L'), W: 4},
			{F: simple.Node('K'), T: simple.Node('L'), W: 12},
		},

		want: 65,
		treeEdges: []simple.WeightedEdge{
			{F: simple.Node('A'), T: simple.Node('C'), W: 6},
			{F: simple.Node('B'), T: simple.Node('C'), W: 7},
			{F: simple.Node('B'), T: simple.Node('D'), W: 1},
			{F: simple.Node('D'), T: simple.Node('F'), W: 3},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('G'), T: simple.Node('H'), W: 15},
			{F: simple.Node('G'), T: simple.Node('I'), W: 5},
			{F: simple.Node('G'), T: simple.Node('K'), W: 10},
			{F: simple.Node('J'), T: simple.Node('L'), W: 4},
			{F: simple.Node('K'), T: simple.Node('L'), W: 12},
		},
	},
}

func testMinumumSpanning(mst func(dst WeightedBuilder, g spanningGraph) float64, t *testing.T) {
	for _, test := range spanningTreeTests {
		g := test.graph()
		for _, e := range test.edges {
			g.SetWeightedEdge(e)
		}

		dst := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
		w := mst(dst, g)
		if w != test.want {
			t.Errorf("unexpected minimum spanning tree weight for %q: got: %f want: %f",
				test.name, w, test.want)
		}
		var got float64
		for _, e := range graph.WeightedEdgesOf(dst.WeightedEdges()) {
			got += e.Weight()
		}
		if got != test.want {
			t.Errorf("unexpected minimum spanning tree edge weight sum for %q: got: %f want: %f",
				test.name, got, test.want)
		}

		gotEdges := graph.EdgesOf(dst.Edges())
		if len(gotEdges) != len(test.treeEdges) {
			t.Errorf("unexpected number of spanning tree edges for %q: got: %d want: %d",
				test.name, len(gotEdges), len(test.treeEdges))
		}
		for _, e := range test.treeEdges {
			w, ok := dst.Weight(e.From().ID(), e.To().ID())
			if !ok {
				t.Errorf("spanning tree edge not found in graph for %q: %+v",
					test.name, e)
			}
			if w != e.Weight() {
				t.Errorf("unexpected spanning tree edge weight for %q: got: %f want: %f",
					test.name, w, e.Weight())
			}
		}
	}
}

func TestKruskal(t *testing.T) {
	testMinumumSpanning(func(dst WeightedBuilder, g spanningGraph) float64 {
		return Kruskal(dst, g)
	}, t)
}

func TestPrim(t *testing.T) {
	testMinumumSpanning(func(dst WeightedBuilder, g spanningGraph) float64 {
		return Prim(dst, g)
	}, t)
}
