// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/gonum/floats"
	"github.com/gonum/graph/concrete"
)

var pageRankTests = []struct {
	g    []set
	damp float64
	tol  float64

	wantTol float64
	want    map[int]float64
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
		want: map[int]float64{
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
		want: map[int]float64{
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
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		got := PageRank(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[n], test.want[n], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func TestPageRankSparse(t *testing.T) {
	for i, test := range pageRankTests {
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		got := PageRankSparse(g, test.damp, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[n], test.want[n], test.wantTol, test.wantTol) {
				t.Errorf("unexpected PageRank result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func orderedFloats(w map[int]float64, prec int) []keyFloatVal {
	o := make(orderedFloatsMap, 0, len(w))
	for k, v := range w {
		o = append(o, keyFloatVal{prec: prec, key: k, val: v})
	}
	sort.Sort(o)
	return o
}

type keyFloatVal struct {
	prec int
	key  int
	val  float64
}

func (kv keyFloatVal) String() string { return fmt.Sprintf("%d:%.*f", kv.key, kv.prec, kv.val) }

type orderedFloatsMap []keyFloatVal

func (o orderedFloatsMap) Len() int           { return len(o) }
func (o orderedFloatsMap) Less(i, j int) bool { return o[i].key < o[j].key }
func (o orderedFloatsMap) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
