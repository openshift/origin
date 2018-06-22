// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"
	"math/rand"

	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
)

// JohnsonAllPaths returns a shortest-path tree for shortest paths in the graph g.
// If the graph does not implement graph.Weighter, UniformCost is used.
//
// The time complexity of JohnsonAllPaths is O(|V|.|E|+|V|^2.log|V|).
func JohnsonAllPaths(g graph.Graph) (paths AllShortest, ok bool) {
	jg := johnsonWeightAdjuster{
		g:      g,
		from:   g.From,
		edgeTo: g.Edge,
	}
	if wg, ok := g.(graph.Weighter); ok {
		jg.weight = wg.Weight
	} else {
		jg.weight = UniformCost(g)
	}

	paths = newAllShortest(g.Nodes(), false)

	sign := -1
	for {
		// Choose a random node ID until we find
		// one that is not in g.
		jg.q = sign * rand.Int()
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
		hu := jg.adjustBy.WeightTo(u)
		for j, v := range paths.nodes {
			if i == j {
				continue
			}
			hv := jg.adjustBy.WeightTo(v)
			paths.dist.Set(i, j, paths.dist.At(i, j)-hu+hv)
		}
	}

	return paths, ok
}

type johnsonWeightAdjuster struct {
	q int
	g graph.Graph

	from   func(graph.Node) []graph.Node
	edgeTo func(graph.Node, graph.Node) graph.Edge
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
	_ graph.Weighter = johnsonWeightAdjuster{}
)

func (g johnsonWeightAdjuster) Has(n graph.Node) bool {
	if g.bellmanFord && n.ID() == g.q {
		return true
	}
	return g.g.Has(n)

}

func (g johnsonWeightAdjuster) Nodes() []graph.Node {
	if g.bellmanFord {
		return append(g.g.Nodes(), johnsonGraphNode(g.q))
	}
	return g.g.Nodes()
}

func (g johnsonWeightAdjuster) From(n graph.Node) []graph.Node {
	if g.bellmanFord && n.ID() == g.q {
		return g.g.Nodes()
	}
	return g.from(n)
}

func (g johnsonWeightAdjuster) Edge(u, v graph.Node) graph.Edge {
	if g.bellmanFord && u.ID() == g.q && g.g.Has(v) {
		return simple.Edge{F: johnsonGraphNode(g.q), T: v}
	}
	return g.edgeTo(u, v)
}

func (g johnsonWeightAdjuster) Weight(x, y graph.Node) (w float64, ok bool) {
	if g.bellmanFord {
		switch g.q {
		case x.ID():
			return 0, true
		case y.ID():
			return math.Inf(1), false
		default:
			return g.weight(x, y)
		}
	}
	w, ok = g.weight(x, y)
	return w + g.adjustBy.WeightTo(x) - g.adjustBy.WeightTo(y), ok
}

func (johnsonWeightAdjuster) HasEdgeBetween(_, _ graph.Node) bool {
	panic("path: unintended use of johnsonWeightAdjuster")
}

type johnsonGraphNode int

func (n johnsonGraphNode) ID() int { return int(n) }
