// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

var dominatorsTests = []struct {
	n     graph.Node
	edges []simple.Edge

	want DominatorTree
}{
	{ // Example from Lengauer and Tarjan http://www.dtic.mil/dtic/tr/fulltext/u2/a054144.pdf fig 1.
		n: char('R'),
		edges: []simple.Edge{
			{F: char('A'), T: char('D')},
			{F: char('B'), T: char('A')},
			{F: char('B'), T: char('D')},
			{F: char('B'), T: char('E')},
			{F: char('C'), T: char('F')},
			{F: char('C'), T: char('G')},
			{F: char('D'), T: char('L')},
			{F: char('E'), T: char('H')},
			{F: char('F'), T: char('I')},
			{F: char('G'), T: char('I')},
			{F: char('G'), T: char('J')},
			{F: char('H'), T: char('E')},
			{F: char('H'), T: char('K')},
			{F: char('I'), T: char('K')},
			{F: char('J'), T: char('I')},
			{F: char('K'), T: char('I')},
			{F: char('K'), T: char('R')},
			{F: char('L'), T: char('H')},
			{F: char('R'), T: char('A')},
			{F: char('R'), T: char('B')},
			{F: char('R'), T: char('C')},
		},

		want: DominatorTree{
			root: char('R'),
			dominatorOf: map[int64]graph.Node{
				'A': char('R'),
				'B': char('R'),
				'C': char('R'),
				'D': char('R'),
				'E': char('R'),
				'F': char('C'),
				'G': char('C'),
				'H': char('R'),
				'I': char('R'),
				'J': char('G'),
				'K': char('R'),
				'L': char('D'),
			},
			dominatedBy: map[int64][]graph.Node{
				'C': {char('F'), char('G')},
				'D': {char('L')},
				'G': {char('J')},
				'R': {char('A'), char('B'), char('C'), char('D'), char('E'), char('H'), char('I'), char('K')},
			},
		},
	},
	{ // WP example: https://en.wikipedia.org/w/index.php?title=Dominator_(graph_theory)&oldid=758099236.
		n: simple.Node(1),
		edges: []simple.Edge{
			{F: simple.Node(1), T: simple.Node(2)},
			{F: simple.Node(2), T: simple.Node(3)},
			{F: simple.Node(2), T: simple.Node(4)},
			{F: simple.Node(2), T: simple.Node(6)},
			{F: simple.Node(3), T: simple.Node(5)},
			{F: simple.Node(4), T: simple.Node(5)},
			{F: simple.Node(5), T: simple.Node(2)},
		},

		want: DominatorTree{
			root: simple.Node(1),
			dominatorOf: map[int64]graph.Node{
				2: simple.Node(1),
				3: simple.Node(2),
				4: simple.Node(2),
				5: simple.Node(2),
				6: simple.Node(2),
			},
			dominatedBy: map[int64][]graph.Node{
				1: {simple.Node(2)},
				2: {simple.Node(3), simple.Node(4), simple.Node(5), simple.Node(6)},
			},
		},
	},
	{ // WP example with node IDs decremented by 1.
		n: simple.Node(0),
		edges: []simple.Edge{
			{F: simple.Node(0), T: simple.Node(1)},
			{F: simple.Node(1), T: simple.Node(2)},
			{F: simple.Node(1), T: simple.Node(3)},
			{F: simple.Node(1), T: simple.Node(5)},
			{F: simple.Node(2), T: simple.Node(4)},
			{F: simple.Node(3), T: simple.Node(4)},
			{F: simple.Node(4), T: simple.Node(1)},
		},

		want: DominatorTree{
			root: simple.Node(0),
			dominatorOf: map[int64]graph.Node{
				1: simple.Node(0),
				2: simple.Node(1),
				3: simple.Node(1),
				4: simple.Node(1),
				5: simple.Node(1),
			},
			dominatedBy: map[int64][]graph.Node{
				0: {simple.Node(1)},
				1: {simple.Node(2), simple.Node(3), simple.Node(4), simple.Node(5)},
			},
		},
	},
}

type char int64

func (n char) ID() int64      { return int64(n) }
func (n char) String() string { return string(n) }

func TestDominators(t *testing.T) {
	for _, test := range dominatorsTests {
		g := simple.NewDirectedGraph()
		for _, e := range test.edges {
			g.SetEdge(e)
		}

		for _, alg := range []struct {
			name string
			fn   func(graph.Node, graph.Directed) DominatorTree
		}{
			{"Dominators", Dominators},
			{"DominatorsSLT", DominatorsSLT},
		} {
			got := alg.fn(test.n, g)
			if !reflect.DeepEqual(got.root, test.want.root) {
				t.Errorf("unexpected dominator tree root from %s: got:%v want:%v",
					alg.name, got.root, test.want.root)
			}

			if !reflect.DeepEqual(got.dominatorOf, test.want.dominatorOf) {
				t.Errorf("unexpected dominator tree from %s: got:%v want:%v",
					alg.name, got.dominatorOf, test.want.dominatorOf)
			}

			for _, nodes := range got.dominatedBy {
				sort.Sort(ordered.ByID(nodes))
			}
			if !reflect.DeepEqual(got.dominatedBy, test.want.dominatedBy) {
				t.Errorf("unexpected dominator tree from %s: got:%v want:%v",
					alg.name, got.dominatedBy, test.want.dominatedBy)
			}
		}
	}
}
