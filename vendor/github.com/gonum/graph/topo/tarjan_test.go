// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package topo_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/internal"
	"github.com/gonum/graph/topo"
)

type interval struct{ start, end int }

var tarjanTests = []struct {
	g []set

	ambiguousOrder []interval
	want           [][]int

	sortedLength      int
	unorderableLength int
	sortable          bool
}{
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(2, 7),
			2: linksTo(3, 6),
			3: linksTo(4),
			4: linksTo(2, 5),
			6: linksTo(3, 5),
			7: linksTo(0, 6),
		},

		want: [][]int{
			{5},
			{2, 3, 4, 6},
			{0, 1, 7},
		},

		sortedLength:      1,
		unorderableLength: 2,
		sortable:          false,
	},
	{
		g: []set{
			0: linksTo(1, 2, 3),
			1: linksTo(2),
			2: linksTo(3),
			3: linksTo(1),
		},

		want: [][]int{
			{1, 2, 3},
			{0},
		},

		sortedLength:      1,
		unorderableLength: 1,
		sortable:          false,
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(0, 2),
			2: linksTo(1),
		},

		want: [][]int{
			{0, 1, 2},
		},

		sortedLength:      0,
		unorderableLength: 1,
		sortable:          false,
	},
	{
		g: []set{
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
		want: [][]int{
			{6}, {5}, {4}, {3}, {2}, {1}, {0},
		},

		sortedLength: 7,
		sortable:     true,
	},
	{
		g: []set{
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
		want: [][]int{
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
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		sorted, err := topo.Sort(g)
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
		if err != nil && len(err.(topo.Unorderable)) != test.unorderableLength {
			t.Errorf("unexpected number of unorderable nodes for test %d: got:%d want:%d", i, len(err.(topo.Unorderable)), test.unorderableLength)
		}
	}
}

func TestTarjanSCC(t *testing.T) {
	for i, test := range tarjanTests {
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		gotSCCs := topo.TarjanSCC(g)
		// tarjan.strongconnect does range iteration over maps,
		// so sort SCC members to ensure consistent ordering.
		gotIDs := make([][]int, len(gotSCCs))
		for i, scc := range gotSCCs {
			gotIDs[i] = make([]int, len(scc))
			for j, id := range scc {
				gotIDs[i][j] = id.ID()
			}
			sort.Ints(gotIDs[i])
		}
		for _, iv := range test.ambiguousOrder {
			sort.Sort(internal.BySliceValues(test.want[iv.start:iv.end]))
			sort.Sort(internal.BySliceValues(gotIDs[iv.start:iv.end]))
		}
		if !reflect.DeepEqual(gotIDs, test.want) {
			t.Errorf("unexpected Tarjan scc result for %d:\n\tgot:%v\n\twant:%v", i, gotIDs, test.want)
		}
	}
}
