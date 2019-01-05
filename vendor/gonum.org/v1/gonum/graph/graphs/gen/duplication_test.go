// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

type duplication struct {
	UndirectedMutator
	addBackwards    bool
	addSelfLoop     bool
	addMultipleEdge bool
}

func (g *duplication) SetEdge(e graph.Edge) {
	switch {
	case e.From().ID() == e.To().ID():
		g.addSelfLoop = true
		return
	case e.From().ID() > e.To().ID():
		g.addBackwards = true
	case g.UndirectedMutator.HasEdgeBetween(e.From().ID(), e.To().ID()):
		g.addMultipleEdge = true
	}

	g.UndirectedMutator.SetEdge(e)
}

func TestDuplication(t *testing.T) {
	for n := 2; n <= 50; n++ {
		for alpha := 0.1; alpha <= 1; alpha += 0.1 {
			for delta := 0.; delta <= 1; delta += 0.2 {
				for sigma := 0.; sigma <= 1; sigma += 0.2 {
					g := &duplication{UndirectedMutator: simple.NewUndirectedGraph()}
					err := Duplication(g, n, delta, alpha, sigma, nil)
					if err != nil {
						t.Fatalf("unexpected error: n=%d, alpha=%v, delta=%v sigma=%v: %v", n, alpha, delta, sigma, err)
					}
					if g.addBackwards {
						t.Errorf("edge added with From.ID > To.ID: n=%d, alpha=%v, delta=%v sigma=%v", n, alpha, delta, sigma)
					}
					if g.addSelfLoop {
						t.Errorf("unexpected self edge: n=%d, alpha=%v, delta=%v sigma=%v", n, alpha, delta, sigma)
					}
					if g.addMultipleEdge {
						t.Errorf("unexpected multiple edge: n=%d, alpha=%v, delta=%v sigma=%v", n, alpha, delta, sigma)
					}
				}
			}
		}
	}
}
