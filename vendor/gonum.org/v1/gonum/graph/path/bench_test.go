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
)

func gnpUndirected(n int, p float64) graph.Undirected {
	g := simple.NewUndirectedGraph()
	gen.Gnp(g, n, p, nil)
	return g
}

func benchmarkAStarNilHeuristic(b *testing.B, g graph.Undirected) {
	var expanded int
	for i := 0; i < b.N; i++ {
		_, expanded = AStar(simple.Node(0), simple.Node(1), g, nil)
	}
	if expanded == 0 {
		b.Fatal("unexpected number of expanded nodes")
	}
}

func BenchmarkAStarGnp_10_tenth(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_10_tenth)
}
func BenchmarkAStarGnp_100_tenth(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_100_tenth)
}
func BenchmarkAStarGnp_1000_tenth(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_1000_tenth)
}
func BenchmarkAStarGnp_10_half(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_10_half)
}
func BenchmarkAStarGnp_100_half(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_100_half)
}
func BenchmarkAStarGnp_1000_half(b *testing.B) {
	benchmarkAStarNilHeuristic(b, gnpUndirected_1000_half)
}

var (
	nswUndirected_10_2_2_2   = navigableSmallWorldUndirected(10, 2, 2, 2)
	nswUndirected_10_2_5_2   = navigableSmallWorldUndirected(10, 2, 5, 2)
	nswUndirected_100_5_10_2 = navigableSmallWorldUndirected(100, 5, 10, 2)
	nswUndirected_100_5_20_2 = navigableSmallWorldUndirected(100, 5, 20, 2)
)

func navigableSmallWorldUndirected(n, p, q int, r float64) graph.Undirected {
	g := simple.NewUndirectedGraph()
	gen.NavigableSmallWorld(g, []int{n, n}, p, q, r, nil)
	return g
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

func benchmarkAStarHeuristic(b *testing.B, g graph.Undirected, h Heuristic) {
	var expanded int
	for i := 0; i < b.N; i++ {
		_, expanded = AStar(simple.Node(0), simple.Node(1), g, h)
	}
	if expanded == 0 {
		b.Fatal("unexpected number of expanded nodes")
	}
}

func BenchmarkAStarUndirectedmallWorld_10_2_2_2(b *testing.B) {
	benchmarkAStarHeuristic(b, nswUndirected_10_2_2_2, nil)
}
func BenchmarkAStarUndirectedmallWorld_10_2_2_2_Heur(b *testing.B) {
	h := func(x, y graph.Node) float64 {
		return manhattanBetween(coordinatesForID(x, 10, 10), coordinatesForID(y, 10, 10))
	}
	benchmarkAStarHeuristic(b, nswUndirected_10_2_2_2, h)
}
func BenchmarkAStarUndirectedmallWorld_10_2_5_2(b *testing.B) {
	benchmarkAStarHeuristic(b, nswUndirected_10_2_5_2, nil)
}
func BenchmarkAStarUndirectedmallWorld_10_2_5_2_Heur(b *testing.B) {
	h := func(x, y graph.Node) float64 {
		return manhattanBetween(coordinatesForID(x, 10, 10), coordinatesForID(y, 10, 10))
	}
	benchmarkAStarHeuristic(b, nswUndirected_10_2_5_2, h)
}
func BenchmarkAStarUndirectedmallWorld_100_5_10_2(b *testing.B) {
	benchmarkAStarHeuristic(b, nswUndirected_100_5_10_2, nil)
}
func BenchmarkAStarUndirectedmallWorld_100_5_10_2_Heur(b *testing.B) {
	h := func(x, y graph.Node) float64 {
		return manhattanBetween(coordinatesForID(x, 100, 100), coordinatesForID(y, 100, 100))
	}
	benchmarkAStarHeuristic(b, nswUndirected_100_5_10_2, h)
}
func BenchmarkAStarUndirectedmallWorld_100_5_20_2(b *testing.B) {
	benchmarkAStarHeuristic(b, nswUndirected_100_5_20_2, nil)
}
func BenchmarkAStarUndirectedmallWorld_100_5_20_2_Heur(b *testing.B) {
	h := func(x, y graph.Node) float64 {
		return manhattanBetween(coordinatesForID(x, 100, 100), coordinatesForID(y, 100, 100))
	}
	benchmarkAStarHeuristic(b, nswUndirected_100_5_20_2, h)
}
