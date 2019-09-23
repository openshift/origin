// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"testing"

	"gonum.org/v1/gonum/graph/simple"
)

func TestTunableClusteringScaleFree(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for m := 0; m < n; m++ {
			for p := 0.; p <= 1; p += 0.1 {
				g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
				err := TunableClusteringScaleFree(g, n, m, p, nil)
				if err != nil {
					t.Fatalf("unexpected error: n=%d, m=%d, p=%v: %v", n, m, p, err)
				}
				if g.addBackwards {
					t.Errorf("edge added with From.ID > To.ID: n=%d, m=%d, p=%v", n, m, p)
				}
				if g.addSelfLoop {
					t.Errorf("unexpected self edge: n=%d, m=%d, p=%v", n, m, p)
				}
				if g.addMultipleEdge {
					t.Errorf("unexpected multiple edge: n=%d, m=%d, p=%v", n, m, p)
				}
			}
		}
	}
}

func TestPreferentialAttachment(t *testing.T) {
	for n := 2; n <= 20; n++ {
		for m := 0; m < n; m++ {
			g := &gnUndirected{UndirectedBuilder: simple.NewUndirectedGraph()}
			err := PreferentialAttachment(g, n, m, nil)
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
