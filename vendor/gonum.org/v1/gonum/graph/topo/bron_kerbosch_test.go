// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package topo

import (
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

var vOrderTests = []struct {
	g        []intset
	wantCore [][]int64
	wantK    int
}{
	{
		g: []intset{
			0: linksTo(1, 2, 4, 6),
			1: linksTo(2, 4, 6),
			2: linksTo(3, 6),
			3: linksTo(4, 5),
			4: linksTo(6),
			5: nil,
			6: nil,
		},
		wantCore: [][]int64{
			{},
			{5},
			{3},
			{0, 1, 2, 4, 6},
		},
		wantK: 3,
	},
	{
		g: batageljZaversnikGraph,
		wantCore: [][]int64{
			{0},
			{5, 9, 10, 16},
			{1, 2, 3, 4, 11, 12, 13, 15},
			{6, 7, 8, 14, 17, 18, 19, 20},
		},
		wantK: 3,
	},
}

func TestDegeneracyOrdering(t *testing.T) {
	for i, test := range vOrderTests {
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
		order, core := DegeneracyOrdering(g)
		if len(core)-1 != test.wantK {
			t.Errorf("unexpected value of k for test %d: got: %d want: %d", i, len(core)-1, test.wantK)
		}
		var offset int
		for k, want := range test.wantCore {
			sort.Sort(ordered.Int64s(want))
			got := make([]int64, len(want))
			for j, n := range order[len(order)-len(want)-offset : len(order)-offset] {
				got[j] = n.ID()
			}
			sort.Sort(ordered.Int64s(got))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected %d-core for test %d:\ngot: %v\nwant:%v", k, i, got, want)
			}

			for j, n := range core[k] {
				got[j] = n.ID()
			}
			sort.Sort(ordered.Int64s(got))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected %d-core for test %d:\ngot: %v\nwant:%v", k, i, got, want)
			}
			offset += len(want)
		}
	}
}

func TestKCore(t *testing.T) {
	for i, test := range vOrderTests {
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

		for k := 0; k <= test.wantK+1; k++ {
			var want []int64
			for _, c := range test.wantCore[k:] {
				want = append(want, c...)
			}
			core := KCore(k, g)
			if len(core) != len(want) {
				t.Errorf("unexpected %d-core length for test %d:\ngot: %v\nwant:%v", k, i, len(core), len(want))
				continue
			}

			var got []int64
			for _, n := range core {
				got = append(got, n.ID())
			}
			sort.Sort(ordered.Int64s(got))
			sort.Sort(ordered.Int64s(want))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected %d-core for test %d:\ngot: %v\nwant:%v", k, i, got, want)
			}
		}
	}
}

var bronKerboschTests = []struct {
	name string
	g    []intset
	want [][]int64
}{
	{
		// This is the example given in the Bron-Kerbosch article on wikipedia (renumbered).
		// http://en.wikipedia.org/w/index.php?title=Bron%E2%80%93Kerbosch_algorithm&oldid=656805858
		name: "wikipedia example",
		g: []intset{
			0: linksTo(1, 4),
			1: linksTo(2, 4),
			2: linksTo(3),
			3: linksTo(4, 5),
			4: nil,
			5: nil,
		},
		want: [][]int64{
			{0, 1, 4},
			{1, 2},
			{2, 3},
			{3, 4},
			{3, 5},
		},
	},
	{
		name: "Batagelj-Zaversnik Graph",
		g:    batageljZaversnikGraph,
		want: [][]int64{
			{0},
			{1, 2},
			{1, 3},
			{2, 4},
			{3, 4},
			{4, 5},
			{6, 7, 8, 14},
			{7, 11, 12},
			{9, 11},
			{10, 11},
			{12, 18},
			{13, 14, 15},
			{14, 15, 17},
			{15, 16},
			{17, 18, 19, 20},
		},
	},
}

func TestBronKerbosch(t *testing.T) {
	for _, test := range bronKerboschTests {
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
		cliques := BronKerbosch(g)
		got := make([][]int64, len(cliques))
		for j, c := range cliques {
			ids := make([]int64, len(c))
			for k, n := range c {
				ids[k] = n.ID()
			}
			sort.Sort(ordered.Int64s(ids))
			got[j] = ids
		}
		sort.Sort(ordered.BySliceValues(got))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected cliques for test %q:\ngot: %v\nwant:%v", test.name, got, test.want)
		}
	}
}

func BenchmarkBronKerbosch(b *testing.B) {
	for _, test := range bronKerboschTests {
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

		b.Run(test.name, func(b *testing.B) {
			var got [][]graph.Node
			for i := 0; i < b.N; i++ {
				got = BronKerbosch(g)
			}
			if len(got) != len(test.want) {
				b.Errorf("unexpected cliques for test %q:\ngot: %v\nwant:%v", test.name, got, test.want)
			}
		})
	}
}
