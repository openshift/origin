// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"github.com/gonum/graph"
	"github.com/gonum/graph/path"
)

// Closeness returns the closeness centrality for nodes in the graph g used to
// construct the given shortest paths.
//
//  C(v) = 1 / \sum_u d(u,v)
//
// For directed graphs the incoming paths are used. Infinite distances are
// not considered.
func Closeness(g graph.Graph, p path.AllShortest) map[int]float64 {
	nodes := g.Nodes()
	c := make(map[int]float64, len(nodes))
	for _, u := range nodes {
		var sum float64
		for _, v := range nodes {
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(v, u)
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
func Farness(g graph.Graph, p path.AllShortest) map[int]float64 {
	nodes := g.Nodes()
	f := make(map[int]float64, len(nodes))
	for _, u := range nodes {
		var sum float64
		for _, v := range nodes {
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(v, u)
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
func Harmonic(g graph.Graph, p path.AllShortest) map[int]float64 {
	nodes := g.Nodes()
	h := make(map[int]float64, len(nodes))
	for i, u := range nodes {
		var sum float64
		for j, v := range nodes {
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(v, u)
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
func Residual(g graph.Graph, p path.AllShortest) map[int]float64 {
	nodes := g.Nodes()
	r := make(map[int]float64, len(nodes))
	for i, u := range nodes {
		var sum float64
		for j, v := range nodes {
			// The ordering here is not relevant for
			// undirected graphs, but we make sure we
			// are counting incoming paths.
			d := p.Weight(v, u)
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
