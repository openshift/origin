// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package iterator_test

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

var orderedNodesTests = []struct {
	nodes []graph.Node
}{
	{nodes: nil},
	{nodes: []graph.Node{simple.Node(1)}},
	{nodes: []graph.Node{simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(5)}},
	{nodes: []graph.Node{simple.Node(5), simple.Node(3), simple.Node(2), simple.Node(1)}},
}

func TestOrderedNodesIterate(t *testing.T) {
	for _, test := range orderedNodesTests {
		for i := 0; i < 2; i++ {
			it := iterator.NewOrderedNodes(test.nodes)
			if it.Len() != len(test.nodes) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.nodes))
			}
			var got []graph.Node
			for it.Next() {
				got = append(got, it.Node())
			}
			want := test.nodes
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

func TestOrderedNodesSlice(t *testing.T) {
	for _, test := range orderedNodesTests {
		for i := 0; i < 2; i++ {
			it := iterator.NewOrderedNodes(test.nodes)
			got := it.NodeSlice()
			want := test.nodes
			if !reflect.DeepEqual(got, want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, want)
			}
			it.Reset()
		}
	}
}

var implicitNodesTests = []struct {
	beg, end int
	new      func(int) graph.Node
	want     []graph.Node
}{
	{
		beg: 1, end: 1,
		want: nil,
	},
	{
		beg: 1, end: 2,
		new:  newSimpleNode,
		want: []graph.Node{simple.Node(1)},
	},
	{
		beg: 1, end: 5,
		new:  newSimpleNode,
		want: []graph.Node{simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(4)},
	},
}

func newSimpleNode(id int) graph.Node { return simple.Node(id) }

func TestImplicitNodesIterate(t *testing.T) {
	for _, test := range implicitNodesTests {
		for i := 0; i < 2; i++ {
			it := iterator.NewImplicitNodes(test.beg, test.end, test.new)
			if it.Len() != len(test.want) {
				t.Errorf("unexpected iterator length for round %d: got:%d want:%d", i, it.Len(), len(test.want))
			}
			var got []graph.Node
			for it.Next() {
				got = append(got, it.Node())
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("unexpected iterator output for round %d: got:%#v want:%#v", i, got, test.want)
			}
			it.Reset()
		}
	}
}
