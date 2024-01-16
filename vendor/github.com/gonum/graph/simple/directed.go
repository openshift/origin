// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"fmt"

	"golang.org/x/tools/container/intsets"

	"github.com/gonum/graph"
)

// DirectedGraph implements a generalized directed graph.
type DirectedGraph struct {
	nodes map[int]graph.Node
	from  map[int]map[int]graph.Edge
	to    map[int]map[int]graph.Edge

	self, absent float64

	freeIDs intsets.Sparse
	usedIDs intsets.Sparse
}

// NewDirectedGraph returns a DirectedGraph with the specified self and absent
// edge weight values.
func NewDirectedGraph(self, absent float64) *DirectedGraph {
	return &DirectedGraph{
		nodes: make(map[int]graph.Node),
		from:  make(map[int]map[int]graph.Edge),
		to:    make(map[int]map[int]graph.Edge),

		self:   self,
		absent: absent,
	}
}

// NewNodeID returns a new unique ID for a node to be added to g. The returned ID does
// not become a valid ID in g until it is added to g.
func (g *DirectedGraph) NewNodeID() int {
	if len(g.nodes) == 0 {
		return 0
	}
	if len(g.nodes) == maxInt {
		panic(fmt.Sprintf("simple: cannot allocate node: no slot"))
	}

	var id int
	if g.freeIDs.Len() != 0 && g.freeIDs.TakeMin(&id) {
		return id
	}
	if id = g.usedIDs.Max(); id < maxInt {
		return id + 1
	}
	for id = 0; id < maxInt; id++ {
		if !g.usedIDs.Has(id) {
			return id
		}
	}
	panic("unreachable")
}

// AddNode adds n to the graph. It panics if the added node ID matches an existing node ID.
func (g *DirectedGraph) AddNode(n graph.Node) {
	if _, exists := g.nodes[n.ID()]; exists {
		panic(fmt.Sprintf("simple: node ID collision: %d", n.ID()))
	}
	g.nodes[n.ID()] = n
	g.from[n.ID()] = make(map[int]graph.Edge)
	g.to[n.ID()] = make(map[int]graph.Edge)

	g.freeIDs.Remove(n.ID())
	g.usedIDs.Insert(n.ID())
}

// RemoveNode removes n from the graph, as well as any edges attached to it. If the node
// is not in the graph it is a no-op.
func (g *DirectedGraph) RemoveNode(n graph.Node) {
	if _, ok := g.nodes[n.ID()]; !ok {
		return
	}
	delete(g.nodes, n.ID())

	for from := range g.from[n.ID()] {
		delete(g.to[from], n.ID())
	}
	delete(g.from, n.ID())

	for to := range g.to[n.ID()] {
		delete(g.from[to], n.ID())
	}
	delete(g.to, n.ID())

	g.freeIDs.Insert(n.ID())
	g.usedIDs.Remove(n.ID())
}

// SetEdge adds e, an edge from one node to another. If the nodes do not exist, they are added.
// It will panic if the IDs of the e.From and e.To are equal.
func (g *DirectedGraph) SetEdge(e graph.Edge) {
	var (
		from = e.From()
		fid  = from.ID()
		to   = e.To()
		tid  = to.ID()
	)

	if fid == tid {
		panic("simple: adding self edge")
	}

	if !g.Has(from) {
		g.AddNode(from)
	}
	if !g.Has(to) {
		g.AddNode(to)
	}

	g.from[fid][tid] = e
	g.to[tid][fid] = e
}

// RemoveEdge removes e from the graph, leaving the terminal nodes. If the edge does not exist
// it is a no-op.
func (g *DirectedGraph) RemoveEdge(e graph.Edge) {
	from, to := e.From(), e.To()
	if _, ok := g.nodes[from.ID()]; !ok {
		return
	}
	if _, ok := g.nodes[to.ID()]; !ok {
		return
	}

	delete(g.from[from.ID()], to.ID())
	delete(g.to[to.ID()], from.ID())
}

// Node returns the node in the graph with the given ID.
func (g *DirectedGraph) Node(id int) graph.Node {
	return g.nodes[id]
}

// Has returns whether the node exists within the graph.
func (g *DirectedGraph) Has(n graph.Node) bool {
	_, ok := g.nodes[n.ID()]

	return ok
}

// Nodes returns all the nodes in the graph.
func (g *DirectedGraph) Nodes() []graph.Node {
	nodes := make([]graph.Node, len(g.from))
	i := 0
	for _, n := range g.nodes {
		nodes[i] = n
		i++
	}

	return nodes
}

// Edges returns all the edges in the graph.
func (g *DirectedGraph) Edges() []graph.Edge {
	var edges []graph.Edge
	for _, u := range g.nodes {
		for _, e := range g.from[u.ID()] {
			edges = append(edges, e)
		}
	}
	return edges
}

// From returns all nodes in g that can be reached directly from n.
func (g *DirectedGraph) From(n graph.Node) []graph.Node {
	if _, ok := g.from[n.ID()]; !ok {
		return nil
	}

	from := make([]graph.Node, len(g.from[n.ID()]))
	i := 0
	for id := range g.from[n.ID()] {
		from[i] = g.nodes[id]
		i++
	}

	return from
}

// To returns all nodes in g that can reach directly to n.
func (g *DirectedGraph) To(n graph.Node) []graph.Node {
	if _, ok := g.from[n.ID()]; !ok {
		return nil
	}

	to := make([]graph.Node, len(g.to[n.ID()]))
	i := 0
	for id := range g.to[n.ID()] {
		to[i] = g.nodes[id]
		i++
	}

	return to
}

// HasEdgeBetween returns whether an edge exists between nodes x and y without
// considering direction.
func (g *DirectedGraph) HasEdgeBetween(x, y graph.Node) bool {
	xid := x.ID()
	yid := y.ID()
	if _, ok := g.nodes[xid]; !ok {
		return false
	}
	if _, ok := g.nodes[yid]; !ok {
		return false
	}
	if _, ok := g.from[xid][yid]; ok {
		return true
	}
	_, ok := g.from[yid][xid]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *DirectedGraph) Edge(u, v graph.Node) graph.Edge {
	if _, ok := g.nodes[u.ID()]; !ok {
		return nil
	}
	if _, ok := g.nodes[v.ID()]; !ok {
		return nil
	}
	edge, ok := g.from[u.ID()][v.ID()]
	if !ok {
		return nil
	}
	return edge
}

// HasEdgeFromTo returns whether an edge exists in the graph from u to v.
func (g *DirectedGraph) HasEdgeFromTo(u, v graph.Node) bool {
	if _, ok := g.nodes[u.ID()]; !ok {
		return false
	}
	if _, ok := g.nodes[v.ID()]; !ok {
		return false
	}
	if _, ok := g.from[u.ID()][v.ID()]; !ok {
		return false
	}
	return true
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node or there is no joining edge between the two nodes the weight
// value returned is either the graph's absent or self value. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g *DirectedGraph) Weight(x, y graph.Node) (w float64, ok bool) {
	xid := x.ID()
	yid := y.ID()
	if xid == yid {
		return g.self, true
	}
	if to, ok := g.from[xid]; ok {
		if e, ok := to[yid]; ok {
			return e.Weight(), true
		}
	}
	return g.absent, false
}

// Degree returns the in+out degree of n in g.
func (g *DirectedGraph) Degree(n graph.Node) int {
	if _, ok := g.nodes[n.ID()]; !ok {
		return 0
	}

	return len(g.from[n.ID()]) + len(g.to[n.ID()])
}
