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
	jg := johnsonWeightAdjuster{
		g:      g,
		from:   g.From,
		edgeTo: g.Edge,
	}
	if wg, ok := g.(Weighted); ok {
		jg.weight = wg.Weight
	} else {
		jg.weight = UniformCost(g)
	}

	paths = newAllShortest(graph.NodesOf(g.Nodes()), false)

	sign := int64(-1)
	for {
		// Choose a random node ID until we find
		// one that is not in g.
		jg.q = sign * rand.Int63()
		if _, exists := paths.indexOf[jg.q]; !exists {
			break
		}
		sign *= -1
	}

	jg.bellmanFord = true
	jg.adjustBy, ok = BellmanFordFrom(johnsonGraphNode(jg.q), jg)
	if !ok {
		return paths, false
	}

	jg.bellmanFord = false
	dijkstraAllPaths(jg, paths)

	for i, u := range paths.nodes {
		hu := jg.adjustBy.WeightTo(u.ID())
		for j, v := range paths.nodes {
			if i == j {
				continue
			}
			hv := jg.adjustBy.WeightTo(v.ID())
			paths.dist.Set(i, j, paths.dist.At(i, j)-hu+hv)
		}
	}

	return paths, ok
}

type johnsonWeightAdjuster struct {
	q int64
	g graph.Graph

	from   func(id int64) graph.Nodes
	edgeTo func(uid, vid int64) graph.Edge
	weight Weighting

	bellmanFord bool
	adjustBy    Shortest
}

var (
	// johnsonWeightAdjuster has the behaviour
	// of a directed graph, but we don't need
	// to be explicit with the type since it
	// is not exported.
	_ graph.Graph    = johnsonWeightAdjuster{}
	_ graph.Weighted = johnsonWeightAdjuster{}
)

func (g johnsonWeightAdjuster) Node(id int64) graph.Node {
	if g.bellmanFord && id == g.q {
		return simple.Node(id)
	}
	panic("path: unintended use of johnsonWeightAdjuster")
}

func (g johnsonWeightAdjuster) Nodes() graph.Nodes {
	if g.bellmanFord {
		return newJohnsonNodeIterator(g.q, g.g.Nodes())
	}
	return g.g.Nodes()
}

func (g johnsonWeightAdjuster) From(id int64) graph.Nodes {
	if g.bellmanFord && id == g.q {
		return g.g.Nodes()
	}
	return g.from(id)
}

func (g johnsonWeightAdjuster) WeightedEdge(_, _ int64) graph.WeightedEdge {
	panic("path: unintended use of johnsonWeightAdjuster")
}

func (g johnsonWeightAdjuster) Edge(uid, vid int64) graph.Edge {
	if g.bellmanFord && uid == g.q && g.g.Node(vid) != nil {
		return simple.Edge{F: johnsonGraphNode(g.q), T: simple.Node(vid)}
	}
	return g.edgeTo(uid, vid)
}

func (g johnsonWeightAdjuster) Weight(xid, yid int64) (w float64, ok bool) {
	if g.bellmanFord {
		switch g.q {
		case xid:
			return 0, true
		case yid:
			return math.Inf(1), false
		default:
			return g.weight(xid, yid)
		}
	}
	w, ok = g.weight(xid, yid)
	return w + g.adjustBy.WeightTo(xid) - g.adjustBy.WeightTo(yid), ok
}

func (johnsonWeightAdjuster) HasEdgeBetween(_, _ int64) bool {
	panic("path: unintended use of johnsonWeightAdjuster")
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
