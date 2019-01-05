// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/traverse"
)

// Weighted is a weighted graph. It is a subset of graph.Weighted.
type Weighted interface {
	// Weight returns the weight for the edge between
	// x and y with IDs xid and yid if Edge(xid, yid)
	// returns a non-nil Edge.
	// If x and y are the same node or there is no
	// joining edge between the two nodes the weight
	// value returned is implementation dependent.
	// Weight returns true if an edge exists between
	// x and y or if x and y have the same ID, false
	// otherwise.
	Weight(xid, yid int64) (w float64, ok bool)
}

// Weighting is a mapping between a pair of nodes and a weight. It follows the
// semantics of the Weighter interface.
type Weighting func(xid, yid int64) (w float64, ok bool)

// UniformCost returns a Weighting that returns an edge cost of 1 for existing
// edges, zero for node identity and Inf for otherwise absent edges.
func UniformCost(g traverse.Graph) Weighting {
	return func(xid, yid int64) (w float64, ok bool) {
		if xid == yid {
			return 0, true
		}
		if e := g.Edge(xid, yid); e != nil {
			return 1, true
		}
		return math.Inf(1), false
	}
}

// Heuristic returns an estimate of the cost of travelling between two nodes.
type Heuristic func(x, y graph.Node) float64

// HeuristicCoster wraps the HeuristicCost method. A graph implementing the
// interface provides a heuristic between any two given nodes.
type HeuristicCoster interface {
	HeuristicCost(x, y graph.Node) float64
}
