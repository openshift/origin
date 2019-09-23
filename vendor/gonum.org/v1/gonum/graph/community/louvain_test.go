// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"fmt"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/graphs/gen"
	"gonum.org/v1/gonum/graph/simple"
)

// intset is an integer set.
type intset map[int]struct{}

func linksTo(i ...int) intset {
	if len(i) == 0 {
		return nil
	}
	s := make(intset)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}

type layer struct {
	g          []intset
	edgeWeight float64 // Zero edge weight is interpreted as 1.0.
	weight     float64
}

var (
	unconnected = []intset{ /* Nodes 0-4 are implicit .*/ 5: nil}

	smallDumbell = []intset{
		0: linksTo(1, 2),
		1: linksTo(2),
		2: linksTo(3),
		3: linksTo(4, 5),
		4: linksTo(5),
		5: nil,
	}
	dumbellRepulsion = []intset{
		0: linksTo(4),
		1: linksTo(5),
		2: nil,
		3: nil,
		4: nil,
		5: nil,
	}

	repulsion = []intset{
		0: linksTo(3, 4, 5),
		1: linksTo(3, 4, 5),
		2: linksTo(3, 4, 5),
		3: linksTo(0, 1, 2),
		4: linksTo(0, 1, 2),
		5: linksTo(0, 1, 2),
	}

	simpleDirected = []intset{
		0: linksTo(1),
		1: linksTo(0, 4),
		2: linksTo(1),
		3: linksTo(0, 4),
		4: linksTo(2),
	}

	// http://www.slate.com/blogs/the_world_/2014/07/17/the_middle_east_friendship_chart.html
	middleEast = struct{ friends, complicated, enemies []intset }{
		// green cells
		friends: []intset{
			0:  nil,
			1:  linksTo(5, 7, 9, 12),
			2:  linksTo(11),
			3:  linksTo(4, 5, 10),
			4:  linksTo(3, 5, 10),
			5:  linksTo(1, 3, 4, 8, 10, 12),
			6:  nil,
			7:  linksTo(1, 12),
			8:  linksTo(5, 9, 11),
			9:  linksTo(1, 8, 12),
			10: linksTo(3, 4, 5),
			11: linksTo(2, 8),
			12: linksTo(1, 5, 7, 9),
		},

		// yellow cells
		complicated: []intset{
			0:  linksTo(2, 4),
			1:  linksTo(4, 8),
			2:  linksTo(0, 3, 4, 5, 8, 9),
			3:  linksTo(2, 8, 11),
			4:  linksTo(0, 1, 2, 8),
			5:  linksTo(2),
			6:  nil,
			7:  linksTo(9, 11),
			8:  linksTo(1, 2, 3, 4, 10, 12),
			9:  linksTo(2, 7, 11),
			10: linksTo(8),
			11: linksTo(3, 7, 9, 12),
			12: linksTo(8, 11),
		},

		// red cells
		enemies: []intset{
			0:  linksTo(1, 3, 5, 6, 7, 8, 9, 10, 11, 12),
			1:  linksTo(0, 2, 3, 6, 10, 11),
			2:  linksTo(1, 6, 7, 10, 12),
			3:  linksTo(0, 1, 6, 7, 9, 12),
			4:  linksTo(6, 7, 9, 11, 12),
			5:  linksTo(0, 6, 7, 9, 11),
			6:  linksTo(0, 1, 2, 3, 4, 5, 7, 8, 9, 10, 11, 12),
			7:  linksTo(0, 2, 3, 4, 5, 6, 8, 10),
			8:  linksTo(0, 6, 7),
			9:  linksTo(0, 3, 4, 5, 6, 10),
			10: linksTo(0, 1, 2, 6, 7, 9, 11, 12),
			11: linksTo(0, 1, 4, 5, 6, 10),
			12: linksTo(0, 2, 3, 4, 6, 10),
		},
	}

	// W. W. Zachary, An information flow model for conflict and fission in small groups,
	// Journal of Anthropological Research 33, 452-473 (1977).
	//
	// The edge list here is constructed such that all link descriptions
	// head from a node with lower Page Rank to a node with higher Page
	// Rank. This has no impact on undirected tests, but allows a sensible
	// view for directed tests.
	zachary = []intset{
		0:  nil,                     // rank=0.097
		1:  linksTo(0, 2),           // rank=0.05288
		2:  linksTo(0, 32),          // rank=0.05708
		3:  linksTo(0, 1, 2),        // rank=0.03586
		4:  linksTo(0, 6, 10),       // rank=0.02198
		5:  linksTo(0, 6),           // rank=0.02911
		6:  linksTo(0, 5),           // rank=0.02911
		7:  linksTo(0, 1, 2, 3),     // rank=0.02449
		8:  linksTo(0, 2, 32, 33),   // rank=0.02977
		9:  linksTo(2, 33),          // rank=0.01431
		10: linksTo(0, 5),           // rank=0.02198
		11: linksTo(0),              // rank=0.009565
		12: linksTo(0, 3),           // rank=0.01464
		13: linksTo(0, 1, 2, 3, 33), // rank=0.02954
		14: linksTo(32, 33),         // rank=0.01454
		15: linksTo(32, 33),         // rank=0.01454
		16: linksTo(5, 6),           // rank=0.01678
		17: linksTo(0, 1),           // rank=0.01456
		18: linksTo(32, 33),         // rank=0.01454
		19: linksTo(0, 1, 33),       // rank=0.0196
		20: linksTo(32, 33),         // rank=0.01454
		21: linksTo(0, 1),           // rank=0.01456
		22: linksTo(32, 33),         // rank=0.01454
		23: linksTo(32, 33),         // rank=0.03152
		24: linksTo(27, 31),         // rank=0.02108
		25: linksTo(23, 24, 31),     // rank=0.02101
		26: linksTo(29, 33),         // rank=0.01504
		27: linksTo(2, 23, 33),      // rank=0.02564
		28: linksTo(2, 31, 33),      // rank=0.01957
		29: linksTo(23, 32, 33),     // rank=0.02629
		30: linksTo(1, 8, 32, 33),   // rank=0.02459
		31: linksTo(0, 32, 33),      // rank=0.03716
		32: linksTo(33),             // rank=0.07169
		33: nil,                     // rank=0.1009
	}

	// doi:10.1088/1742-5468/2008/10/P10008 figure 1
	//
	// The edge list here is constructed such that all link descriptions
	// head from a node with lower Page Rank to a node with higher Page
	// Rank. This has no impact on undirected tests, but allows a sensible
	// view for directed tests.
	blondel = []intset{
		0:  linksTo(2),           // rank=0.06858
		1:  linksTo(2, 4, 7),     // rank=0.05264
		2:  nil,                  // rank=0.08249
		3:  linksTo(0, 7),        // rank=0.03884
		4:  linksTo(0, 2, 10),    // rank=0.06754
		5:  linksTo(0, 2, 7, 11), // rank=0.06738
		6:  linksTo(2, 7, 11),    // rank=0.0528
		7:  nil,                  // rank=0.07008
		8:  linksTo(10),          // rank=0.09226
		9:  linksTo(8),           // rank=0.05821
		10: nil,                  // rank=0.1035
		11: linksTo(8, 10),       // rank=0.08538
		12: linksTo(9, 10),       // rank=0.04052
		13: linksTo(10, 11),      // rank=0.03855
		14: linksTo(8, 9, 10),    // rank=0.05621
		15: linksTo(8),           // rank=0.02506
	}
)

type structure struct {
	resolution  float64
	memberships []intset
	want, tol   float64
}

type level struct {
	q           float64
	communities [][]graph.Node
}

type moveStructures struct {
	memberships []intset
	targetNodes []graph.Node

	resolution float64
	tol        float64
}

func reverse(f []float64) {
	for i, j := 0, len(f)-1; i < j; i, j = i+1, j-1 {
		f[i], f[j] = f[j], f[i]
	}
}

func hasNegative(f []float64) bool {
	for _, v := range f {
		if v < 0 {
			return true
		}
	}
	return false
}

var (
	dupGraph         = simple.NewUndirectedGraph()
	dupGraphDirected = simple.NewDirectedGraph()
)

func init() {
	err := gen.Duplication(dupGraph, 1000, 0.8, 0.1, 0.5, rand.New(rand.NewSource(1)))
	if err != nil {
		panic(err)
	}

	// Construct a directed graph from dupGraph
	// such that every edge dupGraph is replaced
	// with an edge that flows from the low node
	// ID to the high node ID.
	for _, e := range graph.EdgesOf(dupGraph.Edges()) {
		if e.To().ID() < e.From().ID() {
			se := e.(simple.Edge)
			se.F, se.T = se.T, se.F
			e = se
		}
		dupGraphDirected.SetEdge(e)
	}
}

// This init function checks the Middle East relationship data.
func init() {
	world := make([]intset, len(middleEast.friends))
	for i := range world {
		world[i] = make(intset)
	}
	for _, relationships := range [][]intset{middleEast.friends, middleEast.complicated, middleEast.enemies} {
		for i, rel := range relationships {
			for inter := range rel {
				if _, ok := world[i][inter]; ok {
					panic(fmt.Sprintf("unexpected relationship: %v--%v", i, inter))
				}
				world[i][inter] = struct{}{}
			}
		}
	}
	for i := range world {
		if len(world[i]) != len(middleEast.friends)-1 {
			panic(fmt.Sprintf("missing relationship in %v: %v", i, world[i]))
		}
	}
}
