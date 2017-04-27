// Copyright ©2015 The gonum Authors. All rights reserved.
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

var cyclesInTests = []struct {
	g    []set
	sccs [][]int
	want [][]int
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
			{0, 1, 7, 0},
			{2, 3, 4, 2},
			{2, 6, 3, 4, 2},
		},
	},
	{
		g: []set{
			0: linksTo(1, 2, 3),
			1: linksTo(2),
			2: linksTo(3),
			3: linksTo(1),
		},
		want: [][]int{
			{1, 2, 3, 1},
		},
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(0, 2),
			2: linksTo(1),
		},
		want: [][]int{
			{0, 1, 0},
			{1, 2, 1},
		},
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
		want: nil,
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(2, 3, 4),
			2: linksTo(0, 3),
			3: linksTo(4),
			4: linksTo(3),
		},
		want: [][]int{
			{0, 1, 2, 0},
			{3, 4, 3},
		},
	},
}

func TestCyclesIn(t *testing.T) {
	for i, test := range cyclesInTests {
		g := concrete.NewDirectedGraph()
		g.AddNode(concrete.Node(-10)) // Make sure we test graphs with sparse IDs.
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		cycles := topo.CyclesIn(g)
		var got [][]int
		if cycles != nil {
			got = make([][]int, len(cycles))
		}
		// johnson.circuit does range iteration over maps,
		// so sort to ensure consistent ordering.
		for j, c := range cycles {
			ids := make([]int, len(c))
			for k, n := range c {
				ids[k] = n.ID()
			}
			got[j] = ids
		}
		sort.Sort(internal.BySliceValues(got))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected johnson result for %d:\n\tgot:%#v\n\twant:%#v", i, got, test.want)
		}
	}
}
