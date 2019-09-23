// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph/simple"
)

var pageRankTests = []struct {
	g    []set
	damp float64
	tol  float64

	wantTol float64
	want    map[int64]float64
}{
	{
		// Example graph from http://en.wikipedia.org/wiki/File:PageRanks-Example.svg 16:17, 8 July 2009
		g: []set{
			A: nil,
			B: linksTo(C),
			C: linksTo(B),
			D: linksTo(A, B),
			E: linksTo(D, B, F),
			F: linksTo(B, E),
			G: linksTo(B, E),
			H: linksTo(B, E),
			I: linksTo(B, E),
			J: linksTo(E),
			K: linksTo(E),
		},
		damp: 0.85,
		tol:  1e-8,

		wantTol: 1e-8,
		want: map[int64]float64{
			A: 0.03278149,
			B: 0.38440095,
			C: 0.34291029,
			D: 0.03908709,
			E: 0.08088569,
			F: 0.03908709,
			G: 0.01616948,
			H: 0.01616948,
			I: 0.01616948,
			J: 0.01616948,
			K: 0.01616948,
		},
	},
	{
		// Example graph from http://en.wikipedia.org/w/index.php?title=PageRank&oldid=659286279#Power_Method
		// Expected result calculated with the given MATLAB code.
		g: []set{
			A: linksTo(B, C),
			B: linksTo(D),
			C: linksTo(D, E),
			D: linksTo(E),
			E: linksTo(A),
		},
		damp: 0.80,
		tol:  1e-3,

		wantTol: 1e-3,
		want: map[int64]float64{
			A: 0.250,
			B: 0.140,
			C: 0.140,
			D: 0.208,
			E: 0.262,
		},
	},
}

func TestPageRank(t *testing.T) {
	for i, test := range pageRankTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		got := pageRank(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[int64(n)], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func TestPageRankSparse(t *testing.T) {
	for i, test := range pageRankTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		got := pageRankSparse(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[int64(n)], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

var edgeWeightedPageRankTests = []struct {
	g            []set
	self, absent float64
	edges        map[int]map[int64]float64
	damp         float64
	tol          float64

	wantTol float64
	want    map[int64]float64
}{
	{
		// This test case is created according to the result with the following python code
		// on python 3.6.4 (using "networkx" of version 2.1)
		//
		// >>> import networkx as nx
		// >>> D = nx.DiGraph()
		// >>> D.add_weighted_edges_from([('A', 'B', 0.3), ('A','C', 1.2), ('B', 'A', 0.4), ('C', 'B', 0.3), ('D', 'A', 0.3), ('D', 'B', 2.1)])
		// >>> nx.pagerank(D, alpha=0.85, tol=1e-10)
		// {'A': 0.3409109390701202, 'B': 0.3522682754411842, 'C': 0.2693207854886954, 'D': 0.037500000000000006}

		g: []set{
			A: linksTo(B, C),
			B: linksTo(A),
			C: linksTo(B),
			D: linksTo(A, B),
		},
		edges: map[int]map[int64]float64{
			A: {
				B: 0.3,
				C: 1.2,
			},
			B: {
				A: 0.4,
			},
			C: {
				B: 0.3,
			},
			D: {
				A: 0.3,
				B: 2.1,
			},
		},
		damp: 0.85,
		tol:  1e-10,

		wantTol: 1e-8,
		want: map[int64]float64{
			A: 0.3409120160955594,
			B: 0.3522678129306601,
			C: 0.2693201709737804,
			D: 0.037500000000000006,
		},
	},
}

func TestEdgeWeightedPageRank(t *testing.T) {
	for i, test := range edgeWeightedPageRankTests {
		g := simple.NewWeightedDirectedGraph(test.self, test.absent)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			ws, ok := test.edges[u]
			if !ok {
				t.Errorf("edges not found for %v", u)
			}

			for v := range e {
				if w, ok := ws[v]; ok {
					g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(u), simple.Node(v), w))
				}
			}
		}
		got := edgeWeightedPageRank(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[int64(n)], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func TestEdgeWeightedPageRankSparse(t *testing.T) {
	for i, test := range edgeWeightedPageRankTests {
		g := simple.NewWeightedDirectedGraph(test.self, test.absent)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			ws, ok := test.edges[u]
			if !ok {
				t.Errorf("edges not found for %v", u)
			}

			for v := range e {
				if w, ok := ws[v]; ok {
					g.SetWeightedEdge(g.NewWeightedEdge(simple.Node(u), simple.Node(v), w))
				}
			}
		}
		got := edgeWeightedPageRankSparse(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[int64(n)], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func orderedFloats(w map[int64]float64, prec int) []keyFloatVal {
	o := make(orderedFloatsMap, 0, len(w))
	for k, v := range w {
		o = append(o, keyFloatVal{prec: prec, key: k, val: v})
	}
	sort.Sort(o)
	return o
}

type keyFloatVal struct {
	prec int
	key  int64
	val  float64
}

func (kv keyFloatVal) String() string { return fmt.Sprintf("%c:%.*f", kv.key+'A', kv.prec, kv.val) }

type orderedFloatsMap []keyFloatVal

func (o orderedFloatsMap) Len() int           { return len(o) }
func (o orderedFloatsMap) Less(i, j int) bool { return o[i].key < o[j].key }
func (o orderedFloatsMap) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
