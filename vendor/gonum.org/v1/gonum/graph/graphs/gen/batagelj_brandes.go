// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The functions in this file are random graph generators from the paper
// by Batagelj and Brandes http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf

package gen

import (
	"fmt"
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// Gnp constructs a Gilbert’s model graph in the destination, dst, of order n. Edges
// between nodes are formed with the probability, p. If src is not nil it is used
// as the random source, otherwise rand.Float64 is used. The graph is constructed
// in O(n+m) time where m is the number of edges added.
func Gnp(dst GraphBuilder, n int, p float64, src rand.Source) error {
	if p == 0 {
		return nil
	}
	if p < 0 || p > 1 {
		return fmt.Errorf("gen: bad probability: p=%v", p)
	}
	var r func() float64
	if src == nil {
		r = rand.Float64
	} else {
		r = rand.New(src).Float64
	}

	for i := 0; i < n; i++ {
		if !dst.Has(int64(i)) {
			dst.AddNode(simple.Node(i))
		}
	}

	lp := math.Log(1 - p)

	// Add forward edges for all graphs.
	for v, w := 1, -1; v < n; {
		w += 1 + int(math.Log(1-r())/lp)
		for w >= v && v < n {
			w -= v
			v++
		}
		if v < n {
			dst.SetEdge(simple.Edge{F: simple.Node(w), T: simple.Node(v)})
		}
	}

	// Add backward edges for directed graphs.
	if _, ok := dst.(graph.Directed); !ok {
		return nil
	}
	for v, w := 1, -1; v < n; {
		w += 1 + int(math.Log(1-r())/lp)
		for w >= v && v < n {
			w -= v
			v++
		}
		if v < n {
			dst.SetEdge(simple.Edge{F: simple.Node(v), T: simple.Node(w)})
		}
	}

	return nil
}

// edgeNodesFor returns the pair of nodes for the ith edge in a simple
// undirected graph. The pair is returned such that w.ID < v.ID.
func edgeNodesFor(i int) (v, w simple.Node) {
	// This is an algebraic simplification of the expressions described
	// on p3 of http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
	v = simple.Node(0.5 + math.Sqrt(float64(1+8*i))/2)
	w = simple.Node(i) - v*(v-1)/2
	return v, w
}

// Gnm constructs a Erdős-Rényi model graph in the destination, dst, of
// order n and size m. If src is not nil it is used as the random source,
// otherwise rand.Intn is used. The graph is constructed in O(m) expected
// time for m ≤ (n choose 2)/2.
func Gnm(dst GraphBuilder, n, m int, src rand.Source) error {
	if m == 0 {
		return nil
	}

	hasEdge := dst.HasEdgeBetween
	d, isDirected := dst.(graph.Directed)
	if isDirected {
		m /= 2
		hasEdge = d.HasEdgeFromTo
	}

	nChoose2 := (n - 1) * n / 2
	if m < 0 || m > nChoose2 {
		return fmt.Errorf("gen: bad size: m=%d", m)
	}

	var rnd func(int) int
	if src == nil {
		rnd = rand.Intn
	} else {
		rnd = rand.New(src).Intn
	}

	for i := 0; i < n; i++ {
		if !dst.Has(int64(i)) {
			dst.AddNode(simple.Node(i))
		}
	}

	// Add forward edges for all graphs.
	for i := 0; i < m; i++ {
		for {
			v, w := edgeNodesFor(rnd(nChoose2))
			e := simple.Edge{F: w, T: v}
			if !hasEdge(e.F.ID(), e.T.ID()) {
				dst.SetEdge(e)
				break
			}
		}
	}

	// Add backward edges for directed graphs.
	if !isDirected {
		return nil
	}
	for i := 0; i < m; i++ {
		for {
			v, w := edgeNodesFor(rnd(nChoose2))
			e := simple.Edge{F: v, T: w}
			if !hasEdge(e.F.ID(), e.T.ID()) {
				dst.SetEdge(e)
				break
			}
		}
	}

	return nil
}

// SmallWorldsBB constructs a small worlds graph of order n in the destination, dst.
// Node degree is specified by d and edge replacement by the probability, p.
// If src is not nil it is used as the random source, otherwise rand.Float64 is used.
// The graph is constructed in O(nd) time.
//
// The algorithm used is described in http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
func SmallWorldsBB(dst GraphBuilder, n, d int, p float64, src rand.Source) error {
	if d < 1 || d > (n-1)/2 {
		return fmt.Errorf("gen: bad degree: d=%d", d)
	}
	if p == 0 {
		return nil
	}
	if p < 0 || p >= 1 {
		return fmt.Errorf("gen: bad replacement: p=%v", p)
	}
	var (
		rnd  func() float64
		rndN func(int) int
	)
	if src == nil {
		rnd = rand.Float64
		rndN = rand.Intn
	} else {
		r := rand.New(src)
		rnd = r.Float64
		rndN = r.Intn
	}

	hasEdge := dst.HasEdgeBetween
	dg, isDirected := dst.(graph.Directed)
	if isDirected {
		hasEdge = dg.HasEdgeFromTo
	}

	for i := 0; i < n; i++ {
		if !dst.Has(int64(i)) {
			dst.AddNode(simple.Node(i))
		}
	}

	nChoose2 := (n - 1) * n / 2

	lp := math.Log(1 - p)

	// Add forward edges for all graphs.
	k := int(math.Log(1-rnd()) / lp)
	m := 0
	replace := make(map[int]int)
	for v := 0; v < n; v++ {
		for i := 1; i <= d; i++ {
			if k > 0 {
				j := v*(v-1)/2 + (v+i)%n
				var ej simple.Edge
				ej.T, ej.F = edgeNodesFor(j)
				if !hasEdge(ej.From().ID(), ej.To().ID()) {
					dst.SetEdge(ej)
				}
				k--
				m++
				var em simple.Edge
				em.T, em.F = edgeNodesFor(m)
				if !hasEdge(em.From().ID(), em.To().ID()) {
					replace[j] = m
				} else {
					replace[j] = replace[m]
				}
			} else {
				k = int(math.Log(1-rnd()) / lp)
			}
		}
	}
	for i := m + 1; i <= n*d && i < nChoose2; i++ {
		r := rndN(nChoose2-i) + i
		var er simple.Edge
		er.T, er.F = edgeNodesFor(r)
		if !hasEdge(er.From().ID(), er.To().ID()) {
			dst.SetEdge(er)
		} else {
			er.T, er.F = edgeNodesFor(replace[r])
			if !hasEdge(er.From().ID(), er.To().ID()) {
				dst.SetEdge(er)
			}
		}
		var ei simple.Edge
		ei.T, ei.F = edgeNodesFor(i)
		if !hasEdge(ei.From().ID(), ei.To().ID()) {
			replace[r] = i
		} else {
			replace[r] = replace[i]
		}
	}

	// Add backward edges for directed graphs.
	if !isDirected {
		return nil
	}
	k = int(math.Log(1-rnd()) / lp)
	m = 0
	replace = make(map[int]int)
	for v := 0; v < n; v++ {
		for i := 1; i <= d; i++ {
			if k > 0 {
				j := v*(v-1)/2 + (v+i)%n
				var ej simple.Edge
				ej.F, ej.T = edgeNodesFor(j)
				if !hasEdge(ej.From().ID(), ej.To().ID()) {
					dst.SetEdge(ej)
				}
				k--
				m++
				if u, v := edgeNodesFor(m); !hasEdge(u.ID(), v.ID()) {
					replace[j] = m
				} else {
					replace[j] = replace[m]
				}
			} else {
				k = int(math.Log(1-rnd()) / lp)
			}
		}
	}
	for i := m + 1; i <= n*d && i < nChoose2; i++ {
		r := rndN(nChoose2-i) + i
		var er simple.Edge
		er.F, er.T = edgeNodesFor(r)
		if !hasEdge(er.From().ID(), er.To().ID()) {
			dst.SetEdge(er)
		} else {
			er.F, er.T = edgeNodesFor(replace[r])
			if !hasEdge(er.From().ID(), er.To().ID()) {
				dst.SetEdge(er)
			}
		}
		if u, v := edgeNodesFor(i); !hasEdge(u.ID(), v.ID()) {
			replace[r] = i
		} else {
			replace[r] = replace[i]
		}
	}

	return nil
}

// Multigraph generators.

// PowerLaw constructs a power-law degree graph by preferential attachment in dst
// with n nodes and minimum degree d. PowerLaw does not consider nodes in dst prior
// to the call. If src is not nil it is used as the random source, otherwise rand.Intn
// is used.
// The graph is constructed in O(nd) — O(n+m) — time.
//
// The algorithm used is described in http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
func PowerLaw(dst graph.MultigraphBuilder, n, d int, src rand.Source) error {
	if d < 1 {
		return fmt.Errorf("gen: bad minimum degree: d=%d", d)
	}
	var rnd func(int) int
	if src == nil {
		rnd = rand.Intn
	} else {
		rnd = rand.New(src).Intn
	}

	m := make([]graph.Node, 2*n*d)
	for v := 0; v < n; v++ {
		x := dst.NewNode()
		dst.AddNode(x)

		for i := 0; i < d; i++ {
			m[2*(v*d+i)] = x
			m[2*(v*d+i)+1] = m[rnd(2*v*d+i+1)]
		}
	}
	for i := 0; i < n*d; i++ {
		dst.SetLine(dst.NewLine(m[2*i], m[2*i+1]))
	}

	return nil
}

// BipartitePowerLaw constructs a bipartite power-law degree graph by preferential attachment
// in dst with 2×n nodes and minimum degree d. BipartitePowerLaw does not consider nodes in
// dst prior to the call. The two partitions are returned in p1 and p2. If src is not nil it is
// used as the random source, otherwise rand.Intn is used.
// The graph is constructed in O(nd) — O(n+m) — time.
//
// The algorithm used is described in http://algo.uni-konstanz.de/publications/bb-eglrn-05.pdf
func BipartitePowerLaw(dst graph.MultigraphBuilder, n, d int, src rand.Source) (p1, p2 []graph.Node, err error) {
	if d < 1 {
		return nil, nil, fmt.Errorf("gen: bad minimum degree: d=%d", d)
	}
	var rnd func(int) int
	if src == nil {
		rnd = rand.Intn
	} else {
		rnd = rand.New(src).Intn
	}

	p := make([]graph.Node, 2*n)
	for i := range p {
		u := dst.NewNode()
		dst.AddNode(u)
		p[i] = u
	}

	m1 := make([]graph.Node, 2*n*d)
	m2 := make([]graph.Node, 2*n*d)
	for v := 0; v < n; v++ {
		for i := 0; i < d; i++ {
			m1[2*(v*d+i)] = p[v]
			m2[2*(v*d+i)] = p[n+v]

			if r := rnd(2*v*d + i + 1); r&0x1 == 0 {
				m1[2*(v*d+i)+1] = m2[r]
			} else {
				m1[2*(v*d+i)+1] = m1[r]
			}

			if r := rnd(2*v*d + i + 1); r&0x1 == 0 {
				m2[2*(v*d+i)+1] = m1[r]
			} else {
				m2[2*(v*d+i)+1] = m2[r]
			}
		}
	}
	for i := 0; i < n*d; i++ {
		dst.SetLine(dst.NewLine(m1[2*i], m1[2*i+1]))
		dst.SetLine(dst.NewLine(m2[2*i], m2[2*i+1]))
	}
	return p[:n], p[n:], nil
}
