// Copyright ©2014 The Gonum Authors. All rights reserved.
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

type interval struct{ start, end int }

var tarjanTests = []struct {
	g []intset

	ambiguousOrder []interval
	want           [][]int64

	sortedLength      int
	unorderableLength int
	sortable          bool
}{
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 7),
			2: linksTo(3, 6),
			3: linksTo(4),
			4: linksTo(2, 5),
			6: linksTo(3, 5),
			7: linksTo(0, 6),
		},

		want: [][]int64{
			{5},
			{2, 3, 4, 6},
			{0, 1, 7},
		},

		sortedLength:      1,
		unorderableLength: 2,
		sortable:          false,
	},
	{
		g: []intset{
			0: linksTo(1, 2, 3),
			1: linksTo(2),
			2: linksTo(3),
			3: linksTo(1),
		},

		want: [][]int64{
			{1, 2, 3},
			{0},
		},

		sortedLength:      1,
		unorderableLength: 1,
		sortable:          false,
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(0, 2),
			2: linksTo(1),
		},

		want: [][]int64{
			{0, 1, 2},
		},

		sortedLength:      0,
		unorderableLength: 1,
		sortable:          false,
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 3),
			2: linksTo(4, 5),
			3: linksTo(4, 5),
			4: linksTo(6),
			5: nil,
			6: nil,
		},

		// Node pairs (2, 3) and (4, 5) are not
		// relatively orderable within each pair.
		ambiguousOrder: []interval{
			{0, 3}, // This includes node 6 since it only needs to be before 4 in topo sort.
			{3, 5},
		},
		want: [][]int64{
			{6}, {5}, {4}, {3}, {2}, {1}, {0},
		},

		sortedLength: 7,
		sortable:     true,
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 3, 4),
			2: linksTo(0, 3),
			3: linksTo(4),
			4: linksTo(3),
		},

		// SCCs are not relatively ordable.
		ambiguousOrder: []interval{
			{0, 2},
		},
		want: [][]int64{
			{0, 1, 2},
			{3, 4},
		},

		sortedLength:      0,
		unorderableLength: 2,
		sortable:          false,
	},
}

func TestSort(t *testing.T) {
	for i, test := range tarjanTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		sorted, err := Sort(g)
		var gotSortedLen int
		for _, n := range sorted {
			if n != nil {
				gotSortedLen++
			}
		}
		if gotSortedLen != test.sortedLength {
			t.Errorf("unexpected number of sortable nodes for test %d: got:%d want:%d", i, gotSortedLen, test.sortedLength)
		}
		if err == nil != test.sortable {
			t.Errorf("unexpected sortability for test %d: got error: %v want: nil-error=%t", i, err, test.sortable)
		}
		if err != nil && len(err.(Unorderable)) != test.unorderableLength {
			t.Errorf("unexpected number of unorderable nodes for test %d: got:%d want:%d", i, len(err.(Unorderable)), test.unorderableLength)
		}
	}
}

func TestTarjanSCC(t *testing.T) {
	for i, test := range tarjanTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		gotSCCs := TarjanSCC(g)
		// tarjan.strongconnect does range iteration over maps,
		// so sort SCC members to ensure consistent ordering.
		gotIDs := make([][]int64, len(gotSCCs))
		for i, scc := range gotSCCs {
			gotIDs[i] = make([]int64, len(scc))
			for j, id := range scc {
				gotIDs[i][j] = id.ID()
			}
			sort.Sort(ordered.Int64s(gotIDs[i]))
		}
		for _, iv := range test.ambiguousOrder {
			sort.Sort(ordered.BySliceValues(test.want[iv.start:iv.end]))
			sort.Sort(ordered.BySliceValues(gotIDs[iv.start:iv.end]))
		}
		if !reflect.DeepEqual(gotIDs, test.want) {
			t.Errorf("unexpected Tarjan scc result for %d:\n\tgot:%v\n\twant:%v", i, gotIDs, test.want)
		}
	}
}

var stabilizedSortTests = []struct {
	g []intset

	want []graph.Node
	err  error
}{
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 7),
			2: linksTo(3, 6),
			3: linksTo(4),
			4: linksTo(2, 5),
			6: linksTo(3, 5),
			7: linksTo(0, 6),
		},

		want: []graph.Node{nil, nil, simple.Node(5)},
		err: Unorderable{
			{simple.Node(0), simple.Node(1), simple.Node(7)},
			{simple.Node(2), simple.Node(3), simple.Node(4), simple.Node(6)},
		},
	},
	{
		g: []intset{
			0: linksTo(1, 2, 3),
			1: linksTo(2),
			2: linksTo(3),
			3: linksTo(1),
		},

		want: []graph.Node{simple.Node(0), nil},
		err: Unorderable{
			{simple.Node(1), simple.Node(2), simple.Node(3)},
		},
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(0, 2),
			2: linksTo(1),
		},

		want: []graph.Node{nil},
		err: Unorderable{
			{simple.Node(0), simple.Node(1), simple.Node(2)},
		},
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 3),
			2: linksTo(4, 5),
			3: linksTo(4, 5),
			4: linksTo(6),
			5: nil,
			6: nil,
		},

		want: []graph.Node{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(4), simple.Node(5), simple.Node(6)},
		err:  nil,
	},
	{
		g: []intset{
			0: linksTo(1),
			1: linksTo(2, 3, 4),
			2: linksTo(0, 3),
			3: linksTo(4),
			4: linksTo(3),
		},

		want: []graph.Node{nil, nil},
		err: Unorderable{
			{simple.Node(0), simple.Node(1), simple.Node(2)},
			{simple.Node(3), simple.Node(4)},
		},
	},
	{
		g: []intset{
			0: linksTo(1, 2, 3, 4, 5, 6),
			1: linksTo(7),
			2: linksTo(7),
			3: linksTo(7),
			4: linksTo(7),
			5: linksTo(7),
			6: linksTo(7),
			7: nil,
		},

		want: []graph.Node{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(4), simple.Node(5), simple.Node(6), simple.Node(7)},
		err:  nil,
	},
}

func TestSortStabilized(t *testing.T) {
	for i, test := range stabilizedSortTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		got, err := SortStabilized(g, nil)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected sort result for test %d: got:%d want:%d", i, got, test.want)
		}
		if !reflect.DeepEqual(err, test.err) {
			t.Errorf("unexpected sort error for test %d: got:%v want:%v", i, err, test.want)
		}
	}
}
