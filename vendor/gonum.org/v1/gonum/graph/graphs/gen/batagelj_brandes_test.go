// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/simple"
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
	case g.UndirectedBuilder.HasEdgeBetween(e.From().ID(), e.To().ID()):
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
	case g.DirectedBuilder.HasEdgeFromTo(e.From().ID(), e.To().ID()):
		g.addMultipleEdge = true
	}

	g.DirectedBuilder.SetEdge(e)
}

func TestGnpUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for p := 0.; p <= 1; p += 0.1 {
			g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
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
			g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph()}
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
			g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
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
			g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph()}
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
				g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
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
				g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph()}
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

func TestPowerLawUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= 5; d++ {
			g := multi.NewUndirectedGraph()
			err := PowerLaw(g, n, d, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, d=%d: %v", n, d, err)
			}

			nodes := g.Nodes()
			if len(nodes) != n {
				t.Errorf("unexpected number of nodes in graph: n=%d, d=%d: got:%d", n, d, len(nodes))
			}

			for _, u := range nodes {
				uid := u.ID()
				var lines int
				for _, v := range g.From(uid) {
					lines += len(g.Lines(uid, v.ID()))
				}
				if lines < d {
					t.Errorf("unexpected degree below d: n=%d, d=%d: got:%d", n, d, lines)
					break
				}
			}
		}
	}
}

func TestPowerLawDirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= 5; d++ {
			g := multi.NewDirectedGraph()
			err := PowerLaw(g, n, d, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, d=%d: %v", n, d, err)
			}

			nodes := g.Nodes()
			if len(nodes) != n {
				t.Errorf("unexpected number of nodes in graph: n=%d, d=%d: got:%d", n, d, len(nodes))
			}

			for _, u := range nodes {
				uid := u.ID()
				var lines int
				for _, v := range g.From(uid) {
					lines += len(g.Lines(uid, v.ID()))
				}
				if lines < d {
					t.Errorf("unexpected degree below d: n=%d, d=%d: got:%d", n, d, lines)
					break
				}
			}
		}
	}
}

func TestBipartitePowerLawUndirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= 5; d++ {
			g := multi.NewUndirectedGraph()
			p1, p2, err := BipartitePowerLaw(g, n, d, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, d=%d: %v", n, d, err)
			}

			nodes := g.Nodes()
			if len(nodes) != 2*n {
				t.Errorf("unexpected number of nodes in graph: n=%d, d=%d: got:%d", n, d, len(nodes))
			}
			if len(p1) != n {
				t.Errorf("unexpected number of nodes in p1: n=%d, d=%d: got:%d", n, d, len(p1))
			}
			if len(p2) != n {
				t.Errorf("unexpected number of nodes in p2: n=%d, d=%d: got:%d", n, d, len(p2))
			}

			p1s := make(set.Nodes)
			for _, u := range p1 {
				p1s.Add(u)
			}
			p2s := make(set.Nodes)
			for _, u := range p2 {
				p2s.Add(u)
			}
			o := make(set.Nodes)
			if o.Intersect(p1s, p2s); len(o) != 0 {
				t.Errorf("unexpected overlap in partition membership: n=%d, d=%d: got:%d", n, d, len(o))
			}

			for _, u := range nodes {
				uid := u.ID()
				var lines int
				for _, v := range g.From(uid) {
					lines += len(g.Lines(uid, v.ID()))
				}
				if lines < d {
					t.Errorf("unexpected degree below d: n=%d, d=%d: got:%d", n, d, lines)
					break
				}
			}
		}
	}
}

func TestBipartitePowerLawDirected(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for d := 1; d <= 5; d++ {
			g := multi.NewDirectedGraph()
			p1, p2, err := BipartitePowerLaw(g, n, d, nil)
			if err != nil {
				t.Fatalf("unexpected error: n=%d, d=%d: %v", n, d, err)
			}

			nodes := g.Nodes()
			if len(nodes) != 2*n {
				t.Errorf("unexpected number of nodes in graph: n=%d, d=%d: got:%d", n, d, len(nodes))
			}
			if len(p1) != n {
				t.Errorf("unexpected number of nodes in p1: n=%d, d=%d: got:%d", n, d, len(p1))
			}
			if len(p2) != n {
				t.Errorf("unexpected number of nodes in p2: n=%d, d=%d: got:%d", n, d, len(p2))
			}

			p1s := make(set.Nodes)
			for _, u := range p1 {
				p1s.Add(u)
			}
			p2s := make(set.Nodes)
			for _, u := range p2 {
				p2s.Add(u)
			}
			o := make(set.Nodes)
			if o.Intersect(p1s, p2s); len(o) != 0 {
				t.Errorf("unexpected overlap in partition membership: n=%d, d=%d: got:%d", n, d, len(o))
			}

			for _, u := range nodes {
				uid := u.ID()
				var lines int
				for _, v := range g.From(uid) {
					lines += len(g.Lines(uid, v.ID()))
				}
				if lines < d {
					t.Errorf("unexpected degree below d: n=%d, d=%d: got:%d", n, d, lines)
					break
				}
			}
		}
	}
}
