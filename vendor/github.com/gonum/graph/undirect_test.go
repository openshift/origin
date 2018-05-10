// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph_test

import (
	"math"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
	"github.com/gonum/matrix/mat64"
)

var directedGraphs = []struct {
	g      func() graph.DirectedBuilder
	edges  []simple.Edge
	absent float64
	merge  func(x, y float64, xe, ye graph.Edge) float64

	want mat64.Matrix
}{
	{
		g: func() graph.DirectedBuilder { return simple.NewDirectedGraph(0, 0) },
		edges: []simple.Edge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		want: mat64.NewSymDense(3, []float64{
			0, (1. + 2.) / 2., 0,
			(1. + 2.) / 2., 0, 1. / 2.,
			0, 1. / 2., 0,
		}),
	},
	{
		g: func() graph.DirectedBuilder { return simple.NewDirectedGraph(0, 0) },
		edges: []simple.Edge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		absent: 1,
		merge:  func(x, y float64, _, _ graph.Edge) float64 { return math.Sqrt(x * y) },
		want: mat64.NewSymDense(3, []float64{
			0, math.Sqrt(1 * 2), 0,
			math.Sqrt(1 * 2), 0, math.Sqrt(1 * 1),
			0, math.Sqrt(1 * 1), 0,
		}),
	},
	{
		g: func() graph.DirectedBuilder { return simple.NewDirectedGraph(0, 0) },
		edges: []simple.Edge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		merge: func(x, y float64, _, _ graph.Edge) float64 { return math.Min(x, y) },
		want: mat64.NewSymDense(3, []float64{
			0, math.Min(1, 2), 0,
			math.Min(1, 2), 0, math.Min(1, 0),
			0, math.Min(1, 0), 0,
		}),
	},
	{
		g: func() graph.DirectedBuilder { return simple.NewDirectedGraph(0, 0) },
		edges: []simple.Edge{
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
		want: mat64.NewSymDense(3, []float64{
			0, math.Min(1, 2), 0,
			math.Min(1, 2), 0, 1,
			0, 1, 0,
		}),
	},
	{
		g: func() graph.DirectedBuilder { return simple.NewDirectedGraph(0, 0) },
		edges: []simple.Edge{
			{F: simple.Node(0), T: simple.Node(1), W: 2},
			{F: simple.Node(1), T: simple.Node(0), W: 1},
			{F: simple.Node(1), T: simple.Node(2), W: 1},
		},
		merge: func(x, y float64, _, _ graph.Edge) float64 { return math.Max(x, y) },
		want: mat64.NewSymDense(3, []float64{
			0, math.Max(1, 2), 0,
			math.Max(1, 2), 0, math.Max(1, 0),
			0, math.Max(1, 0), 0,
		}),
	},
}

func TestUndirect(t *testing.T) {
	for _, test := range directedGraphs {
		g := test.g()
		for _, e := range test.edges {
			g.SetEdge(e)
		}

		src := graph.Undirect{G: g, Absent: test.absent, Merge: test.merge}
		dst := simple.NewUndirectedMatrixFrom(src.Nodes(), 0, 0, 0)
		for _, u := range src.Nodes() {
			for _, v := range src.From(u) {
				dst.SetEdge(src.Edge(u, v))
			}
		}

		if !mat64.Equal(dst.Matrix(), test.want) {
			t.Errorf("unexpected result:\ngot:\n%.4v\nwant:\n%.4v",
				mat64.Formatted(dst.Matrix()),
				mat64.Formatted(test.want),
			)
		}
	}
}
