// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"github.com/gonum/graph"
	"github.com/gonum/graph/internal/set"
)

// Dominators returns all dominators for all nodes in g. It does not
// prune for strict post-dominators, immediate dominators etc.
//
// A dominates B if and only if the only path through B travels through A.
func Dominators(start graph.Node, g graph.Graph) map[int]set.Nodes {
	allNodes := make(set.Nodes)
	nlist := g.Nodes()
	dominators := make(map[int]set.Nodes, len(nlist))
	for _, node := range nlist {
		allNodes.Add(node)
	}

	var to func(graph.Node) []graph.Node
	switch g := g.(type) {
	case graph.Directed:
		to = g.To
	default:
		to = g.From
	}

	for _, node := range nlist {
		dominators[node.ID()] = make(set.Nodes)
		if node.ID() == start.ID() {
			dominators[node.ID()].Add(start)
		} else {
			dominators[node.ID()].Copy(allNodes)
		}
	}

	for somethingChanged := true; somethingChanged; {
		somethingChanged = false
		for _, node := range nlist {
			if node.ID() == start.ID() {
				continue
			}
			preds := to(node)
			if len(preds) == 0 {
				continue
			}
			tmp := make(set.Nodes).Copy(dominators[preds[0].ID()])
			for _, pred := range preds[1:] {
				tmp.Intersect(tmp, dominators[pred.ID()])
			}

			dom := make(set.Nodes)
			dom.Add(node)

			dom.Union(dom, tmp)
			if !set.Equal(dom, dominators[node.ID()]) {
				dominators[node.ID()] = dom
				somethingChanged = true
			}
		}
	}

	return dominators
}

// PostDominators returns all post-dominators for all nodes in g. It does not
// prune for strict post-dominators, immediate post-dominators etc.
//
// A post-dominates B if and only if all paths from B travel through A.
func PostDominators(end graph.Node, g graph.Graph) map[int]set.Nodes {
	allNodes := make(set.Nodes)
	nlist := g.Nodes()
	dominators := make(map[int]set.Nodes, len(nlist))
	for _, node := range nlist {
		allNodes.Add(node)
	}

	for _, node := range nlist {
		dominators[node.ID()] = make(set.Nodes)
		if node.ID() == end.ID() {
			dominators[node.ID()].Add(end)
		} else {
			dominators[node.ID()].Copy(allNodes)
		}
	}

	for somethingChanged := true; somethingChanged; {
		somethingChanged = false
		for _, node := range nlist {
			if node.ID() == end.ID() {
				continue
			}
			succs := g.From(node)
			if len(succs) == 0 {
				continue
			}
			tmp := make(set.Nodes).Copy(dominators[succs[0].ID()])
			for _, succ := range succs[1:] {
				tmp.Intersect(tmp, dominators[succ.ID()])
			}

			dom := make(set.Nodes)
			dom.Add(node)

			dom.Union(dom, tmp)
			if !set.Equal(dom, dominators[node.ID()]) {
				dominators[node.ID()] = dom
				somethingChanged = true
			}
		}
	}

	return dominators
}
