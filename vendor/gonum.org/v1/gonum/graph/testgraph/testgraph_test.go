// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testgraph

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

var randomNodesTests = []struct {
	n    int
	seed uint64
	new  func(int64) graph.Node
	want []graph.Node
}{
	{
		n:    0,
		want: nil,
	},
	{
		n:    1,
		seed: 1,
		new:  newSimpleNode,
		want: []graph.Node{simple.Node(-106976941678315313)},
	},
	{
		n:    1,
		seed: 2,
		new:  newSimpleNode,
		want: []graph.Node{simple.Node(6816453162648937526)},
	},
	{
		n:    4,
		seed: 1,
		new:  newSimpleNode,
		want: []graph.Node{
			simple.Node(-106976941678315313),
			simple.Node(867649948573917593),
			simple.Node(-4246677790793934368),
			simple.Node(406519965772129914),
		},
	},
	{
		n:    4,
		seed: 2,
		new:  newSimpleNode,
		want: []graph.Node{
			simple.Node(6816453162648937526),
			simple.Node(-4921844272880608907),
			simple.Node(159088832891557680),
			simple.Node(-2611333848016927708),
		},
	},
}

func newSimpleNode(id int64) graph.Node { return simple.Node(id) }

func TestRandomNodesIterate(t *testing.T) {
	for _, test := range randomNodesTests {
		for i := 0; i < 2; i++ {
			it := NewRandomNodes(test.n, test.seed, test.new)
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
