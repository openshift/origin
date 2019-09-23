// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"
	"reflect"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path/internal/testgraphs"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

var aStarTests = []struct {
	name string
	g    graph.Graph

	s, t      int64
	heuristic Heuristic
	wantPath  []int64
}{
	{
		name: "simple path",
		g: func() graph.Graph {
			return testgraphs.NewGridFrom(
				"*..*",
				"**.*",
				"**.*",
				"**.*",
			)
		}(),

		s: 1, t: 14,
		wantPath: []int64{1, 2, 6, 10, 14},
	},
	{
		name: "small open graph",
		g:    testgraphs.NewGrid(3, 3, true),

		s: 0, t: 8,
	},
	{
		name: "large open graph",
		g:    testgraphs.NewGrid(1000, 1000, true),

		s: 0, t: 999*1000 + 999,
	},
	{
		name: "no path",
		g: func() graph.Graph {
			tg := testgraphs.NewGrid(5, 5, true)

			// Create a complete "wall" across the middle row.
			tg.Set(2, 0, false)
			tg.Set(2, 1, false)
			tg.Set(2, 2, false)
			tg.Set(2, 3, false)
			tg.Set(2, 4, false)

			return tg
		}(),

		s: 2, t: 22,
	},
	{
		name: "partially obstructed",
		g: func() graph.Graph {
			tg := testgraphs.NewGrid(10, 10, true)

			// Create a partial "wall" across the middle
			// row with a gap at the left-hand end.
			tg.Set(4, 1, false)
			tg.Set(4, 2, false)
			tg.Set(4, 3, false)
			tg.Set(4, 4, false)
			tg.Set(4, 5, false)
			tg.Set(4, 6, false)
			tg.Set(4, 7, false)
			tg.Set(4, 8, false)
			tg.Set(4, 9, false)

			return tg
		}(),

		s: 5, t: 9*10 + 9,
	},
	{
		name: "partially obstructed with heuristic",
		g: func() graph.Graph {
			tg := testgraphs.NewGrid(10, 10, true)

			// Create a partial "wall" across the middle
			// row with a gap at the left-hand end.
			tg.Set(4, 1, false)
			tg.Set(4, 2, false)
			tg.Set(4, 3, false)
			tg.Set(4, 4, false)
			tg.Set(4, 5, false)
			tg.Set(4, 6, false)
			tg.Set(4, 7, false)
			tg.Set(4, 8, false)
			tg.Set(4, 9, false)

			return tg
		}(),

		s: 5, t: 9*10 + 9,
		// Manhattan Heuristic
		heuristic: func(u, v graph.Node) float64 {
			uid := u.ID()
			cu := (uid % 10)
			ru := (uid - cu) / 10

			vid := v.ID()
			cv := (vid % 10)
			rv := (vid - cv) / 10

			return math.Abs(float64(ru-rv)) + math.Abs(float64(cu-cv))
		},
	},
}

func TestAStar(t *testing.T) {
	for _, test := range aStarTests {
		pt, _ := AStar(simple.Node(test.s), simple.Node(test.t), test.g, test.heuristic)

		p, cost := pt.To(test.t)

		if !topo.IsPathIn(test.g, p) {
			t.Errorf("got path that is not path in input graph for %q", test.name)
		}

		bfp, ok := BellmanFordFrom(simple.Node(test.s), test.g)
		if !ok {
			t.Fatalf("unexpected negative cycle in %q", test.name)
		}
		if want := bfp.WeightTo(test.t); cost != want {
			t.Errorf("unexpected cost for %q: got:%v want:%v", test.name, cost, want)
		}

		var got = make([]int64, 0, len(p))
		for _, n := range p {
			got = append(got, n.ID())
		}
		if test.wantPath != nil && !reflect.DeepEqual(got, test.wantPath) {
			t.Errorf("unexpected result for %q:\ngot: %v\nwant:%v", test.name, got, test.wantPath)
		}
	}
}

func TestExhaustiveAStar(t *testing.T) {
	g := simple.NewWeightedUndirectedGraph(0, math.Inf(1))
	nodes := []locatedNode{
		{id: 1, x: 0, y: 6},
		{id: 2, x: 1, y: 0},
		{id: 3, x: 8, y: 7},
		{id: 4, x: 16, y: 0},
		{id: 5, x: 17, y: 6},
		{id: 6, x: 9, y: 8},
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	edges := []weightedEdge{
		{from: g.Node(1), to: g.Node(2), cost: 7},
		{from: g.Node(1), to: g.Node(3), cost: 9},
		{from: g.Node(1), to: g.Node(6), cost: 14},
		{from: g.Node(2), to: g.Node(3), cost: 10},
		{from: g.Node(2), to: g.Node(4), cost: 15},
		{from: g.Node(3), to: g.Node(4), cost: 11},
		{from: g.Node(3), to: g.Node(6), cost: 2},
		{from: g.Node(4), to: g.Node(5), cost: 7},
		{from: g.Node(5), to: g.Node(6), cost: 9},
	}
	for _, e := range edges {
		g.SetWeightedEdge(e)
	}

	heuristic := func(u, v graph.Node) float64 {
		lu := u.(locatedNode)
		lv := v.(locatedNode)
		return math.Hypot(lu.x-lv.x, lu.y-lv.y)
	}

	if ok, edge, goal := isMonotonic(g, heuristic); !ok {
		t.Fatalf("non-monotonic heuristic at edge:%v for goal:%v", edge, goal)
	}

	ps := DijkstraAllPaths(g)
	for _, start := range g.Nodes() {
		for _, goal := range g.Nodes() {
			pt, _ := AStar(start, goal, g, heuristic)
			gotPath, gotWeight := pt.To(goal.ID())
			wantPath, wantWeight, _ := ps.Between(start.ID(), goal.ID())
			if gotWeight != wantWeight {
				t.Errorf("unexpected path weight from %v to %v result: got:%f want:%f",
					start, goal, gotWeight, wantWeight)
			}
			if !reflect.DeepEqual(gotPath, wantPath) {
				t.Errorf("unexpected path from %v to %v result:\ngot: %v\nwant:%v",
					start, goal, gotPath, wantPath)
			}
		}
	}
}

type locatedNode struct {
	id   int64
	x, y float64
}

func (n locatedNode) ID() int64 { return n.id }

type weightedEdge struct {
	from, to graph.Node
	cost     float64
}

func (e weightedEdge) From() graph.Node { return e.from }
func (e weightedEdge) To() graph.Node   { return e.to }
func (e weightedEdge) Weight() float64  { return e.cost }

func isMonotonic(g UndirectedWeightLister, h Heuristic) (ok bool, at graph.Edge, goal graph.Node) {
	for _, goal := range g.Nodes() {
		for _, edge := range g.WeightedEdges() {
			from := edge.From()
			to := edge.To()
			w, ok := g.Weight(from.ID(), to.ID())
			if !ok {
				panic("A*: unexpected invalid weight")
			}
			if h(from, goal) > w+h(to, goal) {
				return false, edge, goal
			}
		}
	}
	return true, nil, nil
}

func TestAStarNullHeuristic(t *testing.T) {
	for _, test := range testgraphs.ShortestPathTests {
		g := test.Graph()
		for _, e := range test.Edges {
			g.SetWeightedEdge(e)
		}

		var (
			pt Shortest

			panicked bool
		)
		func() {
			defer func() {
				panicked = recover() != nil
			}()
			pt, _ = AStar(test.Query.From(), test.Query.To(), g.(graph.Graph), nil)
		}()
		if panicked || test.HasNegativeWeight {
			if !test.HasNegativeWeight {
				t.Errorf("%q: unexpected panic", test.Name)
			}
			if !panicked {
				t.Errorf("%q: expected panic for negative edge weight", test.Name)
			}
			continue
		}

		if pt.From().ID() != test.Query.From().ID() {
			t.Fatalf("%q: unexpected from node ID: got:%d want:%d", test.Name, pt.From().ID(), test.Query.From().ID())
		}

		p, weight := pt.To(test.Query.To().ID())
		if weight != test.Weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
				test.Name, weight, test.Weight)
		}
		if weight := pt.WeightTo(test.Query.To().ID()); weight != test.Weight {
			t.Errorf("%q: unexpected weight from Weight: got:%f want:%f",
				test.Name, weight, test.Weight)
		}

		var got []int64
		for _, n := range p {
			got = append(got, n.ID())
		}
		ok := len(got) == 0 && len(test.WantPaths) == 0
		for _, sp := range test.WantPaths {
			if reflect.DeepEqual(got, sp) {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%q: unexpected shortest path:\ngot: %v\nwant from:%v",
				test.Name, p, test.WantPaths)
		}

		np, weight := pt.To(test.NoPathFor.To().ID())
		if pt.From().ID() == test.NoPathFor.From().ID() && (np != nil || !math.IsInf(weight, 1)) {
			t.Errorf("%q: unexpected path:\ngot: path=%v weight=%f\nwant:path=<nil> weight=+Inf",
				test.Name, np, weight)
		}
	}
}
