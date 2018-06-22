// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"

	"github.com/gonum/graph"
)

// Weighting is a mapping between a pair of nodes and a weight. It follows the
// semantics of the Weighter interface.
type Weighting func(x, y graph.Node) (w float64, ok bool)

// UniformCost returns a Weighting that returns an edge cost of 1 for existing
// edges, zero for node identity and Inf for otherwise absent edges.
func UniformCost(g graph.Graph) Weighting {
	return func(x, y graph.Node) (w float64, ok bool) {
		xid := x.ID()
		yid := y.ID()
		if xid == yid {
			return 0, true
		}
		if e := g.Edge(x, y); e != nil {
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
