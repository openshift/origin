// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"errors"
	"math"

	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
)

// LimitedVisionGrid is a 2D grid planar undirected graph where the capacity
// to determine the presence of edges is dependent on the current and past
// positions on the grid. In the absence of information, the grid is
// optimistic.
type LimitedVisionGrid struct {
	Grid *Grid

	// Location is the current
	// location on the grid.
	Location graph.Node

	// VisionRadius specifies how far
	// away edges can be detected.
	VisionRadius float64

	// Known holds a store of known
	// nodes, if not nil.
	Known map[int]bool
}

// MoveTo moves to the node n on the grid and returns a slice of newly seen and
// already known edges. MoveTo panics if n is nil.
func (l *LimitedVisionGrid) MoveTo(n graph.Node) (new, old []graph.Edge) {
	l.Location = n
	row, column := l.RowCol(n.ID())
	x := float64(column)
	y := float64(row)
	seen := make(map[[2]int]bool)
	bound := int(l.VisionRadius + 0.5)
	for r := row - bound; r <= row+bound; r++ {
		for c := column - bound; c <= column+bound; c++ {
			u := l.NodeAt(r, c)
			if u == nil {
				continue
			}
			ux, uy := l.XY(u)
			if math.Hypot(x-ux, y-uy) > l.VisionRadius {
				continue
			}
			for _, v := range l.allPossibleFrom(u) {
				if seen[[2]int{u.ID(), v.ID()}] {
					continue
				}
				seen[[2]int{u.ID(), v.ID()}] = true

				vx, vy := l.XY(v)
				if !l.Known[v.ID()] && math.Hypot(x-vx, y-vy) > l.VisionRadius {
					continue
				}

				e := simple.Edge{F: u, T: v}
				if !l.Known[u.ID()] || !l.Known[v.ID()] {
					new = append(new, e)
				} else {
					old = append(old, e)
				}
			}
		}
	}

	if l.Known != nil {
		for r := row - bound; r <= row+bound; r++ {
			for c := column - bound; c <= column+bound; c++ {
				u := l.NodeAt(r, c)
				if u == nil {
					continue
				}
				ux, uy := l.XY(u)
				if math.Hypot(x-ux, y-uy) > l.VisionRadius {
					continue
				}
				for _, v := range l.allPossibleFrom(u) {
					vx, vy := l.XY(v)
					if math.Hypot(x-vx, y-vy) > l.VisionRadius {
						continue
					}
					l.Known[v.ID()] = true
				}
				l.Known[u.ID()] = true
			}
		}

	}

	return new, old
}

// allPossibleFrom returns all the nodes possibly reachable from u.
func (l *LimitedVisionGrid) allPossibleFrom(u graph.Node) []graph.Node {
	if !l.Has(u) {
		return nil
	}
	nr, nc := l.RowCol(u.ID())
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			v := l.NodeAt(r, c)
			if v == nil || u.ID() == v.ID() {
				continue
			}
			ur, uc := l.RowCol(u.ID())
			vr, vc := l.RowCol(v.ID())
			if abs(ur-vr) > 1 || abs(uc-vc) > 1 {
				continue
			}
			if !l.Grid.AllowDiagonal && ur != vr && uc != vc {
				continue
			}
			to = append(to, v)
		}
	}
	return to
}

// RowCol returns the row and column of the id. RowCol will panic if the
// node id is outside the range of the grid.
func (l *LimitedVisionGrid) RowCol(id int) (r, c int) {
	return l.Grid.RowCol(id)
}

// XY returns the cartesian coordinates of n. If n is not a node
// in the grid, (NaN, NaN) is returned.
func (l *LimitedVisionGrid) XY(n graph.Node) (x, y float64) {
	if !l.Has(n) {
		return math.NaN(), math.NaN()
	}
	r, c := l.RowCol(n.ID())
	return float64(c), float64(r)
}

// Nodes returns all the nodes in the grid.
func (l *LimitedVisionGrid) Nodes() []graph.Node {
	nodes := make([]graph.Node, 0, len(l.Grid.open))
	for id := range l.Grid.open {
		nodes = append(nodes, simple.Node(id))
	}
	return nodes
}

// NodeAt returns the node at (r, c). The returned node may be open or closed.
func (l *LimitedVisionGrid) NodeAt(r, c int) graph.Node {
	return l.Grid.NodeAt(r, c)
}

// Has returns whether n is a node in the grid.
func (l *LimitedVisionGrid) Has(n graph.Node) bool {
	return l.has(n.ID())
}

func (l *LimitedVisionGrid) has(id int) bool {
	return id >= 0 && id < len(l.Grid.open)
}

// From returns nodes that are optimistically reachable from u.
func (l *LimitedVisionGrid) From(u graph.Node) []graph.Node {
	if !l.Has(u) {
		return nil
	}

	nr, nc := l.RowCol(u.ID())
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			if v := l.NodeAt(r, c); v != nil && l.HasEdgeBetween(u, v) {
				to = append(to, v)
			}
		}
	}
	return to
}

// HasEdgeBetween optimistically returns whether an edge is exists between u and v.
func (l *LimitedVisionGrid) HasEdgeBetween(u, v graph.Node) bool {
	if u.ID() == v.ID() {
		return false
	}
	ur, uc := l.RowCol(u.ID())
	vr, vc := l.RowCol(v.ID())
	if abs(ur-vr) > 1 || abs(uc-vc) > 1 {
		return false
	}
	if !l.Grid.AllowDiagonal && ur != vr && uc != vc {
		return false
	}

	x, y := l.XY(l.Location)
	ux, uy := l.XY(u)
	vx, vy := l.XY(v)
	uKnown := l.Known[u.ID()] || math.Hypot(x-ux, y-uy) <= l.VisionRadius
	vKnown := l.Known[v.ID()] || math.Hypot(x-vx, y-vy) <= l.VisionRadius

	switch {
	case uKnown && vKnown:
		return l.Grid.HasEdgeBetween(u, v)
	case uKnown:
		return l.Grid.HasOpen(u)
	case vKnown:
		return l.Grid.HasOpen(v)
	default:
		return true
	}
}

// Edge optimistically returns the edge from u to v.
func (l *LimitedVisionGrid) Edge(u, v graph.Node) graph.Edge {
	return l.EdgeBetween(u, v)
}

// EdgeBetween optimistically returns the edge between u and v.
func (l *LimitedVisionGrid) EdgeBetween(u, v graph.Node) graph.Edge {
	if l.HasEdgeBetween(u, v) {
		if !l.Grid.AllowDiagonal || l.Grid.UnitEdgeWeight {
			return simple.Edge{F: u, T: v, W: 1}
		}
		ux, uy := l.XY(u)
		vx, vy := l.XY(v)
		return simple.Edge{F: u, T: v, W: math.Hypot(ux-vx, uy-vy)}
	}
	return nil
}

// Weight returns the weight of the given edge.
func (l *LimitedVisionGrid) Weight(x, y graph.Node) (w float64, ok bool) {
	if x.ID() == y.ID() {
		return 0, true
	}
	if !l.HasEdgeBetween(x, y) {
		return math.Inf(1), false
	}
	if e := l.EdgeBetween(x, y); e != nil {
		if !l.Grid.AllowDiagonal || l.Grid.UnitEdgeWeight {
			return 1, true
		}
		ux, uy := l.XY(e.From())
		vx, vy := l.XY(e.To())
		return math.Hypot(ux-vx, uy-vy), true

	}
	return math.Inf(1), true
}

// String returns a string representation of the grid.
func (l *LimitedVisionGrid) String() string {
	b, _ := l.Render(nil)
	return string(b)
}

// Render returns a text representation of the graph
// with the given path included. If the path is not a path
// in the grid Render returns a non-nil error and the
// path up to that point.
func (l *LimitedVisionGrid) Render(path []graph.Node) ([]byte, error) {
	rows, cols := l.Grid.Dims()
	b := make([]byte, rows*(cols+1)-1)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if !l.Known[r*cols+c] {
				b[r*(cols+1)+c] = Unknown
			} else if l.Grid.open[r*cols+c] {
				b[r*(cols+1)+c] = Open
			} else {
				b[r*(cols+1)+c] = Closed
			}
		}
		if r < rows-1 {
			b[r*(cols+1)+cols] = '\n'
		}
	}

	// We don't use topo.IsPathIn at the outset because we
	// want to draw as much as possible before failing.
	for i, n := range path {
		if !l.Has(n) || (i != 0 && !l.HasEdgeBetween(path[i-1], n)) {
			id := n.ID()
			if id >= 0 && id < len(l.Grid.open) {
				r, c := l.RowCol(n.ID())
				b[r*(cols+1)+c] = '!'
			}
			return b, errors.New("grid: not a path in graph")
		}
		r, c := l.RowCol(n.ID())
		switch i {
		case len(path) - 1:
			b[r*(cols+1)+c] = 'G'
		case 0:
			b[r*(cols+1)+c] = 'S'
		default:
			b[r*(cols+1)+c] = 'o'
		}
	}
	return b, nil
}
