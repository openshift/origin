// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package multi

import "gonum.org/v1/gonum/graph"

// Node here is a duplication of simple.Node
// to avoid needing to import both packages.

// Node is a simple graph node.
type Node int64

// ID returns the ID number of the node.
func (n Node) ID() int64 {
	return int64(n)
}

// Edge is a collection of multigraph edges sharing end points.
type Edge []graph.Line

// From returns the from-node of the edge.
func (e Edge) From() graph.Node {
	if len(e) == 0 {
		return nil
	}
	return e[0].From()
}

// To returns the to-node of the edge.
func (e Edge) To() graph.Node {
	if len(e) == 0 {
		return nil
	}
	return e[0].To()
}

// Line is a multigraph edge.
type Line struct {
	F, T graph.Node

	UID int64
}

// From returns the from-node of the line.
func (l Line) From() graph.Node { return l.F }

// To returns the to-node of the line.
func (l Line) To() graph.Node { return l.T }

// ID returns the ID of the line.
func (l Line) ID() int64 { return l.UID }

// WeightedEdge is a collection of weighted multigraph edges sharing end points.
type WeightedEdge struct {
	Lines []graph.WeightedLine

	// WeightFunc calculates the aggregate
	// weight of the lines in Lines. If
	// WeightFunc is nil, the sum of weights
	// is used as the edge weight.
	WeightFunc func([]graph.WeightedLine) float64
}

// From returns the from-node of the edge.
func (e WeightedEdge) From() graph.Node {
	if len(e.Lines) == 0 {
		return nil
	}
	return e.Lines[0].From()
}

// To returns the to-node of the edge.
func (e WeightedEdge) To() graph.Node {
	if len(e.Lines) == 0 {
		return nil
	}
	return e.Lines[0].To()
}

// Weight returns the weight of the edge.
func (e WeightedEdge) Weight() float64 {
	if e.WeightFunc == nil {
		var w float64
		for _, l := range e.Lines {
			w += l.Weight()
		}
		return w
	}
	return e.WeightFunc(e.Lines)
}

// WeightedLine is a weighted multigraph edge.
type WeightedLine struct {
	F, T graph.Node
	W    float64

	UID int64
}

// From returns the from-node of the line.
func (l WeightedLine) From() graph.Node { return l.F }

// To returns the to-node of the line.
func (l WeightedLine) To() graph.Node { return l.T }

// ID returns the ID of the line.
func (l WeightedLine) ID() int64 { return l.UID }

// Weight returns the weight of the edge.
func (l WeightedLine) Weight() float64 { return l.W }
