// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// JohnsonAllPaths returns a shortest-path tree for shortest paths in the graph g.
// If the graph does not implement Weighted, UniformCost is used. If a negative cycle
// exists in g, ok will be returned false and paths will not contain valid data.
//
// The time complexity of JohnsonAllPaths is O(|V|.|E|+|V|^2.log|V|).
func JohnsonAllPaths(g graph.Graph) (paths AllShortest, ok bool) {
	adjusted := johnsonWeightAdjuster{Graph: g}
	if wg, ok := g.(Weighted); ok {
		adjusted.weight = wg.Weight
	} else {
		adjusted.weight = UniformCost(g)
	}

	paths = newAllShortest(graph.NodesOf(g.Nodes()), false)

	var q int64
	sign := int64(-1)
	for {
		// Choose a random node ID until we find
		// one that is not in g.
		q = sign * rand.Int63()
		if _, exists := paths.indexOf[q]; !exists {
			break
		}
		sign *= -1
	}

	adjusted.adjustBy, ok = BellmanFordFrom(johnsonGraphNode(q), johnsonReWeight{adjusted, q})
	if !ok {
		return paths, false
	}

	dijkstraAllPaths(adjusted, paths)

	for i, u := range paths.nodes {
		hu := adjusted.adjustBy.WeightTo(u.ID())
		for j, v := range paths.nodes {
			if i == j {
				continue
			}
			hv := adjusted.adjustBy.WeightTo(v.ID())
			paths.dist.Set(i, j, paths.dist.At(i, j)-hu+hv)
		}
	}

	return paths, ok
}

// johnsonWeightAdjuster is an edge re-weighted graph constructed
// by the first phase of the Johnson algorithm such that no negative
// edge weights exist in the graph.
type johnsonWeightAdjuster struct {
	graph.Graph
	weight Weighting

	adjustBy Shortest
}

var _ graph.Weighted = johnsonWeightAdjuster{}

func (g johnsonWeightAdjuster) Node(id int64) graph.Node {
	panic("path: unintended use of johnsonWeightAdjuster")
}

func (g johnsonWeightAdjuster) WeightedEdge(_, _ int64) graph.WeightedEdge {
	panic("path: unintended use of johnsonWeightAdjuster")
}

func (g johnsonWeightAdjuster) Weight(xid, yid int64) (w float64, ok bool) {
	w, ok = g.weight(xid, yid)
	return w + g.adjustBy.WeightTo(xid) - g.adjustBy.WeightTo(yid), ok
}

func (johnsonWeightAdjuster) HasEdgeBetween(_, _ int64) bool {
	panic("path: unintended use of johnsonWeightAdjuster")
}

// johnsonReWeight provides a query node to allow edge re-weighting
// using the Bellman-Ford algorithm for the first phase of the
// Johnson algorithm.
type johnsonReWeight struct {
	johnsonWeightAdjuster
	q int64
}

func (g johnsonReWeight) Node(id int64) graph.Node {
	if id != g.q {
		panic("path: unintended use of johnsonReWeight")
	}
	return simple.Node(id)
}

func (g johnsonReWeight) Nodes() graph.Nodes {
	return newJohnsonNodeIterator(g.q, g.Graph.Nodes())
}

func (g johnsonReWeight) From(id int64) graph.Nodes {
	if id == g.q {
		return g.Graph.Nodes()
	}
	return g.Graph.From(id)
}

func (g johnsonReWeight) Edge(uid, vid int64) graph.Edge {
	if uid == g.q && g.Graph.Node(vid) != nil {
		return simple.Edge{F: johnsonGraphNode(g.q), T: simple.Node(vid)}
	}
	return g.Graph.Edge(uid, vid)
}

func (g johnsonReWeight) Weight(xid, yid int64) (w float64, ok bool) {
	switch g.q {
	case xid:
		return 0, true
	case yid:
		return math.Inf(1), false
	default:
		return g.weight(xid, yid)
	}
}

type johnsonGraphNode int64

func (n johnsonGraphNode) ID() int64 { return int64(n) }

func newJohnsonNodeIterator(q int64, nodes graph.Nodes) *johnsonNodeIterator {
	return &johnsonNodeIterator{q: q, nodes: nodes}
}

type johnsonNodeIterator struct {
	q          int64
	nodes      graph.Nodes
	qUsed, qOK bool
}

func (it *johnsonNodeIterator) Len() int {
	var len int
	if it.nodes != nil {
		len = it.nodes.Len()
	}
	if !it.qUsed {
		len++
	}
	return len
}

func (it *johnsonNodeIterator) Next() bool {
	if it.nodes != nil {
		ok := it.nodes.Next()
		if ok {
			return true
		}
	}
	if !it.qUsed {
		it.qOK = true
		it.qUsed = true
		return true
	}
	it.qOK = false
	return false
}

func (it *johnsonNodeIterator) Node() graph.Node {
	if it.qOK {
		return johnsonGraphNode(it.q)
	}
	if it.nodes == nil {
		return nil
	}
	return it.nodes.Node()
}

func (it *johnsonNodeIterator) Reset() {
	it.qOK = false
	it.qUsed = false
	if it.nodes == nil {
		return
	}
	it.nodes.Reset()
}
