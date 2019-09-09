// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flow

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/graphs/gen"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

var slta = flag.Bool("slta", false, "specify DominatorsSLT benchmark")

func BenchmarkDominators(b *testing.B) {
	testdata := filepath.FromSlash("./testdata/flow")

	fis, err := ioutil.ReadDir(testdata)
	if err != nil {
		b.Fatalf("failed to open control flow testdata: %v", err)
	}
	for _, fi := range fis {
		name := fi.Name()
		ext := filepath.Ext(name)
		if ext != ".dot" {
			continue
		}
		test := name[:len(name)-len(ext)]

		data, err := ioutil.ReadFile(filepath.Join(testdata, name))
		if err != nil {
			b.Errorf("failed to open control flow case: %v", err)
			continue
		}
		g := &labeled{DirectedGraph: simple.NewDirectedGraph()}
		err = dot.Unmarshal(data, g)
		if err != nil {
			b.Errorf("failed to unmarshal graph data: %v", err)
			continue
		}
		want := g.root
		if want == nil {
			b.Error("no entry node label for graph")
			continue
		}

		if *slta {
			b.Run(test, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					d := DominatorsSLT(g.root, g)
					if got := d.Root(); got.ID() != want.ID() {
						b.Fatalf("unexpected root node: got:%d want:%d", got.ID(), want.ID())
					}
				}
			})
		} else {
			b.Run(test, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					d := Dominators(g.root, g)
					if got := d.Root(); got.ID() != want.ID() {
						b.Fatalf("unexpected root node: got:%d want:%d", got.ID(), want.ID())
					}
				}
			})
		}
	}
}

type labeled struct {
	*simple.DirectedGraph

	root *node
}

func (g *labeled) NewNode() graph.Node {
	return &node{Node: g.DirectedGraph.NewNode(), g: g}
}

func (g *labeled) SetEdge(e graph.Edge) {
	if e.To().ID() == e.From().ID() {
		// Do not attempt to add self edges.
		return
	}
	g.DirectedGraph.SetEdge(e)
}

type node struct {
	graph.Node
	name string
	g    *labeled
}

func (n *node) SetDOTID(id string) {
	n.name = id
}

func (n *node) SetAttribute(attr encoding.Attribute) error {
	if attr.Key != "label" {
		return nil
	}
	switch attr.Value {
	default:
		if attr.Value != `"{%0}"` && !strings.HasPrefix(attr.Value, `"{%0|`) {
			return nil
		}
		fallthrough
	case "entry", "root":
		if n.g.root != nil {
			return fmt.Errorf("set root for graph with existing root: old=%q new=%q", n.g.root.name, n.name)
		}
		n.g.root = n
	}
	return nil
}

func BenchmarkRandomGraphDominators(b *testing.B) {
	tests := []struct {
		name string
		g    func() *simple.DirectedGraph
	}{
		{name: "gnm-n=1e3-m=1e3", g: gnm(1e3, 1e3)},
		{name: "gnm-n=1e3-m=3e3", g: gnm(1e3, 3e3)},
		{name: "gnm-n=1e3-m=1e4", g: gnm(1e3, 1e4)},
		{name: "gnm-n=1e3-m=3e4", g: gnm(1e3, 3e4)},

		{name: "gnm-n=1e4-m=1e4", g: gnm(1e4, 1e4)},
		{name: "gnm-n=1e4-m=3e4", g: gnm(1e4, 3e4)},
		{name: "gnm-n=1e4-m=1e5", g: gnm(1e4, 1e5)},
		{name: "gnm-n=1e4-m=3e5", g: gnm(1e4, 3e5)},

		{name: "gnm-n=1e5-m=1e5", g: gnm(1e5, 1e5)},
		{name: "gnm-n=1e5-m=3e5", g: gnm(1e5, 3e5)},
		{name: "gnm-n=1e5-m=1e6", g: gnm(1e5, 1e6)},
		{name: "gnm-n=1e5-m=3e6", g: gnm(1e5, 3e6)},

		{name: "gnm-n=1e6-m=1e6", g: gnm(1e6, 1e6)},
		{name: "gnm-n=1e6-m=3e6", g: gnm(1e6, 3e6)},
		{name: "gnm-n=1e6-m=1e7", g: gnm(1e6, 1e7)},
		{name: "gnm-n=1e6-m=3e7", g: gnm(1e6, 3e7)},

		{name: "dup-n=1e3-d=0.8-a=0.1", g: duplication(1e3, 0.8, 0.1, math.NaN())},
		{name: "dup-n=1e3-d=0.5-a=0.2", g: duplication(1e3, 0.5, 0.2, math.NaN())},

		{name: "dup-n=1e4-d=0.8-a=0.1", g: duplication(1e4, 0.8, 0.1, math.NaN())},
		{name: "dup-n=1e4-d=0.5-a=0.2", g: duplication(1e4, 0.5, 0.2, math.NaN())},

		{name: "dup-n=1e5-d=0.8-a=0.1", g: duplication(1e5, 0.8, 0.1, math.NaN())},
		{name: "dup-n=1e5-d=0.5-a=0.2", g: duplication(1e5, 0.5, 0.2, math.NaN())},
	}

	for _, test := range tests {
		rnd := rand.New(rand.NewSource(1))
		g := test.g()

		// Guess a maximally expensive entry to the graph.
		sort, err := topo.Sort(g)
		root := sort[0]
		if root == nil {
			// If we did not get a node in the first position
			// then there must be an unorderable set of nodes
			// in the first position of the error. Pick one
			// of the nodes at random.
			unordered := err.(topo.Unorderable)
			root = unordered[0][rnd.Intn(len(unordered[0]))]
		}
		if root == nil {
			b.Error("no entry node label for graph")
			continue
		}

		if len(sort) > 1 {
			// Ensure that the graph has a complete path
			// through the sorted nodes.

			// unordered will only be accessed if there is
			// a sort element that is nil, in which case
			// unordered will contain a set of nodes from
			// an SCC.
			unordered, _ := err.(topo.Unorderable)

			var ui int
			for i, v := range sort[1:] {
				u := sort[i]
				if u == nil {
					u = unordered[ui][rnd.Intn(len(unordered[ui]))]
					ui++
				}
				if v == nil {
					v = unordered[ui][rnd.Intn(len(unordered[ui]))]
				}
				if !g.HasEdgeFromTo(u.ID(), v.ID()) {
					g.SetEdge(g.NewEdge(u, v))
				}
			}
		}

		b.Run(test.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				d := Dominators(root, g)
				if got := d.Root(); got.ID() != root.ID() {
					b.Fatalf("unexpected root node: got:%d want:%d", got.ID(), root.ID())
				}
			}
		})
	}
}

// gnm returns a directed G(n,m) Erdõs-Rényi graph.
func gnm(n, m int) func() *simple.DirectedGraph {
	return func() *simple.DirectedGraph {
		dg := simple.NewDirectedGraph()
		err := gen.Gnm(dg, n, m, rand.New(rand.NewSource(1)))
		if err != nil {
			panic(err)
		}
		return dg
	}
}

// duplication returns an edge-induced directed subgraph of a
// duplication graph.
func duplication(n int, delta, alpha, sigma float64) func() *simple.DirectedGraph {
	return func() *simple.DirectedGraph {
		g := undirected{simple.NewDirectedGraph()}
		rnd := rand.New(rand.NewSource(1))
		err := gen.Duplication(g, n, delta, alpha, sigma, rnd)
		if err != nil {
			panic(err)
		}
		for _, e := range graph.EdgesOf(g.Edges()) {
			if rnd.Intn(2) == 0 {
				g.RemoveEdge(e.From().ID(), e.To().ID())
			}
		}
		return g.DirectedGraph
	}
}

type undirected struct {
	*simple.DirectedGraph
}

func (g undirected) From(id int64) graph.Nodes {
	return iterator.NewOrderedNodes(append(
		graph.NodesOf(g.DirectedGraph.From(id)),
		graph.NodesOf(g.DirectedGraph.To(id))...))
}

func (g undirected) HasEdgeBetween(xid, yid int64) bool {
	return g.DirectedGraph.HasEdgeFromTo(xid, yid)
}

func (g undirected) EdgeBetween(xid, yid int64) graph.Edge {
	return g.DirectedGraph.Edge(xid, yid)
}

func (g undirected) SetEdge(e graph.Edge) {
	g.DirectedGraph.SetEdge(e)
	g.DirectedGraph.SetEdge(g.DirectedGraph.NewEdge(e.To(), e.From()))
}
