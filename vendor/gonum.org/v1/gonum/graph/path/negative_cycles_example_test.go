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
	// set of negative cycles in a graph.

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
	nodes := g.Nodes()
	for nodes.Next() {
		g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node('Q'), T: nodes.Node()})
	}

	// Find the shortest path to each node from Q.
	pt, ok := path.BellmanFordFrom(simple.Node('Q'), g)
	if ok {
		fmt.Println("no negative cycle present")
		return
	}
	for _, id := range []int64{'a', 'b', 'c', 'd', 'e', 'f'} {
		p, w := pt.To(id)
		if math.IsInf(w, -1) {
			fmt.Printf("negative cycle in path to %c path:%c\n", id, p)
		}
	}

	// Output:
	// negative cycle in path to a path:[a b c a]
	// negative cycle in path to b path:[b c a b]
	// negative cycle in path to c path:[c a b c]
	// negative cycle in path to f path:[a b c a f]
}

func ExampleFloydWarshall_negativecycles() {
	// FloydWarshall can be used to find an exhaustive
	// set of nodes in negative cycles in a graph.

	// Construct a graph with a negative cycle.
	edges := []simple.WeightedEdge{
		{F: simple.Node('a'), T: simple.Node('f'), W: -1},
		{F: simple.Node('b'), T: simple.Node('a'), W: 1},
		{F: simple.Node('b'), T: simple.Node('c'), W: -1},
		{F: simple.Node('b'), T: simple.Node('d'), W: 1},
		{F: simple.Node('c'), T: simple.Node('b'), W: 0},
		{F: simple.Node('e'), T: simple.Node('a'), W: 1},
		{F: simple.Node('f'), T: simple.Node('e'), W: -1},
	}
	g := simple.NewWeightedDirectedGraph(0, math.Inf(1))
	for _, e := range edges {
		g.SetWeightedEdge(e)
	}

	// Find the shortest path to each node from Q.
	pt, ok := path.FloydWarshall(g)
	if ok {
		fmt.Println("no negative cycle present")
		return
	}

	ids := []int64{'a', 'b', 'c', 'd', 'e', 'f'}

	for _, id := range ids {
		if math.IsInf(pt.Weight(id, id), -1) {
			fmt.Printf("%c is in a negative cycle\n", id)
		}
	}

	for _, uid := range ids {
		for _, vid := range ids {
			_, w, unique := pt.Between(uid, vid)
			if math.IsInf(w, -1) {
				fmt.Printf("negative cycle in path from %c to %c unique=%t\n", uid, vid, unique)
			}
		}
	}

	// Output:
	// a is in a negative cycle
	// b is in a negative cycle
	// c is in a negative cycle
	// e is in a negative cycle
	// f is in a negative cycle
	// negative cycle in path from a to a unique=false
	// negative cycle in path from a to e unique=false
	// negative cycle in path from a to f unique=false
	// negative cycle in path from b to a unique=false
	// negative cycle in path from b to b unique=false
	// negative cycle in path from b to c unique=false
	// negative cycle in path from b to d unique=false
	// negative cycle in path from b to e unique=false
	// negative cycle in path from b to f unique=false
	// negative cycle in path from c to a unique=false
	// negative cycle in path from c to b unique=false
	// negative cycle in path from c to c unique=false
	// negative cycle in path from c to d unique=false
	// negative cycle in path from c to e unique=false
	// negative cycle in path from c to f unique=false
	// negative cycle in path from e to a unique=false
	// negative cycle in path from e to e unique=false
	// negative cycle in path from e to f unique=false
	// negative cycle in path from f to a unique=false
	// negative cycle in path from f to e unique=false
	// negative cycle in path from f to f unique=false
}
