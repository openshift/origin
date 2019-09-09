// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"container/heap"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
)

// AStar finds the A*-shortest path from s to t in g using the heuristic h. The path and
// its cost are returned in a Shortest along with paths and costs to all nodes explored
// during the search. The number of expanded nodes is also returned. This value may help
// with heuristic tuning.
//
// The path will be the shortest path if the heuristic is admissible. A heuristic is
// admissible if for any node, n, in the graph, the heuristic estimate of the cost of
// the path from n to t is less than or equal to the true cost of that path.
//
// If h is nil, AStar will use the g.HeuristicCost method if g implements HeuristicCoster,
// falling back to NullHeuristic otherwise. If the graph does not implement Weighted,
// UniformCost is used. AStar will panic if g has an A*-reachable negative edge weight.
func AStar(s, t graph.Node, g graph.Graph, h Heuristic) (path Shortest, expanded int) {
	if g.Node(s.ID()) == nil || g.Node(t.ID()) == nil {
		return Shortest{from: s}, 0
	}
	var weight Weighting
	if wg, ok := g.(Weighted); ok {
		weight = wg.Weight
	} else {
		weight = UniformCost(g)
	}
	if h == nil {
		if g, ok := g.(HeuristicCoster); ok {
			h = g.HeuristicCost
		} else {
			h = NullHeuristic
		}
	}

	path = newShortestFrom(s, graph.NodesOf(g.Nodes()))
	tid := t.ID()

	visited := make(set.Int64s)
	open := &aStarQueue{indexOf: make(map[int64]int)}
	heap.Push(open, aStarNode{node: s, gscore: 0, fscore: h(s, t)})

	for open.Len() != 0 {
		u := heap.Pop(open).(aStarNode)
		uid := u.node.ID()
		i := path.indexOf[uid]
		expanded++

		if uid == tid {
			break
		}

		visited.Add(uid)
		for _, v := range graph.NodesOf(g.From(u.node.ID())) {
			vid := v.ID()
			if visited.Has(vid) {
				continue
			}
			j := path.indexOf[vid]

			w, ok := weight(u.node.ID(), vid)
			if !ok {
				panic("A*: unexpected invalid weight")
			}
			if w < 0 {
				panic("A*: negative edge weight")
			}
			g := u.gscore + w
			if n, ok := open.node(vid); !ok {
				path.set(j, g, i)
				heap.Push(open, aStarNode{node: v, gscore: g, fscore: g + h(v, t)})
			} else if g < n.gscore {
				path.set(j, g, i)
				open.update(vid, g, g+h(v, t))
			}
		}
	}

	return path, expanded
}

// NullHeuristic is an admissible, consistent heuristic that will not speed up computation.
func NullHeuristic(_, _ graph.Node) float64 {
	return 0
}

// aStarNode adds A* accounting to a graph.Node.
type aStarNode struct {
	node   graph.Node
	gscore float64
	fscore float64
}

// aStarQueue is an A* priority queue.
type aStarQueue struct {
	indexOf map[int64]int
	nodes   []aStarNode
}

func (q *aStarQueue) Less(i, j int) bool {
	return q.nodes[i].fscore < q.nodes[j].fscore
}

func (q *aStarQueue) Swap(i, j int) {
	q.indexOf[q.nodes[i].node.ID()] = j
	q.indexOf[q.nodes[j].node.ID()] = i
	q.nodes[i], q.nodes[j] = q.nodes[j], q.nodes[i]
}

func (q *aStarQueue) Len() int {
	return len(q.nodes)
}

func (q *aStarQueue) Push(x interface{}) {
	n := x.(aStarNode)
	q.indexOf[n.node.ID()] = len(q.nodes)
	q.nodes = append(q.nodes, n)
}

func (q *aStarQueue) Pop() interface{} {
	n := q.nodes[len(q.nodes)-1]
	q.nodes = q.nodes[:len(q.nodes)-1]
	delete(q.indexOf, n.node.ID())
	return n
}

func (q *aStarQueue) update(id int64, g, f float64) {
	i, ok := q.indexOf[id]
	if !ok {
		return
	}
	q.nodes[i].gscore = g
	q.nodes[i].fscore = f
	heap.Fix(q, i)
}

func (q *aStarQueue) node(id int64) (aStarNode, bool) {
	loc, ok := q.indexOf[id]
	if ok {
		return q.nodes[loc], true
	}
	return aStarNode{}, false
}
