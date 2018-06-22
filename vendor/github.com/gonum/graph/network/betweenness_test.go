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
	"github.com/gonum/graph/path"
	"github.com/gonum/graph/simple"
)

var betweennessTests = []struct {
	g []set

	wantTol   float64
	want      map[int]float64
	wantEdges map[[2]int]float64
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

		wantTol: 1e-1,
		want: map[int]float64{
			B: 32,
			D: 18,
			E: 48,
		},
		wantEdges: map[[2]int]float64{
			[2]int{A, D}: 20,
			[2]int{B, C}: 20,
			[2]int{B, D}: 16,
			[2]int{B, E}: 12,
			[2]int{B, F}: 9,
			[2]int{B, G}: 9,
			[2]int{B, H}: 9,
			[2]int{B, I}: 9,
			[2]int{D, E}: 20,
			[2]int{E, F}: 11,
			[2]int{E, G}: 11,
			[2]int{E, H}: 11,
			[2]int{E, I}: 11,
			[2]int{E, J}: 20,
			[2]int{E, K}: 20,
		},
	},
	{
		// Example graph from http://en.wikipedia.org/w/index.php?title=PageRank&oldid=659286279#Power_Method
		g: []set{
			A: linksTo(B, C),
			B: linksTo(D),
			C: linksTo(D, E),
			D: linksTo(E),
			E: linksTo(A),
		},

		wantTol: 1e-3,
		want: map[int]float64{
			A: 2,
			B: 0.6667,
			C: 0.6667,
			D: 2,
			E: 0.6667,
		},
		wantEdges: map[[2]int]float64{
			[2]int{A, B}: 2 + 2/3. + 4/2.,
			[2]int{A, C}: 2 + 2/3. + 2/2.,
			[2]int{A, E}: 2 + 2/3. + 2/2.,
			[2]int{B, D}: 2 + 2/3. + 4/2.,
			[2]int{C, D}: 2 + 2/3. + 2/2.,
			[2]int{C, E}: 2,
			[2]int{D, E}: 2 + 2/3. + 2/2.,
		},
	},
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: nil,
		},

		wantTol: 1e-3,
		want: map[int]float64{
			B: 2,
		},
		wantEdges: map[[2]int]float64{
			[2]int{A, B}: 4,
			[2]int{B, C}: 4,
		},
	},
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: linksTo(D),
			D: linksTo(E),
			E: nil,
		},

		wantTol: 1e-3,
		want: map[int]float64{
			B: 6,
			C: 8,
			D: 6,
		},
		wantEdges: map[[2]int]float64{
			[2]int{A, B}: 8,
			[2]int{B, C}: 12,
			[2]int{C, D}: 12,
			[2]int{D, E}: 8,
		},
	},
	{
		g: []set{
			A: linksTo(C),
			B: linksTo(C),
			C: nil,
			D: linksTo(C),
			E: linksTo(C),
		},

		wantTol: 1e-3,
		want: map[int]float64{
			C: 12,
		},
		wantEdges: map[[2]int]float64{
			[2]int{A, C}: 8,
			[2]int{B, C}: 8,
			[2]int{C, D}: 8,
			[2]int{C, E}: 8,
		},
	},
	{
		g: []set{
			A: linksTo(B, C, D, E),
			B: linksTo(C, D, E),
			C: linksTo(D, E),
			D: linksTo(E),
			E: nil,
		},

		wantTol: 1e-3,
		want:    map[int]float64{},
		wantEdges: map[[2]int]float64{
			[2]int{A, B}: 2,
			[2]int{A, C}: 2,
			[2]int{A, D}: 2,
			[2]int{A, E}: 2,
			[2]int{B, C}: 2,
			[2]int{B, D}: 2,
			[2]int{B, E}: 2,
			[2]int{C, D}: 2,
			[2]int{C, E}: 2,
			[2]int{D, E}: 2,
		},
	},
}

func TestBetweenness(t *testing.T) {
	for i, test := range betweennessTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				// Weight omitted to show weight-independence.
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v), W: 0})
			}
		}
		got := Betweenness(g)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			wantN, gotOK := got[n]
			gotN, wantOK := test.want[n]
			if gotOK != wantOK {
				t.Errorf("unexpected betweenness result for test %d, node %c", i, n+'A')
			}
			if !floats.EqualWithinAbsOrRel(gotN, wantN, test.wantTol, test.wantTol) {
				t.Errorf("unexpected betweenness result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func TestEdgeBetweenness(t *testing.T) {
	for i, test := range betweennessTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				// Weight omitted to show weight-independence.
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v), W: 0})
			}
		}
		got := EdgeBetweenness(g)
		prec := 1 - int(math.Log10(test.wantTol))
	outer:
		for u := range test.g {
			for v := range test.g {
				wantQ, gotOK := got[[2]int{u, v}]
				gotQ, wantOK := test.wantEdges[[2]int{u, v}]
				if gotOK != wantOK {
					t.Errorf("unexpected betweenness result for test %d, edge (%c,%c)", i, u+'A', v+'A')
				}
				if !floats.EqualWithinAbsOrRel(gotQ, wantQ, test.wantTol, test.wantTol) {
					t.Errorf("unexpected betweenness result for test %d:\ngot: %v\nwant:%v",
						i, orderedPairFloats(got, prec), orderedPairFloats(test.wantEdges, prec))
					break outer
				}
			}
		}
	}
}

func TestBetweennessWeighted(t *testing.T) {
	for i, test := range betweennessTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		p, ok := path.FloydWarshall(g)
		if !ok {
			t.Errorf("unexpected negative cycle in test %d", i)
			continue
		}

		got := BetweennessWeighted(g, p)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			gotN, gotOK := got[n]
			wantN, wantOK := test.want[n]
			if gotOK != wantOK {
				t.Errorf("unexpected betweenness existence for test %d, node %c", i, n+'A')
			}
			if !floats.EqualWithinAbsOrRel(gotN, wantN, test.wantTol, test.wantTol) {
				t.Errorf("unexpected betweenness result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}
	}
}

func TestEdgeBetweennessWeighted(t *testing.T) {
	for i, test := range betweennessTests {
		g := simple.NewUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(simple.Node(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		p, ok := path.FloydWarshall(g)
		if !ok {
			t.Errorf("unexpected negative cycle in test %d", i)
			continue
		}

		got := EdgeBetweennessWeighted(g, p)
		prec := 1 - int(math.Log10(test.wantTol))
	outer:
		for u := range test.g {
			for v := range test.g {
				wantQ, gotOK := got[[2]int{u, v}]
				gotQ, wantOK := test.wantEdges[[2]int{u, v}]
				if gotOK != wantOK {
					t.Errorf("unexpected betweenness result for test %d, edge (%c,%c)", i, u+'A', v+'A')
				}
				if !floats.EqualWithinAbsOrRel(gotQ, wantQ, test.wantTol, test.wantTol) {
					t.Errorf("unexpected betweenness result for test %d:\ngot: %v\nwant:%v",
						i, orderedPairFloats(got, prec), orderedPairFloats(test.wantEdges, prec))
					break outer
				}
			}
		}
	}
}

func orderedPairFloats(w map[[2]int]float64, prec int) []pairKeyFloatVal {
	o := make(orderedPairFloatsMap, 0, len(w))
	for k, v := range w {
		o = append(o, pairKeyFloatVal{prec: prec, key: k, val: v})
	}
	sort.Sort(o)
	return o
}

type pairKeyFloatVal struct {
	prec int
	key  [2]int
	val  float64
}

func (kv pairKeyFloatVal) String() string {
	return fmt.Sprintf("(%c,%c):%.*f", kv.key[0]+'A', kv.key[1]+'A', kv.prec, kv.val)
}

type orderedPairFloatsMap []pairKeyFloatVal

func (o orderedPairFloatsMap) Len() int { return len(o) }
func (o orderedPairFloatsMap) Less(i, j int) bool {
	return o[i].key[0] < o[j].key[0] || (o[i].key[0] == o[j].key[0] && o[i].key[1] < o[j].key[1])
}
func (o orderedPairFloatsMap) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
