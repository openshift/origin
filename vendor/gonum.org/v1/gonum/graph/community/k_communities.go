// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
	"gonum.org/v1/gonum/graph/traverse"
)

// KCliqueCommunities returns the k-clique communties of the undirected graph g for
// k greater than zero. The returned communities are identified by linkage via k-clique
// adjacency, where adjacency is defined as having k-1 common nodes. KCliqueCommunities
// returns a single component including the full set of nodes of g when k is 1,
// and the classical connected components of g when k is 2. Note that k-clique
// communities may contain common nodes from g.
//
// k-clique communities are described in Palla et al. doi:10.1038/nature03607.
func KCliqueCommunities(k int, g graph.Undirected) [][]graph.Node {
	if k < 1 {
		panic("community: invalid k for k-clique communities")
	}
	switch k {
	case 1:
		return [][]graph.Node{g.Nodes()}
	case 2:
		return topo.ConnectedComponents(g)
	default:
		cg := simple.NewUndirectedGraph()
		topo.CliqueGraph(cg, g)
		cc := kConnectedComponents(k, cg)

		// Extract the nodes in g from cg,
		// removing duplicates and separating
		// cliques smaller than k into separate
		// single nodes.
		var kcc [][]graph.Node
		single := make(set.Nodes)
		inCommunity := make(set.Nodes)
		for _, c := range cc {
			nodes := make(set.Nodes, len(c))
			for _, cn := range c {
				for _, n := range cn.(topo.Clique).Nodes() {
					nodes.Add(n)
				}
			}
			if len(nodes) < k {
				for _, n := range nodes {
					single.Add(n)
				}
				continue
			}
			var kc []graph.Node
			for _, n := range nodes {
				inCommunity.Add(n)
				kc = append(kc, n)
			}
			kcc = append(kcc, kc)
		}
		for _, n := range single {
			if !inCommunity.Has(n) {
				kcc = append(kcc, []graph.Node{n})
			}
		}

		return kcc
	}
}

// kConnectedComponents returns the connected components of topo.Clique nodes that
// are joined by k-1 underlying shared nodes in the graph that created the clique
// graph cg.
func kConnectedComponents(k int, cg graph.Undirected) [][]graph.Node {
	var (
		c  []graph.Node
		cc [][]graph.Node
	)
	during := func(n graph.Node) {
		c = append(c, n)
	}
	after := func() {
		cc = append(cc, []graph.Node(nil))
		cc[len(cc)-1] = append(cc[len(cc)-1], c...)
		c = c[:0]
	}
	w := traverse.DepthFirst{
		EdgeFilter: func(e graph.Edge) bool {
			return len(e.(topo.CliqueGraphEdge).Nodes()) >= k-1
		},
	}
	w.WalkAll(cg, nil, after, during)

	return cc
}
