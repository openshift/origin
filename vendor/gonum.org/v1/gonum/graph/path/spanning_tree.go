// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"container/heap"
	"math"
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

// WeightedBuilder is a type that can add nodes and weighted edges.
type WeightedBuilder interface {
	AddNode(graph.Node)
	SetWeightedEdge(graph.WeightedEdge)
}

// Prim generates a minimum spanning tree of g by greedy tree extension, placing
// the result in the destination, dst. If the edge weights of g are distinct
// it will be the unique minimum spanning tree of g. The destination is not cleared
// first. The weight of the minimum spanning tree is returned. If g is not connected,
// a minimum spanning forest will be constructed in dst and the sum of minimum
// spanning tree weights will be returned.
//
// Nodes and Edges from g are used to construct dst, so if the Node and Edge
// types used in g are pointer or reference-like, then the values will be shared
// between the graphs.
//
// If dst has nodes that exist in g, Prim will panic.
func Prim(dst WeightedBuilder, g graph.WeightedUndirected) float64 {
	nodes := graph.NodesOf(g.Nodes())
	if len(nodes) == 0 {
		return 0
	}

	q := &primQueue{
		indexOf: make(map[int64]int, len(nodes)-1),
		nodes:   make([]simple.WeightedEdge, 0, len(nodes)-1),
	}
	dst.AddNode(nodes[0])
	for _, u := range nodes[1:] {
		dst.AddNode(u)
		heap.Push(q, simple.WeightedEdge{F: u, W: math.Inf(1)})
	}

	u := nodes[0]
	uid := u.ID()
	for _, v := range graph.NodesOf(g.From(uid)) {
		w, ok := g.Weight(uid, v.ID())
		if !ok {
			panic("prim: unexpected invalid weight")
		}
		q.update(v, u, w)
	}

	var w float64
	for q.Len() > 0 {
		e := heap.Pop(q).(simple.WeightedEdge)
		if e.To() != nil && g.HasEdgeBetween(e.From().ID(), e.To().ID()) {
			dst.SetWeightedEdge(g.WeightedEdge(e.From().ID(), e.To().ID()))
			w += e.Weight()
		}

		u = e.From()
		uid := u.ID()
		for _, n := range graph.NodesOf(g.From(uid)) {
			if key, ok := q.key(n); ok {
				w, ok := g.Weight(uid, n.ID())
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
	indexOf map[int64]int
	nodes   []simple.WeightedEdge
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
	n := x.(simple.WeightedEdge)
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
	graph.WeightedUndirected
	WeightedEdges() graph.WeightedEdges
}

// Kruskal generates a minimum spanning tree of g by greedy tree coalescence, placing
// the result in the destination, dst. If the edge weights of g are distinct
// it will be the unique minimum spanning tree of g. The destination is not cleared
// first. The weight of the minimum spanning tree is returned. If g is not connected,
// a minimum spanning forest will be constructed in dst and the sum of minimum
// spanning tree weights will be returned.
//
// Nodes and Edges from g are used to construct dst, so if the Node and Edge
// types used in g are pointer or reference-like, then the values will be shared
// between the graphs.
//
// If dst has nodes that exist in g, Kruskal will panic.
func Kruskal(dst WeightedBuilder, g UndirectedWeightLister) float64 {
	edges := graph.WeightedEdgesOf(g.WeightedEdges())
	sort.Sort(byWeight(edges))

	ds := newDisjointSet()
	for _, node := range graph.NodesOf(g.Nodes()) {
		dst.AddNode(node)
		ds.makeSet(node.ID())
	}

	var w float64
	for _, e := range edges {
		if s1, s2 := ds.find(e.From().ID()), ds.find(e.To().ID()); s1 != s2 {
			ds.union(s1, s2)
			dst.SetWeightedEdge(g.WeightedEdge(e.From().ID(), e.To().ID()))
			w += e.Weight()
		}
	}
	return w
}

type byWeight []graph.WeightedEdge

func (e byWeight) Len() int           { return len(e) }
func (e byWeight) Less(i, j int) bool { return e[i].Weight() < e[j].Weight() }
func (e byWeight) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
