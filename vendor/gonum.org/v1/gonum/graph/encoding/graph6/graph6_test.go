// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph6

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
)

var testGraphs = []struct {
	g    string
	bin  string
	want []set
}{
	// Wanted graphs were obtained from showg using the input graph string.
	// The showg output is included for comparison.
	//
	// showg is available here: https://hog.grinvin.org/data/generators/decoders/showg
	{
		// Graph 1, order 0.
		g:    "?",
		bin:  "0:0",
		want: []set{},
	},
	{
		// Graph 1, order 5.
		//   0 : 2 4;
		//   1 : 3;
		//   2 : 0;
		//   3 : 1 4;
		//   4 : 0 3;
		g:   "DQc",
		bin: "5:0100101001",
		want: []set{
			0: linksToInt(2, 4),
			1: linksToInt(3),
			2: linksToInt(0),
			3: linksToInt(1, 4),
			4: linksToInt(0, 3),
		},
	},
	{
		// Graph 1, order 4.
		//   0 : 1 2 3;
		//   1 : 0 2 3;
		//   2 : 0 1 3;
		//   3 : 0 1 2;
		g:   "C~",
		bin: "4:111111",
		want: []set{
			0: linksToInt(1, 2, 3),
			1: linksToInt(0, 2, 3),
			2: linksToInt(0, 1, 3),
			3: linksToInt(0, 1, 2),
		},
	},
	{
		// Graph 1, order 6.
		//   0 : 1 3 4;
		//   1 : 0 2 5;
		//   2 : 1 3 4;
		//   3 : 0 2 5;
		//   4 : 0 2 5;
		//   5 : 1 3 4;
		g:   "ElhW",
		bin: "6:101101101001011",
		want: []set{
			0: linksToInt(1, 3, 4),
			1: linksToInt(0, 2, 5),
			2: linksToInt(1, 3, 4),
			3: linksToInt(0, 2, 5),
			4: linksToInt(0, 2, 5),
			5: linksToInt(1, 3, 4),
		},
	},
	{
		// Graph 1, order 10.
		//   0 : 1 2 3;
		//   1 : 0 4 5;
		//   2 : 0 6 7;
		//   3 : 0 8 9;
		//   4 : 1 6 8;
		//   5 : 1 7 9;
		//   6 : 2 4 9;
		//   7 : 2 5 8;
		//   8 : 3 4 7;
		//   9 : 3 5 6;
		g:   "IsP@PGXD_",
		bin: "10:110100010001000001010001001000011001000101100",
		want: []set{
			0: linksToInt(1, 2, 3),
			1: linksToInt(0, 4, 5),
			2: linksToInt(0, 6, 7),
			3: linksToInt(0, 8, 9),
			4: linksToInt(1, 6, 8),
			5: linksToInt(1, 7, 9),
			6: linksToInt(2, 4, 9),
			7: linksToInt(2, 5, 8),
			8: linksToInt(3, 4, 7),
			9: linksToInt(3, 5, 6),
		},
	},
	{
		// Graph 1, order 17.
		//   0 : 1 15 16;
		//   1 : 0 2 5;
		//   2 : 1 3 14;
		//   3 : 2 4 16;
		//   4 : 3 5 15;
		//   5 : 1 4 6;
		//   6 : 5 7 16;
		//   7 : 6 8 11;
		//   8 : 7 9 13;
		//   9 : 8 10 16;
		//  10 : 9 11 14;
		//  11 : 7 10 12;
		//  12 : 11 13 16;
		//  13 : 8 12 14;
		//  14 : 2 10 13 15;
		//  15 : 0 4 14;
		//  16 : 0 3 6 9 12;
		g:   "PhDGGC@?G?_H?@?Gc@KO@cc_",
		bin: "17:1010010001010010000010000001000000010000000010000000001000000010010000000000010000000010001001000000010011000100000000011001001001001000",
		want: []set{
			0:  linksToInt(1, 15, 16),
			1:  linksToInt(0, 2, 5),
			2:  linksToInt(1, 3, 14),
			3:  linksToInt(2, 4, 16),
			4:  linksToInt(3, 5, 15),
			5:  linksToInt(1, 4, 6),
			6:  linksToInt(5, 7, 16),
			7:  linksToInt(6, 8, 11),
			8:  linksToInt(7, 9, 13),
			9:  linksToInt(8, 10, 16),
			10: linksToInt(9, 11, 14),
			11: linksToInt(7, 10, 12),
			12: linksToInt(11, 13, 16),
			13: linksToInt(8, 12, 14),
			14: linksToInt(2, 10, 13, 15),
			15: linksToInt(0, 4, 14),
			16: linksToInt(0, 3, 6, 9, 12),
		},
	},
}

func TestNumberOf(t *testing.T) {
	for _, test := range testGraphs {
		n := numberOf(Graph(test.g))
		if n != int64(len(test.want)) {
			t.Errorf("unexpected graph n: got:%d want:%d", n, len(test.want))
		}
	}
}

func TestGoString(t *testing.T) {
	for _, test := range testGraphs {
		gosyntax := Graph(test.g).GoString()
		if gosyntax != test.bin {
			t.Errorf("unexpected graph string: got:%s want:%s", gosyntax, test.bin)
		}
	}
}

func TestGraph(t *testing.T) {
	for _, test := range testGraphs {
		g := Graph(test.g)
		if !IsValid(g) {
			t.Errorf("unexpected invalid graph %q", g)
		}
		nodes := g.Nodes()
		if nodes.Len() != len(test.want) {
			t.Errorf("unexpected graph n: got:%d want:%d", nodes.Len(), len(test.want))
		}
		got := make([]set, nodes.Len())
		for nodes.Next() {
			n := nodes.Node()
			got[n.ID()] = linksTo(graph.NodesOf(g.From(n.ID()))...)
		}
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected graph: got:%v want:%v", got, test.want)
		}
		for i, s := range got {
			f := g.From(int64(i)).Len()
			if f != len(s) {
				t.Errorf("unexpected number of nodes from %d: got:%d want:%d", i, f, len(s))
			}
		}

		dst := simple.NewUndirectedGraph()
		graph.Copy(dst, g)
		enc := Encode(dst)
		if enc != g {
			t.Errorf("unexpected round trip: got:%q want:%q", enc, g)
		}
	}
}

type set map[int]struct{}

func linksToInt(nodes ...int) map[int]struct{} {
	s := make(map[int]struct{})
	for _, n := range nodes {
		s[n] = struct{}{}
	}
	return s
}

func linksTo(nodes ...graph.Node) map[int]struct{} {
	s := make(map[int]struct{})
	for _, n := range nodes {
		s[int(n.ID())] = struct{}{}
	}
	return s
}
