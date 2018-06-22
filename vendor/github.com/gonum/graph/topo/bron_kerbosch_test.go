// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package topo

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/gonum/graph/internal/ordered"
	"github.com/gonum/graph/simple"
)

var vOrderTests = []struct {
	g        []intset
	wantCore [][]int
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
		wantCore: [][]int{
			{},
			{5},
			{3},
			{0, 1, 2, 4, 6},
		},
		wantK: 3,
	},
	{
		g: batageljZaversnikGraph,
		wantCore: [][]int{
			{0},
			{5, 9, 10, 16},
			{1, 2, 3, 4, 11, 12, 13, 15},
			{6, 7, 8, 14, 17, 18, 19, 20},
		},
		wantK: 3,
	},
}

func TestVertexOrdering(t *testing.T) {
	for i, test := range vOrderTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		order, core := VertexOrdering(g)
		if len(core)-1 != test.wantK {
			t.Errorf("unexpected value of k for test %d: got: %d want: %d", i, len(core)-1, test.wantK)
		}
		var offset int
		for k, want := range test.wantCore {
			sort.Ints(want)
			got := make([]int, len(want))
			for j, n := range order[len(order)-len(want)-offset : len(order)-offset] {
				got[j] = n.ID()
			}
			sort.Ints(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected %d-core for test %d:\ngot: %v\nwant:%v", got, test.wantCore)
			}

			for j, n := range core[k] {
				got[j] = n.ID()
			}
			sort.Ints(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected %d-core for test %d:\ngot: %v\nwant:%v", got, test.wantCore)
			}
			offset += len(want)
		}
	}
}

var bronKerboschTests = []struct {
	g    []intset
	want [][]int
}{
	{
		// This is the example given in the Bron-Kerbosch article on wikipedia (renumbered).
		// http://en.wikipedia.org/w/index.php?title=Bron%E2%80%93Kerbosch_algorithm&oldid=656805858
		g: []intset{
			0: linksTo(1, 4),
			1: linksTo(2, 4),
			2: linksTo(3),
			3: linksTo(4, 5),
			4: nil,
			5: nil,
		},
		want: [][]int{
			{0, 1, 4},
			{1, 2},
			{2, 3},
			{3, 4},
			{3, 5},
		},
	},
	{
		g: batageljZaversnikGraph,
		want: [][]int{
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
	for i, test := range bronKerboschTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		cliques := BronKerbosch(g)
		got := make([][]int, len(cliques))
		for j, c := range cliques {
			ids := make([]int, len(c))
			for k, n := range c {
				ids[k] = n.ID()
			}
			sort.Ints(ids)
			got[j] = ids
		}
		sort.Sort(ordered.BySliceValues(got))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected cliques for test %d:\ngot: %v\nwant:%v", i, got, test.want)
		}
	}
}
