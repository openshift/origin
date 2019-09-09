// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package digraph6

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
	// showg with dgraph6 support, in nauty v2.7, is available here: http://pallini.di.uniroma1.it/
	{
		// Graph 1, order 0.
		g:    "&?",
		bin:  "0:0",
		want: []set{},
	},
	{
		// Graph 1, order 5.
		//   0 : 2 4;
		//   1 : ;
		//   2 : ;
		//   3 : 1 4;
		//   4 : ;
		g:   "&DI?AO?",
		bin: "5:0010100000000000100100000",
		want: []set{
			0: linksToInt(2, 4),
			1: linksToInt(),
			2: linksToInt(),
			3: linksToInt(1, 4),
			4: linksToInt(),
		},
	},
	{
		// Graph 1, order 5.
		//   0 : 1 3;
		//   1 : 0 2 3 4;
		//   2 : 0 1 3 4;
		//   3 : 0 2;
		//   4 : 0 1 2 3;
		g:   "&DT^\\N?",
		bin: "5:0101010111110111010011110",
		want: []set{
			0: linksToInt(1, 3),
			1: linksToInt(0, 2, 3, 4),
			2: linksToInt(0, 1, 3, 4),
			3: linksToInt(0, 2),
			4: linksToInt(0, 1, 2, 3),
		},
	},
	{
		// Graph 1, order 5.
		//   0 : ;
		//   1 : 3;
		//   2 : 0;
		//   3 : ;
		//   4 : 0 3;
		g:   "&D?I?H?",
		bin: "5:0000000010100000000010010",
		want: []set{
			0: linksToInt(),
			1: linksToInt(3),
			2: linksToInt(0),
			3: linksToInt(),
			4: linksToInt(0, 3),
		},
	},
	{
		// Graph 1, order 6.
		//   0 : 1 2 5;
		//   1 : 2 3 4 5;
		//   2 : 3 4;
		//   3 : 0 4 5;
		//   4 : 0 5;
		//   5 : 2;
		g:   "&EXNEb`G",
		bin: "6:011001001111000110100011100001001000",
		want: []set{
			0: linksToInt(1, 2, 5),
			1: linksToInt(2, 3, 4, 5),
			2: linksToInt(3, 4),
			3: linksToInt(0, 4, 5),
			4: linksToInt(0, 5),
			5: linksToInt(2),
		},
	},
	{
		// Graph 1, order 9.
		//   0 : 1 3 5 7 8;
		//   1 : 2 3 4 7 8;
		//   2 : 0 3 4;
		//   3 : 4 5 6 7 8;
		//   4 : 0 5 6 8;
		//   5 : 1 2 6 7 8;
		//   6 : 0 1 2 7 8;
		//   7 : 2 4 8;
		//   8 : 2;
		g:   "&HTXre?^`jFwXPG?",
		bin: "9:010101011001110011100110000000011111100001101011000111111000011001010001001000000",
		want: []set{
			0: linksToInt(1, 3, 5, 7, 8),
			1: linksToInt(2, 3, 4, 7, 8),
			2: linksToInt(0, 3, 4),
			3: linksToInt(4, 5, 6, 7, 8),
			4: linksToInt(0, 5, 6, 8),
			5: linksToInt(1, 2, 6, 7, 8),
			6: linksToInt(0, 1, 2, 7, 8),
			7: linksToInt(2, 4, 8),
			8: linksToInt(2),
		},
	},
	{
		// Graph 1, order 12.
		//   0 : 1 2 3 4 5 6 7 8 9 10 11;
		//   1 : 2 3 4 5 6 7 8 11;
		//   2 : 3 4 5 6 7 8 9 11;
		//   3 : 4 5 6 7 8 10 11;
		//   4 : 5 6 7 8 9 11;
		//   5 : 6 7 9 10;
		//   6 : 7 8 9 10 11;
		//   7 : 8 9 10 11;
		//   8 : 5 9 10;
		//   9 : 1 3 10;
		//  10 : 1 2 4 11;
		//  11 : 5 8 9;
		g:   "&K^~NxF|Bz@|?u?^?N@ESAY@@K",
		bin: "12:011111111111001111111001000111111101000011111011000001111101000000110110000000011111000000001111000001000110010100000010011010000001000001001100",
		want: []set{
			0:  linksToInt(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11),
			1:  linksToInt(2, 3, 4, 5, 6, 7, 8, 11),
			2:  linksToInt(3, 4, 5, 6, 7, 8, 9, 11),
			3:  linksToInt(4, 5, 6, 7, 8, 10, 11),
			4:  linksToInt(5, 6, 7, 8, 9, 11),
			5:  linksToInt(6, 7, 9, 10),
			6:  linksToInt(7, 8, 9, 10, 11),
			7:  linksToInt(8, 9, 10, 11),
			8:  linksToInt(5, 9, 10),
			9:  linksToInt(1, 3, 10),
			10: linksToInt(1, 2, 4, 11),
			11: linksToInt(5, 8, 9),
		},
	},
	{
		// Graph 1, order 17.
		//   0 : 1 2 3 4 5 6 7 8 9 10 11 12 14 15 16;
		//   1 : 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16;
		//   2 : 3 4 5 6 7 8 9 10 11 12 13 14 16;
		//   3 : 4 5 6 7 8 9 10 11 12 13 14 15 16;
		//   4 : 5 6 7 8 9 10 11 12 13 14 15 16;
		//   5 : 6 7 8 9 10 11 12 13 14 15 16;
		//   6 : 7 8 9 10 11 12 13 14 15;
		//   7 : 8 9 10 11 12 13 14 15 16;
		//   8 : 9 10 11 12 13 15 16;
		//   9 : 10 11 12 13 14 15 16;
		//  10 : 11 12 13 14 16;
		//  11 : 12 13 14 15 16;
		//  12 : 13 14 15 16;
		//  13 : 0 14 15 16;
		//  14 : 8 15;
		//  15 : 2 10 16;
		//  16 : 6 14;
		g:   "&P^~m^~{^~g^~o^~_^~?^{?^{?^W?^o?]_?^??^??[?_P?OOGA?",
		bin: "17:0111111111111011100111111111111111000111111111111010000111111111111100000111111111111000000111111111110000000111111111000000000111111111000000000111110110000000000111111100000000000111101000000000000111110000000000000111110000000000000111000000001000000100010000000100000100000010000000100",
		want: []set{
			0:  linksToInt(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 14, 15, 16),
			1:  linksToInt(2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			2:  linksToInt(3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 16),
			3:  linksToInt(4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			4:  linksToInt(5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			5:  linksToInt(6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16),
			6:  linksToInt(7, 8, 9, 10, 11, 12, 13, 14, 15),
			7:  linksToInt(8, 9, 10, 11, 12, 13, 14, 15, 16),
			8:  linksToInt(9, 10, 11, 12, 13, 15, 16),
			9:  linksToInt(10, 11, 12, 13, 14, 15, 16),
			10: linksToInt(11, 12, 13, 14, 16),
			11: linksToInt(12, 13, 14, 15, 16),
			12: linksToInt(13, 14, 15, 16),
			13: linksToInt(0, 14, 15, 16),
			14: linksToInt(8, 15),
			15: linksToInt(2, 10, 16),
			16: linksToInt(6, 14),
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
			t.Errorf("unexpected graph:\ngot: %v\nwant:%v", got, test.want)
		}
		reverse := make([]set, len(got))
		for i := range reverse {
			reverse[i] = make(set)
		}
		for i, s := range got {
			for j := range s {
				reverse[j][i] = struct{}{}
			}
		}
		for i, s := range got {
			from := g.From(int64(i)).Len()
			if from != len(s) {
				t.Errorf("unexpected number of nodes from %d: got:%d want:%d", i, from, len(s))
			}
			to := g.To(int64(i)).Len()
			if to != len(reverse[i]) {
				t.Errorf("unexpected number of nodes to %d: got:%d want:%d", i, to, len(reverse))
			}
		}

		dst := simple.NewDirectedGraph()
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
