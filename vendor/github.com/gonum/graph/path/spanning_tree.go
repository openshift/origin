// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"container/heap"
	"math"
	"sort"

	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
)

// UndirectedWeighter is an undirected graph that returns edge weights.
type UndirectedWeighter interface {
	graph.Undirected
	graph.Weighter
}

// Prim generates a minimum spanning tree of g by greedy tree extension, placing
// the result in the destination, dst. If the edge weights of g are distinct
// it will be the unique minimum spanning tree of g. The destination is not cleared
// first. The weight of the minimum spanning tree is returned. If g is not connected,
// a minimum spanning forest will be constructed in dst and the sum of minimum
// spanning tree weights will be returned.
func Prim(dst graph.UndirectedBuilder, g UndirectedWeighter) float64 {
	nodes := g.Nodes()
	if len(nodes) == 0 {
		return 0
	}

	q := &primQueue{
		indexOf: make(map[int]int, len(nodes)-1),
		nodes:   make([]simple.Edge, 0, len(nodes)-1),
	}
	for _, u := range nodes[1:] {
		heap.Push(q, simple.Edge{F: u, W: math.Inf(1)})
	}

	u := nodes[0]
	for _, v := range g.From(u) {
		w, ok := g.Weight(u, v)
		if !ok {
			panic("prim: unexpected invalid weight")
		}
		q.update(v, u, w)
	}

	var w float64
	for q.Len() > 0 {
		e := heap.Pop(q).(simple.Edge)
		if e.To() != nil && g.HasEdgeBetween(e.From(), e.To()) {
			dst.SetEdge(e)
			w += e.Weight()
		}

		u = e.From()
		for _, n := range g.From(u) {
			if key, ok := q.key(n); ok {
				w, ok := g.Weight(u, n)
				if !ok {
					panic("prim: unexpected invalid weight")
				}
				if w < key {
					q.update(n, u, w)
				}
			}
		}
	}
	return w
}

// primQueue is a Prim's priority queue. The priority queue is a
// queue of edge From nodes keyed on the minimum edge weight to
// a node in the set of nodes already connected to the minimum
// spanning forest.
type primQueue struct {
	indexOf map[int]int
	nodes   []simple.Edge
}

func (q *primQueue) Less(i, j int) bool {
	return q.nodes[i].Weight() < q.nodes[j].Weight()
}

func (q *primQueue) Swap(i, j int) {
	q.indexOf[q.nodes[i].From().ID()] = j
	q.indexOf[q.nodes[j].From().ID()] = i
	q.nodes[i], q.nodes[j] = q.nodes[j], q.nodes[i]
}

func (q *primQueue) Len() int {
	return len(q.nodes)
}

func (q *primQueue) Push(x interface{}) {
	n := x.(simple.Edge)
	q.indexOf[n.From().ID()] = len(q.nodes)
	q.nodes = append(q.nodes, n)
}

func (q *primQueue) Pop() interface{} {
	n := q.nodes[len(q.nodes)-1]
	q.nodes = q.nodes[:len(q.nodes)-1]
	delete(q.indexOf, n.From().ID())
	return n
}

// key returns the key for the node u and whether the node is
// in the queue. If the node is not in the queue, key is returned
// as +Inf.
func (q *primQueue) key(u graph.Node) (key float64, ok bool) {
	i, ok := q.indexOf[u.ID()]
	if !ok {
		return math.Inf(1), false
	}
	return q.nodes[i].Weight(), ok
}

// update updates u's position in the queue with the new closest
// MST-connected neighbour, v, and the key weight between u and v.
func (q *primQueue) update(u, v graph.Node, key float64) {
	id := u.ID()
	i, ok := q.indexOf[id]
	if !ok {
		return
	}
	q.nodes[i].T = v
	q.nodes[i].W = key
	heap.Fix(q, i)
}

// UndirectedWeightLister is an undirected graph that returns edge weights and
// the set of edges in the graph.
type UndirectedWeightLister interface {
	UndirectedWeighter
	Edges() []graph.Edge
}

// Kruskal generates a minimum spanning tree of g by greedy tree coalescence, placing
// the result in the destination, dst. If the edge weights of g are distinct
// it will be the unique minimum spanning tree of g. The destination is not cleared
// first. The weight of the minimum spanning tree is returned. If g is not connected,
// a minimum spanning forest will be constructed in dst and the sum of minimum
// spanning tree weights will be returned.
func Kruskal(dst graph.UndirectedBuilder, g UndirectedWeightLister) float64 {
	edges := g.Edges()
	ascend := make([]simple.Edge, 0, len(edges))
	for _, e := range edges {
		u := e.From()
		v := e.To()
		w, ok := g.Weight(u, v)
		if !ok {
			panic("kruskal: unexpected invalid weight")
		}
		ascend = append(ascend, simple.Edge{F: u, T: v, W: w})
	}
	sort.Sort(byWeight(ascend))

	ds := newDisjointSet()
	for _, node := range g.Nodes() {
		ds.makeSet(node.ID())
	}

	var w float64
	for _, e := range ascend {
		if s1, s2 := ds.find(e.From().ID()), ds.find(e.To().ID()); s1 != s2 {
			ds.union(s1, s2)
			dst.SetEdge(e)
			w += e.Weight()
		}
	}
	return w
}

type byWeight []simple.Edge

func (e byWeight) Len() int           { return len(e) }
func (e byWeight) Less(i, j int) bool { return e[i].Weight() < e[j].Weight() }
func (e byWeight) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
