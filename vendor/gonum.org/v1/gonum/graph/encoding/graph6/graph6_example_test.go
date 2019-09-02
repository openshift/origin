// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph6_test

import (
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding/graph6"
)

func ExampleGraph() {
	// Construct a graph from HOG graph 32194.
	// https://hog.grinvin.org/ViewGraphInfo.action?id=32194
	g := graph6.Graph("H@BQPS^")

	// Get the nodes of the graph and print
	// an adjacency list.
	nodes := g.Nodes()
	fmt.Printf("Number of nodes: %d\n", nodes.Len())
	fmt.Println("Adjacency:")
	for nodes.Next() {
		fmt.Printf("\t%d: %d\n", nodes.Node().ID(), graph.NodesOf(g.From(nodes.Node().ID())))
	}

	// Output:
	//
	// Number of nodes: 9
	// Adjacency:
	// 	0: [5]
	// 	1: [5 6]
	// 	2: [3 7]
	// 	3: [2 5 8]
	// 	4: [6 7 8]
	// 	5: [0 1 3 8]
	// 	6: [1 4 7 8]
	// 	7: [2 4 6 8]
	// 	8: [3 4 5 6 7]
}
