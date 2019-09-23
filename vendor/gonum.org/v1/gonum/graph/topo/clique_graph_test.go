// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package topo

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/graph/encoding"
	"gonum.org/v1/gonum/graph/encoding/dot"
	"gonum.org/v1/gonum/graph/simple"
)

var cliqueGraphTests = []struct {
	name string
	g    []intset
	want string
}{
	{
		name: "simple",
		g: []intset{
			0: linksTo(1, 2, 4, 6),
			1: linksTo(2, 4, 6),
			2: linksTo(3, 6),
			3: linksTo(4, 5),
			4: linksTo(6),
			5: nil,
			6: nil,
		},
		want: `strict graph {
  // Node definitions.
  0 [nodes="[0 1 2 6]"];
  1 [nodes="[0 1 4 6]"];
  2 [nodes="[2 3]"];
  3 [nodes="[3 4]"];
  4 [nodes="[3 5]"];

  // Edge definitions.
  0 -- 1 [nodes="[0 1 6]"];
  0 -- 2 [nodes="[2]"];
  1 -- 3 [nodes="[4]"];
  2 -- 3 [nodes="[3]"];
  2 -- 4 [nodes="[3]"];
  3 -- 4 [nodes="[3]"];
}`,
	},
	{
		name: "Batagelj-Zaversnik Graph",
		g:    batageljZaversnikGraph,
		want: `strict graph {
  // Node definitions.
  0 [nodes="[0]"];
  1 [nodes="[1 2]"];
  2 [nodes="[1 3]"];
  3 [nodes="[2 4]"];
  4 [nodes="[3 4]"];
  5 [nodes="[4 5]"];
  6 [nodes="[6 7 8 14]"];
  7 [nodes="[7 11 12]"];
  8 [nodes="[9 11]"];
  9 [nodes="[10 11]"];
  10 [nodes="[12 18]"];
  11 [nodes="[13 14 15]"];
  12 [nodes="[14 15 17]"];
  13 [nodes="[15 16]"];
  14 [nodes="[17 18 19 20]"];

  // Edge definitions.
  1 -- 2 [nodes="[1]"];
  1 -- 3 [nodes="[2]"];
  2 -- 4 [nodes="[3]"];
  3 -- 4 [nodes="[4]"];
  3 -- 5 [nodes="[4]"];
  4 -- 5 [nodes="[4]"];
  6 -- 7 [nodes="[7]"];
  6 -- 11 [nodes="[14]"];
  6 -- 12 [nodes="[14]"];
  7 -- 8 [nodes="[11]"];
  7 -- 9 [nodes="[11]"];
  7 -- 10 [nodes="[12]"];
  8 -- 9 [nodes="[11]"];
  10 -- 14 [nodes="[18]"];
  11 -- 12 [nodes="[14 15]"];
  11 -- 13 [nodes="[15]"];
  12 -- 13 [nodes="[15]"];
  12 -- 14 [nodes="[17]"];
}`,
	},
}

func TestCliqueGraph(t *testing.T) {
	for _, test := range cliqueGraphTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}
		dst := simple.NewUndirectedGraph()
		CliqueGraph(dst, g)

		b, _ := dot.Marshal(dst, "", "", "  ")
		got := string(b)

		if got != test.want {
			t.Errorf("unexpected clique graph result for %q: got:\n%s\nwant:\n%s", test.name, got, test.want)
		}
	}
}

func (n Clique) Attributes() []encoding.Attribute {
	return []encoding.Attribute{{Key: "nodes", Value: fmt.Sprintf(`"%v"`, n.Nodes())}}
}

func (e CliqueGraphEdge) Attributes() []encoding.Attribute {
	return []encoding.Attribute{{Key: "nodes", Value: fmt.Sprintf(`"%v"`, e.Nodes())}}
}

func BenchmarkCliqueGraph(b *testing.B) {
	for _, test := range cliqueGraphTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		b.Run(test.name, func(b *testing.B) {
			var dst *simple.UndirectedGraph
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				dst = simple.NewUndirectedGraph()
				b.StartTimer()
				CliqueGraph(dst, g)
			}
		})
	}
}
