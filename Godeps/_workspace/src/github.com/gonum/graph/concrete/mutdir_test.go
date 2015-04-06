// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concrete_test

import (
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

var _ graph.Graph = &concrete.DirectedGraph{}
var _ graph.DirectedGraph = &concrete.DirectedGraph{}
var _ graph.DirectedGraph = &concrete.DirectedGraph{}

// Tests Issue #27
func TestEdgeOvercounting(t *testing.T) {
	g := generateDummyGraph()

	if neigh := g.Neighbors(concrete.Node(concrete.Node(2))); len(neigh) != 3 {
		t.Errorf("Node 2 has incorrect number of neighbors got neighbors %v (count %d), expected 3 neighbors {0,1,2}", neigh, len(neigh))
	}
}

func generateDummyGraph() *concrete.DirectedGraph {
	nodes := [5]struct{ srcId, targetId int }{
		{2, 1},
		{2, 2},
		{1, 0},
		{2, 0},
		{0, 2},
	}

	g := concrete.NewDirectedGraph()

	for _, n := range nodes {
		g.AddDirectedEdge(concrete.Edge{concrete.Node(n.srcId), concrete.Node(n.targetId)}, 1)
	}

	return g
}
