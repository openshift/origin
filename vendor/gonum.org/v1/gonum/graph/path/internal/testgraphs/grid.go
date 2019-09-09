// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testgraphs

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
)

const (
	Closed  = '*' // Closed is the closed grid node representation.
	Open    = '.' // Open is the open grid node repesentation.
	Unknown = '?' // Unknown is the unknown grid node repesentation.
)

// Grid is a 2D grid planar undirected graph.
type Grid struct {
	// AllowDiagonal specifies whether
	// diagonally adjacent nodes can
	// be connected by an edge.
	AllowDiagonal bool
	// UnitEdgeWeight specifies whether
	// finite edge weights are returned as
	// the unit length. Otherwise edge
	// weights are the Euclidean distance
	// between connected nodes.
	UnitEdgeWeight bool

	// AllVisible specifies whether
	// non-open nodes are visible
	// in calls to Nodes and HasNode.
	AllVisible bool

	open []bool
	r, c int
}

// NewGrid returns an r by c grid with all positions
// set to the specified open state.
func NewGrid(r, c int, open bool) *Grid {
	states := make([]bool, r*c)
	if open {
		for i := range states {
			states[i] = true
		}
	}
	return &Grid{
		open: states,
		r:    r,
		c:    c,
	}
}

// NewGridFrom returns a grid specified by the rows strings. All rows must
// be the same length and must only contain the Open or Closed characters,
// NewGridFrom will panic otherwise.
func NewGridFrom(rows ...string) *Grid {
	if len(rows) == 0 {
		return nil
	}
	for i, r := range rows[:len(rows)-1] {
		if len(r) != len(rows[i+1]) {
			panic("grid: unequal row lengths")
		}
	}
	states := make([]bool, 0, len(rows)*len(rows[0]))
	for _, r := range rows {
		for _, b := range r {
			switch b {
			case Closed:
				states = append(states, false)
			case Open:
				states = append(states, true)
			default:
				panic(fmt.Sprintf("grid: invalid state: %q", r))
			}
		}
	}
	return &Grid{
		open: states,
		r:    len(rows),
		c:    len(rows[0]),
	}
}

// Nodes returns all the open nodes in the grid if AllVisible is
// false, otherwise all nodes are returned.
func (g *Grid) Nodes() graph.Nodes {
	var nodes []graph.Node
	for id, ok := range g.open {
		if ok || g.AllVisible {
			nodes = append(nodes, simple.Node(id))
		}
	}
	return iterator.NewOrderedNodes(nodes)
}

// Node returns the node with the given ID if it exists in the graph,
// and nil otherwise.
func (g *Grid) Node(id int64) graph.Node {
	if g.has(id) {
		return simple.Node(id)
	}
	return nil
}

// has returns whether id represents a node in the grid. The state of
// the AllVisible field determines whether a non-open node is present.
func (g *Grid) has(id int64) bool {
	return 0 <= id && id < int64(len(g.open)) && (g.AllVisible || g.open[id])
}

// HasOpen returns whether n is an open node in the grid.
func (g *Grid) HasOpen(id int64) bool {
	return 0 <= id && id < int64(len(g.open)) && g.open[id]
}

// Set sets the node at position (r, c) to the specified open state.
func (g *Grid) Set(r, c int, open bool) {
	if r < 0 || r >= g.r {
		panic("grid: illegal row index")
	}
	if c < 0 || c >= g.c {
		panic("grid: illegal column index")
	}
	g.open[r*g.c+c] = open
}

// Dims returns the dimensions of the grid.
func (g *Grid) Dims() (r, c int) {
	return g.r, g.c
}

// RowCol returns the row and column of the id. RowCol will panic if the
// node id is outside the range of the grid.
func (g *Grid) RowCol(id int64) (r, c int) {
	if id < 0 || int64(len(g.open)) <= id {
		panic("grid: illegal node id")
	}
	return int(id) / g.c, int(id) % g.c
}

// XY returns the cartesian coordinates of n. If n is not a node
// in the grid, (NaN, NaN) is returned.
func (g *Grid) XY(id int64) (x, y float64) {
	if !g.has(id) {
		return math.NaN(), math.NaN()
	}
	r, c := g.RowCol(id)
	return float64(c), float64(r)
}

// NodeAt returns the node at (r, c). The returned node may be open or closed.
func (g *Grid) NodeAt(r, c int) graph.Node {
	if r < 0 || r >= g.r || c < 0 || c >= g.c {
		return nil
	}
	return simple.Node(r*g.c + c)
}

// From returns all the nodes reachable from u. Reachabilty requires that both
// ends of an edge must be open.
func (g *Grid) From(uid int64) graph.Nodes {
	if !g.HasOpen(uid) {
		return graph.Empty
	}
	nr, nc := g.RowCol(uid)
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			if v := g.NodeAt(r, c); v != nil && g.HasEdgeBetween(uid, v.ID()) {
				to = append(to, v)
			}
		}
	}
	if len(to) == 0 {
		return graph.Empty
	}
	return iterator.NewOrderedNodes(to)
}

// HasEdgeBetween returns whether there is an edge between u and v.
func (g *Grid) HasEdgeBetween(uid, vid int64) bool {
	if !g.HasOpen(uid) || !g.HasOpen(vid) || uid == vid {
		return false
	}
	ur, uc := g.RowCol(uid)
	vr, vc := g.RowCol(vid)
	if abs(ur-vr) > 1 || abs(uc-vc) > 1 {
		return false
	}
	return g.AllowDiagonal || ur == vr || uc == vc
}

func abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

// Edge returns the edge between u and v.
func (g *Grid) Edge(uid, vid int64) graph.Edge {
	return g.WeightedEdgeBetween(uid, vid)
}

// WeightedEdge returns the weighted edge between u and v.
func (g *Grid) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	return g.WeightedEdgeBetween(uid, vid)
}

// EdgeBetween returns the edge between u and v.
func (g *Grid) EdgeBetween(uid, vid int64) graph.Edge {
	return g.WeightedEdgeBetween(uid, vid)
}

// WeightedEdgeBetween returns the weighted edge between u and v.
func (g *Grid) WeightedEdgeBetween(uid, vid int64) graph.WeightedEdge {
	if g.HasEdgeBetween(uid, vid) {
		if !g.AllowDiagonal || g.UnitEdgeWeight {
			return simple.WeightedEdge{F: simple.Node(uid), T: simple.Node(vid), W: 1}
		}
		ux, uy := g.XY(uid)
		vx, vy := g.XY(vid)
		return simple.WeightedEdge{F: simple.Node(uid), T: simple.Node(vid), W: math.Hypot(ux-vx, uy-vy)}
	}
	return nil
}

// Weight returns the weight of the given edge.
func (g *Grid) Weight(xid, yid int64) (w float64, ok bool) {
	if xid == yid {
		return 0, true
	}
	if !g.HasEdgeBetween(xid, yid) {
		return math.Inf(1), false
	}
	if e := g.EdgeBetween(xid, yid); e != nil {
		if !g.AllowDiagonal || g.UnitEdgeWeight {
			return 1, true
		}
		ux, uy := g.XY(e.From().ID())
		vx, vy := g.XY(e.To().ID())
		return math.Hypot(ux-vx, uy-vy), true
	}
	return math.Inf(1), true
}

// String returns a string representation of the grid.
func (g *Grid) String() string {
	b, _ := g.Render(nil)
	return string(b)
}

// Render returns a text representation of the graph
// with the given path included. If the path is not a path
// in the grid Render returns a non-nil error and the
// path up to that point.
func (g *Grid) Render(path []graph.Node) ([]byte, error) {
	b := make([]byte, g.r*(g.c+1)-1)
	for r := 0; r < g.r; r++ {
		for c := 0; c < g.c; c++ {
			if g.open[r*g.c+c] {
				b[r*(g.c+1)+c] = Open
			} else {
				b[r*(g.c+1)+c] = Closed
			}
		}
		if r < g.r-1 {
			b[r*(g.c+1)+g.c] = '\n'
		}
	}

	// We don't use topo.IsPathIn at the outset because we
	// want to draw as much as possible before failing.
	for i, n := range path {
		id := n.ID()
		if !g.has(id) || (i != 0 && !g.HasEdgeBetween(path[i-1].ID(), id)) {
			if 0 <= id && id < int64(len(g.open)) {
				r, c := g.RowCol(id)
				b[r*(g.c+1)+c] = '!'
			}
			return b, errors.New("grid: not a path in graph")
		}
		r, c := g.RowCol(id)
		switch i {
		case len(path) - 1:
			b[r*(g.c+1)+c] = 'G'
		case 0:
			b[r*(g.c+1)+c] = 'S'
		default:
			b[r*(g.c+1)+c] = 'o'
		}
	}
	return b, nil
}
