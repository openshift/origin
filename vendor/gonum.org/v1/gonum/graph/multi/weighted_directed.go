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
	wdg *WeightedDirectedGraph

	_ graph.Graph                      = wdg
	_ graph.Directed                   = wdg
	_ graph.WeightedDirected           = wdg
	_ graph.Multigraph                 = wdg
	_ graph.DirectedMultigraph         = wdg
	_ graph.WeightedDirectedMultigraph = wdg
)

// WeightedDirectedGraph implements a generalized directed graph.
type WeightedDirectedGraph struct {
	EdgeWeightFunc func([]graph.WeightedLine) float64

	nodes map[int64]graph.Node
	from  map[int64]map[int64]map[int64]graph.WeightedLine
	to    map[int64]map[int64]map[int64]graph.WeightedLine

	nodeIDs uid.Set
	lineIDs uid.Set
}

// NewWeightedDirectedGraph returns a WeightedDirectedGraph.
func NewWeightedDirectedGraph() *WeightedDirectedGraph {
	return &WeightedDirectedGraph{
		nodes: make(map[int64]graph.Node),
		from:  make(map[int64]map[int64]map[int64]graph.WeightedLine),
		to:    make(map[int64]map[int64]map[int64]graph.WeightedLine),

		nodeIDs: uid.NewSet(),
		lineIDs: uid.NewSet(),
	}
}

// NewNode returns a new unique Node to be added to g. The Node's ID does
// not become valid in g until the Node is added to g.
func (g *WeightedDirectedGraph) NewNode() graph.Node {
	if len(g.nodes) == 0 {
		return Node(0)
	}
	if int64(len(g.nodes)) == uid.Max {
		panic("simple: cannot allocate node: no slot")
	}
	return Node(g.nodeIDs.NewID())
}

// AddNode adds n to the graph. It panics if the added node ID matches an existing node ID.
func (g *WeightedDirectedGraph) AddNode(n graph.Node) {
	if _, exists := g.nodes[n.ID()]; exists {
		panic(fmt.Sprintf("simple: node ID collision: %d", n.ID()))
	}
	g.nodes[n.ID()] = n
	g.from[n.ID()] = make(map[int64]map[int64]graph.WeightedLine)
	g.to[n.ID()] = make(map[int64]map[int64]graph.WeightedLine)
	g.nodeIDs.Use(n.ID())
}

// RemoveNode removes the node with the given ID from the graph, as well as any edges attached
// to it. If the node is not in the graph it is a no-op.
func (g *WeightedDirectedGraph) RemoveNode(id int64) {
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

// NewWeightedLine returns a new WeightedLine from the source to the destination node.
// The returned WeightedLine will have a graph-unique ID.
// The Line's ID does not become valid in g until the Line is added to g.
func (g *WeightedDirectedGraph) NewWeightedLine(from, to graph.Node, weight float64) graph.WeightedLine {
	return &WeightedLine{F: from, T: to, W: weight, UID: g.lineIDs.NewID()}
}

// SetWeightedLine adds l, a line from one node to another. If the nodes do not exist, they are added.
func (g *WeightedDirectedGraph) SetWeightedLine(l graph.WeightedLine) {
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
		g.from[fid][tid] = make(map[int64]graph.WeightedLine)
	}
	if !g.Has(tid) {
		g.AddNode(to)
	}
	if g.to[tid][fid] == nil {
		g.to[tid][fid] = make(map[int64]graph.WeightedLine)
	}

	g.from[fid][tid][lid] = l
	g.to[tid][fid][lid] = l
	g.lineIDs.Use(l.ID())
}

// RemoveWeightedLine removes l from the graph, leaving the terminal nodes. If the line does not exist
// it is a no-op.
func (g *WeightedDirectedGraph) RemoveWeightedLine(l graph.WeightedLine) {
	from, to := l.From(), l.To()
	if _, ok := g.nodes[from.ID()]; !ok {
		return
	}
	if _, ok := g.nodes[to.ID()]; !ok {
		return
	}

	delete(g.from[from.ID()][to.ID()], l.ID())
	if len(g.from[from.ID()][to.ID()]) == 0 {
		delete(g.from[from.ID()], to.ID())
	}
	delete(g.to[to.ID()][from.ID()], l.ID())
	if len(g.to[to.ID()][from.ID()]) == 0 {
		delete(g.to[to.ID()], from.ID())
	}
	g.lineIDs.Release(l.ID())
}

// Node returns the node in the graph with the given ID.
func (g *WeightedDirectedGraph) Node(id int64) graph.Node {
	return g.nodes[id]
}

// Has returns whether the node exists within the graph.
func (g *WeightedDirectedGraph) Has(id int64) bool {
	_, ok := g.nodes[id]

	return ok
}

// Nodes returns all the nodes in the graph.
func (g *WeightedDirectedGraph) Nodes() []graph.Node {
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
// is a multi.WeightedEdge.
func (g *WeightedDirectedGraph) Edges() []graph.Edge {
	var edges []graph.Edge
	for _, u := range g.nodes {
		for _, e := range g.from[u.ID()] {
			var lines WeightedEdge
			for _, l := range e {
				lines.Lines = append(lines.Lines, l)
			}
			if len(lines.Lines) != 0 {
				lines.WeightFunc = g.EdgeWeightFunc
				edges = append(edges, lines)
			}
		}
	}
	return edges
}

// WeightedEdges returns all the edges in the graph. Each edge in the returned slice
// is a multi.WeightedEdge.
func (g *WeightedDirectedGraph) WeightedEdges() []graph.WeightedEdge {
	var edges []graph.WeightedEdge
	for _, u := range g.nodes {
		for _, e := range g.from[u.ID()] {
			var lines WeightedEdge
			for _, l := range e {
				lines.Lines = append(lines.Lines, l)
			}
			if len(lines.Lines) != 0 {
				lines.WeightFunc = g.EdgeWeightFunc
				edges = append(edges, lines)
			}
		}
	}
	return edges
}

// From returns all nodes in g that can be reached directly from n.
func (g *WeightedDirectedGraph) From(id int64) []graph.Node {
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
func (g *WeightedDirectedGraph) To(id int64) []graph.Node {
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
func (g *WeightedDirectedGraph) HasEdgeBetween(xid, yid int64) bool {
	if _, ok := g.from[xid][yid]; ok {
		return true
	}
	_, ok := g.from[yid][xid]
	return ok
}

// HasEdgeFromTo returns whether an edge exists in the graph from u to v.
func (g *WeightedDirectedGraph) HasEdgeFromTo(uid, vid int64) bool {
	if _, ok := g.from[uid][vid]; !ok {
		return false
	}
	return true
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
// The returned graph.Edge is a multi.WeightedEdge if an edge exists.
func (g *WeightedDirectedGraph) Edge(uid, vid int64) graph.Edge {
	return g.WeightedEdge(uid, vid)
}

// WeightedEdge returns the weighted edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
// The returned graph.WeightedEdge is a multi.WeightedEdge if an edge exists.
func (g *WeightedDirectedGraph) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	lines := g.WeightedLines(uid, vid)
	if len(lines) == 0 {
		return nil
	}
	return WeightedEdge{Lines: lines, WeightFunc: g.EdgeWeightFunc}
}

// Lines returns the lines from u to v if such any such lines exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *WeightedDirectedGraph) Lines(uid, vid int64) []graph.Line {
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

// WeightedLines returns the weighted lines from u to v if such any such lines exists
// and nil otherwise. The node v must be directly reachable from u as defined by the From method.
func (g *WeightedDirectedGraph) WeightedLines(uid, vid int64) []graph.WeightedLine {
	edge := g.from[uid][vid]
	if len(edge) == 0 {
		return nil
	}
	var lines []graph.WeightedLine
	for _, l := range edge {
		lines = append(lines, l)
	}
	return lines
}

// Weight returns the weight for the lines between x and y summarised by the receiver's
// EdgeWeightFunc. Weight returns true if an edge exists between x and y, false otherwise.
func (g *WeightedDirectedGraph) Weight(uid, vid int64) (w float64, ok bool) {
	lines := g.WeightedLines(uid, vid)
	return WeightedEdge{Lines: lines, WeightFunc: g.EdgeWeightFunc}.Weight(), len(lines) != 0
}

// Degree returns the in+out degree of n in g.
func (g *WeightedDirectedGraph) Degree(id int64) int {
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
