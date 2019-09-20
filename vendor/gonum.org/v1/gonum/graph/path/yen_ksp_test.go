// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

var yenShortestPathTests = []struct {
	name  string
	graph func() graph.WeightedEdgeAdder
	edges []simple.WeightedEdge

	query     simple.Edge
	k         int
	wantPaths [][]int64

	relaxed bool
}{
	{
		// https://en.wikipedia.org/w/index.php?title=Yen%27s_algorithm&oldid=841018784#Example
		name:  "wikipedia example",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node('C'), T: simple.Node('D'), W: 3},
			{F: simple.Node('C'), T: simple.Node('E'), W: 2},
			{F: simple.Node('E'), T: simple.Node('D'), W: 1},
			{F: simple.Node('D'), T: simple.Node('F'), W: 4},
			{F: simple.Node('E'), T: simple.Node('F'), W: 2},
			{F: simple.Node('E'), T: simple.Node('G'), W: 3},
			{F: simple.Node('F'), T: simple.Node('G'), W: 2},
			{F: simple.Node('F'), T: simple.Node('H'), W: 1},
			{F: simple.Node('G'), T: simple.Node('H'), W: 2},
		},
		query: simple.Edge{F: simple.Node('C'), T: simple.Node('H')},
		k:     3,
		wantPaths: [][]int64{
			{'C', 'E', 'F', 'H'},
			{'C', 'E', 'G', 'H'},
			{'C', 'D', 'F', 'H'},
		},
	},
	{
		name:  "1 edge graph",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 3},
		},
		query: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:     10,
		wantPaths: [][]int64{
			{0, 1},
		},
	},
	{
		name:      "empty graph",
		graph:     func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges:     []simple.WeightedEdge{},
		query:     simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:         1,
		wantPaths: nil,
	},
	{
		name:  "n-star graph",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 3},
			{F: simple.Node(0), T: simple.Node(2), W: 3},
			{F: simple.Node(0), T: simple.Node(3), W: 3},
		},
		query: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		k:     1,
		wantPaths: [][]int64{
			{0, 1},
		},
	},
	{
		name:  "bipartite small",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: bipartite(5, 3, 0),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     10,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:  "bipartite parity",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: bipartite(5, 3, 0),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:    "bipartite large",
		graph:   func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges:   bipartite(10, 3, 0),
		query:   simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:       5,
		relaxed: true,
	},
	{
		name:  "bipartite inc",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: bipartite(5, 10, 1),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 2, 1},
			{-1, 3, 1},
			{-1, 4, 1},
			{-1, 5, 1},
			{-1, 6, 1},
		},
	},
	{
		name:  "bipartite dec",
		graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		edges: bipartite(5, 10, -1),
		query: simple.Edge{F: simple.Node(-1), T: simple.Node(1)},
		k:     5,
		wantPaths: [][]int64{
			{-1, 6, 1},
			{-1, 5, 1},
			{-1, 4, 1},
			{-1, 3, 1},
			{-1, 2, 1},
		},
	},
}

func bipartite(n int, weight, inc float64) []simple.WeightedEdge {
	var edges []simple.WeightedEdge
	for i := 2; i < n+2; i++ {
		edges = append(edges,
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(i), W: weight},
			simple.WeightedEdge{F: simple.Node(i), T: simple.Node(1), W: weight},
		)
		weight += inc
	}
	return edges
}

func pathIDs(paths [][]graph.Node) [][]int64 {
	if paths == nil {
		return nil
	}
	ids := make([][]int64, len(paths))
	for i, p := range paths {
		if p == nil {
			continue
		}
		ids[i] = make([]int64, len(p))
		for j, n := range p {
			ids[i][j] = n.ID()
		}
	}
	return ids
}

func TestYenKSP(t *testing.T) {
	for _, test := range yenShortestPathTests {
		g := test.graph()
		for _, e := range test.edges {
			g.SetWeightedEdge(e)
		}

		got := YenKShortestPaths(g.(graph.Graph), test.k, test.query.From(), test.query.To())
		gotIDs := pathIDs(got)

		paths := make(byPathWeight, len(gotIDs))
		for i, p := range got {
			paths[i] = yenShortest{path: p, weight: pathWeight(p, g.(graph.Weighted))}
		}
		if !sort.IsSorted(paths) {
			t.Errorf("unexpected result for %q: got:%+v", test.name, paths)
		}
		if test.relaxed {
			continue
		}

		if len(gotIDs) != 0 {
			first := 0
			last := pathWeight(got[0], g.(graph.Weighted))
			for i := 1; i < len(got); i++ {
				w := pathWeight(got[i], g.(graph.Weighted))
				if w == last {
					continue
				}
				sort.Sort(ordered.BySliceValues(gotIDs[first:i]))
				first = i
				last = w
			}
			sort.Sort(ordered.BySliceValues(gotIDs[first:]))
		}

		if !reflect.DeepEqual(test.wantPaths, gotIDs) {
			t.Errorf("unexpected result for %q:\ngot: %v\nwant:%v", test.name, gotIDs, test.wantPaths)
		}
	}
}

func pathWeight(path []graph.Node, g graph.Weighted) float64 {
	switch len(path) {
	case 0:
		return math.NaN()
	case 1:
		return 0
	default:
		var w float64
		for i, u := range path[:len(path)-1] {
			_w, _ := g.Weight(u.ID(), path[i+1].ID())
			w += _w
		}
		return w
	}
}
