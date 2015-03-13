// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package concrete

import (
	"github.com/gonum/graph"
)

// A dense graph is a graph such that all IDs are in a contiguous block from 0 to
// TheNumberOfNodes-1. It uses an adjacency matrix and should be relatively fast for both access
// and writing.
//
// This graph implements the CrunchGraph, but since it's naturally dense this is superfluous.
type DenseGraph struct {
	adjacencyMatrix []float64
	numNodes        int
}

// Creates a dense graph with the proper number of nodes. If passable is true all nodes will have
// an edge with unit cost, otherwise every node will start unconnected (cost of +Inf).
func NewDenseGraph(numNodes int, passable bool) *DenseGraph {
	g := &DenseGraph{adjacencyMatrix: make([]float64, numNodes*numNodes), numNodes: numNodes}
	if passable {
		for i := range g.adjacencyMatrix {
			g.adjacencyMatrix[i] = 1
		}
	} else {
		for i := range g.adjacencyMatrix {
			g.adjacencyMatrix[i] = inf
		}
	}

	return g
}

func (g *DenseGraph) NodeExists(n graph.Node) bool {
	return n.ID() < g.numNodes
}

func (g *DenseGraph) Degree(n graph.Node) int {
	deg := 0
	for i := 0; i < g.numNodes; i++ {
		if g.adjacencyMatrix[i*g.numNodes+n.ID()] != inf {
			deg++
		}

		if g.adjacencyMatrix[n.ID()*g.numNodes+i] != inf {
			deg++
		}
	}

	return deg
}

func (g *DenseGraph) NodeList() []graph.Node {
	nodes := make([]graph.Node, g.numNodes)
	for i := 0; i < g.numNodes; i++ {
		nodes[i] = Node(i)
	}

	return nodes
}

func (g *DenseGraph) DirectedEdgeList() []graph.Edge {
	edges := make([]graph.Edge, 0, len(g.adjacencyMatrix))
	for i := 0; i < g.numNodes; i++ {
		for j := 0; j < g.numNodes; j++ {
			if g.adjacencyMatrix[i*g.numNodes+j] != inf {
				edges = append(edges, Edge{Node(i), Node(j)})
			}
		}
	}

	return edges
}

func (g *DenseGraph) Neighbors(n graph.Node) []graph.Node {
	neighbors := make([]graph.Node, 0)
	for i := 0; i < g.numNodes; i++ {
		if g.adjacencyMatrix[i*g.numNodes+n.ID()] != inf ||
			g.adjacencyMatrix[n.ID()*g.numNodes+i] != inf {
			neighbors = append(neighbors, Node(i))
		}
	}

	return neighbors
}

func (g *DenseGraph) EdgeBetween(n, neighbor graph.Node) graph.Edge {
	if g.adjacencyMatrix[neighbor.ID()*g.numNodes+n.ID()] != inf ||
		g.adjacencyMatrix[n.ID()*g.numNodes+neighbor.ID()] != inf {
		return Edge{n, neighbor}
	}

	return nil
}

func (g *DenseGraph) Successors(n graph.Node) []graph.Node {
	neighbors := make([]graph.Node, 0)
	for i := 0; i < g.numNodes; i++ {
		if g.adjacencyMatrix[n.ID()*g.numNodes+i] != inf {
			neighbors = append(neighbors, Node(i))
		}
	}

	return neighbors
}

func (g *DenseGraph) EdgeTo(n, succ graph.Node) graph.Edge {
	if g.adjacencyMatrix[n.ID()*g.numNodes+succ.ID()] != inf {
		return Edge{n, succ}
	}

	return nil
}

func (g *DenseGraph) Predecessors(n graph.Node) []graph.Node {
	neighbors := make([]graph.Node, 0)
	for i := 0; i < g.numNodes; i++ {
		if g.adjacencyMatrix[i*g.numNodes+n.ID()] != inf {
			neighbors = append(neighbors, Node(i))
		}
	}

	return neighbors
}

// DenseGraph is naturally dense, we don't need to do anything
func (g *DenseGraph) Crunch() {
}

func (g *DenseGraph) Cost(e graph.Edge) float64 {
	return g.adjacencyMatrix[e.Head().ID()*g.numNodes+e.Tail().ID()]
}

// Sets the cost of an edge. If the cost is +Inf, it will remove the edge,
// if directed is true, it will only remove the edge one way. If it's false it will change the cost
// of the edge from succ to node as well.
func (g *DenseGraph) SetEdgeCost(e graph.Edge, cost float64, directed bool) {
	g.adjacencyMatrix[e.Head().ID()*g.numNodes+e.Tail().ID()] = cost
	if !directed {
		g.adjacencyMatrix[e.Tail().ID()*g.numNodes+e.Head().ID()] = cost
	}
}

// Equivalent to SetEdgeCost(edge, math.Inf(1), directed)
func (g *DenseGraph) RemoveEdge(e graph.Edge, directed bool) {
	g.SetEdgeCost(e, inf, directed)
}
