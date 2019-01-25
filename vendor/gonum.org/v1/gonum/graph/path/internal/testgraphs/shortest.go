// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testgraphs

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

func init() {
	for _, test := range ShortestPathTests {
		if len(test.WantPaths) != 1 && test.HasUniquePath {
			panic(fmt.Sprintf("%q: bad shortest path test: non-unique paths marked unique", test.Name))
		}
	}
}

// ShortestPathTests are graphs used to test the static shortest path routines in path: BellmanFord,
// DijkstraAllPaths, DijkstraFrom, FloydWarshall and Johnson, and the static degenerate case for the
// dynamic shortest path routine in path/dynamic: DStarLite.
var ShortestPathTests = []struct {
	Name              string
	Graph             func() graph.WeightedEdgeAdder
	Edges             []simple.WeightedEdge
	HasNegativeWeight bool
	HasNegativeCycle  bool

	Query         simple.Edge
	Weight        float64
	WantPaths     [][]int64
	HasUniquePath bool

	NoPathFor simple.Edge
}{
	// Positive weighted graphs.
	{
		Name:  "empty directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		Weight: math.Inf(1),

		NoPathFor: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
	},
	{
		Name:  "empty undirected",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		Weight: math.Inf(1),

		NoPathFor: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
	},
	{
		Name:  "one edge directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 1},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		Weight: 1,
		WantPaths: [][]int64{
			{0, 1},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
	{
		Name:  "one edge self directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 1},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(0)},
		Weight: 0,
		WantPaths: [][]int64{
			{0},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
	{
		Name:  "one edge undirected",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 1},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		Weight: 1,
		WantPaths: [][]int64{
			{0, 1},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
	{
		Name:  "two paths directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(2), W: 2},
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(2)},
		Weight: 2,
		WantPaths: [][]int64{
			{0, 1, 2},
			{0, 2},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(1)},
	},
	{
		Name:  "two paths undirected",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(2), W: 2},
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(2)},
		Weight: 2,
		WantPaths: [][]int64{
			{0, 1, 2},
			{0, 2},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(4)},
	},
	{
		Name:  "confounding paths directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->5 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(5), W: 1},

			// Add direct edge to goal of weight 4
			{F: simple.Node(0), T: simple.Node(5), W: 4},

			// Add edge to a node that's still optimal
			{F: simple.Node(0), T: simple.Node(2), W: 2},

			// Add edge to 3 that's overpriced
			{F: simple.Node(0), T: simple.Node(3), W: 4},

			// Add very cheap edge to 4 which is a dead end
			{F: simple.Node(0), T: simple.Node(4), W: 0.25},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(5)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 5},
			{0, 2, 3, 5},
			{0, 5},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "confounding paths undirected",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->5 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(5), W: 1},

			// Add direct edge to goal of weight 4
			{F: simple.Node(0), T: simple.Node(5), W: 4},

			// Add edge to a node that's still optimal
			{F: simple.Node(0), T: simple.Node(2), W: 2},

			// Add edge to 3 that's overpriced
			{F: simple.Node(0), T: simple.Node(3), W: 4},

			// Add very cheap edge to 4 which is a dead end
			{F: simple.Node(0), T: simple.Node(4), W: 0.25},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(5)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 5},
			{0, 2, 3, 5},
			{0, 5},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(5), T: simple.Node(6)},
	},
	{
		Name:  "confounding paths directed 2-step",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->5 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(5), W: 1},

			// Add two step path to goal of weight 4
			{F: simple.Node(0), T: simple.Node(6), W: 2},
			{F: simple.Node(6), T: simple.Node(5), W: 2},

			// Add edge to a node that's still optimal
			{F: simple.Node(0), T: simple.Node(2), W: 2},

			// Add edge to 3 that's overpriced
			{F: simple.Node(0), T: simple.Node(3), W: 4},

			// Add very cheap edge to 4 which is a dead end
			{F: simple.Node(0), T: simple.Node(4), W: 0.25},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(5)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 5},
			{0, 2, 3, 5},
			{0, 6, 5},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "confounding paths undirected 2-step",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->5 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(5), W: 1},

			// Add two step path to goal of weight 4
			{F: simple.Node(0), T: simple.Node(6), W: 2},
			{F: simple.Node(6), T: simple.Node(5), W: 2},

			// Add edge to a node that's still optimal
			{F: simple.Node(0), T: simple.Node(2), W: 2},

			// Add edge to 3 that's overpriced
			{F: simple.Node(0), T: simple.Node(3), W: 4},

			// Add very cheap edge to 4 which is a dead end
			{F: simple.Node(0), T: simple.Node(4), W: 0.25},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(5)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 5},
			{0, 2, 3, 5},
			{0, 6, 5},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(5), T: simple.Node(7)},
	},
	{
		Name:  "zero-weight cycle directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(1), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(1), W: 0},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight cycle^2 directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(1), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(1), W: 0},
			// With its own zero-weight cycle.
			{F: simple.Node(5), T: simple.Node(6), W: 0},
			{F: simple.Node(6), T: simple.Node(5), W: 0},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight cycle^2 confounding directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(1), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(1), W: 0},
			// With its own zero-weight cycle.
			{F: simple.Node(5), T: simple.Node(6), W: 0},
			{F: simple.Node(6), T: simple.Node(5), W: 0},
			// But leading to the target.
			{F: simple.Node(5), T: simple.Node(4), W: 3},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
			{0, 1, 5, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight cycle^3 directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(1), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(1), W: 0},
			// With its own zero-weight cycle.
			{F: simple.Node(5), T: simple.Node(6), W: 0},
			{F: simple.Node(6), T: simple.Node(5), W: 0},
			// With its own zero-weight cycle.
			{F: simple.Node(6), T: simple.Node(7), W: 0},
			{F: simple.Node(7), T: simple.Node(6), W: 0},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight 3·cycle^2 confounding directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(1), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(1), W: 0},
			// With 3 of its own zero-weight cycles.
			{F: simple.Node(5), T: simple.Node(6), W: 0},
			{F: simple.Node(6), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(7), W: 0},
			{F: simple.Node(7), T: simple.Node(5), W: 0},
			// Each leading to the target.
			{F: simple.Node(5), T: simple.Node(4), W: 3},
			{F: simple.Node(6), T: simple.Node(4), W: 3},
			{F: simple.Node(7), T: simple.Node(4), W: 3},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
			{0, 1, 5, 4},
			{0, 1, 5, 6, 4},
			{0, 1, 5, 7, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight reversed 3·cycle^2 confounding directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			// Add a path from 0->4 of weight 4
			{F: simple.Node(0), T: simple.Node(1), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
			{F: simple.Node(2), T: simple.Node(3), W: 1},
			{F: simple.Node(3), T: simple.Node(4), W: 1},

			// Add a zero-weight cycle.
			{F: simple.Node(3), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(3), W: 0},
			// With 3 of its own zero-weight cycles.
			{F: simple.Node(5), T: simple.Node(6), W: 0},
			{F: simple.Node(6), T: simple.Node(5), W: 0},
			{F: simple.Node(5), T: simple.Node(7), W: 0},
			{F: simple.Node(7), T: simple.Node(5), W: 0},
			// Each leading from the source.
			{F: simple.Node(0), T: simple.Node(5), W: 3},
			{F: simple.Node(0), T: simple.Node(6), W: 3},
			{F: simple.Node(0), T: simple.Node(7), W: 3},
		},

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
			{0, 5, 3, 4},
			{0, 6, 5, 3, 4},
			{0, 7, 5, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight |V|·cycle^(n/|V|) directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: func() []simple.WeightedEdge {
			e := []simple.WeightedEdge{
				// Add a path from 0->4 of weight 4
				{F: simple.Node(0), T: simple.Node(1), W: 1},
				{F: simple.Node(1), T: simple.Node(2), W: 1},
				{F: simple.Node(2), T: simple.Node(3), W: 1},
				{F: simple.Node(3), T: simple.Node(4), W: 1},
			}
			next := len(e) + 1

			// Add n zero-weight cycles.
			const n = 100
			for i := 0; i < n; i++ {
				e = append(e,
					simple.WeightedEdge{F: simple.Node(next + i), T: simple.Node(i), W: 0},
					simple.WeightedEdge{F: simple.Node(i), T: simple.Node(next + i), W: 0},
				)
			}
			return e
		}(),

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight n·cycle directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: func() []simple.WeightedEdge {
			e := []simple.WeightedEdge{
				// Add a path from 0->4 of weight 4
				{F: simple.Node(0), T: simple.Node(1), W: 1},
				{F: simple.Node(1), T: simple.Node(2), W: 1},
				{F: simple.Node(2), T: simple.Node(3), W: 1},
				{F: simple.Node(3), T: simple.Node(4), W: 1},
			}
			next := len(e) + 1

			// Add n zero-weight cycles.
			const n = 100
			for i := 0; i < n; i++ {
				e = append(e,
					simple.WeightedEdge{F: simple.Node(next + i), T: simple.Node(1), W: 0},
					simple.WeightedEdge{F: simple.Node(1), T: simple.Node(next + i), W: 0},
				)
			}
			return e
		}(),

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},
	{
		Name:  "zero-weight bi-directional tree with single exit directed",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: func() []simple.WeightedEdge {
			e := []simple.WeightedEdge{
				// Add a path from 0->4 of weight 4
				{F: simple.Node(0), T: simple.Node(1), W: 1},
				{F: simple.Node(1), T: simple.Node(2), W: 1},
				{F: simple.Node(2), T: simple.Node(3), W: 1},
				{F: simple.Node(3), T: simple.Node(4), W: 1},
			}

			// Make a bi-directional tree rooted at node 2 with
			// a single exit to node 4 and co-equal cost from
			// 2 to 4.
			const (
				depth     = 4
				branching = 4
			)

			next := len(e) + 1
			src := 2
			var i, last int
			for l := 0; l < depth; l++ {
				for i = 0; i < branching; i++ {
					last = next + i
					e = append(e, simple.WeightedEdge{F: simple.Node(src), T: simple.Node(last), W: 0})
					e = append(e, simple.WeightedEdge{F: simple.Node(last), T: simple.Node(src), W: 0})
				}
				src = next + 1
				next += branching
			}
			e = append(e, simple.WeightedEdge{F: simple.Node(last), T: simple.Node(4), W: 2})
			return e
		}(),

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(4)},
		Weight: 4,
		WantPaths: [][]int64{
			{0, 1, 2, 3, 4},
			{0, 1, 2, 6, 10, 14, 20, 4},
		},
		HasUniquePath: false,

		NoPathFor: simple.Edge{F: simple.Node(4), T: simple.Node(5)},
	},

	// Negative weighted graphs.
	{
		Name:  "one edge directed negative",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: -1},
		},
		HasNegativeWeight: true,

		Query:  simple.Edge{F: simple.Node(0), T: simple.Node(1)},
		Weight: -1,
		WantPaths: [][]int64{
			{0, 1},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
	{
		Name:  "one edge undirected negative",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedUndirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: -1},
		},
		HasNegativeWeight: true,
		HasNegativeCycle:  true,

		Query: simple.Edge{F: simple.Node(0), T: simple.Node(1)},
	},
	{
		Name:  "wp graph negative", // http://en.wikipedia.org/w/index.php?title=Johnson%27s_algorithm&oldid=564595231
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node('w'), T: simple.Node('z'), W: 2},
			{F: simple.Node('x'), T: simple.Node('w'), W: 6},
			{F: simple.Node('x'), T: simple.Node('y'), W: 3},
			{F: simple.Node('y'), T: simple.Node('w'), W: 4},
			{F: simple.Node('y'), T: simple.Node('z'), W: 5},
			{F: simple.Node('z'), T: simple.Node('x'), W: -7},
			{F: simple.Node('z'), T: simple.Node('y'), W: -3},
		},
		HasNegativeWeight: true,

		Query:  simple.Edge{F: simple.Node('z'), T: simple.Node('y')},
		Weight: -4,
		WantPaths: [][]int64{
			{'z', 'x', 'y'},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
	{
		Name:  "roughgarden negative",
		Graph: func() graph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) },
		Edges: []simple.WeightedEdge{
			{F: simple.Node('a'), T: simple.Node('b'), W: -2},
			{F: simple.Node('b'), T: simple.Node('c'), W: -1},
			{F: simple.Node('c'), T: simple.Node('a'), W: 4},
			{F: simple.Node('c'), T: simple.Node('x'), W: 2},
			{F: simple.Node('c'), T: simple.Node('y'), W: -3},
			{F: simple.Node('z'), T: simple.Node('x'), W: 1},
			{F: simple.Node('z'), T: simple.Node('y'), W: -4},
		},
		HasNegativeWeight: true,

		Query:  simple.Edge{F: simple.Node('a'), T: simple.Node('y')},
		Weight: -6,
		WantPaths: [][]int64{
			{'a', 'b', 'c', 'y'},
		},
		HasUniquePath: true,

		NoPathFor: simple.Edge{F: simple.Node(2), T: simple.Node(3)},
	},
}
