// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/graphs/gen"
	"gonum.org/v1/gonum/graph/simple"
)

var (
	gnpUndirected_10_tenth   = gnpUndirected(10, 0.1)
	gnpUndirected_100_tenth  = gnpUndirected(100, 0.1)
	gnpUndirected_1000_tenth = gnpUndirected(1000, 0.1)
	gnpUndirected_10_half    = gnpUndirected(10, 0.5)
	gnpUndirected_100_half   = gnpUndirected(100, 0.5)
	gnpUndirected_1000_half  = gnpUndirected(1000, 0.5)

	nswUndirected_10_2_2_2   = navigableSmallWorldUndirected(10, 2, 2, 2)
	nswUndirected_10_2_5_2   = navigableSmallWorldUndirected(10, 2, 5, 2)
	nswUndirected_100_5_10_2 = navigableSmallWorldUndirected(100, 5, 10, 2)
	nswUndirected_100_5_20_2 = navigableSmallWorldUndirected(100, 5, 20, 2)
)

func gnpUndirected(n int, p float64) graph.Undirected {
	g := simple.NewUndirectedGraph()
	gen.Gnp(g, n, p, nil)
	return g
}

func navigableSmallWorldUndirected(n, p, q int, r float64) graph.Undirected {
	g := simple.NewUndirectedGraph()
	gen.NavigableSmallWorld(g, []int{n, n}, p, q, r, nil)
	return g
}

func manhattan(size int) func(x, y graph.Node) float64 {
	return func(x, y graph.Node) float64 {
		return manhattanBetween(coordinatesForID(x, size, size), coordinatesForID(y, size, size))
	}
}

func coordinatesForID(n graph.Node, c, r int) [2]int {
	id := n.ID()
	if id >= int64(c*r) {
		panic("out of range")
	}
	return [2]int{int(id) / r, int(id) % r}
}

// manhattanBetween returns the Manhattan distance between a and b.
func manhattanBetween(a, b [2]int) float64 {
	var d int
	for i, v := range a {
		d += abs(v - b[i])
	}
	return float64(d)
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func BenchmarkAStarUndirected(b *testing.B) {
	benchmarks := []struct {
		name  string
		graph graph.Undirected
		h     Heuristic
	}{
		{"GNP Undirected 10 tenth", gnpUndirected_10_tenth, nil},
		{"GNP Undirected 100 tenth", gnpUndirected_100_tenth, nil},
		{"GNP Undirected 1000 tenth", gnpUndirected_1000_tenth, nil},
		{"GNP Undirected 10 half", gnpUndirected_10_half, nil},
		{"GNP Undirected 100 half", gnpUndirected_100_half, nil},
		{"GNP Undirected 1000 half", gnpUndirected_1000_half, nil},

		{"NSW Undirected 10 2 2 2", nswUndirected_10_2_2_2, nil},
		{"NSW Undirected 10 2 2 2 heuristic", nswUndirected_10_2_2_2, manhattan(10)},
		{"NSW Undirected 10 2 5 2", nswUndirected_10_2_5_2, nil},
		{"NSW Undirected 10 2 5 2 heuristic", nswUndirected_10_2_5_2, manhattan(10)},
		{"NSW Undirected 100 5 10 2", nswUndirected_100_5_10_2, nil},
		{"NSW Undirected 100 5 10 2 heuristic", nswUndirected_100_5_10_2, manhattan(100)},
		{"NSW Undirected 100 5 20 2", nswUndirected_100_5_20_2, nil},
		{"NSW Undirected 100 5 20 2 heuristic", nswUndirected_100_5_20_2, manhattan(100)},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			var expanded int
			for i := 0; i < b.N; i++ {
				_, expanded = AStar(simple.Node(0), simple.Node(1), bm.graph, bm.h)
			}
			if expanded == 0 {
				b.Fatal("unexpected number of expanded nodes")
			}
		})
	}
}

var (
	gnpDirected_500_tenth  = gnpDirected(500, 0.1)
	gnpDirected_1000_tenth = gnpDirected(1000, 0.1)
	gnpDirected_2000_tenth = gnpDirected(2000, 0.1)
	gnpDirected_500_half   = gnpDirected(500, 0.5)
	gnpDirected_1000_half  = gnpDirected(1000, 0.5)
	gnpDirected_2000_half  = gnpDirected(2000, 0.5)
	gnpDirected_500_full   = gnpDirected(500, 1)
	gnpDirected_1000_full  = gnpDirected(1000, 1)
	gnpDirected_2000_full  = gnpDirected(2000, 1)
)

func gnpDirected(n int, p float64) graph.Directed {
	g := simple.NewDirectedGraph()
	gen.Gnp(g, n, p, nil)
	return g
}

func BenchmarkBellmanFordFrom(b *testing.B) {
	benchmarks := []struct {
		name  string
		graph graph.Directed
	}{
		{"500 tenth", gnpDirected_500_tenth},
		{"1000 tenth", gnpDirected_1000_tenth},
		{"2000 tenth", gnpDirected_2000_tenth},
		{"500 half", gnpDirected_500_half},
		{"1000 half", gnpDirected_1000_half},
		{"2000 half", gnpDirected_2000_half},
		{"500 full", gnpDirected_500_full},
		{"1000 full", gnpDirected_1000_full},
		{"2000 full", gnpDirected_2000_full},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				BellmanFordFrom(bm.graph.Node(0), bm.graph)
			}
		})
	}
}
