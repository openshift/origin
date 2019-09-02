// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package traverse

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/graphs/gen"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

var (
	// batageljZaversnikGraph is the example graph from
	// figure 1 of http://arxiv.org/abs/cs/0310049v1
	batageljZaversnikGraph = []intset{
		0: nil,

		1: linksTo(2, 3),
		2: linksTo(4),
		3: linksTo(4),
		4: linksTo(5),
		5: nil,

		6:  linksTo(7, 8, 14),
		7:  linksTo(8, 11, 12, 14),
		8:  linksTo(14),
		9:  linksTo(11),
		10: linksTo(11),
		11: linksTo(12),
		12: linksTo(18),
		13: linksTo(14, 15),
		14: linksTo(15, 17),
		15: linksTo(16, 17),
		16: nil,
		17: linksTo(18, 19, 20),
		18: linksTo(19, 20),
		19: linksTo(20),
		20: nil,
	}

	// wpBronKerboschGraph is the example given in the Bron-Kerbosch article on wikipedia (renumbered).
	// http://en.wikipedia.org/w/index.php?title=Bron%E2%80%93Kerbosch_algorithm&oldid=656805858
	wpBronKerboschGraph = []intset{
		0: linksTo(1, 4),
		1: linksTo(2, 4),
		2: linksTo(3),
		3: linksTo(4, 5),
		4: nil,
		5: nil,
	}
)

var breadthFirstTests = []struct {
	g     []intset
	from  graph.Node
	edge  func(graph.Edge) bool
	until func(graph.Node, int) bool
	final map[graph.Node]bool
	want  [][]int64
}{
	{
		g:     wpBronKerboschGraph,
		from:  simple.Node(1),
		final: map[graph.Node]bool{nil: true},
		want: [][]int64{
			{1},
			{0, 2, 4},
			{3},
			{5},
		},
	},
	{
		g: wpBronKerboschGraph,
		edge: func(e graph.Edge) bool {
			// Do not traverse an edge between 3 and 5.
			return (e.From().ID() != 3 || e.To().ID() != 5) && (e.From().ID() != 5 || e.To().ID() != 3)
		},
		from:  simple.Node(1),
		final: map[graph.Node]bool{nil: true},
		want: [][]int64{
			{1},
			{0, 2, 4},
			{3},
		},
	},
	{
		g:     wpBronKerboschGraph,
		from:  simple.Node(1),
		until: func(n graph.Node, _ int) bool { return n == simple.Node(3) },
		final: map[graph.Node]bool{simple.Node(3): true},
		want: [][]int64{
			{1},
			{0, 2, 4},
		},
	},
	{
		g:     batageljZaversnikGraph,
		from:  simple.Node(13),
		final: map[graph.Node]bool{nil: true},
		want: [][]int64{
			{13},
			{14, 15},
			{6, 7, 8, 16, 17},
			{11, 12, 18, 19, 20},
			{9, 10},
		},
	},
	{
		g:     batageljZaversnikGraph,
		from:  simple.Node(13),
		until: func(_ graph.Node, d int) bool { return d > 2 },
		final: map[graph.Node]bool{
			simple.Node(11): true,
			simple.Node(12): true,
			simple.Node(18): true,
			simple.Node(19): true,
			simple.Node(20): true,
		},
		want: [][]int64{
			{13},
			{14, 15},
			{6, 7, 8, 16, 17},
		},
	},
}

func TestBreadthFirst(t *testing.T) {
	for i, test := range breadthFirstTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		w := BreadthFirst{
			Traverse: test.edge,
		}
		var got [][]int64
		final := w.Walk(g, test.from, func(n graph.Node, d int) bool {
			if test.until != nil && test.until(n, d) {
				return true
			}
			if d >= len(got) {
				got = append(got, []int64(nil))
			}
			got[d] = append(got[d], n.ID())
			return false
		})
		if !test.final[final] {
			t.Errorf("unexepected final node for test %d:\ngot:  %v\nwant: %v", i, final, test.final)
		}
		for _, l := range got {
			sort.Sort(ordered.Int64s(l))
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexepected BFS level structure for test %d:\ngot:  %v\nwant: %v", i, got, test.want)
		}
	}
}

var depthFirstTests = []struct {
	g     []intset
	from  graph.Node
	edge  func(graph.Edge) bool
	until func(graph.Node) bool
	final map[graph.Node]bool
	want  []int64
}{
	{
		g:     wpBronKerboschGraph,
		from:  simple.Node(1),
		final: map[graph.Node]bool{nil: true},
		want:  []int64{0, 1, 2, 3, 4, 5},
	},
	{
		g: wpBronKerboschGraph,
		edge: func(e graph.Edge) bool {
			// Do not traverse an edge between 3 and 5.
			return (e.From().ID() != 3 || e.To().ID() != 5) && (e.From().ID() != 5 || e.To().ID() != 3)
		},
		from:  simple.Node(1),
		final: map[graph.Node]bool{nil: true},
		want:  []int64{0, 1, 2, 3, 4},
	},
	{
		g:     wpBronKerboschGraph,
		from:  simple.Node(1),
		until: func(n graph.Node) bool { return n == simple.Node(3) },
		final: map[graph.Node]bool{simple.Node(3): true},
	},
	{
		g:     batageljZaversnikGraph,
		from:  simple.Node(0),
		final: map[graph.Node]bool{nil: true},
		want:  []int64{0},
	},
	{
		g:     batageljZaversnikGraph,
		from:  simple.Node(3),
		final: map[graph.Node]bool{nil: true},
		want:  []int64{1, 2, 3, 4, 5},
	},
	{
		g:     batageljZaversnikGraph,
		from:  simple.Node(13),
		final: map[graph.Node]bool{nil: true},
		want:  []int64{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
	},
}

func TestDepthFirst(t *testing.T) {
	for i, test := range depthFirstTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		w := DepthFirst{
			Traverse: test.edge,
		}
		var got []int64
		final := w.Walk(g, test.from, func(n graph.Node) bool {
			if test.until != nil && test.until(n) {
				return true
			}
			got = append(got, n.ID())
			return false
		})
		if !test.final[final] {
			t.Errorf("unexepected final node for test %d:\ngot:  %v\nwant: %v", i, final, test.final)
		}
		sort.Sort(ordered.Int64s(got))
		if test.want != nil && !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexepected DFS traversed nodes for test %d:\ngot:  %v\nwant: %v", i, got, test.want)
		}
	}
}

var walkAllTests = []struct {
	g    []intset
	edge func(graph.Edge) bool
	want [][]int64
}{
	{
		g: batageljZaversnikGraph,
		want: [][]int64{
			{0},
			{1, 2, 3, 4, 5},
			{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		},
	},
	{
		g: batageljZaversnikGraph,
		edge: func(e graph.Edge) bool {
			// Do not traverse an edge between 3 and 5.
			return (e.From().ID() != 4 || e.To().ID() != 5) && (e.From().ID() != 5 || e.To().ID() != 4)
		},
		want: [][]int64{
			{0},
			{1, 2, 3, 4},
			{5},
			{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		},
	},
}

func TestWalkAll(t *testing.T) {
	for i, test := range walkAllTests {
		g := simple.NewUndirectedGraph()

		for u, e := range test.g {
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				if g.Node(int64(v)) == nil {
					g.AddNode(simple.Node(v))
				}
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		type walker interface {
			WalkAll(g graph.Undirected, before, after func(), during func(graph.Node))
		}
		for _, w := range []walker{
			&BreadthFirst{},
			&DepthFirst{},
		} {
			var (
				c  []graph.Node
				cc [][]graph.Node
			)
			switch w := w.(type) {
			case *BreadthFirst:
				w.Traverse = test.edge
			case *DepthFirst:
				w.Traverse = test.edge
			default:
				panic(fmt.Sprintf("bad walker type: %T", w))
			}
			during := func(n graph.Node) {
				c = append(c, n)
			}
			after := func() {
				cc = append(cc, []graph.Node(nil))
				cc[len(cc)-1] = append(cc[len(cc)-1], c...)
				c = c[:0]
			}
			w.WalkAll(g, nil, after, during)

			got := make([][]int64, len(cc))
			for j, c := range cc {
				ids := make([]int64, len(c))
				for k, n := range c {
					ids[k] = n.ID()
				}
				sort.Sort(ordered.Int64s(ids))
				got[j] = ids
			}
			sort.Sort(ordered.BySliceValues(got))
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("unexpected connected components for test %d using %T:\ngot: %v\nwant:%v", i, w, got, test.want)
			}
		}
	}
}

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

func benchmarkWalkAllBreadthFirst(b *testing.B, g graph.Undirected) {
	n := g.Nodes().Len()
	b.ResetTimer()
	var bft BreadthFirst
	for i := 0; i < b.N; i++ {
		bft.WalkAll(g, nil, nil, nil)
	}
	if len(bft.visited) != n {
		b.Fatalf("unexpected number of nodes visited: want: %d got %d", n, len(bft.visited))
	}
}

func BenchmarkWalkAllBreadthFirstGnp_10_tenth(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_10_tenth)
}
func BenchmarkWalkAllBreadthFirstGnp_100_tenth(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_100_tenth)
}
func BenchmarkWalkAllBreadthFirstGnp_1000_tenth(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_1000_tenth)
}
func BenchmarkWalkAllBreadthFirstGnp_10_half(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_10_half)
}
func BenchmarkWalkAllBreadthFirstGnp_100_half(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_100_half)
}
func BenchmarkWalkAllBreadthFirstGnp_1000_half(b *testing.B) {
	benchmarkWalkAllBreadthFirst(b, gnpUndirected_1000_half)
}

func benchmarkWalkAllDepthFirst(b *testing.B, g graph.Undirected) {
	n := g.Nodes().Len()
	b.ResetTimer()
	var dft DepthFirst
	for i := 0; i < b.N; i++ {
		dft.WalkAll(g, nil, nil, nil)
	}
	if len(dft.visited) != n {
		b.Fatalf("unexpected number of nodes visited: want: %d got %d", n, len(dft.visited))
	}
}

func BenchmarkWalkAllDepthFirstGnp_10_tenth(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_10_tenth)
}
func BenchmarkWalkAllDepthFirstGnp_100_tenth(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_100_tenth)
}
func BenchmarkWalkAllDepthFirstGnp_1000_tenth(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_1000_tenth)
}
func BenchmarkWalkAllDepthFirstGnp_10_half(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_10_half)
}
func BenchmarkWalkAllDepthFirstGnp_100_half(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_100_half)
}
func BenchmarkWalkAllDepthFirstGnp_1000_half(b *testing.B) {
	benchmarkWalkAllDepthFirst(b, gnpUndirected_1000_half)
}
