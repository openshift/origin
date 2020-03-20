// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/linear"
)

// BellmanFordFrom returns a shortest-path tree for a shortest path from u to all nodes in
// the graph g, or false indicating that a negative cycle exists in the graph. If the graph
// does not implement Weighted, UniformCost is used.
//
// The time complexity of BellmanFordFrom is O(|V|.|E|).
func BellmanFordFrom(u graph.Node, g graph.Graph) (path Shortest, ok bool) {
	if g.Node(u.ID()) == nil {
		return Shortest{from: u}, true
	}
	var weight Weighting
	if wg, ok := g.(Weighted); ok {
		weight = wg.Weight
	} else {
		weight = UniformCost(g)
	}

	nodes := graph.NodesOf(g.Nodes())

	path = newShortestFrom(u, nodes)
	path.dist[path.indexOf[u.ID()]] = 0

	// Queue to keep track which nodes need to be relaxed.
	// Only nodes whose vertex distance changed in the previous iterations need to be relaxed again.
	queue := newBellmanFordQueue(path.indexOf)
	queue.enqueue(u)

	// The maximum of edges in a graph is |V| * (|V|-1) which is also the worst case complexity.
	// If the queue-loop has more iterations than the amount of maximum edges
	// it indicates that we have a negative cycle.
	maxEdges := len(nodes) * (len(nodes) - 1)
	var loops int

	// TODO(kortschak): Consider adding further optimisations
	// from http://arxiv.org/abs/1111.5414.
	for queue.len() != 0 {
		u := queue.dequeue()
		uid := u.ID()
		j := path.indexOf[uid]

		for _, v := range graph.NodesOf(g.From(uid)) {
			vid := v.ID()
			k := path.indexOf[vid]
			w, ok := weight(uid, vid)
			if !ok {
				panic("bellman-ford: unexpected invalid weight")
			}

			joint := path.dist[j] + w
			if joint < path.dist[k] {
				path.set(k, joint, j)

				if !queue.has(vid) {
					queue.enqueue(v)
				}
			}
		}

		if loops > maxEdges {
			path.hasNegativeCycle = true
			return path, false
		}
		loops++
	}

	return path, true
}

// bellmanFordQueue is a queue for the Queue-based Bellman-Ford algorithm.
type bellmanFordQueue struct {
	// queue holds the nodes which need to be relaxed.
	queue linear.NodeQueue

	// onQueue keeps track whether a node is on the queue or not.
	onQueue []bool

	// indexOf contains a mapping holding the id of a node with its index in the onQueue array.
	indexOf map[int64]int
}

// enqueue adds a node to the bellmanFordQueue.
func (q *bellmanFordQueue) enqueue(n graph.Node) {
	i := q.indexOf[n.ID()]
	if q.onQueue[i] {
		panic("bellman-ford: already queued")
	}
	q.onQueue[i] = true
	q.queue.Enqueue(n)
}

// dequeue returns the first value of the bellmanFordQueue.
func (q *bellmanFordQueue) dequeue() graph.Node {
	n := q.queue.Dequeue()
	q.onQueue[q.indexOf[n.ID()]] = false
	return n
}

// len returns the number of nodes in the bellmanFordQueue.
func (q *bellmanFordQueue) len() int { return q.queue.Len() }

// has returns whether a node with the given id is in the queue.
func (q bellmanFordQueue) has(id int64) bool { return q.onQueue[q.indexOf[id]] }

// newBellmanFordQueue creates a new bellmanFordQueue.
func newBellmanFordQueue(indexOf map[int64]int) bellmanFordQueue {
	return bellmanFordQueue{
		onQueue: make([]bool, len(indexOf)),
		indexOf: indexOf,
	}
}
