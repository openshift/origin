// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multi

import (
	"fmt"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/uid"
)

var (
	dg *DirectedGraph

	_ graph.Graph              = dg
	_ graph.Directed           = dg
	_ graph.Multigraph         = dg
	_ graph.DirectedMultigraph = dg
)

// DirectedGraph implements a generalized directed graph.
type DirectedGraph struct {
	nodes map[int64]graph.Node
	from  map[int64]map[int64]map[int64]graph.Line
	to    map[int64]map[int64]map[int64]graph.Line

	nodeIDs uid.Set
	lineIDs uid.Set
}

// NewDirectedGraph returns a DirectedGraph.
func NewDirectedGraph() *DirectedGraph {
	return &DirectedGraph{
		nodes: make(map[int64]graph.Node),
		from:  make(map[int64]map[int64]map[int64]graph.Line),
		to:    make(map[int64]map[int64]map[int64]graph.Line),

		nodeIDs: uid.NewSet(),
		lineIDs: uid.NewSet(),
	}
}

// NewNode returns a new unique Node to be added to g. The Node's ID does
// not become valid in g until the Node is added to g.
func (g *DirectedGraph) NewNode() graph.Node {
	if len(g.nodes) == 0 {
		return Node(0)
	}
	if int64(len(g.nodes)) == uid.Max {
		panic("simple: cannot allocate node: no slot")
	}
	return Node(g.nodeIDs.NewID())
}

// AddNode adds n to the graph. It panics if the added node ID matches an existing node ID.
func (g *DirectedGraph) AddNode(n graph.Node) {
	if _, exists := g.nodes[n.ID()]; exists {
		panic(fmt.Sprintf("simple: node ID collision: %d", n.ID()))
	}
	g.nodes[n.ID()] = n
	g.from[n.ID()] = make(map[int64]map[int64]graph.Line)
	g.to[n.ID()] = make(map[int64]map[int64]graph.Line)
	g.nodeIDs.Use(n.ID())
}

// RemoveNode removes the node with the given ID from the graph, as well as any edges attached
// to it. If the node is not in the graph it is a no-op.
func (g *DirectedGraph) RemoveNode(id int64) {
	if _, ok := g.nodes[id]; !ok {
		return
	}
	delete(g.nodes, id)

	for from := range g.from[id] {
		delete(g.to[from], id)
	}
	delete(g.from, id)

	for to := range g.to[id] {
		delete(g.from[to], id)
	}
	delete(g.to, id)

	g.nodeIDs.Release(id)
}

// NewLine returns a new Line from the source to the destination node.
// The returned Line will have a graph-unique ID.
// The Line's ID does not become valid in g until the Line is added to g.
func (g *DirectedGraph) NewLine(from, to graph.Node) graph.Line {
	return &Line{F: from, T: to, UID: g.lineIDs.NewID()}
}

// SetLine adds l, a line from one node to another. If the nodes do not exist, they are added.
func (g *DirectedGraph) SetLine(l graph.Line) {
	var (
		from = l.From()
		fid  = from.ID()
		to   = l.To()
		tid  = to.ID()
		lid  = l.ID()
	)

	if !g.Has(fid) {
		g.AddNode(from)
	}
	if g.from[fid][tid] == nil {
		g.from[fid][tid] = make(map[int64]graph.Line)
	}
	if !g.Has(tid) {
		g.AddNode(to)
	}
	if g.to[tid][fid] == nil {
		g.to[tid][fid] = make(map[int64]graph.Line)
	}

	g.from[fid][tid][lid] = l
	g.to[tid][fid][lid] = l
	g.lineIDs.Use(lid)
}

// RemoveLine removes the line with the given end point and line IDs from the graph, leaving
// the terminal nodes. If the line does not exist it is a no-op.
func (g *DirectedGraph) RemoveLine(fid, tid, id int64) {
	if _, ok := g.nodes[fid]; !ok {
		return
	}
	if _, ok := g.nodes[tid]; !ok {
		return
	}

	delete(g.from[fid][tid], id)
	if len(g.from[fid][tid]) == 0 {
		delete(g.from[fid], tid)
	}
	delete(g.to[tid][fid], id)
	if len(g.to[tid][fid]) == 0 {
		delete(g.to[tid], fid)
	}
	g.lineIDs.Release(id)
}

// Node returns the node in the graph with the given ID.
func (g *DirectedGraph) Node(id int64) graph.Node {
	return g.nodes[id]
}

// Has returns whether the node exists within the graph.
func (g *DirectedGraph) Has(id int64) bool {
	_, ok := g.nodes[id]
	return ok
}

// Nodes returns all the nodes in the graph.
func (g *DirectedGraph) Nodes() []graph.Node {
	if len(g.nodes) == 0 {
		return nil
	}
	nodes := make([]graph.Node, len(g.nodes))
	i := 0
	for _, n := range g.nodes {
		nodes[i] = n
		i++
	}
	return nodes
}

// Edges returns all the edges in the graph. Each edge in the returned slice
// is a multi.Edge.
func (g *DirectedGraph) Edges() []graph.Edge {
	var edges []graph.Edge
	for _, u := range g.nodes {
		for _, e := range g.from[u.ID()] {
			var lines Edge
			for _, l := range e {
				lines = append(lines, l)
			}
			if len(lines) != 0 {
				edges = append(edges, lines)
			}
		}
	}
	return edges
}

// From returns all nodes in g that can be reached directly from n.
func (g *DirectedGraph) From(id int64) []graph.Node {
	if _, ok := g.from[id]; !ok {
		return nil
	}

	from := make([]graph.Node, len(g.from[id]))
	i := 0
	for vid := range g.from[id] {
		from[i] = g.nodes[vid]
		i++
	}
	return from
}

// To returns all nodes in g that can reach directly to n.
func (g *DirectedGraph) To(id int64) []graph.Node {
	if _, ok := g.from[id]; !ok {
		return nil
	}

	to := make([]graph.Node, len(g.to[id]))
	i := 0
	for uid := range g.to[id] {
		to[i] = g.nodes[uid]
		i++
	}
	return to
}

// HasEdgeBetween returns whether an edge exists between nodes x and y without
// considering direction.
func (g *DirectedGraph) HasEdgeBetween(xid, yid int64) bool {
	if _, ok := g.from[xid][yid]; ok {
		return true
	}
	_, ok := g.from[yid][xid]
	return ok
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
// The returned graph.Edge is a multi.Edge if an edge exists.
func (g *DirectedGraph) Edge(uid, vid int64) graph.Edge {
	lines := g.Lines(uid, vid)
	if len(lines) == 0 {
		return nil
	}
	return Edge(lines)
}

// Lines returns the lines from u to v if such any such lines exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *DirectedGraph) Lines(uid, vid int64) []graph.Line {
	edge := g.from[uid][vid]
	if len(edge) == 0 {
		return nil
	}
	var lines []graph.Line
	for _, l := range edge {
		lines = append(lines, l)
	}
	return lines
}

// HasEdgeFromTo returns whether an edge exists in the graph from u to v.
func (g *DirectedGraph) HasEdgeFromTo(uid, vid int64) bool {
	_, ok := g.from[uid][vid]
	return ok
}

// Degree returns the in+out degree of n in g.
func (g *DirectedGraph) Degree(id int64) int {
	if _, ok := g.nodes[id]; !ok {
		return 0
	}
	var deg int
	for _, e := range g.from[id] {
		deg += len(e)
	}
	for _, e := range g.to[id] {
		deg += len(e)
	}
	return deg
}
