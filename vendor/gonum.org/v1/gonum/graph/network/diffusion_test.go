// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"
	"sort"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/mat"
)

var diffuseTests = []struct {
	g []set
	h map[int64]float64
	t float64

	wantTol float64
	want    map[bool]map[int64]float64
}{
	{
		g: grid(5),
		h: map[int64]float64{0: 1},
		t: 0.1,

		wantTol: 1e-9,
		want: map[bool]map[int64]float64{
			false: {
				A: 0.826684055, B: 0.078548060, C: 0.003858840, D: 0.000127487, E: 0.000003233,
				F: 0.078548060, G: 0.007463308, H: 0.000366651, I: 0.000012113, J: 0.000000307,
				K: 0.003858840, L: 0.000366651, M: 0.000018012, N: 0.000000595, O: 0.000000015,
				P: 0.000127487, Q: 0.000012113, R: 0.000000595, S: 0.000000020, T: 0.000000000,
				U: 0.000003233, V: 0.000000307, W: 0.000000015, X: 0.000000000, Y: 0.000000000,
			},
			true: {
				A: 0.9063462486, B: 0.0369774705, C: 0.0006161414, D: 0.0000068453, E: 0.0000000699,
				F: 0.0369774705, G: 0.0010670895, H: 0.0000148186, I: 0.0000001420, J: 0.0000000014,
				K: 0.0006161414, L: 0.0000148186, M: 0.0000001852, N: 0.0000000016, O: 0.0000000000,
				P: 0.0000068453, Q: 0.0000001420, R: 0.0000000016, S: 0.0000000000, T: 0.0000000000,
				U: 0.0000000699, V: 0.0000000014, W: 0.0000000000, X: 0.0000000000, Y: 0.0000000000,
			},
		},
	},
	{
		g: grid(5),
		h: map[int64]float64{0: 1},
		t: 1,

		wantTol: 1e-9,
		want: map[bool]map[int64]float64{
			false: {
				A: 0.2743435076, B: 0.1615920872, C: 0.0639346641, D: 0.0188054933, E: 0.0051023569,
				F: 0.1615920872, G: 0.0951799548, H: 0.0376583937, I: 0.0110766934, J: 0.0030053582,
				K: 0.0639346641, L: 0.0376583937, M: 0.0148997194, N: 0.0043825455, O: 0.0011890840,
				P: 0.0188054933, Q: 0.0110766934, R: 0.0043825455, S: 0.0012890649, T: 0.0003497525,
				U: 0.0051023569, V: 0.0030053582, W: 0.0011890840, X: 0.0003497525, Y: 0.0000948958,
			},
			true: {
				A: 0.4323917545, B: 0.1660487336, C: 0.0270298904, D: 0.0029720194, E: 0.0003007247,
				F: 0.1660487336, G: 0.0463974679, H: 0.0063556078, I: 0.0006056850, J: 0.0000589574,
				K: 0.0270298904, L: 0.0063556078, M: 0.0007860810, N: 0.0000691647, O: 0.0000065586,
				P: 0.0029720194, Q: 0.0006056850, R: 0.0000691647, S: 0.0000057466, T: 0.0000005475,
				U: 0.0003007247, V: 0.0000589574, W: 0.0000065586, X: 0.0000005475, Y: 0.0000000555,
			},
		},
	},
	{
		g: grid(5),
		h: map[int64]float64{0: 1},
		t: 10,

		wantTol: 1e-9,
		want: map[bool]map[int64]float64{
			false: {
				A: 0.0432375924, B: 0.0426071834, C: 0.0415872351, D: 0.0405673794, E: 0.0399371202,
				F: 0.0426071834, G: 0.0419859658, H: 0.0409808885, I: 0.0399759024, J: 0.0393548325,
				K: 0.0415872351, L: 0.0409808885, M: 0.0399998711, N: 0.0390189428, O: 0.0384127403,
				P: 0.0405673794, Q: 0.0399759024, R: 0.0390189428, S: 0.0380620700, T: 0.0374707336,
				U: 0.0399371202, V: 0.0393548325, W: 0.0384127403, X: 0.0374707336, Y: 0.0368885843,
			},
			true: {
				A: 0.0532814862, B: 0.0594280160, C: 0.0462076361, D: 0.0330529557, E: 0.0211688130,
				F: 0.0594280160, G: 0.0612529898, H: 0.0462850376, I: 0.0319891593, J: 0.0213123519,
				K: 0.0462076361, L: 0.0462850376, M: 0.0340410963, N: 0.0229646704, O: 0.0152763556,
				P: 0.0330529557, Q: 0.0319891593, R: 0.0229646704, S: 0.0153031853, T: 0.0103681461,
				U: 0.0211688130, V: 0.0213123519, W: 0.0152763556, X: 0.0103681461, Y: 0.0068893147,
			},
		},
	},
	{
		g: grid(5),
		h: func() map[int64]float64 {
			m := make(map[int64]float64, 25)
			for i := int64(A); i <= Y; i++ {
				m[i] = 1
			}
			return m
		}(),
		t: 1, // FIXME(kortschak): Low t used due to instability in mat.Exp.

		wantTol: 1e-1, // FIXME(kortschak): High tolerance used due to instability in mat.Exp.
		want: map[bool]map[int64]float64{
			false: {
				A: 1, B: 1, C: 1, D: 1, E: 1,
				F: 1, G: 1, H: 1, I: 1, J: 1,
				K: 1, L: 1, M: 1, N: 1, O: 1,
				P: 1, Q: 1, R: 1, S: 1, T: 1,
				U: 1, V: 1, W: 1, X: 1, Y: 1,
			},
			true: {
				// Output from the python implementation associated with doi:10.1371/journal.pcbi.1005598.
				A: 0.98264450473308107, B: 1.002568278028513, C: 0.9958911385307706, D: 1.002568278028513, E: 0.98264450473308107,
				F: 1.002568278028513, G: 1.0075291695232433, H: 1.0038067383118021, I: 1.0075291695232433, J: 1.002568278028513,
				K: 0.9958911385307706, L: 1.0038067383118021, M: 1.0001850837547184, N: 1.0038067383118021, O: 0.9958911385307706,
				P: 1.002568278028513, Q: 1.0075291695232433, R: 1.0038067383118021, S: 1.0075291695232433, T: 1.002568278028513,
				U: 0.98264450473308107, V: 1.002568278028513, W: 0.9958911385307706, X: 1.002568278028513, Y: 0.98264450473308107,
			},
		},
	},
	{
		g: []set{
			A: linksTo(B, C),
			B: linksTo(D),
			C: nil,
			D: nil,
			E: linksTo(F),
			F: nil,
		},
		h: map[int64]float64{A: 1, E: 10},
		t: 0.1,

		wantTol: 1e-9,
		want: map[bool]map[int64]float64{
			false: {
				A: 0.8270754166, B: 0.0822899600, C: 0.0863904410, D: 0.0042441824, E: 9.0936537654, F: 0.9063462346,
			},
			true: {
				A: 0.9082331512, B: 0.0453361743, C: 0.0640616812, D: 0.0016012085, E: 9.0936537654, F: 0.9063462346,
			},
		},
	},
}

func TestDiffuse(t *testing.T) {
	for i, test := range diffuseTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(int64(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		for j, lfn := range []func(g graph.Undirected) Laplacian{NewLaplacian, NewSymNormLaplacian} {
			normalize := j == 1
			var wantTemp float64
			for _, v := range test.h {
				wantTemp += v
			}
			got := Diffuse(nil, test.h, lfn(g), test.t)
			prec := 1 - int(math.Log10(test.wantTol))
			for n := range test.g {
				if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[normalize][int64(n)], test.wantTol, test.wantTol) {
					t.Errorf("unexpected Diffuse result for test %d with normalize=%t:\ngot: %v\nwant:%v",
						i, normalize, orderedFloats(got, prec), orderedFloats(test.want[normalize], prec))
					break
				}
			}

			if j == 1 {
				continue
			}

			var gotTemp float64
			for _, v := range got {
				gotTemp += v
			}
			gotTemp /= float64(len(got))
			wantTemp /= float64(len(got))
			if !floats.EqualWithinAbsOrRel(gotTemp, wantTemp, test.wantTol, test.wantTol) {
				t.Errorf("unexpected total heat for test %d with normalize=%t: got:%v want:%v",
					i, normalize, gotTemp, wantTemp)
			}
		}
	}
}

var randomWalkLaplacianTests = []struct {
	g    []set
	damp float64

	want *mat.Dense
}{
	{
		g: []set{
			A: linksTo(B, C),
			B: linksTo(C),
			C: nil,
		},

		want: mat.NewDense(3, 3, []float64{
			1, 0, 0,
			-0.5, 1, 0,
			-0.5, -1, 0,
		}),
	},
	{
		g: []set{
			A: linksTo(B, C),
			B: linksTo(C),
			C: nil,
		},
		damp: 0.85,

		want: mat.NewDense(3, 3, []float64{
			0.15, 0, 0,
			-0.075, 0.15, 0,
			-0.075, -0.15, 0,
		}),
	},
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: linksTo(A),
		},
		damp: 0.85,

		want: mat.NewDense(3, 3, []float64{
			0.15, 0, -0.15,
			-0.15, 0.15, 0,
			0, -0.15, 0.15,
		}),
	},
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

		want: mat.NewDense(11, 11, []float64{
			0, 0, 0, -0.5, 0, 0, 0, 0, 0, 0, 0,
			0, 1, -1, -0.5, -1. / 3., -0.5, -0.5, -0.5, -0.5, 0, 0,
			0, -1, 1, 0, 0, 0, 0, 0, 0, 0, 0,
			0, 0, 0, 1, -1. / 3., 0, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 1, -0.5, -0.5, -0.5, -0.5, -1, -1,
			0, 0, 0, 0, -1. / 3., 1, 0, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0,
			0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		}),
	},
}

func TestRandomWalkLaplacian(t *testing.T) {
	const tol = 1e-14
	for i, test := range randomWalkLaplacianTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(int64(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		l := NewRandomWalkLaplacian(g, test.damp)
		_, c := l.Dims()
		for j := 0; j < c; j++ {
			if got := mat.Sum(l.Matrix.(*mat.Dense).ColView(j)); !floats.EqualWithinAbsOrRel(got, 0, tol, tol) {
				t.Errorf("unexpected column sum for test %d, column %d: got:%v want:0", i, j, got)
			}
		}
		l = NewRandomWalkLaplacian(sortedNodeGraph{g}, test.damp)
		if !mat.EqualApprox(l, test.want, tol) {
			t.Errorf("unexpected result for test %d:\ngot:\n% .2v\nwant:\n% .2v",
				i, mat.Formatted(l), mat.Formatted(test.want))
		}
	}
}

type sortedNodeGraph struct {
	graph.Graph
}

func (g sortedNodeGraph) Nodes() []graph.Node {
	n := g.Graph.Nodes()
	sort.Sort(ordered.ByID(n))
	return n
}

var diffuseToEquilibriumTests = []struct {
	g       []set
	builder builder
	h       map[int64]float64
	damp    float64
	tol     float64
	iter    int

	want   map[int64]float64
	wantOK bool
}{
	{
		g:       grid(5),
		builder: simple.NewUndirectedGraph(),
		h:       map[int64]float64{0: 1},
		damp:    0.85,
		tol:     1e-6,
		iter:    1e4,

		want: map[int64]float64{
			A: 0.025000, B: 0.037500, C: 0.037500, D: 0.037500, E: 0.025000,
			F: 0.037500, G: 0.050000, H: 0.050000, I: 0.050000, J: 0.037500,
			K: 0.037500, L: 0.050000, M: 0.050000, N: 0.050000, O: 0.037500,
			P: 0.037500, Q: 0.050000, R: 0.050000, S: 0.050000, T: 0.037500,
			U: 0.025000, V: 0.037500, W: 0.037500, X: 0.037500, Y: 0.025000,
		},
		wantOK: true,
	},
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
		builder: simple.NewDirectedGraph(),
		h: map[int64]float64{
			A: 1. / 11.,
			B: 1. / 11.,
			C: 1. / 11.,
			D: 1. / 11.,
			E: 1. / 11.,
			F: 1. / 11.,
			G: 1. / 11.,
			H: 1. / 11.,
			I: 1. / 11.,
			J: 1. / 11.,
			K: 1. / 11.,
		},
		damp: 0.85,
		tol:  1e-6,
		iter: 1e4,

		// This does not look like Page Rank because we do not
		// do the random node hops. An alternative Laplacian
		// value that does do that would replicate PageRank. This
		// is left as an excercise for the reader.
		want: map[int64]float64{
			A: 0.227273,
			B: 0.386364,
			C: 0.386364,
			D: 0.000000,
			E: 0.000000,
			F: 0.000000,
			G: 0.000000,
			H: 0.000000,
			I: 0.000000,
			J: 0.000000,
			K: 0.000000,
		},
		wantOK: true,
	},
	{
		g: []set{
			A: linksTo(B, C),
			B: linksTo(D, C),
			C: nil,
			D: nil,
			E: linksTo(F),
			F: nil,
		},
		builder: simple.NewDirectedGraph(),
		h:       map[int64]float64{A: 1, E: -10},
		tol:     1e-6,
		iter:    3,

		want: map[int64]float64{
			A: 0, B: 0, C: 0.75, D: 0.25, E: 0, F: -10,
		},
		wantOK: true,
	},
	{
		g: []set{
			A: linksTo(B, C),
			B: linksTo(D, C),
			C: nil,
			D: nil,
			E: linksTo(F),
			F: nil,
		},
		builder: simple.NewUndirectedGraph(),
		h:       map[int64]float64{A: 1, E: -10},
		damp:    0.85,
		tol:     1e-6,
		iter:    1e4,

		want: map[int64]float64{
			A: 0.25, B: 0.375, C: 0.25, D: 0.125, E: -5, F: -5,
		},
		wantOK: true,
	},
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: nil,
		},
		builder: simple.NewUndirectedGraph(),
		h:       map[int64]float64{B: 1},
		iter:    1,
		tol:     1e-6,
		want: map[int64]float64{
			A: 0.5, B: 0, C: 0.5,
		},
		wantOK: false,
	},
	{
		g: []set{
			A: linksTo(B),
			B: linksTo(C),
			C: nil,
		},
		builder: simple.NewUndirectedGraph(),
		h:       map[int64]float64{B: 1},
		iter:    2,
		tol:     1e-6,
		want: map[int64]float64{
			A: 0, B: 1, C: 0,
		},
		wantOK: false,
	},
}

func TestDiffuseToEquilibrium(t *testing.T) {
	for i, test := range diffuseToEquilibriumTests {
		g := test.builder
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(int64(u)) {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		var wantTemp float64
		for _, v := range test.h {
			wantTemp += v
		}
		got, ok := DiffuseToEquilibrium(nil, test.h, NewRandomWalkLaplacian(g, test.damp), test.tol*test.tol, test.iter)
		if ok != test.wantOK {
			t.Errorf("unexpected success value for test %d: got:%t want:%t", i, ok, test.wantOK)
		}
		prec := -int(math.Log10(test.tol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[int64(n)], test.want[int64(n)], test.tol, test.tol) {
				t.Errorf("unexpected DiffuseToEquilibrium result for test %d:\ngot: %v\nwant:%v",
					i, orderedFloats(got, prec), orderedFloats(test.want, prec))
				break
			}
		}

		var gotTemp float64
		for _, v := range got {
			gotTemp += v
		}
		gotTemp /= float64(len(got))
		wantTemp /= float64(len(got))
		if !floats.EqualWithinAbsOrRel(gotTemp, wantTemp, test.tol, test.tol) {
			t.Errorf("unexpected total heat for test %d: got:%v want:%v",
				i, gotTemp, wantTemp)
		}
	}
}

type builder interface {
	graph.Graph
	graph.Builder
}

func grid(d int) []set {
	dim := int64(d)
	s := make([]set, dim*dim)
	for i := range s {
		s[i] = make(set)
	}
	for i := int64(0); i < dim*dim; i++ {
		if i%dim != 0 {
			s[i][i-1] = struct{}{}
		}
		if i/dim != 0 {
			s[i][i-dim] = struct{}{}
		}
	}
	return s
}
