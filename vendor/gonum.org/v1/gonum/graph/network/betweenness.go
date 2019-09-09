// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/linear"
	"gonum.org/v1/gonum/graph/path"
)

// Betweenness returns the non-zero betweenness centrality for nodes in the unweighted graph g.
//
//  C_B(v) = \sum_{s ≠ v ≠ t ∈ V} (\sigma_{st}(v) / \sigma_{st})
//
// where \sigma_{st} and \sigma_{st}(v) are the number of shortest paths from s to t,
// and the subset of those paths containing v respectively.
func Betweenness(g graph.Graph) map[int64]float64 {
	// Brandes' algorithm for finding betweenness centrality for nodes in
	// and unweighted graph:
	//
	// http://www.inf.uni-konstanz.de/algo/publications/b-fabc-01.pdf

	// TODO(kortschak): Consider using the parallel algorithm when
	// GOMAXPROCS != 1.
	//
	// http://htor.inf.ethz.ch/publications/img/edmonds-hoefler-lumsdaine-bc.pdf

	// Also note special case for sparse networks:
	// http://wwwold.iit.cnr.it/staff/marco.pellegrini/papiri/asonam-final.pdf

	cb := make(map[int64]float64)
	brandes(g, func(s graph.Node, stack linear.NodeStack, p map[int64][]graph.Node, delta, sigma map[int64]float64) {
		for stack.Len() != 0 {
			w := stack.Pop()
			for _, v := range p[w.ID()] {
				delta[v.ID()] += sigma[v.ID()] / sigma[w.ID()] * (1 + delta[w.ID()])
			}
			if w.ID() != s.ID() {
				if d := delta[w.ID()]; d != 0 {
					cb[w.ID()] += d
				}
			}
		}
	})
	return cb
}

// EdgeBetweenness returns the non-zero betweenness centrality for edges in the
// unweighted graph g. For an edge e the centrality C_B is computed as
//
//  C_B(e) = \sum_{s ≠ t ∈ V} (\sigma_{st}(e) / \sigma_{st}),
//
// where \sigma_{st} and \sigma_{st}(e) are the number of shortest paths from s
// to t, and the subset of those paths containing e, respectively.
//
// If g is undirected, edges are retained such that u.ID < v.ID where u and v are
// the nodes of e.
func EdgeBetweenness(g graph.Graph) map[[2]int64]float64 {
	// Modified from Brandes' original algorithm as described in Algorithm 7
	// with the exception that node betweenness is not calculated:
	//
	// http://algo.uni-konstanz.de/publications/b-vspbc-08.pdf

	_, isUndirected := g.(graph.Undirected)
	cb := make(map[[2]int64]float64)
	brandes(g, func(s graph.Node, stack linear.NodeStack, p map[int64][]graph.Node, delta, sigma map[int64]float64) {
		for stack.Len() != 0 {
			w := stack.Pop()
			for _, v := range p[w.ID()] {
				c := sigma[v.ID()] / sigma[w.ID()] * (1 + delta[w.ID()])
				vid := v.ID()
				wid := w.ID()
				if isUndirected && wid < vid {
					vid, wid = wid, vid
				}
				cb[[2]int64{vid, wid}] += c
				delta[v.ID()] += c
			}
		}
	})
	return cb
}

// brandes is the common code for Betweenness and EdgeBetweenness. It corresponds
// to algorithm 1 in http://algo.uni-konstanz.de/publications/b-vspbc-08.pdf with
// the accumulation loop provided by the accumulate closure.
func brandes(g graph.Graph, accumulate func(s graph.Node, stack linear.NodeStack, p map[int64][]graph.Node, delta, sigma map[int64]float64)) {
	var (
		nodes = graph.NodesOf(g.Nodes())
		stack linear.NodeStack
		p     = make(map[int64][]graph.Node, len(nodes))
		sigma = make(map[int64]float64, len(nodes))
		d     = make(map[int64]int, len(nodes))
		delta = make(map[int64]float64, len(nodes))
		queue linear.NodeQueue
	)
	for _, s := range nodes {
		stack = stack[:0]

		for _, w := range nodes {
			p[w.ID()] = p[w.ID()][:0]
		}

		for _, t := range nodes {
			sigma[t.ID()] = 0
			d[t.ID()] = -1
		}
		sigma[s.ID()] = 1
		d[s.ID()] = 0

		queue.Enqueue(s)
		for queue.Len() != 0 {
			v := queue.Dequeue()
			vid := v.ID()
			stack.Push(v)
			for _, w := range graph.NodesOf(g.From(vid)) {
				wid := w.ID()
				// w found for the first time?
				if d[wid] < 0 {
					queue.Enqueue(w)
					d[wid] = d[vid] + 1
				}
				// shortest path to w via v?
				if d[wid] == d[vid]+1 {
					sigma[wid] += sigma[vid]
					p[wid] = append(p[wid], v)
				}
			}
		}

		for _, v := range nodes {
			delta[v.ID()] = 0
		}

		// S returns vertices in order of non-increasing distance from s
		accumulate(s, stack, p, delta, sigma)
	}
}

// BetweennessWeighted returns the non-zero betweenness centrality for nodes in the weighted
// graph g used to construct the given shortest paths.
//
//  C_B(v) = \sum_{s ≠ v ≠ t ∈ V} (\sigma_{st}(v) / \sigma_{st})
//
// where \sigma_{st} and \sigma_{st}(v) are the number of shortest paths from s to t,
// and the subset of those paths containing v respectively.
func BetweennessWeighted(g graph.Weighted, p path.AllShortest) map[int64]float64 {
	cb := make(map[int64]float64)

	nodes := graph.NodesOf(g.Nodes())
	for i, s := range nodes {
		sid := s.ID()
		for j, t := range nodes {
			if i == j {
				continue
			}
			tid := t.ID()
			d := p.Weight(sid, tid)
			if math.IsInf(d, 0) {
				continue
			}

			// If we have a unique path, don't do the
			// extra work needed to get all paths.
			path, _, unique := p.Between(sid, tid)
			if unique {
				for _, v := range path[1 : len(path)-1] {
					// For undirected graphs we double count
					// passage though nodes. This is consistent
					// with Brandes' algorithm's behaviour.
					cb[v.ID()]++
				}
				continue
			}

			// Otherwise iterate over all paths.
			paths, _ := p.AllBetween(sid, tid)
			stFrac := 1 / float64(len(paths))
			for _, path := range paths {
				for _, v := range path[1 : len(path)-1] {
					cb[v.ID()] += stFrac
				}
			}
		}
	}

	return cb
}

// EdgeBetweennessWeighted returns the non-zero betweenness centrality for edges in
// the weighted graph g. For an edge e the centrality C_B is computed as
//
//  C_B(e) = \sum_{s ≠ t ∈ V} (\sigma_{st}(e) / \sigma_{st}),
//
// where \sigma_{st} and \sigma_{st}(e) are the number of shortest paths from s
// to t, and the subset of those paths containing e, respectively.
//
// If g is undirected, edges are retained such that u.ID < v.ID where u and v are
// the nodes of e.
func EdgeBetweennessWeighted(g graph.Weighted, p path.AllShortest) map[[2]int64]float64 {
	cb := make(map[[2]int64]float64)

	_, isUndirected := g.(graph.Undirected)
	nodes := graph.NodesOf(g.Nodes())
	for i, s := range nodes {
		sid := s.ID()
		for j, t := range nodes {
			if i == j {
				continue
			}
			tid := t.ID()
			d := p.Weight(sid, tid)
			if math.IsInf(d, 0) {
				continue
			}

			// If we have a unique path, don't do the
			// extra work needed to get all paths.
			path, _, unique := p.Between(sid, tid)
			if unique {
				for k, v := range path[1:] {
					// For undirected graphs we double count
					// passage though edges. This is consistent
					// with Brandes' algorithm's behaviour.
					uid := path[k].ID()
					vid := v.ID()
					if isUndirected && vid < uid {
						uid, vid = vid, uid
					}
					cb[[2]int64{uid, vid}]++
				}
				continue
			}

			// Otherwise iterate over all paths.
			paths, _ := p.AllBetween(sid, tid)
			stFrac := 1 / float64(len(paths))
			for _, path := range paths {
				for k, v := range path[1:] {
					uid := path[k].ID()
					vid := v.ID()
					if isUndirected && vid < uid {
						uid, vid = vid, uid
					}
					cb[[2]int64{uid, vid}] += stFrac
				}
			}
		}
	}

	return cb
}
