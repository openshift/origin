// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"path/filepath"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/spatial/r2"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
)

// tag is modified in isomap_noasm_test.go to "_noasm" when any
// build tag prevents use of the assembly numerical kernels.
var tag string

var isomapR2Tests = []struct {
	name string
	g    graph.Graph
}{
	{
		name: "line_isomap",
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
	},
	{
		name: "square_isomap",
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
	},
	{
		name: "tetrahedron_isomap",
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
	},
	{
		name: "sheet_isomap",
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
	},
	{
		name: "tube_isomap",
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
	},
	{
		name: "wp_page_isomap", // https://en.wikipedia.org/wiki/PageRank#/media/File:PageRanks-Example.jpg
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
	},
}

func TestIsomapR2(t *testing.T) {
	for _, test := range isomapR2Tests {
		o := NewOptimizerR2(test.g, IsomapR2{}.Update)
		var n int
		for o.Update() {
			n++
		}
		p, err := plot.New()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		p.Add(render{o})
		p.HideAxes()
		path := filepath.Join("testdata", test.name+tag+".png")
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
