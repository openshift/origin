// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"path/filepath"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/spatial/r2"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
)

var eadesR2Tests = []struct {
	name      string
	g         graph.Graph
	param     EadesR2
	wantIters int
}{
	{
		name: "line",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
	{
		name: "square",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
				{simple.Node(0), simple.Node(2)},
				{simple.Node(1), simple.Node(3)},
				{simple.Node(2), simple.Node(3)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
	{
		name: "tetrahedron",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
				{simple.Node(0), simple.Node(2)},
				{simple.Node(0), simple.Node(3)},
				{simple.Node(1), simple.Node(2)},
				{simple.Node(1), simple.Node(3)},
				{simple.Node(2), simple.Node(3)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
	{
		name: "sheet",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
				{simple.Node(0), simple.Node(3)},
				{simple.Node(1), simple.Node(2)},
				{simple.Node(1), simple.Node(4)},
				{simple.Node(2), simple.Node(5)},
				{simple.Node(3), simple.Node(4)},
				{simple.Node(3), simple.Node(6)},
				{simple.Node(4), simple.Node(5)},
				{simple.Node(4), simple.Node(7)},
				{simple.Node(5), simple.Node(8)},
				{simple.Node(6), simple.Node(7)},
				{simple.Node(7), simple.Node(8)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
	{
		name: "tube",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
				{simple.Node(0), simple.Node(2)},
				{simple.Node(0), simple.Node(3)},
				{simple.Node(1), simple.Node(2)},
				{simple.Node(1), simple.Node(4)},
				{simple.Node(2), simple.Node(5)},
				{simple.Node(3), simple.Node(4)},
				{simple.Node(3), simple.Node(5)},
				{simple.Node(3), simple.Node(6)},
				{simple.Node(4), simple.Node(5)},
				{simple.Node(4), simple.Node(7)},
				{simple.Node(5), simple.Node(8)},
				{simple.Node(6), simple.Node(7)},
				{simple.Node(6), simple.Node(8)},
				{simple.Node(7), simple.Node(8)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
	{
		// This test does not produce a good layout, but is here to
		// ensure that Update does not panic with steep decent rates.
		name: "tube-steep",
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(1)},
				{simple.Node(0), simple.Node(2)},
				{simple.Node(0), simple.Node(3)},
				{simple.Node(1), simple.Node(2)},
				{simple.Node(1), simple.Node(4)},
				{simple.Node(2), simple.Node(5)},
				{simple.Node(3), simple.Node(4)},
				{simple.Node(3), simple.Node(5)},
				{simple.Node(3), simple.Node(6)},
				{simple.Node(4), simple.Node(5)},
				{simple.Node(4), simple.Node(7)},
				{simple.Node(5), simple.Node(8)},
				{simple.Node(6), simple.Node(7)},
				{simple.Node(6), simple.Node(8)},
				{simple.Node(7), simple.Node(8)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 99,
	},

	{
		name: "wp_page", // https://en.wikipedia.org/wiki/PageRank#/media/File:PageRanks-Example.jpg
		g: func() graph.Graph {
			edges := []simple.Edge{
				{simple.Node(0), simple.Node(3)},
				{simple.Node(1), simple.Node(2)},
				{simple.Node(1), simple.Node(3)},
				{simple.Node(1), simple.Node(4)},
				{simple.Node(1), simple.Node(5)},
				{simple.Node(1), simple.Node(6)},
				{simple.Node(1), simple.Node(7)},
				{simple.Node(1), simple.Node(8)},
				{simple.Node(3), simple.Node(4)},
				{simple.Node(4), simple.Node(5)},
				{simple.Node(4), simple.Node(6)},
				{simple.Node(4), simple.Node(7)},
				{simple.Node(4), simple.Node(8)},
				{simple.Node(4), simple.Node(9)},
				{simple.Node(4), simple.Node(10)},
			}
			g := simple.NewUndirectedGraph()
			for _, e := range edges {
				g.SetEdge(e)
			}
			return orderedGraph{g}
		}(),
		param:     EadesR2{Repulsion: 1, Rate: 0.1, Updates: 100, Theta: 0.1, Src: rand.NewSource(1)},
		wantIters: 100,
	},
}

func TestEadesR2(t *testing.T) {
	for _, test := range eadesR2Tests {
		eades := test.param
		o := NewOptimizerR2(test.g, eades.Update)
		var n int
		for o.Update() {
			n++
		}
		if n != test.wantIters {
			t.Errorf("unexpected number of iterations for %q: got:%d want:%d", test.name, n, test.wantIters)
		}

		p, err := plot.New()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		p.Add(render{o})
		p.HideAxes()
		path := filepath.Join("testdata", test.name+".png")
		err = p.Save(10*vg.Centimeter, 10*vg.Centimeter, path)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		ok := checkRenderedLayout(t, path)
		if !ok {
			got := make(map[int64]r2.Vec)
			nodes := test.g.Nodes()
			for nodes.Next() {
				id := nodes.Node().ID()
				got[id] = o.Coord2(id)
			}
			t.Logf("got node positions: %#v", got)
		}
	}
}
