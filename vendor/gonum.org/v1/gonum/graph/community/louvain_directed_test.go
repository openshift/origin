// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

type communityDirectedQTest struct {
	name       string
	g          []intset
	structures []structure

	wantLevels []level
}

var communityDirectedQTests = []communityDirectedQTest{
	{
		name: "simple_directed",
		g:    simpleDirected,
		// community structure and modularity calculated by C++ implementation: louvain igraph.
		// Note that louvain igraph returns Q as an unscaled value.
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1),
					1: linksTo(2, 3, 4),
				},
				want: 0.5714285714285716 / 7,
				tol:  1e-10,
			},
		},
		wantLevels: []level{
			{
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1)},
					{simple.Node(2), simple.Node(3), simple.Node(4)},
				},
				q: 0.5714285714285716 / 7,
			},
			{
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
				},
				q: -1.2857142857142856 / 7,
			},
		},
	},
	{
		name: "zachary",
		g:    zachary,
		// community structure and modularity calculated by C++ implementation: louvain igraph.
		// Note that louvain igraph returns Q as an unscaled value.
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2, 3, 7, 11, 12, 13, 17, 19, 21),
					1: linksTo(4, 5, 6, 10, 16),
					2: linksTo(8, 9, 14, 15, 18, 20, 22, 26, 29, 30, 32, 33),
					3: linksTo(23, 24, 25, 27, 28, 31),
				},
				want: 34.3417721519 / 79 /* 5->6 and 6->5 because of co-equal rank */, tol: 1e-4,
			},
		},
		wantLevels: []level{
			{
				q: 0.43470597660631316,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(7), simple.Node(11), simple.Node(12), simple.Node(13), simple.Node(17), simple.Node(19), simple.Node(21)},
					{simple.Node(4), simple.Node(5), simple.Node(6), simple.Node(10), simple.Node(16)},
					{simple.Node(8), simple.Node(9), simple.Node(14), simple.Node(15), simple.Node(18), simple.Node(20), simple.Node(22), simple.Node(26), simple.Node(29), simple.Node(30), simple.Node(32), simple.Node(33)},
					{simple.Node(23), simple.Node(24), simple.Node(25), simple.Node(27), simple.Node(28), simple.Node(31)},
				},
			},
			{
				q: 0.3911232174331037,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(7), simple.Node(11), simple.Node(12), simple.Node(13), simple.Node(17), simple.Node(19), simple.Node(21)},
					{simple.Node(4), simple.Node(10)},
					{simple.Node(5), simple.Node(6), simple.Node(16)},
					{simple.Node(8), simple.Node(30)},
					{simple.Node(9), simple.Node(14), simple.Node(15), simple.Node(18), simple.Node(20), simple.Node(22), simple.Node(32), simple.Node(33)},
					{simple.Node(23), simple.Node(24), simple.Node(25), simple.Node(27), simple.Node(28), simple.Node(31)},
					{simple.Node(26), simple.Node(29)},
				},
			},
			{
				q: -0.014580996635154624,
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
					{simple.Node(5)},
					{simple.Node(6)},
					{simple.Node(7)},
					{simple.Node(8)},
					{simple.Node(9)},
					{simple.Node(10)},
					{simple.Node(11)},
					{simple.Node(12)},
					{simple.Node(13)},
					{simple.Node(14)},
					{simple.Node(15)},
					{simple.Node(16)},
					{simple.Node(17)},
					{simple.Node(18)},
					{simple.Node(19)},
					{simple.Node(20)},
					{simple.Node(21)},
					{simple.Node(22)},
					{simple.Node(23)},
					{simple.Node(24)},
					{simple.Node(25)},
					{simple.Node(26)},
					{simple.Node(27)},
					{simple.Node(28)},
					{simple.Node(29)},
					{simple.Node(30)},
					{simple.Node(31)},
					{simple.Node(32)},
					{simple.Node(33)},
				},
			},
		},
	},
	{
		name: "blondel",
		g:    blondel,
		// community structure and modularity calculated by C++ implementation: louvain igraph.
		// Note that louvain igraph returns Q as an unscaled value.
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2, 3, 4, 5, 6, 7),
					1: linksTo(8, 9, 10, 11, 12, 13, 14, 15),
				},
				want: 11.1428571429 / 28, tol: 1e-4,
			},
		},
		wantLevels: []level{
			{
				q: 0.3979591836734694,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(3), simple.Node(4), simple.Node(5), simple.Node(6), simple.Node(7)},
					{simple.Node(8), simple.Node(9), simple.Node(10), simple.Node(11), simple.Node(12), simple.Node(13), simple.Node(14), simple.Node(15)},
				},
			},
			{
				q: 0.36862244897959184,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2), simple.Node(4), simple.Node(5)},
					{simple.Node(3), simple.Node(6), simple.Node(7)},
					{simple.Node(8), simple.Node(9), simple.Node(10), simple.Node(11), simple.Node(12), simple.Node(13), simple.Node(14), simple.Node(15)},
				},
			},
			{
				q: -0.022959183673469385,
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
					{simple.Node(5)},
					{simple.Node(6)},
					{simple.Node(7)},
					{simple.Node(8)},
					{simple.Node(9)},
					{simple.Node(10)},
					{simple.Node(11)},
					{simple.Node(12)},
					{simple.Node(13)},
					{simple.Node(14)},
					{simple.Node(15)},
				},
			},
		},
	},
}

func TestCommunityQDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testCommunityQDirected(t, test, g)
	}
}

func TestCommunityQWeightedDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testCommunityQDirected(t, test, g)
	}
}

func testCommunityQDirected(t *testing.T, test communityDirectedQTest, g graph.Directed) {
	for _, structure := range test.structures {
		communities := make([][]graph.Node, len(structure.memberships))
		for i, c := range structure.memberships {
			for n := range c {
				communities[i] = append(communities[i], simple.Node(n))
			}
		}
		got := Q(g, communities, structure.resolution)
		if !floats.EqualWithinAbsOrRel(got, structure.want, structure.tol, structure.tol) && !math.IsNaN(structure.want) {
			for _, c := range communities {
				sort.Sort(ordered.ByID(c))
			}
			t.Errorf("unexpected Q value for %q %v: got: %v want: %v",
				test.name, communities, got, structure.want)
		}
	}
}

func TestCommunityDeltaQDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testCommunityDeltaQDirected(t, test, g)
	}
}

func TestCommunityDeltaQWeightedDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testCommunityDeltaQDirected(t, test, g)
	}
}

func testCommunityDeltaQDirected(t *testing.T, test communityDirectedQTest, g graph.Directed) {
	rnd := rand.New(rand.NewSource(1)).Intn
	for _, structure := range test.structures {
		communityOf := make(map[int64]int)
		communities := make([][]graph.Node, len(structure.memberships))
		for i, c := range structure.memberships {
			for n := range c {
				n := int64(n)
				communityOf[n] = i
				communities[i] = append(communities[i], simple.Node(n))
			}
			sort.Sort(ordered.ByID(communities[i]))
		}

		before := Q(g, communities, structure.resolution)

		l := newDirectedLocalMover(reduceDirected(g, nil), communities, structure.resolution)
		if l == nil {
			if !math.IsNaN(before) {
				t.Errorf("unexpected nil localMover with non-NaN Q graph: Q=%.4v", before)
			}
			return
		}

		// This is done to avoid run-to-run
		// variation due to map iteration order.
		sort.Sort(ordered.ByID(l.nodes))

		l.shuffle(rnd)

		for _, target := range l.nodes {
			got, gotDst, gotSrc := l.deltaQ(target)

			want, wantDst := math.Inf(-1), -1
			migrated := make([][]graph.Node, len(structure.memberships))
			for i, c := range structure.memberships {
				for n := range c {
					n := int64(n)
					if n == target.ID() {
						continue
					}
					migrated[i] = append(migrated[i], simple.Node(n))
				}
				sort.Sort(ordered.ByID(migrated[i]))
			}

			for i, c := range structure.memberships {
				if i == communityOf[target.ID()] {
					continue
				}
				connected := false
				for n := range c {
					if g.HasEdgeBetween(int64(n), target.ID()) {
						connected = true
						break
					}
				}
				if !connected {
					continue
				}
				migrated[i] = append(migrated[i], target)
				after := Q(g, migrated, structure.resolution)
				migrated[i] = migrated[i][:len(migrated[i])-1]
				if after-before > want {
					want = after - before
					wantDst = i
				}
			}

			if !floats.EqualWithinAbsOrRel(got, want, structure.tol, structure.tol) || gotDst != wantDst {
				t.Errorf("unexpected result moving n=%d in c=%d of %s/%.4v: got: %.4v,%d want: %.4v,%d"+
					"\n\t%v\n\t%v",
					target.ID(), communityOf[target.ID()], test.name, structure.resolution, got, gotDst, want, wantDst,
					communities, migrated)
			}
			if gotSrc.community != communityOf[target.ID()] {
				t.Errorf("unexpected source community index: got: %d want: %d", gotSrc, communityOf[target.ID()])
			} else if communities[gotSrc.community][gotSrc.node].ID() != target.ID() {
				wantNodeIdx := -1
				for i, n := range communities[gotSrc.community] {
					if n.ID() == target.ID() {
						wantNodeIdx = i
						break
					}
				}
				t.Errorf("unexpected source node index: got: %d want: %d", gotSrc.node, wantNodeIdx)
			}
		}
	}
}

func TestReduceQConsistencyDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testReduceQConsistencyDirected(t, test, g)
	}
}

func TestReduceQConsistencyWeightedDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testReduceQConsistencyDirected(t, test, g)
	}
}

func testReduceQConsistencyDirected(t *testing.T, test communityDirectedQTest, g graph.Directed) {
	for _, structure := range test.structures {
		if math.IsNaN(structure.want) {
			return
		}

		communities := make([][]graph.Node, len(structure.memberships))
		for i, c := range structure.memberships {
			for n := range c {
				communities[i] = append(communities[i], simple.Node(n))
			}
			sort.Sort(ordered.ByID(communities[i]))
		}

		gQ := Q(g, communities, structure.resolution)
		gQnull := Q(g, nil, 1)

		cg0 := reduceDirected(g, nil)
		cg0Qnull := Q(cg0, cg0.Structure(), 1)
		if !floats.EqualWithinAbsOrRel(gQnull, cg0Qnull, structure.tol, structure.tol) {
			t.Errorf("disagreement between null Q from method: %v and function: %v", cg0Qnull, gQnull)
		}
		cg0Q := Q(cg0, communities, structure.resolution)
		if !floats.EqualWithinAbsOrRel(gQ, cg0Q, structure.tol, structure.tol) {
			t.Errorf("unexpected Q result after initial reduction: got: %v want :%v", cg0Q, gQ)
		}

		cg1 := reduceDirected(cg0, communities)
		cg1Q := Q(cg1, cg1.Structure(), structure.resolution)
		if !floats.EqualWithinAbsOrRel(gQ, cg1Q, structure.tol, structure.tol) {
			t.Errorf("unexpected Q result after second reduction: got: %v want :%v", cg1Q, gQ)
		}
	}
}

type localDirectedMoveTest struct {
	name       string
	g          []intset
	structures []moveStructures
}

var localDirectedMoveTests = []localDirectedMoveTest{
	{
		name: "blondel",
		g:    blondel,
		structures: []moveStructures{
			{
				memberships: []intset{
					0: linksTo(0, 1, 2, 4, 5),
					1: linksTo(3, 6, 7),
					2: linksTo(8, 9, 10, 12, 14, 15),
					3: linksTo(11, 13),
				},
				targetNodes: []graph.Node{simple.Node(0)},
				resolution:  1,
				tol:         1e-14,
			},
			{
				memberships: []intset{
					0: linksTo(0, 1, 2, 4, 5),
					1: linksTo(3, 6, 7),
					2: linksTo(8, 9, 10, 12, 14, 15),
					3: linksTo(11, 13),
				},
				targetNodes: []graph.Node{simple.Node(3)},
				resolution:  1,
				tol:         1e-14,
			},
			{
				memberships: []intset{
					0: linksTo(0, 1, 2, 4, 5),
					1: linksTo(3, 6, 7),
					2: linksTo(8, 9, 10, 12, 14, 15),
					3: linksTo(11, 13),
				},
				// Case to demonstrate when A_aa != k_a^𝛼.
				targetNodes: []graph.Node{simple.Node(3), simple.Node(2)},
				resolution:  1,
				tol:         1e-14,
			},
		},
	},
}

func TestMoveLocalDirected(t *testing.T) {
	for _, test := range localDirectedMoveTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testMoveLocalDirected(t, test, g)
	}
}

func TestMoveLocalWeightedDirected(t *testing.T) {
	for _, test := range localDirectedMoveTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testMoveLocalDirected(t, test, g)
	}
}

func testMoveLocalDirected(t *testing.T, test localDirectedMoveTest, g graph.Directed) {
	for _, structure := range test.structures {
		communities := make([][]graph.Node, len(structure.memberships))
		for i, c := range structure.memberships {
			for n := range c {
				communities[i] = append(communities[i], simple.Node(n))
			}
			sort.Sort(ordered.ByID(communities[i]))
		}

		r := reduceDirected(reduceDirected(g, nil), communities)

		l := newDirectedLocalMover(r, r.communities, structure.resolution)
		for _, n := range structure.targetNodes {
			dQ, dst, src := l.deltaQ(n)
			if dQ > 0 {
				before := Q(r, l.communities, structure.resolution)
				l.move(dst, src)
				after := Q(r, l.communities, structure.resolution)
				want := after - before
				if !floats.EqualWithinAbsOrRel(dQ, want, structure.tol, structure.tol) {
					t.Errorf("unexpected deltaQ: got: %v want: %v", dQ, want)
				}
			}
		}
	}
}

func TestModularizeDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testModularizeDirected(t, test, g)
	}
}

func TestModularizeWeightedDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testModularizeDirected(t, test, g)
	}
}

func testModularizeDirected(t *testing.T, test communityDirectedQTest, g graph.Directed) {
	const louvainIterations = 20

	if test.structures[0].resolution != 1 {
		panic("bad test: expect resolution=1")
	}
	want := make([][]graph.Node, len(test.structures[0].memberships))
	for i, c := range test.structures[0].memberships {
		for n := range c {
			want[i] = append(want[i], simple.Node(n))
		}
		sort.Sort(ordered.ByID(want[i]))
	}
	sort.Sort(ordered.BySliceIDs(want))

	var (
		got   *ReducedDirected
		bestQ = math.Inf(-1)
	)
	// Modularize is randomised so we do this to
	// ensure the level tests are consistent.
	src := rand.New(rand.NewSource(1))
	for i := 0; i < louvainIterations; i++ {
		r := Modularize(g, 1, src).(*ReducedDirected)
		if q := Q(r, nil, 1); q > bestQ || math.IsNaN(q) {
			bestQ = q
			got = r

			if math.IsNaN(q) {
				// Don't try again for non-connected case.
				break
			}
		}

		var qs []float64
		for p := r; p != nil; p = p.Expanded().(*ReducedDirected) {
			qs = append(qs, Q(p, nil, 1))
		}

		// Recovery of Q values is reversed.
		if reverse(qs); !sort.Float64sAreSorted(qs) {
			t.Errorf("Q values not monotonically increasing: %.5v", qs)
		}
	}

	gotCommunities := got.Communities()
	for _, c := range gotCommunities {
		sort.Sort(ordered.ByID(c))
	}
	sort.Sort(ordered.BySliceIDs(gotCommunities))
	if !reflect.DeepEqual(gotCommunities, want) {
		t.Errorf("unexpected community membership for %s Q=%.4v:\n\tgot: %v\n\twant:%v",
			test.name, bestQ, gotCommunities, want)
		return
	}

	var levels []level
	for p := got; p != nil; p = p.Expanded().(*ReducedDirected) {
		var communities [][]graph.Node
		if p.parent != nil {
			communities = p.parent.Communities()
			for _, c := range communities {
				sort.Sort(ordered.ByID(c))
			}
			sort.Sort(ordered.BySliceIDs(communities))
		} else {
			communities = reduceDirected(g, nil).Communities()
		}
		q := Q(p, nil, 1)
		if math.IsNaN(q) {
			// Use an equalable flag value in place of NaN.
			q = math.Inf(-1)
		}
		levels = append(levels, level{q: q, communities: communities})
	}
	if !reflect.DeepEqual(levels, test.wantLevels) {
		t.Errorf("unexpected level structure:\n\tgot: %v\n\twant:%v", levels, test.wantLevels)
	}
}

func TestNonContiguousDirected(t *testing.T) {
	g := simple.NewDirectedGraph()
	for _, e := range []simple.Edge{
		{F: simple.Node(0), T: simple.Node(1)},
		{F: simple.Node(4), T: simple.Node(5)},
	} {
		g.SetEdge(e)
	}

	func() {
		defer func() {
			r := recover()
			if r != nil {
				t.Error("unexpected panic with non-contiguous ID range")
			}
		}()
		Modularize(g, 1, nil)
	}()
}

func TestNonContiguousWeightedDirected(t *testing.T) {
	g := simple.NewWeightedDirectedGraph(0, 0)
	for _, e := range []simple.WeightedEdge{
		{F: simple.Node(0), T: simple.Node(1), W: 1},
		{F: simple.Node(4), T: simple.Node(5), W: 1},
	} {
		g.SetWeightedEdge(e)
	}

	func() {
		defer func() {
			r := recover()
			if r != nil {
				t.Error("unexpected panic with non-contiguous ID range")
			}
		}()
		Modularize(g, 1, nil)
	}()
}

func BenchmarkLouvainDirected(b *testing.B) {
	src := rand.New(rand.NewSource(1))
	for i := 0; i < b.N; i++ {
		Modularize(dupGraphDirected, 1, src)
	}
}
