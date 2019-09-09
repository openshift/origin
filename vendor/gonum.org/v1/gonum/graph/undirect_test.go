// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph_test

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/mat"
)

type weightedDirectedBuilder interface {
	graph.WeightedBuilder
	graph.WeightedDirected
}

var weightedDirectedGraphs = []struct {
	skipUnweighted bool

	g      func() weightedDirectedBuilder
	edges  []simple.WeightedEdge
	absent float64
	merge  func(x, y float64, xe, ye graph.Edge) float64

	want mat.Matrix
}{
	{
		g: func() weightedDirectedBuilder { return simple.NewWeightedDirectedGraph(0, 0) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		want: mat.NewSymDense(3, []float64{
			0, (1. + 2.) / 2., 0,
			(1. + 2.) / 2., 0, 1. / 2.,
			0, 1. / 2., 0,
		}),
	},
	{
		g: func() weightedDirectedBuilder { return simple.NewWeightedDirectedGraph(0, 0) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		absent: 1,
		merge:  func(x, y float64, _, _ graph.Edge) float64 { return math.Sqrt(x * y) },
		want: mat.NewSymDense(3, []float64{
			0, math.Sqrt(1 * 2), 0,
			math.Sqrt(1 * 2), 0, math.Sqrt(1 * 1),
			0, math.Sqrt(1 * 1), 0,
		}),
	},
	{
		skipUnweighted: true, // The min merge function cannot be used in the unweighted case.

		g: func() weightedDirectedBuilder { return simple.NewWeightedDirectedGraph(0, 0) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		merge: func(x, y float64, _, _ graph.Edge) float64 { return math.Min(x, y) },
		want: mat.NewSymDense(3, []float64{
			0, math.Min(1, 2), 0,
			math.Min(1, 2), 0, math.Min(1, 0),
			0, math.Min(1, 0), 0,
		}),
	},
	{
		g: func() weightedDirectedBuilder { return simple.NewWeightedDirectedGraph(0, 0) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		merge: func(x, y float64, xe, ye graph.Edge) float64 {
			if xe == nil {
				return y
			}
			if ye == nil {
				return x
			}
			return math.Min(x, y)
		},
		want: mat.NewSymDense(3, []float64{
			0, math.Min(1, 2), 0,
			math.Min(1, 2), 0, 1,
			0, 1, 0,
		}),
	},
	{
		g: func() weightedDirectedBuilder { return simple.NewWeightedDirectedGraph(0, 0) },
		edges: []simple.WeightedEdge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		merge: func(x, y float64, _, _ graph.Edge) float64 { return math.Max(x, y) },
		want: mat.NewSymDense(3, []float64{
			0, math.Max(1, 2), 0,
			math.Max(1, 2), 0, math.Max(1, 0),
			0, math.Max(1, 0), 0,
		}),
	},
}

func TestUndirect(t *testing.T) {
	for i, test := range weightedDirectedGraphs {
		if test.skipUnweighted {
			continue
		}
		g := test.g()
		for _, e := range test.edges {
			g.SetWeightedEdge(e)
		}

		src := graph.Undirect{G: g}
		nodes := graph.NodesOf(src.Nodes())
		dst := simple.NewUndirectedMatrixFrom(nodes, 0, 0, 0)
		for _, u := range nodes {
			for _, v := range graph.NodesOf(src.From(u.ID())) {
				dst.SetEdge(src.Edge(u.ID(), v.ID()))
			}
		}

		want := unit{test.want}
		if !mat.Equal(dst.Matrix(), want) {
			t.Errorf("unexpected result for case %d:\ngot:\n%.4v\nwant:\n%.4v", i,
				mat.Formatted(dst.Matrix()),
				mat.Formatted(want),
			)
		}
	}
}

func TestUndirectWeighted(t *testing.T) {
	for i, test := range weightedDirectedGraphs {
		g := test.g()
		for _, e := range test.edges {
			g.SetWeightedEdge(e)
		}

		src := graph.UndirectWeighted{G: g, Absent: test.absent, Merge: test.merge}
		nodes := graph.NodesOf(src.Nodes())
		dst := simple.NewUndirectedMatrixFrom(nodes, 0, 0, 0)
		for _, u := range nodes {
			for _, v := range graph.NodesOf(src.From(u.ID())) {
				dst.SetWeightedEdge(src.WeightedEdge(u.ID(), v.ID()))
			}
		}

		if !mat.Equal(dst.Matrix(), test.want) {
			t.Errorf("unexpected result for case %d:\ngot:\n%.4v\nwant:\n%.4v", i,
				mat.Formatted(dst.Matrix()),
				mat.Formatted(test.want),
			)
		}
	}
}

type unit struct {
	mat.Matrix
}

func (m unit) At(i, j int) float64 {
	v := m.Matrix.At(i, j)
	if v == 0 {
		return 0
	}
	return 1
}
