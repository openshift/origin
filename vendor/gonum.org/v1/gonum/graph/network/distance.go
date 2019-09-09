// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
)

// Closeness returns the closeness centrality for nodes in the graph g used to
// construct the given shortest paths.
//
//  C(v) = 1 / \sum_u d(u,v)
//
// For directed graphs the incoming paths are used. Infinite distances are
// not considered.
func Closeness(g graph.Graph, p path.AllShortest) map[int64]float64 {
	nodes := graph.NodesOf(g.Nodes())
	c := make(map[int64]float64, len(nodes))
	for _, u := range nodes {
		uid := u.ID()
		var sum float64
		for _, v := range nodes {
			vid := v.ID()
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(vid, uid)
			if math.IsInf(d, 0) {
				continue
			}
			sum += d
		}
		c[u.ID()] = 1 / sum
	}
	return c
}

// Farness returns the farness for nodes in the graph g used to construct
// the given shortest paths.
//
//  F(v) = \sum_u d(u,v)
//
// For directed graphs the incoming paths are used. Infinite distances are
// not considered.
func Farness(g graph.Graph, p path.AllShortest) map[int64]float64 {
	nodes := graph.NodesOf(g.Nodes())
	f := make(map[int64]float64, len(nodes))
	for _, u := range nodes {
		uid := u.ID()
		var sum float64
		for _, v := range nodes {
			vid := v.ID()
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(vid, uid)
			if math.IsInf(d, 0) {
				continue
			}
			sum += d
		}
		f[u.ID()] = sum
	}
	return f
}

// Harmonic returns the harmonic centrality for nodes in the graph g used to
// construct the given shortest paths.
//
//  H(v)= \sum_{u ≠ v} 1 / d(u,v)
//
// For directed graphs the incoming paths are used. Infinite distances are
// not considered.
func Harmonic(g graph.Graph, p path.AllShortest) map[int64]float64 {
	nodes := graph.NodesOf(g.Nodes())
	h := make(map[int64]float64, len(nodes))
	for i, u := range nodes {
		uid := u.ID()
		var sum float64
		for j, v := range nodes {
			vid := v.ID()
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(vid, uid)
			if math.IsInf(d, 0) {
				continue
			}
			if i != j {
				sum += 1 / d
			}
		}
		h[u.ID()] = sum
	}
	return h
}

// Residual returns the Dangalchev's residual closeness for nodes in the graph
// g used to construct the given shortest paths.
//
//  C(v)= \sum_{u ≠ v} 1 / 2^d(u,v)
//
// For directed graphs the incoming paths are used. Infinite distances are
// not considered.
func Residual(g graph.Graph, p path.AllShortest) map[int64]float64 {
	nodes := graph.NodesOf(g.Nodes())
	r := make(map[int64]float64, len(nodes))
	for i, u := range nodes {
		uid := u.ID()
		var sum float64
		for j, v := range nodes {
			vid := v.ID()
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(vid, uid)
			if math.IsInf(d, 0) {
				continue
			}
			if i != j {
				sum += math.Exp2(-d)
			}
		}
		r[u.ID()] = sum
	}
	return r
}
