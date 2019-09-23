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

var undirectedCyclesInTests = []struct {
	g    []intset
	want [][][]int64
}{
	{
		g: []intset{
			0:  linksTo(1, 2),
			1:  linksTo(2, 4, 5, 9),
			2:  linksTo(4, 7, 9),
			3:  linksTo(5),
			4:  linksTo(8),
			5:  linksTo(7, 8),
			6:  nil,
			7:  nil,
			8:  nil,
			9:  nil,
			10: linksTo(11, 12),
			11: linksTo(12),
			12: nil,
		},
		want: [][][]int64{
			{
				{0, 1, 2, 0},
				{1, 2, 7, 5, 1},
				{1, 2, 9, 1},
				{1, 4, 8, 5, 1},
				{2, 4, 8, 5, 7, 2},
				{10, 11, 12, 10},
			},
			{
				{0, 1, 2, 0},
				{1, 2, 4, 1},
				{1, 2, 7, 5, 1},
				{1, 2, 9, 1},
				{1, 4, 8, 5, 1},
				{10, 11, 12, 10},
			},
			{
				{0, 1, 2, 0},
				{1, 2, 4, 1},
				{1, 2, 9, 1},
				{1, 4, 8, 5, 1},
				{2, 4, 8, 5, 7, 2},
				{10, 11, 12, 10},
			},
			{
				{0, 1, 2, 0},
				{1, 2, 4, 1},
				{1, 2, 7, 5, 1},
				{1, 2, 9, 1},
				{2, 4, 8, 5, 7, 2},
				{10, 11, 12, 10},
			},
		},
	},
}

func TestUndirectedCyclesIn(t *testing.T) {
	for i, test := range undirectedCyclesInTests {
		g := simple.NewUndirectedGraph()
		g.AddNode(simple.Node(-10)) // Make sure we test graphs with sparse IDs.
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		cycles := UndirectedCyclesIn(g)
		var got [][]int64
		if cycles != nil {
			got = make([][]int64, len(cycles))
		}
		// Canonicalise the cycles.
		for j, c := range cycles {
			ids := make([]int64, len(c))
			for k, n := range canonicalise(c[:len(c)-1]) {
				ids[k] = n.ID()
			}
			ids[len(ids)-1] = ids[0]
			got[j] = ids
		}
		sort.Sort(ordered.BySliceValues(got))
		var matched bool
		for _, want := range test.want {
			if reflect.DeepEqual(got, want) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("unexpected paton result for %d:\n\tgot:%#v\n\twant from:%#v", i, got, test.want)
		}
	}
}

// canonicalise returns the cycle path c cyclicly permuted such that
// the first element has the lowest ID and then conditionally
// reversed so that the second element has the lowest possible
// neighbouring ID.
// c lists each node only onces - the final node must not be a
// reiteration of the first node.
func canonicalise(c []graph.Node) []graph.Node {
	if len(c) < 2 {
		return c
	}
	idx := 0
	min := c[0].ID()
	for i, n := range c[1:] {
		if id := n.ID(); id < min {
			idx = i + 1
			min = id
		}
	}
	if idx != 0 {
		c = append(c[idx:], c[:idx]...)
	}
	if c[len(c)-1].ID() < c[1].ID() {
		ordered.Reverse(c[1:])
	}
	return c
}
