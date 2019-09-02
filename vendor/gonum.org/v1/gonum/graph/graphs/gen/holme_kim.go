// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"errors"
	"fmt"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/stat/sampleuv"
)

// TunableClusteringScaleFree constructs a subgraph in the destination, dst, of order n.
// The graph is constructed successively starting from an m order graph with one node
// having degree m-1. At each iteration of graph addition, one node is added with m
// additional edges joining existing nodes with probability proportional to the nodes'
// degrees. The edges are formed as a triad with probability, p.
// If src is not nil it is used as the random source, otherwise rand.Float64 and
// rand.Intn are used for the random number generators.
//
// The algorithm is essentially as described in http://arxiv.org/abs/cond-mat/0110452.
func TunableClusteringScaleFree(dst graph.UndirectedBuilder, n, m int, p float64, src rand.Source) error {
	if p < 0 || p > 1 {
		return fmt.Errorf("gen: bad probability: p=%v", p)
	}
	if n <= m {
		return fmt.Errorf("gen: n <= m: n=%v m=%d", n, m)
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

	// Initial condition.
	wt := make([]float64, n)
	id := make([]int64, n)
	for u := 0; u < m; u++ {
		un := dst.NewNode()
		dst.AddNode(un)
		id[u] = un.ID()
		// We need to give equal probability for
		// adding the first generation of edges.
		wt[u] = 1
	}
	ws := sampleuv.NewWeighted(wt, src)
	for i := range wt {
		// These weights will organically grow
		// after the first growth iteration.
		wt[i] = 0
	}

	// Growth.
	for v := m; v < n; v++ {
		vn := dst.NewNode()
		dst.AddNode(vn)
		id[v] = vn.ID()
		var u int
	pa:
		for i := 0; i < m; i++ {
			// Triad formation.
			if i != 0 && rnd() < p {
				// TODO(kortschak): Decide whether the node
				// order in this input to permute should be
				// sorted first to allow repeatable runs.
				for _, w := range permute(graph.NodesOf(dst.From(id[u])), rndN) {
					wid := w.ID()
					if wid == id[v] || dst.HasEdgeBetween(wid, id[v]) {
						continue
					}

					dst.SetEdge(dst.NewEdge(w, vn))
					wt[wid]++
					wt[v]++
					continue pa
				}
			}

			// Preferential attachment.
			for {
				var ok bool
				u, ok = ws.Take()
				if !ok {
					return errors.New("gen: depleted distribution")
				}
				if u == v || dst.HasEdgeBetween(id[u], id[v]) {
					continue
				}
				dst.SetEdge(dst.NewEdge(dst.Node(id[u]), vn))
				wt[u]++
				wt[v]++
				break
			}
		}

		ws.ReweightAll(wt)
	}

	return nil
}

func permute(n []graph.Node, rnd func(int) int) []graph.Node {
	for i := range n[:len(n)-1] {
		j := rnd(len(n)-i) + i
		n[i], n[j] = n[j], n[i]
	}
	return n
}

// PreferentialAttachment constructs a graph in the destination, dst, of order n.
// The graph is constructed successively starting from an m order graph with one
// node having degree m-1. At each iteration of graph addition, one node is added
// with m additional edges joining existing nodes with probability proportional
// to the nodes' degrees. If src is not nil it is used as the random source,
// otherwise rand.Float64 is used for the random number generator.
//
// The algorithm is essentially as described in http://arxiv.org/abs/cond-mat/0110452
// after 10.1126/science.286.5439.509.
func PreferentialAttachment(dst graph.UndirectedBuilder, n, m int, src rand.Source) error {
	if n <= m {
		return fmt.Errorf("gen: n <= m: n=%v m=%d", n, m)
	}

	// Initial condition.
	wt := make([]float64, n)
	for u := 0; u < m; u++ {
		if dst.Node(int64(u)) == nil {
			dst.AddNode(simple.Node(u))
		}
		// We need to give equal probability for
		// adding the first generation of edges.
		wt[u] = 1
	}
	ws := sampleuv.NewWeighted(wt, src)
	for i := range wt {
		// These weights will organically grow
		// after the first growth iteration.
		wt[i] = 0
	}

	// Growth.
	for v := m; v < n; v++ {
		for i := 0; i < m; i++ {
			// Preferential attachment.
			u, ok := ws.Take()
			if !ok {
				return errors.New("gen: depleted distribution")
			}
			dst.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			wt[u]++
			wt[v]++
		}
		ws.ReweightAll(wt)
	}

	return nil
}
