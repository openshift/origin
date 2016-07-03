// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"fmt"
	"math"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
)

const (
	Closed = '*' // Closed is the closed grid node representation.
	Open   = '.' // Open is the open grid node repesentation.
)

// Grid is a 2D grid planar undirected graph.
type Grid struct {
	// AllowDiagonal specifies whether
	// diagonally adjacent nodes can
	// be connected by an edge.
	AllowDiagonal bool

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

// Nodes returns all the open nodes in the grid.
func (g *Grid) Nodes() []graph.Node {
	var nodes []graph.Node
	for id, ok := range g.open {
		if !ok {
			continue
		}
		nodes = append(nodes, concrete.Node(id))
	}
	return nodes
}

// Has returns whether n is an open node in the grid.
func (g *Grid) Has(n graph.Node) bool {
	id := n.ID()
	return id >= 0 && id < len(g.open) && g.open[id]
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
func (g *Grid) RowCol(id int) (r, c int) {
	if id < 0 || id >= len(g.open) {
		panic("grid: illegal node id")
	}
	return id / g.c, id % g.c
}

// XY returns the cartesian coordinates of n. If n is not a node
// in the grid, (NaN, NaN) is returned.
func (g *Grid) XY(n graph.Node) (x, y float64) {
	if !g.Has(n) {
		return math.NaN(), math.NaN()
	}
	r, c := g.RowCol(n.ID())
	return float64(c), float64(r)
}

// NodeAt returns the node at (r, c). The returned node may be open or closed.
func (g *Grid) NodeAt(r, c int) graph.Node {
	if r < 0 || r >= g.r || c < 0 || c >= g.c {
		return nil
	}
	return concrete.Node(r*g.c + c)
}

// From returns all the nodes reachable from u.
func (g *Grid) From(u graph.Node) []graph.Node {
	if !g.Has(u) {
		return nil
	}
	nr, nc := g.RowCol(u.ID())
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			if v := g.NodeAt(r, c); v != nil && g.HasEdge(u, v) {
				to = append(to, v)
			}
		}
	}
	return to
}

// HasEdge returns whether there is an edge between u and v.
func (g *Grid) HasEdge(u, v graph.Node) bool {
	if !g.Has(u) || !g.Has(v) || u.ID() == v.ID() {
		return false
	}
	ur, uc := g.RowCol(u.ID())
	vr, vc := g.RowCol(v.ID())
	if abs(ur-vr) > 1 && abs(uc-vc) > 1 {
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
func (g *Grid) Edge(u, v graph.Node) graph.Edge {
	return g.EdgeBetween(u, v)
}

// EdgeBetween returns the edge between u and v.
func (g *Grid) EdgeBetween(u, v graph.Node) graph.Edge {
	if g.HasEdge(u, v) {
		return concrete.Edge{u, v}
	}
	return nil
}

// Weight returns the weight of the given edge.
func (g *Grid) Weight(e graph.Edge) float64 {
	if e := g.EdgeBetween(e.From(), e.To()); e != nil {
		if !g.AllowDiagonal {
			return 1
		}
		ux, uy := g.XY(e.From())
		vx, vy := g.XY(e.To())
		return math.Hypot(ux-vx, uy-vy)

	}
	return math.Inf(1)
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
		if !g.Has(n) || (i != 0 && !g.HasEdge(path[i-1], n)) {
			id := n.ID()
			if id >= 0 && id < len(g.open) {
				r, c := g.RowCol(n.ID())
				b[r*(g.c+1)+c] = '!'
			}
			return b, errors.New("grid: not a path in graph")
		}
		r, c := g.RowCol(n.ID())
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
