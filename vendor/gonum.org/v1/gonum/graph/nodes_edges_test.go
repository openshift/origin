// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph_test

import (
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/iterator"
	"gonum.org/v1/gonum/graph/multi"
	"gonum.org/v1/gonum/graph/simple"
)

// nodes
// edges
// weightededges
// lines
// weightedlines
// empty

var nodesOfTests = []struct {
	name  string
	nodes graph.Nodes
	want  []graph.Node
}{
	{
		name:  "nil",
		nodes: nil,
		want:  nil,
	},
	{
		name:  "empty",
		nodes: graph.Empty,
		want:  nil,
	},
	{
		name:  "no nodes",
		nodes: iterator.NewOrderedNodes(nil),
		want:  nil,
	},
	{
		name:  "implicit nodes",
		nodes: iterator.NewImplicitNodes(-1, 4, func(id int) graph.Node { return simple.Node(id) }),
		want:  []graph.Node{simple.Node(-1), simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3)},
	},
	{
		name:  "no slice method",
		nodes: basicNodes{iterator.NewOrderedNodes([]graph.Node{simple.Node(-1), simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3)})},
		want:  []graph.Node{simple.Node(-1), simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3)},
	},
	{
		name:  "explicit nodes",
		nodes: iterator.NewOrderedNodes([]graph.Node{simple.Node(-1), simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3)}),
		want:  []graph.Node{simple.Node(-1), simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3)},
	},
}

type basicNodes struct {
	graph.Nodes
}

func TestNodesOf(t *testing.T) {
	for _, test := range nodesOfTests {
		got := graph.NodesOf(test.nodes)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}
	}
}

var edgesOfTests = []struct {
	name  string
	edges graph.Edges
	want  []graph.Edge
}{
	{
		name:  "nil",
		edges: nil,
		want:  nil,
	},
	{
		name:  "empty",
		edges: graph.Empty,
		want:  nil,
	},
	{
		name:  "no edges",
		edges: iterator.NewOrderedEdges(nil),
		want:  nil,
	},
	{
		name: "no slice method",
		edges: basicEdges{iterator.NewOrderedEdges([]graph.Edge{
			simple.Edge{F: simple.Node(-1), T: simple.Node(0)},
			simple.Edge{F: simple.Node(1), T: simple.Node(2)},
			simple.Edge{F: simple.Node(3), T: simple.Node(4)},
		})},
		want: []graph.Edge{
			simple.Edge{F: simple.Node(-1), T: simple.Node(0)},
			simple.Edge{F: simple.Node(1), T: simple.Node(2)},
			simple.Edge{F: simple.Node(3), T: simple.Node(4)},
		},
	},
	{
		name: "explicit edges",
		edges: iterator.NewOrderedEdges([]graph.Edge{
			simple.Edge{F: simple.Node(-1), T: simple.Node(0)},
			simple.Edge{F: simple.Node(1), T: simple.Node(2)},
			simple.Edge{F: simple.Node(3), T: simple.Node(4)},
		}),
		want: []graph.Edge{
			simple.Edge{F: simple.Node(-1), T: simple.Node(0)},
			simple.Edge{F: simple.Node(1), T: simple.Node(2)},
			simple.Edge{F: simple.Node(3), T: simple.Node(4)},
		},
	},
}

type basicEdges struct {
	graph.Edges
}

func TestEdgesOf(t *testing.T) {
	for _, test := range edgesOfTests {
		got := graph.EdgesOf(test.edges)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}
	}
}

var weightedEdgesOfTests = []struct {
	name  string
	edges graph.WeightedEdges
	want  []graph.WeightedEdge
}{
	{
		name:  "nil",
		edges: nil,
		want:  nil,
	},
	{
		name:  "empty",
		edges: graph.Empty,
		want:  nil,
	},
	{
		name:  "no edges",
		edges: iterator.NewOrderedWeightedEdges(nil),
		want:  nil,
	},
	{
		name: "no slice method",
		edges: basicWeightedEdges{iterator.NewOrderedWeightedEdges([]graph.WeightedEdge{
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(0), W: 1},
			simple.WeightedEdge{F: simple.Node(1), T: simple.Node(2), W: 2},
			simple.WeightedEdge{F: simple.Node(3), T: simple.Node(4), W: 3},
		})},
		want: []graph.WeightedEdge{
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(0), W: 1},
			simple.WeightedEdge{F: simple.Node(1), T: simple.Node(2), W: 2},
			simple.WeightedEdge{F: simple.Node(3), T: simple.Node(4), W: 3},
		},
	},
	{
		name: "explicit edges",
		edges: iterator.NewOrderedWeightedEdges([]graph.WeightedEdge{
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(0), W: 1},
			simple.WeightedEdge{F: simple.Node(1), T: simple.Node(2), W: 2},
			simple.WeightedEdge{F: simple.Node(3), T: simple.Node(4), W: 3},
		}),
		want: []graph.WeightedEdge{
			simple.WeightedEdge{F: simple.Node(-1), T: simple.Node(0), W: 1},
			simple.WeightedEdge{F: simple.Node(1), T: simple.Node(2), W: 2},
			simple.WeightedEdge{F: simple.Node(3), T: simple.Node(4), W: 3},
		},
	},
}

type basicWeightedEdges struct {
	graph.WeightedEdges
}

func TestWeightedEdgesOf(t *testing.T) {
	for _, test := range weightedEdgesOfTests {
		got := graph.WeightedEdgesOf(test.edges)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}
	}
}

var linesOfTests = []struct {
	name  string
	lines graph.Lines
	want  []graph.Line
}{
	{
		name:  "nil",
		lines: nil,
		want:  nil,
	},
	{
		name:  "empty",
		lines: graph.Empty,
		want:  nil,
	},
	{
		name:  "no edges",
		lines: iterator.NewOrderedLines(nil),
		want:  nil,
	},
	{
		name: "no slice method",
		lines: basicLines{iterator.NewOrderedLines([]graph.Line{
			multi.Line{F: multi.Node(-1), T: multi.Node(0), UID: -1},
			multi.Line{F: multi.Node(1), T: multi.Node(2), UID: 0},
			multi.Line{F: multi.Node(3), T: multi.Node(4), UID: 1},
		})},
		want: []graph.Line{
			multi.Line{F: multi.Node(-1), T: multi.Node(0), UID: -1},
			multi.Line{F: multi.Node(1), T: multi.Node(2), UID: 0},
			multi.Line{F: multi.Node(3), T: multi.Node(4), UID: 1},
		},
	},
	{
		name: "explicit edges",
		lines: iterator.NewOrderedLines([]graph.Line{
			multi.Line{F: multi.Node(-1), T: multi.Node(0), UID: -1},
			multi.Line{F: multi.Node(1), T: multi.Node(2), UID: 0},
			multi.Line{F: multi.Node(3), T: multi.Node(4), UID: 1},
		}),
		want: []graph.Line{
			multi.Line{F: multi.Node(-1), T: multi.Node(0), UID: -1},
			multi.Line{F: multi.Node(1), T: multi.Node(2), UID: 0},
			multi.Line{F: multi.Node(3), T: multi.Node(4), UID: 1},
		},
	},
}

type basicLines struct {
	graph.Lines
}

func TestLinesOf(t *testing.T) {
	for _, test := range linesOfTests {
		got := graph.LinesOf(test.lines)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}
	}
}

var weightedLinesOfTests = []struct {
	name  string
	lines graph.WeightedLines
	want  []graph.WeightedLine
}{
	{
		name:  "nil",
		lines: nil,
		want:  nil,
	},
	{
		name:  "empty",
		lines: graph.Empty,
		want:  nil,
	},
	{
		name:  "no edges",
		lines: iterator.NewOrderedWeightedLines(nil),
		want:  nil,
	},
	{
		name: "no slice method",
		lines: basicWeightedLines{iterator.NewOrderedWeightedLines([]graph.WeightedLine{
			multi.WeightedLine{F: multi.Node(-1), T: multi.Node(0), W: 1, UID: -1},
			multi.WeightedLine{F: multi.Node(1), T: multi.Node(2), W: 2, UID: 0},
			multi.WeightedLine{F: multi.Node(3), T: multi.Node(4), W: 3, UID: 1},
		})},
		want: []graph.WeightedLine{
			multi.WeightedLine{F: multi.Node(-1), T: multi.Node(0), W: 1, UID: -1},
			multi.WeightedLine{F: multi.Node(1), T: multi.Node(2), W: 2, UID: 0},
			multi.WeightedLine{F: multi.Node(3), T: multi.Node(4), W: 3, UID: 1},
		},
	},
	{
		name: "explicit edges",
		lines: iterator.NewOrderedWeightedLines([]graph.WeightedLine{
			multi.WeightedLine{F: multi.Node(-1), T: multi.Node(0), W: 1, UID: -1},
			multi.WeightedLine{F: multi.Node(1), T: multi.Node(2), W: 2, UID: 0},
			multi.WeightedLine{F: multi.Node(3), T: multi.Node(4), W: 3, UID: 1},
		}),
		want: []graph.WeightedLine{
			multi.WeightedLine{F: multi.Node(-1), T: multi.Node(0), W: 1, UID: -1},
			multi.WeightedLine{F: multi.Node(1), T: multi.Node(2), W: 2, UID: 0},
			multi.WeightedLine{F: multi.Node(3), T: multi.Node(4), W: 3, UID: 1},
		},
	},
}

type basicWeightedLines struct {
	graph.WeightedLines
}

func TestWeightedLinesOf(t *testing.T) {
	for _, test := range weightedLinesOfTests {
		got := graph.WeightedLinesOf(test.lines)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}
	}
}
