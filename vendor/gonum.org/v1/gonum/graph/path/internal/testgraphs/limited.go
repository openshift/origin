// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testgraphs

import (
	"errors"
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
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
	Known map[int64]bool
}

// MoveTo moves to the node n on the grid and returns a slice of newly seen and
// already known edges. MoveTo panics if n is nil.
func (l *LimitedVisionGrid) MoveTo(n graph.Node) (new, old []graph.Edge) {
	l.Location = n
	row, column := l.RowCol(n.ID())
	x := float64(column)
	y := float64(row)
	seen := make(map[[2]int64]bool)
	bound := int(l.VisionRadius + 0.5)
	for r := row - bound; r <= row+bound; r++ {
		for c := column - bound; c <= column+bound; c++ {
			u := l.NodeAt(r, c)
			if u == nil {
				continue
			}
			uid := u.ID()
			ux, uy := l.XY(uid)
			if math.Hypot(x-ux, y-uy) > l.VisionRadius {
				continue
			}
			for _, v := range l.allPossibleFrom(uid) {
				vid := v.ID()
				if seen[[2]int64{uid, vid}] {
					continue
				}
				seen[[2]int64{uid, vid}] = true

				vx, vy := l.XY(vid)
				if !l.Known[vid] && math.Hypot(x-vx, y-vy) > l.VisionRadius {
					continue
				}

				e := simple.Edge{F: u, T: v}
				if !l.Known[uid] || !l.Known[vid] {
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
				uid := u.ID()
				ux, uy := l.XY(uid)
				if math.Hypot(x-ux, y-uy) > l.VisionRadius {
					continue
				}
				for _, v := range l.allPossibleFrom(uid) {
					vid := v.ID()
					vx, vy := l.XY(vid)
					if math.Hypot(x-vx, y-vy) > l.VisionRadius {
						continue
					}
					l.Known[vid] = true
				}
				l.Known[uid] = true
			}
		}

	}

	return new, old
}

// allPossibleFrom returns all the nodes possibly reachable from u.
func (l *LimitedVisionGrid) allPossibleFrom(uid int64) []graph.Node {
	if !l.has(uid) {
		return nil
	}
	nr, nc := l.RowCol(uid)
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			v := l.NodeAt(r, c)
			if v == nil || uid == v.ID() {
				continue
			}
			ur, uc := l.RowCol(uid)
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
func (l *LimitedVisionGrid) RowCol(id int64) (r, c int) {
	return l.Grid.RowCol(id)
}

// XY returns the cartesian coordinates of n. If n is not a node
// in the grid, (NaN, NaN) is returned.
func (l *LimitedVisionGrid) XY(id int64) (x, y float64) {
	if !l.has(id) {
		return math.NaN(), math.NaN()
	}
	r, c := l.RowCol(id)
	return float64(c), float64(r)
}

// Nodes returns all the nodes in the grid.
func (l *LimitedVisionGrid) Nodes() graph.Nodes {
	nodes := make([]graph.Node, 0, len(l.Grid.open))
	for id := range l.Grid.open {
		nodes = append(nodes, simple.Node(id))
	}
	return iterator.NewOrderedNodes(nodes)
}

// NodeAt returns the node at (r, c). The returned node may be open or closed.
func (l *LimitedVisionGrid) NodeAt(r, c int) graph.Node {
	return l.Grid.NodeAt(r, c)
}

// Node returns the node with the given ID if it exists in the graph,
// and nil otherwise.
func (l *LimitedVisionGrid) Node(id int64) graph.Node {
	if l.has(id) {
		return simple.Node(id)
	}
	return nil
}

// has returns whether the node with the given ID is a node in the grid.
func (l *LimitedVisionGrid) has(id int64) bool {
	return 0 <= id && id < int64(len(l.Grid.open))
}

// From returns nodes that are optimistically reachable from u.
func (l *LimitedVisionGrid) From(uid int64) graph.Nodes {
	if !l.has(uid) {
		return graph.Empty
	}

	nr, nc := l.RowCol(uid)
	var to []graph.Node
	for r := nr - 1; r <= nr+1; r++ {
		for c := nc - 1; c <= nc+1; c++ {
			if v := l.NodeAt(r, c); v != nil && l.HasEdgeBetween(uid, v.ID()) {
				to = append(to, v)
			}
		}
	}
	if len(to) == 0 {
		return graph.Empty
	}
	return iterator.NewOrderedNodes(to)
}

// HasEdgeBetween optimistically returns whether an edge is exists between u and v.
func (l *LimitedVisionGrid) HasEdgeBetween(uid, vid int64) bool {
	if uid == vid {
		return false
	}
	ur, uc := l.RowCol(uid)
	vr, vc := l.RowCol(vid)
	if abs(ur-vr) > 1 || abs(uc-vc) > 1 {
		return false
	}
	if !l.Grid.AllowDiagonal && ur != vr && uc != vc {
		return false
	}

	x, y := l.XY(l.Location.ID())
	ux, uy := l.XY(uid)
	vx, vy := l.XY(vid)
	uKnown := l.Known[uid] || math.Hypot(x-ux, y-uy) <= l.VisionRadius
	vKnown := l.Known[vid] || math.Hypot(x-vx, y-vy) <= l.VisionRadius

	switch {
	case uKnown && vKnown:
		return l.Grid.HasEdgeBetween(uid, vid)
	case uKnown:
		return l.Grid.HasOpen(uid)
	case vKnown:
		return l.Grid.HasOpen(vid)
	default:
		return true
	}
}

// Edge optimistically returns the edge from u to v.
func (l *LimitedVisionGrid) Edge(uid, vid int64) graph.Edge {
	return l.WeightedEdgeBetween(uid, vid)
}

// Edge optimistically returns the weighted edge from u to v.
func (l *LimitedVisionGrid) WeightedEdge(uid, vid int64) graph.WeightedEdge {
	return l.WeightedEdgeBetween(uid, vid)
}

// WeightedEdgeBetween optimistically returns the edge between u and v.
func (l *LimitedVisionGrid) EdgeBetween(uid, vid int64) graph.Edge {
	return l.WeightedEdgeBetween(uid, vid)
}

// WeightedEdgeBetween optimistically returns the weighted edge between u and v.
func (l *LimitedVisionGrid) WeightedEdgeBetween(uid, vid int64) graph.WeightedEdge {
	if l.HasEdgeBetween(uid, vid) {
		if !l.Grid.AllowDiagonal || l.Grid.UnitEdgeWeight {
			return simple.WeightedEdge{F: simple.Node(uid), T: simple.Node(vid), W: 1}
		}
		ux, uy := l.XY(uid)
		vx, vy := l.XY(vid)
		return simple.WeightedEdge{F: simple.Node(uid), T: simple.Node(vid), W: math.Hypot(ux-vx, uy-vy)}
	}
	return nil
}

// Weight returns the weight of the given edge.
func (l *LimitedVisionGrid) Weight(xid, yid int64) (w float64, ok bool) {
	if xid == yid {
		return 0, true
	}
	if !l.HasEdgeBetween(xid, yid) {
		return math.Inf(1), false
	}
	if e := l.EdgeBetween(xid, yid); e != nil {
		if !l.Grid.AllowDiagonal || l.Grid.UnitEdgeWeight {
			return 1, true
		}
		ux, uy := l.XY(e.From().ID())
		vx, vy := l.XY(e.To().ID())
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
			if !l.Known[int64(r*cols+c)] {
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
		id := n.ID()
		if !l.has(id) || (i != 0 && !l.HasEdgeBetween(path[i-1].ID(), id)) {
			if 0 <= id && id < int64(len(l.Grid.open)) {
				r, c := l.RowCol(id)
				b[r*(cols+1)+c] = '!'
			}
			return b, errors.New("grid: not a path in graph")
		}
		r, c := l.RowCol(id)
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
