// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import "gonum.org/v1/gonum/graph"

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

	// TODO(kortschak): Consider adding further optimisations
	// from http://arxiv.org/abs/1111.5414.
	for i := 1; i < len(nodes); i++ {
		changed := false
		for j, u := range nodes {
			uid := u.ID()
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
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}

	for j, u := range nodes {
		uid := u.ID()
		for _, v := range graph.NodesOf(g.From(uid)) {
			vid := v.ID()
			k := path.indexOf[vid]
			w, ok := weight(uid, vid)
			if !ok {
				panic("bellman-ford: unexpected invalid weight")
			}
			if path.dist[j]+w < path.dist[k] {
				path.hasNegativeCycle = true
				return path, false
			}
		}
	}

	return path, true
}
