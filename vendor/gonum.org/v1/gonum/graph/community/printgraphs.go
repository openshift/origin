// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// printgraphs allows us to generate a consistent directed view of
// a set of edges that follows a reasonably real-world-meaningful
// graph. The interpretation of the links in the resulting directed
// graphs are either "suggests" in the context of a Page Ranking or
// possibly "looks up to" in the Zachary graph.
package main

import (
	"fmt"
	"sort"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

// set is an integer set.
type set map[int]struct{}

func linksTo(i ...int) set {
	if len(i) == 0 {
		return nil
	}
	s := make(set)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}

var (
	zachary = []set{
		0:  linksTo(1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 17, 19, 21, 31),
		1:  linksTo(2, 3, 7, 13, 17, 19, 21, 30),
		2:  linksTo(3, 7, 8, 9, 13, 27, 28, 32),
		3:  linksTo(7, 12, 13),
		4:  linksTo(6, 10),
		5:  linksTo(6, 10, 16),
		6:  linksTo(16),
		8:  linksTo(30, 32, 33),
		9:  linksTo(33),
		13: linksTo(33),
		14: linksTo(32, 33),
		15: linksTo(32, 33),
		18: linksTo(32, 33),
		19: linksTo(33),
		20: linksTo(32, 33),
		22: linksTo(32, 33),
		23: linksTo(25, 27, 29, 32, 33),
		24: linksTo(25, 27, 31),
		25: linksTo(31),
		26: linksTo(29, 33),
		27: linksTo(33),
		28: linksTo(31, 33),
		29: linksTo(32, 33),
		30: linksTo(32, 33),
		31: linksTo(32, 33),
		32: linksTo(33),
		33: nil,
	}

	blondel = []set{
		0:  linksTo(2, 3, 4, 5),
		1:  linksTo(2, 4, 7),
		2:  linksTo(4, 5, 6),
		3:  linksTo(7),
		4:  linksTo(10),
		5:  linksTo(7, 11),
		6:  linksTo(7, 11),
		8:  linksTo(9, 10, 11, 14, 15),
		9:  linksTo(12, 14),
		10: linksTo(11, 12, 13, 14),
		11: linksTo(13),
		15: nil,
	}
)

func main() {
	for _, raw := range []struct {
		name string
		set  []set
	}{
		{"zachary", zachary},
		{"blondel", blondel},
	} {
		g := simple.NewUndirectedGraph(0, 0)
		for u, e := range raw.set {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		nodes := g.Nodes()
		sort.Sort(ordered.ByID(nodes))

		fmt.Printf("%s = []set{\n", raw.name)
		rank := network.PageRank(asDirected{g}, 0.85, 1e-8)
		for _, u := range nodes {
			to := g.From(nodes[u.ID()])
			sort.Sort(ordered.ByID(to))
			var links []int
			for _, v := range to {
				if rank[u.ID()] <= rank[v.ID()] {
					links = append(links, v.ID())
				}
			}

			if links == nil {
				fmt.Printf("\t%d: nil, // rank=%.4v\n", u.ID(), rank[u.ID()])
				continue
			}

			fmt.Printf("\t%d: linksTo(", u.ID())
			for i, v := range links {
				if i != 0 {
					fmt.Print(", ")
				}
				fmt.Print(v)
			}
			fmt.Printf("), // rank=%.4v\n", rank[u.ID()])
		}
		fmt.Println("}")
	}
}

type asDirected struct{ *simple.UndirectedGraph }

func (g asDirected) HasEdgeFromTo(u, v graph.Node) bool {
	return g.UndirectedGraph.HasEdgeBetween(u, v)
}
func (g asDirected) To(v graph.Node) []graph.Node { return g.From(v) }
