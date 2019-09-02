// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"

	"gonum.org/v1/gonum/graph"
)

// FloydWarshall returns a shortest-path tree for the graph g or false indicating
// that a negative cycle exists in the graph. If a negative cycle exists in the graph
// the returned paths will be valid and edge weights on the negative cycle will be
// set to -Inf. If the graph does not implement Weighted, UniformCost is used.
//
// The time complexity of FloydWarshall is O(|V|^3).
func FloydWarshall(g graph.Graph) (paths AllShortest, ok bool) {
	var weight Weighting
	if wg, ok := g.(Weighted); ok {
		weight = wg.Weight
	} else {
		weight = UniformCost(g)
	}

	nodes := graph.NodesOf(g.Nodes())
	paths = newAllShortest(nodes, true)
	for i, u := range nodes {
		paths.dist.Set(i, i, 0)
		uid := u.ID()
		to := g.From(uid)
		for to.Next() {
			vid := to.Node().ID()
			j := paths.indexOf[vid]
			w, ok := weight(uid, vid)
			if !ok {
				panic("floyd-warshall: unexpected invalid weight")
			}
			paths.set(i, j, w, j)
		}
	}

	for k := range nodes {
		for i := range nodes {
			for j := range nodes {
				ij := paths.dist.At(i, j)
				joint := paths.dist.At(i, k) + paths.dist.At(k, j)
				if ij > joint {
					paths.set(i, j, joint, paths.at(i, k)...)
				} else if ij-joint == 0 {
					paths.add(i, j, paths.at(i, k)...)
				}
			}
		}
	}

	ok = true
	for i := range nodes {
		if paths.dist.At(i, i) < 0 {
			ok = false
			break
		}
	}

	if !ok {
		// If we have a negative cycle, mark all
		// the edges in the cycles with NaN(0xdefaced)
		// weight. These weights are internal, being
		// returned as -Inf in user calls.

		d := paths.dist
		for i := range nodes {
			for j := range nodes {
				for k := range nodes {
					if math.IsInf(d.At(i, k), 1) || math.IsInf(d.At(k, j), 1) {
						continue
					}
					if d.At(k, k) < 0 {
						d.Set(k, k, defaced)
						d.Set(i, j, defaced)
					} else if math.Float64bits(d.At(k, k)) == defacedBits {
						d.Set(i, j, defaced)
					}
				}
			}
		}
	}

	return paths, ok
}
