// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

var undirectedCentralityTests = []struct {
	g []set

	farness  map[int64]float64
	harmonic map[int64]float64
	residual map[int64]float64
}{
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: nil,
		},

		farness: map[int64]float64{
			A: 1 + 2,
			B: 1 + 1,
			C: 2 + 1,
		},
		harmonic: map[int64]float64{
			A: 1 + 1.0/2.0,
			B: 1 + 1,
			C: 1.0/2.0 + 1,
		},
		residual: map[int64]float64{
			A: 1/math.Exp2(1) + 1/math.Exp2(2),
			B: 1/math.Exp2(1) + 1/math.Exp2(1),
			C: 1/math.Exp2(2) + 1/math.Exp2(1),
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

		farness: map[int64]float64{
			A: 1 + 2 + 3 + 4,
			B: 1 + 1 + 2 + 3,
			C: 2 + 1 + 1 + 2,
			D: 3 + 2 + 1 + 1,
			E: 4 + 3 + 2 + 1,
		},
		harmonic: map[int64]float64{
			A: 1 + 1.0/2.0 + 1.0/3.0 + 1.0/4.0,
			B: 1 + 1 + 1.0/2.0 + 1.0/3.0,
			C: 1.0/2.0 + 1 + 1 + 1.0/2.0,
			D: 1.0/3.0 + 1.0/2.0 + 1 + 1,
			E: 1.0/4.0 + 1.0/3.0 + 1.0/2.0 + 1,
		},
		residual: map[int64]float64{
			A: 1/math.Exp2(1) + 1/math.Exp2(2) + 1/math.Exp2(3) + 1/math.Exp2(4),
			B: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(2) + 1/math.Exp2(3),
			C: 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(2),
			D: 1/math.Exp2(3) + 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(1),
			E: 1/math.Exp2(4) + 1/math.Exp2(3) + 1/math.Exp2(2) + 1/math.Exp2(1),
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

		farness: map[int64]float64{
			A: 2 + 2 + 1 + 2,
			B: 2 + 1 + 2 + 2,
			C: 1 + 1 + 1 + 1,
			D: 2 + 1 + 2 + 2,
			E: 2 + 2 + 1 + 2,
		},
		harmonic: map[int64]float64{
			A: 1.0/2.0 + 1.0/2.0 + 1 + 1.0/2.0,
			B: 1.0/2.0 + 1 + 1.0/2.0 + 1.0/2.0,
			C: 1 + 1 + 1 + 1,
			D: 1.0/2.0 + 1 + 1.0/2.0 + 1.0/2.0,
			E: 1.0/2.0 + 1.0/2.0 + 1 + 1.0/2.0,
		},
		residual: map[int64]float64{
			A: 1/math.Exp2(2) + 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(2),
			B: 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(2) + 1/math.Exp2(2),
			C: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			D: 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(2) + 1/math.Exp2(2),
			E: 1/math.Exp2(2) + 1/math.Exp2(2) + 1/math.Exp2(1) + 1/math.Exp2(2),
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

		farness: map[int64]float64{
			A: 1 + 1 + 1 + 1,
			B: 1 + 1 + 1 + 1,
			C: 1 + 1 + 1 + 1,
			D: 1 + 1 + 1 + 1,
			E: 1 + 1 + 1 + 1,
		},
		harmonic: map[int64]float64{
			A: 1 + 1 + 1 + 1,
			B: 1 + 1 + 1 + 1,
			C: 1 + 1 + 1 + 1,
			D: 1 + 1 + 1 + 1,
			E: 1 + 1 + 1 + 1,
		},
		residual: map[int64]float64{
			A: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			B: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			C: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			D: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			E: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
		},
	},
}

func TestDistanceCentralityUndirected(t *testing.T) {
	const tol = 1e-12
	prec := 1 - int(math.Log10(tol))

	for i, test := range undirectedCentralityTests {
		g := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}
		p, ok := path.FloydWarshall(g)
		if !ok {
			t.Errorf("unexpected negative cycle in test %d", i)
			continue
		}

		var got map[int64]float64

		got = Closeness(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], 1/test.farness[int64(n)], tol, tol) {
				want := make(map[int64]float64)
				for n, v := range test.farness {
					want[n] = 1 / v
				}
				t.Errorf("unexpected closeness centrality for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(want, prec))
				break
			}
		}

		got = Farness(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.farness[int64(n)], tol, tol) {
				t.Errorf("unexpected farness for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.farness, prec))
				break
			}
		}

		got = Harmonic(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.harmonic[int64(n)], tol, tol) {
				t.Errorf("unexpected harmonic centrality for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.harmonic, prec))
				break
			}
		}

		got = Residual(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.residual[int64(n)], tol, tol) {
				t.Errorf("unexpected residual closeness for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.residual, prec))
				break
			}
		}
	}
}

var directedCentralityTests = []struct {
	g []set

	farness  map[int64]float64
	harmonic map[int64]float64
	residual map[int64]float64
}{
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: nil,
		},

		farness: map[int64]float64{
			A: 0,
			B: 1,
			C: 2 + 1,
		},
		harmonic: map[int64]float64{
			A: 0,
			B: 1,
			C: 1.0/2.0 + 1,
		},
		residual: map[int64]float64{
			A: 0,
			B: 1 / math.Exp2(1),
			C: 1/math.Exp2(2) + 1/math.Exp2(1),
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

		farness: map[int64]float64{
			A: 0,
			B: 1,
			C: 2 + 1,
			D: 3 + 2 + 1,
			E: 4 + 3 + 2 + 1,
		},
		harmonic: map[int64]float64{
			A: 0,
			B: 1,
			C: 1.0/2.0 + 1,
			D: 1.0/3.0 + 1.0/2.0 + 1,
			E: 1.0/4.0 + 1.0/3.0 + 1.0/2.0 + 1,
		},
		residual: map[int64]float64{
			A: 0,
			B: 1 / math.Exp2(1),
			C: 1/math.Exp2(2) + 1/math.Exp2(1),
			D: 1/math.Exp2(3) + 1/math.Exp2(2) + 1/math.Exp2(1),
			E: 1/math.Exp2(4) + 1/math.Exp2(3) + 1/math.Exp2(2) + 1/math.Exp2(1),
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

		farness: map[int64]float64{
			A: 0,
			B: 0,
			C: 1 + 1 + 1 + 1,
			D: 0,
			E: 0,
		},
		harmonic: map[int64]float64{
			A: 0,
			B: 0,
			C: 1 + 1 + 1 + 1,
			D: 0,
			E: 0,
		},
		residual: map[int64]float64{
			A: 0,
			B: 0,
			C: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			D: 0,
			E: 0,
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

		farness: map[int64]float64{
			A: 0,
			B: 1,
			C: 1 + 1,
			D: 1 + 1 + 1,
			E: 1 + 1 + 1 + 1,
		},
		harmonic: map[int64]float64{
			A: 0,
			B: 1,
			C: 1 + 1,
			D: 1 + 1 + 1,
			E: 1 + 1 + 1 + 1,
		},
		residual: map[int64]float64{
			A: 0,
			B: 1 / math.Exp2(1),
			C: 1/math.Exp2(1) + 1/math.Exp2(1),
			D: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
			E: 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1) + 1/math.Exp2(1),
		},
	},
}

func TestDistanceCentralityDirected(t *testing.T) {
	const tol = 1e-12
	prec := 1 - int(math.Log10(tol))

	for i, test := range directedCentralityTests {
		g := simple.NewWeightedDirectedGraph(0, math.Inf(1))
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}
		p, ok := path.FloydWarshall(g)
		if !ok {
			t.Errorf("unexpected negative cycle in test %d", i)
			continue
		}

		var got map[int64]float64

		got = Closeness(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], 1/test.farness[int64(n)], tol, tol) {
				want := make(map[int64]float64)
				for n, v := range test.farness {
					want[int64(n)] = 1 / v
				}
				t.Errorf("unexpected closeness centrality for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(want, prec))
				break
			}
		}

		got = Farness(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.farness[int64(n)], tol, tol) {
				t.Errorf("unexpected farness for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.farness, prec))
				break
			}
		}

		got = Harmonic(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.harmonic[int64(n)], tol, tol) {
				t.Errorf("unexpected harmonic centrality for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.harmonic, prec))
				break
			}
		}

		got = Residual(g, p)
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.residual[int64(n)], tol, tol) {
				t.Errorf("unexpected residual closeness for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.residual, prec))
				break
			}
		}
	}
}
