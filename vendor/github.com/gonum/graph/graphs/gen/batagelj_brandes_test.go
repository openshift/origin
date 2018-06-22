// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"math"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
)

type gnUndirected struct {
	graph.UndirectedBuilder
	addBackwards    bool
	addSelfLoop     bool
	addMultipleEdge bool
}

func (g *gnUndirected) SetEdge(e graph.Edge) {
	switch {
	case e.From().ID() == e.To().ID():
		g.addSelfLoop = true
		return
	case e.From().ID() > e.To().ID():
		g.addBackwards = true
	case g.UndirectedBuilder.HasEdgeBetween(e.From(), e.To()):
		g.addMultipleEdge = true
	}

	g.UndirectedBuilder.SetEdge(e)
}

type gnDirected struct {
	graph.DirectedBuilder
	addSelfLoop     bool
	addMultipleEdge bool
}

func (g *gnDirected) SetEdge(e graph.Edge) {
	switch {
	case e.From().ID() == e.To().ID():
		g.addSelfLoop = true
		return
	case g.DirectedBuilder.HasEdgeFromTo(e.From(), e.To()):
		g.addMultipleEdge = true
	}

	g.DirectedBuilder.SetEdge(e)
}

func TestGnpUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for p := 0.; p <= 1; p += 0.1 {
			g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph(0, math.Inf(1))}
			err := Gnp(g, n, p, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, p=%v: %v", n, p, err)
			}
			if g.addBackwards {
				t.Errorf("edge added with From.ID > To.ID: n=%d, p=%v", n, p)
			}
			if g.addSelfLoop {
				t.Errorf("unexpected self edge: n=%d, p=%v", n, p)
			}
			if g.addMultipleEdge {
				t.Errorf("unexpected multiple edge: n=%d, p=%v", n, p)
			}
		}
	}
}

func TestGnpDirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for p := 0.; p <= 1; p += 0.1 {
			g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph(0, math.Inf(1))}
			err := Gnp(g, n, p, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, p=%v: %v", n, p, err)
			}
			if g.addSelfLoop {
				t.Errorf("unexpected self edge: n=%d, p=%v", n, p)
			}
			if g.addMultipleEdge {
				t.Errorf("unexpected multiple edge: n=%d, p=%v", n, p)
			}
		}
	}
}

func TestGnmUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		nChoose2 := (n - 1) * n / 2
		for m := 0; m <= nChoose2; m++ {
			g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph(0, math.Inf(1))}
			err := Gnm(g, n, m, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, m=%d: %v", n, m, err)
			}
			if g.addBackwards {
				t.Errorf("edge added with From.ID > To.ID: n=%d, m=%d", n, m)
			}
			if g.addSelfLoop {
				t.Errorf("unexpected self edge: n=%d, m=%d", n, m)
			}
			if g.addMultipleEdge {
				t.Errorf("unexpected multiple edge: n=%d, m=%d", n, m)
			}
		}
	}
}

func TestGnmDirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		nChoose2 := (n - 1) * n / 2
		for m := 0; m <= nChoose2*2; m++ {
			g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph(0, math.Inf(1))}
			err := Gnm(g, n, m, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, m=%d: %v", n, m, err)
			}
			if g.addSelfLoop {
				t.Errorf("unexpected self edge: n=%d, m=%d", n, m)
			}
			if g.addMultipleEdge {
				t.Errorf("unexpected multiple edge: n=%d, m=%d", n, m)
			}
		}
	}
}

func TestSmallWorldsBBUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= (n-1)/2; d++ {
			for p := 0.; p < 1; p += 0.1 {
				g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph(0, math.Inf(1))}
				err := SmallWorldsBB(g, n, d, p, nil)
				if err != nil {
					t.Fatalf("unexpected error: n=%d, d=%d, p=%v: %v", n, d, p, err)
				}
				if g.addBackwards {
					t.Errorf("edge added with From.ID > To.ID: n=%d, d=%d, p=%v", n, d, p)
				}
				if g.addSelfLoop {
					t.Errorf("unexpected self edge: n=%d, d=%d, p=%v", n, d, p)
				}
				if g.addMultipleEdge {
					t.Errorf("unexpected multiple edge: n=%d, d=%d, p=%v", n, d, p)
				}
			}
		}
	}
}

func TestSmallWorldsBBDirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= (n-1)/2; d++ {
			for p := 0.; p < 1; p += 0.1 {
				g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph(0, math.Inf(1))}
				err := SmallWorldsBB(g, n, d, p, nil)
				if err != nil {
					t.Fatalf("unexpected error: n=%d, d=%d, p=%v: %v", n, d, p, err)
				}
				if g.addSelfLoop {
					t.Errorf("unexpected self edge: n=%d, d=%d, p=%v", n, d, p)
				}
				if g.addMultipleEdge {
					t.Errorf("unexpected multiple edge: n=%d, d=%d, p=%v", n, d, p)
				}
			}
		}
	}
}
