// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/spatial/r2"
)

// GraphR2 is a graph with planar spatial representation of node positions.
type GraphR2 interface {
	graph.Graph
	LayoutNodeR2(id int64) NodeR2
}

// NodeR2 is a graph node with planar spatial representation of its position.
// A NodeR2 is only valid when the graph.Node is not nil.
type NodeR2 struct {
	graph.Node
	Coord2 r2.Vec
}
