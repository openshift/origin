// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"testing"

	"gonum.org/v1/gonum/graph/simple"
)

var smallWorldDimensionParameters = [][]int{
	{50},
	{10, 10},
	{6, 5, 4},
}

func TestNavigableSmallWorldUndirected(t *testing.T) {
	for p := 1; p < 5; p++ {
		for q := 0; q < 10; q++ {
			for r := 0.5; r < 10; r++ {
				for _, dims := range smallWorldDimensionParameters {
					g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
					err := NavigableSmallWorld(g, dims, p, q, r, nil)
					n := 1
					for _, d := range dims {
						n *= d
					}
					if err != nil {
						t.Fatalf("unexpected error: dims=%v n=%d, p=%d, q=%d, r=%v: %v", dims, n, p, q, r, err)
					}
					if g.addBackwards {
						t.Errorf("edge added with From.ID > To.ID: dims=%v n=%d, p=%d, q=%d, r=%v", dims, n, p, q, r)
					}
					if g.addSelfLoop {
						t.Errorf("unexpected self edge: dims=%v n=%d, p=%d, q=%d, r=%v", dims, n, p, q, r)
					}
					if g.addMultipleEdge {
						t.Errorf("unexpected multiple edge: dims=%v n=%d, p=%d, q=%d, r=%v", dims, n, p, q, r)
					}
				}
			}
		}
	}
}

func TestNavigableSmallWorldDirected(t *testing.T) {
	for p := 1; p < 5; p++ {
		for q := 0; q < 10; q++ {
			for r := 0.5; r < 10; r++ {
				for _, dims := range smallWorldDimensionParameters {
					g := &gnDirected{DirectedBuilder: simple.NewDirectedGraph()}
					err := NavigableSmallWorld(g, dims, p, q, r, nil)
					n := 1
					for _, d := range dims {
						n *= d
					}
					if err != nil {
						t.Fatalf("unexpected error: dims=%v n=%d, p=%d, q=%d, r=%v: %v", dims, n, p, q, r, err)
					}
					if g.addSelfLoop {
						t.Errorf("unexpected self edge: dims=%v n=%d, p=%d, q=%d, r=%v", dims, n, p, q, r)
					}
					if g.addMultipleEdge {
						t.Errorf("unexpected multiple edge: dims=%v n=%d, p=%d, q=%d, r=%v", dims, n, p, q, r)
					}
				}
			}
		}
	}
}
