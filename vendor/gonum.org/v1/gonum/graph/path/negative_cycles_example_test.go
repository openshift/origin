// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

func ExampleBellmanFordFrom_negativecycles() {
	// BellmanFordFrom can be used to find a non-exhaustive
	// set of negative cycles in a graph. Enumerating the
	// exhaustive list requires iterations of the procedure
	// here successively omitting links from the new node
	// to already found negative cycles.

	// Construct a graph with a negative cycle.
	edges := []simple.WeightedEdge{
		{F: simple.Node('a'), T: simple.Node('b'), W: -2},
		{F: simple.Node('a'), T: simple.Node('f'), W: 2},
		{F: simple.Node('b'), T: simple.Node('c'), W: 6},
		{F: simple.Node('c'), T: simple.Node('a'), W: -5},
		{F: simple.Node('d'), T: simple.Node('c'), W: -3},
		{F: simple.Node('d'), T: simple.Node('e'), W: 8},
		{F: simple.Node('e'), T: simple.Node('b'), W: 9},
		{F: simple.Node('e'), T: simple.Node('c'), W: 2},
	}
	g := simple.NewWeightedDirectedGraph(0, math.Inf(1))
	for _, e := range edges {
		g.SetWeightedEdge(e)
	}

	// Add a zero-cost path to all nodes from a new node Q.
	for _, n := range g.Nodes() {
		g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node('Q'), T: n})
	}

	// Find the shortest path to each node from Q.
	pt, ok := path.BellmanFordFrom(simple.Node('Q'), g)
	if ok {
		fmt.Println("no negative cycle present")
		return
	}
	for _, n := range []simple.Node{'a', 'b', 'c', 'd', 'e', 'f'} {
		p, w := pt.To(n.ID())
		if math.IsNaN(w) {
			fmt.Printf("negative cycle in path to %c path:%c\n", n, p)
		}
	}

	// Output:
	// negative cycle in path to a path:[a b c a]
	// negative cycle in path to b path:[b c a b]
	// negative cycle in path to c path:[c a b c]
	// negative cycle in path to f path:[a b c a f]
}
