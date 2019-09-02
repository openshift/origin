// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
)

// YenKShortestPaths returns the k-shortest loopless paths from s to t in g.
// YenKShortestPaths will panic if g contains a negative edge weight.
func YenKShortestPaths(g graph.Graph, k int, s, t graph.Node) [][]graph.Node {
	_, isDirected := g.(graph.Directed)
	yk := yenKSPAdjuster{
		Graph:      g,
		isDirected: isDirected,
	}

	if wg, ok := g.(Weighted); ok {
		yk.weight = wg.Weight
	} else {
		yk.weight = UniformCost(g)
	}

	shortest, _ := DijkstraFrom(s, yk).To(t.ID())
	switch len(shortest) {
	case 0:
		return nil
	case 1:
		return [][]graph.Node{shortest}
	}
	paths := [][]graph.Node{shortest}

	var pot []yenShortest
	var root []graph.Node
	for i := int64(1); i < int64(k); i++ {
		for n := 0; n < len(paths[i-1])-1; n++ {
			yk.reset()

			spur := paths[i-1][n]
			root := append(root[:0], paths[i-1][:n+1]...)

			for _, path := range paths {
				if len(path) <= n {
					continue
				}
				ok := true
				for x := 0; x < len(root); x++ {
					if path[x].ID() != root[x].ID() {
						ok = false
						break
					}
				}
				if ok {
					yk.removeEdge(path[n].ID(), path[n+1].ID())
				}
			}

			spath, weight := DijkstraFrom(spur, yk).To(t.ID())
			if len(root) > 1 {
				var rootWeight float64
				for x := 1; x < len(root); x++ {
					w, _ := yk.weight(root[x-1].ID(), root[x].ID())
					rootWeight += w
				}
				root = append(root[:len(root)-1], spath...)
				pot = append(pot, yenShortest{root, weight + rootWeight})
			} else {
				pot = append(pot, yenShortest{spath, weight})
			}
		}

		if len(pot) == 0 {
			break
		}

		sort.Sort(byPathWeight(pot))
		best := pot[0].path
		if len(best) <= 1 {
			break
		}
		paths = append(paths, best)
		pot = pot[1:]
	}

	return paths
}

// yenShortest holds a path and its weight for sorting.
type yenShortest struct {
	path   []graph.Node
	weight float64
}

type byPathWeight []yenShortest

func (s byPathWeight) Len() int           { return len(s) }
func (s byPathWeight) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s byPathWeight) Less(i, j int) bool { return s[i].weight < s[j].weight }

// yenKSPAdjuster allows walked edges to be omitted from a graph
// without altering the embedded graph.
type yenKSPAdjuster struct {
	graph.Graph
	isDirected bool

	// weight is the edge weight function
	// used for shortest path calculation.
	weight Weighting

	// visitedEdges holds the edges that have
	// been removed by Yen's algorithm.
	visitedEdges map[[2]int64]struct{}
}

func (g yenKSPAdjuster) From(id int64) graph.Nodes {
	nodes := graph.NodesOf(g.Graph.From(id))
	for i := 0; i < len(nodes); {
		if g.canWalk(id, nodes[i].ID()) {
			i++
			continue
		}
		nodes[i] = nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
	}
	return iterator.NewOrderedNodes(nodes)
}

func (g yenKSPAdjuster) canWalk(u, v int64) bool {
	_, ok := g.visitedEdges[[2]int64{u, v}]
	return !ok
}

func (g yenKSPAdjuster) removeEdge(u, v int64) {
	g.visitedEdges[[2]int64{u, v}] = struct{}{}
	if g.isDirected {
		g.visitedEdges[[2]int64{v, u}] = struct{}{}
	}
}

func (g *yenKSPAdjuster) reset() {
	g.visitedEdges = make(map[[2]int64]struct{})
}

func (g yenKSPAdjuster) Weight(xid, yid int64) (w float64, ok bool) {
	return g.weight(xid, yid)
}
