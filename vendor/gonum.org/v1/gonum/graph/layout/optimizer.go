// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/spatial/r2"
)

// LayoutR2 implements graph layout updates and representations.
type LayoutR2 interface {
	// IsInitialized returns whether the Layout is initialized.
	IsInitialized() bool

	// SetCoord2 sets the coordinates of the node with the given
	// id to coords.
	SetCoord2(id int64, coords r2.Vec)

	// Coord2 returns the coordinated of the node with the given
	// id in the graph layout.
	Coord2(id int64) r2.Vec
}

// NewOptimizerR2 returns a new layout optimizer. If g implements LayoutR2 the layout
// will be updated into g, otherwise the OptimizerR2 will hold the graph layout. A nil
// value for update is a valid no-op layout update function.
func NewOptimizerR2(g graph.Graph, update func(graph.Graph, LayoutR2) bool) OptimizerR2 {
	l, ok := g.(LayoutR2)
	if !ok {
		l = make(coordinatesR2)
	}
	return OptimizerR2{
		g:       g,
		layout:  l,
		Updater: update,
	}
}

// coordinatesR2 is the default layout store for R2.
type coordinatesR2 map[int64]r2.Vec

func (c coordinatesR2) IsInitialized() bool            { return len(c) != 0 }
func (c coordinatesR2) SetCoord2(id int64, pos r2.Vec) { c[id] = pos }
func (c coordinatesR2) Coord2(id int64) r2.Vec         { return c[id] }

// OptimizerR2 is a helper type that holds a graph and layout
// optimization state.
type OptimizerR2 struct {
	g      graph.Graph
	layout LayoutR2

	// Updater is the function called for each call to Update.
	// It updates the OptimizerR2's spatial distribution of the
	// nodes in the backing graph.
	Updater func(graph.Graph, LayoutR2) bool
}

// Coord2 returns the location of the node with the given
// ID. The returned value is only valid if the node exists
// in the graph.
func (g OptimizerR2) Coord2(id int64) r2.Vec {
	return g.layout.Coord2(id)
}

// Update updates the locations of the nodes in the graph
// according to the provided update function. It returns whether
// the update function is able to further refine the graph's
// node locations.
func (g OptimizerR2) Update() bool {
	if g.Updater == nil {
		return false
	}
	return g.Updater(g.g, g.layout)
}

// LayoutNodeR2 implements the GraphR2 interface.
func (g OptimizerR2) LayoutNodeR2(id int64) NodeR2 {
	n := g.g.Node(id)
	if n == nil {
		return NodeR2{}
	}
	return NodeR2{Node: n, Coord2: g.Coord2(id)}
}

// Node returns the node with the given ID if it exists
// in the graph, and nil otherwise.
func (g OptimizerR2) Node(id int64) graph.Node { return g.g.Node(id) }

// Nodes returns all the nodes in the graph.
func (g OptimizerR2) Nodes() graph.Nodes { return g.g.Nodes() }

// From returns all nodes that can be reached directly
// from the node with the given ID.
func (g OptimizerR2) From(id int64) graph.Nodes { return g.g.From(id) }

// HasEdgeBetween returns whether an edge exists between
// nodes with IDs xid and yid without considering direction.
func (g OptimizerR2) HasEdgeBetween(xid, yid int64) bool { return g.g.HasEdgeBetween(xid, yid) }

// Edge returns the edge from u to v, with IDs uid and vid,
// if such an edge exists and nil otherwise. The node v
// must be directly reachable from u as defined by the
// From method.
func (g OptimizerR2) Edge(uid, vid int64) graph.Edge { return g.g.Edge(uid, vid) }
