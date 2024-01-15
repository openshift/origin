// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package simple

import (
	"sort"

	"github.com/gonum/graph"
	"github.com/gonum/graph/internal/ordered"
	"github.com/gonum/matrix/mat64"
)

// DirectedMatrix represents a directed graph using an adjacency
// matrix such that all IDs are in a contiguous block from 0 to n-1.
// Edges are stored implicitly as an edge weight, so edges stored in
// the graph are not recoverable.
type DirectedMatrix struct {
	mat   *mat64.Dense
	nodes []graph.Node

	self   float64
	absent float64
}

// NewDirectedMatrix creates a directed dense graph with n nodes.
// All edges are initialized with the weight given by init. The self parameter
// specifies the cost of self connection, and absent specifies the weight
// returned for absent edges.
func NewDirectedMatrix(n int, init, self, absent float64) *DirectedMatrix {
	mat := make([]float64, n*n)
	if init != 0 {
		for i := range mat {
			mat[i] = init
		}
	}
	for i := 0; i < len(mat); i += n + 1 {
		mat[i] = self
	}
	return &DirectedMatrix{
		mat:    mat64.NewDense(n, n, mat),
		self:   self,
		absent: absent,
	}
}

// NewDirectedMatrixFrom creates a directed dense graph with the given nodes.
// The IDs of the nodes must be contiguous from 0 to len(nodes)-1, but may
// be in any order. If IDs are not contiguous NewDirectedMatrixFrom will panic.
// All edges are initialized with the weight given by init. The self parameter
// specifies the cost of self connection, and absent specifies the weight
// returned for absent edges.
func NewDirectedMatrixFrom(nodes []graph.Node, init, self, absent float64) *DirectedMatrix {
	sort.Sort(ordered.ByID(nodes))
	for i, n := range nodes {
		if i != n.ID() {
			panic("simple: non-contiguous node IDs")
		}
	}
	g := NewDirectedMatrix(len(nodes), init, self, absent)
	g.nodes = nodes
	return g
}

// Node returns the node in the graph with the given ID.
func (g *DirectedMatrix) Node(id int) graph.Node {
	if !g.has(id) {
		return nil
	}
	if g.nodes == nil {
		return Node(id)
	}
	return g.nodes[id]
}

// Has returns whether the node exists within the graph.
func (g *DirectedMatrix) Has(n graph.Node) bool {
	return g.has(n.ID())
}

func (g *DirectedMatrix) has(id int) bool {
	r, _ := g.mat.Dims()
	return 0 <= id && id < r
}

// Nodes returns all the nodes in the graph.
func (g *DirectedMatrix) Nodes() []graph.Node {
	if g.nodes != nil {
		nodes := make([]graph.Node, len(g.nodes))
		copy(nodes, g.nodes)
		return nodes
	}
	r, _ := g.mat.Dims()
	nodes := make([]graph.Node, r)
	for i := 0; i < r; i++ {
		nodes[i] = Node(i)
	}
	return nodes
}

// Edges returns all the edges in the graph.
func (g *DirectedMatrix) Edges() []graph.Edge {
	var edges []graph.Edge
	r, _ := g.mat.Dims()
	for i := 0; i < r; i++ {
		for j := 0; j < r; j++ {
			if i == j {
				continue
			}
			if w := g.mat.At(i, j); !isSame(w, g.absent) {
				edges = append(edges, Edge{F: g.Node(i), T: g.Node(j), W: w})
			}
		}
	}
	return edges
}

// From returns all nodes in g that can be reached directly from n.
func (g *DirectedMatrix) From(n graph.Node) []graph.Node {
	id := n.ID()
	if !g.has(id) {
		return nil
	}
	var neighbors []graph.Node
	_, c := g.mat.Dims()
	for j := 0; j < c; j++ {
		if j == id {
			continue
		}
		if !isSame(g.mat.At(id, j), g.absent) {
			neighbors = append(neighbors, g.Node(j))
		}
	}
	return neighbors
}

// To returns all nodes in g that can reach directly to n.
func (g *DirectedMatrix) To(n graph.Node) []graph.Node {
	id := n.ID()
	if !g.has(id) {
		return nil
	}
	var neighbors []graph.Node
	r, _ := g.mat.Dims()
	for i := 0; i < r; i++ {
		if i == id {
			continue
		}
		if !isSame(g.mat.At(i, id), g.absent) {
			neighbors = append(neighbors, g.Node(i))
		}
	}
	return neighbors
}

// HasEdgeBetween returns whether an edge exists between nodes x and y without
// considering direction.
func (g *DirectedMatrix) HasEdgeBetween(x, y graph.Node) bool {
	xid := x.ID()
	if !g.has(xid) {
		return false
	}
	yid := y.ID()
	if !g.has(yid) {
		return false
	}
	return xid != yid && (!isSame(g.mat.At(xid, yid), g.absent) || !isSame(g.mat.At(yid, xid), g.absent))
}

// Edge returns the edge from u to v if such an edge exists and nil otherwise.
// The node v must be directly reachable from u as defined by the From method.
func (g *DirectedMatrix) Edge(u, v graph.Node) graph.Edge {
	if g.HasEdgeFromTo(u, v) {
		return Edge{F: g.Node(u.ID()), T: g.Node(v.ID()), W: g.mat.At(u.ID(), v.ID())}
	}
	return nil
}

// HasEdgeFromTo returns whether an edge exists in the graph from u to v.
func (g *DirectedMatrix) HasEdgeFromTo(u, v graph.Node) bool {
	uid := u.ID()
	if !g.has(uid) {
		return false
	}
	vid := v.ID()
	if !g.has(vid) {
		return false
	}
	return uid != vid && !isSame(g.mat.At(uid, vid), g.absent)
}

// Weight returns the weight for the edge between x and y if Edge(x, y) returns a non-nil Edge.
// If x and y are the same node or there is no joining edge between the two nodes the weight
// value returned is either the graph's absent or self value. Weight returns true if an edge
// exists between x and y or if x and y have the same ID, false otherwise.
func (g *DirectedMatrix) Weight(x, y graph.Node) (w float64, ok bool) {
	xid := x.ID()
	yid := y.ID()
	if xid == yid {
		return g.self, true
	}
	if g.has(xid) && g.has(yid) {
		return g.mat.At(xid, yid), true
	}
	return g.absent, false
}

// SetEdge sets e, an edge from one node to another. If the ends of the edge are not in g
// or the edge is a self loop, SetEdge panics.
func (g *DirectedMatrix) SetEdge(e graph.Edge) {
	fid := e.From().ID()
	tid := e.To().ID()
	if fid == tid {
		panic("simple: set illegal edge")
	}
	g.mat.Set(fid, tid, e.Weight())
}

// RemoveEdge removes e from the graph, leaving the terminal nodes. If the edge does not exist
// it is a no-op.
func (g *DirectedMatrix) RemoveEdge(e graph.Edge) {
	fid := e.From().ID()
	if !g.has(fid) {
		return
	}
	tid := e.To().ID()
	if !g.has(tid) {
		return
	}
	g.mat.Set(fid, tid, g.absent)
}

// Degree returns the in+out degree of n in g.
func (g *DirectedMatrix) Degree(n graph.Node) int {
	id := n.ID()
	var deg int
	r, c := g.mat.Dims()
	for i := 0; i < r; i++ {
		if i == id {
			continue
		}
		if !isSame(g.mat.At(id, i), g.absent) {
			deg++
		}
	}
	for i := 0; i < c; i++ {
		if i == id {
			continue
		}
		if !isSame(g.mat.At(i, id), g.absent) {
			deg++
		}
	}
	return deg
}

// Matrix returns the mat64.Matrix representation of the graph. The orientation
// of the matrix is such that the matrix entry at G_{ij} is the weight of the edge
// from node i to node j.
func (g *DirectedMatrix) Matrix() mat64.Matrix {
	// Prevent alteration of dimensions of the returned matrix.
	m := *g.mat
	return &m
}
