// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"testing"

	"gonum.org/v1/gonum/graph"
)

var (
	undirectedGraph = (*UndirectedGraph)(nil)

	_ graph.Graph      = undirectedGraph
	_ graph.Undirected = undirectedGraph
)

func TestAssertMutableNotDirected(t *testing.T) {
	var g graph.UndirectedBuilder = NewUndirectedGraph()
	if _, ok := g.(graph.Directed); ok {
		t.Fatal("Graph is directed, but a MutableGraph cannot safely be directed!")
	}
}

func TestMaxID(t *testing.T) {
	g := NewUndirectedGraph()
	nodes := make(map[graph.Node]struct{})
	for i := Node(0); i < 3; i++ {
		g.AddNode(i)
		nodes[i] = struct{}{}
	}
	g.RemoveNode(int64(0))
	delete(nodes, Node(0))
	g.RemoveNode(int64(2))
	delete(nodes, Node(2))
	n := g.NewNode()
	g.AddNode(n)
	if !g.Has(n.ID()) {
		t.Error("added node does not exist in graph")
	}
	if _, exists := nodes[n]; exists {
		t.Errorf("Created already existing node id: %v", n.ID())
	}
}

// Test for issue #123 https://github.com/gonum/graph/issues/123
func TestIssue123UndirectedGraph(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	g := NewUndirectedGraph()

	n0 := g.NewNode()
	g.AddNode(n0)

	n1 := g.NewNode()
	g.AddNode(n1)

	g.RemoveNode(n0.ID())

	n2 := g.NewNode()
	g.AddNode(n2)
}
