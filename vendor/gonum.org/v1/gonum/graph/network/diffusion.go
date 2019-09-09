// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/mat"
)

// Diffuse performs a heat diffusion across nodes of the undirected
// graph described by the given Laplacian using the initial heat distribution,
// h, according to the Laplacian with a diffusion time of t.
// The resulting heat distribution is returned, written into the map dst and
// returned,
//  d = exp(-Lt)×h
// where L is the graph Laplacian. Indexing into h and dst is defined by the
// Laplacian Index field. If dst is nil, a new map is created.
//
// Nodes without corresponding entries in h are given an initial heat of zero,
// and entries in h without a corresponding node in the original graph are
// not altered when written to dst.
func Diffuse(dst, h map[int64]float64, by Laplacian, t float64) map[int64]float64 {
	heat := make([]float64, len(by.Index))
	for id, i := range by.Index {
		heat[i] = h[id]
	}
	v := mat.NewVecDense(len(heat), heat)

	var m, tl mat.Dense
	tl.Scale(-t, by)
	m.Exp(&tl)
	v.MulVec(&m, v)

	if dst == nil {
		dst = make(map[int64]float64)
	}
	for i, n := range heat {
		dst[by.Nodes[i].ID()] = n
	}
	return dst
}

// DiffuseToEquilibrium performs a heat diffusion across nodes of the
// graph described by the given Laplacian using the initial heat
// distribution, h, according to the Laplacian until the update function
//  h_{n+1} = h_n - L×h_n
// results in a 2-norm update difference within tol, or iters updates have
// been made.
// The resulting heat distribution is returned as eq, written into the map dst,
// and a boolean indicating whether the equilibrium converged to within tol.
// Indexing into h and dst is defined by the Laplacian Index field. If dst
// is nil, a new map is created.
//
// Nodes without corresponding entries in h are given an initial heat of zero,
// and entries in h without a corresponding node in the original graph are
// not altered when written to dst.
func DiffuseToEquilibrium(dst, h map[int64]float64, by Laplacian, tol float64, iters int) (eq map[int64]float64, ok bool) {
	heat := make([]float64, len(by.Index))
	for id, i := range by.Index {
		heat[i] = h[id]
	}
	v := mat.NewVecDense(len(heat), heat)

	last := make([]float64, len(by.Index))
	for id, i := range by.Index {
		last[i] = h[id]
	}
	lastV := mat.NewVecDense(len(last), last)

	var tmp mat.VecDense
	for {
		iters--
		if iters < 0 {
			break
		}
		lastV, v = v, lastV
		tmp.MulVec(by.Matrix, lastV)
		v.SubVec(lastV, &tmp)
		if normDiff(heat, last) < tol {
			ok = true
			break
		}
	}

	if dst == nil {
		dst = make(map[int64]float64)
	}
	for i, n := range v.RawVector().Data {
		dst[by.Nodes[i].ID()] = n
	}
	return dst, ok
}

// Laplacian is a graph Laplacian matrix.
type Laplacian struct {
	// Matrix holds the Laplacian matrix.
	mat.Matrix

	// Nodes holds the input graph nodes.
	Nodes []graph.Node

	// Index is a mapping from the graph
	// node IDs to row and column indices.
	Index map[int64]int
}

// NewLaplacian returns a Laplacian matrix for the simple undirected graph g.
// The Laplacian is defined as D-A where D is a diagonal matrix holding the
// degree of each node and A is the graph adjacency matrix of the input graph.
// If g contains self edges, NewLaplacian will panic.
func NewLaplacian(g graph.Undirected) Laplacian {
	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		id := n.ID()
		indexOf[id] = i
	}

	l := mat.NewSymDense(len(nodes), nil)
	for j, u := range nodes {
		uid := u.ID()
		to := graph.NodesOf(g.From(uid))
		l.SetSym(j, j, float64(len(to)))
		for _, v := range to {
			vid := v.ID()
			if uid == vid {
				panic("network: self edge in graph")
			}
			if uid < vid {
				l.SetSym(indexOf[vid], j, -1)
			}
		}
	}

	return Laplacian{Matrix: l, Nodes: nodes, Index: indexOf}
}

// NewSymNormLaplacian returns a symmetric normalized Laplacian matrix for the
// simple undirected graph g.
// The normalized Laplacian is defined as I-D^(-1/2)AD^(-1/2) where D is a
// diagonal matrix holding the degree of each node and A is the graph adjacency
// matrix of the input graph.
// If g contains self edges, NewSymNormLaplacian will panic.
func NewSymNormLaplacian(g graph.Undirected) Laplacian {
	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		id := n.ID()
		indexOf[id] = i
	}

	l := mat.NewSymDense(len(nodes), nil)
	for j, u := range nodes {
		uid := u.ID()
		to := graph.NodesOf(g.From(uid))
		if len(to) == 0 {
			continue
		}
		l.SetSym(j, j, 1)
		squdeg := math.Sqrt(float64(len(to)))
		for _, v := range to {
			vid := v.ID()
			if uid == vid {
				panic("network: self edge in graph")
			}
			if uid < vid {
				l.SetSym(indexOf[vid], j, -1/(squdeg*math.Sqrt(float64(g.From(vid).Len()))))
			}
		}
	}

	return Laplacian{Matrix: l, Nodes: nodes, Index: indexOf}
}

// NewRandomWalkLaplacian returns a damp-scaled random walk Laplacian matrix for
// the simple graph g.
// The random walk Laplacian is defined as I-D^(-1)A where D is a diagonal matrix
// holding the degree of each node and A is the graph adjacency matrix of the input
// graph.
// If g contains self edges, NewRandomWalkLaplacian will panic.
func NewRandomWalkLaplacian(g graph.Graph, damp float64) Laplacian {
	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		id := n.ID()
		indexOf[id] = i
	}

	l := mat.NewDense(len(nodes), len(nodes), nil)
	for j, u := range nodes {
		uid := u.ID()
		to := graph.NodesOf(g.From(uid))
		if len(to) == 0 {
			continue
		}
		l.Set(j, j, 1-damp)
		rudeg := (damp - 1) / float64(len(to))
		for _, v := range to {
			vid := v.ID()
			if uid == vid {
				panic("network: self edge in graph")
			}
			l.Set(indexOf[vid], j, rudeg)
		}
	}

	return Laplacian{Matrix: l, Nodes: nodes, Index: indexOf}
}
