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

var communityDirectedMultiplexQTests = []struct {
	name       string
	layers     []layer
	structures []structure

	wantLevels []level
}{
	{
		name:   "unconnected",
		layers: []layer{{g: unconnected, weight: 1}},
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0),
					1: linksTo(1),
					2: linksTo(2),
					3: linksTo(3),
					4: linksTo(4),
					5: linksTo(5),
				},
				want: math.NaN(),
			},
		},
		wantLevels: []level{
			{
				q: math.Inf(-1), // Here math.Inf(-1) is used as a place holder for NaN to allow use of reflect.DeepEqual.
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
					{simple.Node(5)},
				},
			},
		},
	},
	{
		name:   "simple_directed",
		layers: []layer{{g: simpleDirected, weight: 1}},
		// community structure and modularity calculated by C++ implementation: louvain igraph.
		// Note that louvain igraph returns Q as an unscaled value.
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1),
					1: linksTo(2, 3, 4),
				},
				want: 0.5714285714285716,
				tol:  1e-10,
			},
		},
		wantLevels: []level{
			{
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1)},
					{simple.Node(2), simple.Node(3), simple.Node(4)},
				},
				q: 0.5714285714285716,
			},
			{
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
				},
				q: -1.2857142857142856,
			},
		},
	},
	{
		name: "simple_directed_twice",
		layers: []layer{
			{g: simpleDirected, weight: 0.5},
			{g: simpleDirected, weight: 0.5},
		},
		// community structure and modularity calculated by C++ implementation: louvain igraph.
		// Note that louvain igraph returns Q as an unscaled value.
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1),
					1: linksTo(2, 3, 4),
				},
				want: 0.5714285714285716,
				tol:  1e-10,
			},
		},
		wantLevels: []level{
			{
				q: 0.5714285714285716,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1)},
					{simple.Node(2), simple.Node(3), simple.Node(4)},
				},
			},
			{
				q: -1.2857142857142856,
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
				},
			},
		},
	},
	{
		name: "small_dumbell",
		layers: []layer{
			{g: smallDumbell, edgeWeight: 1, weight: 1},
			{g: dumbellRepulsion, edgeWeight: -1, weight: -1},
		},
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2),
					1: linksTo(3, 4, 5),
				},
				want: 2.5714285714285716, tol: 1e-10,
			},
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2, 3, 4, 5),
				},
				want: 0, tol: 1e-14,
			},
		},
		wantLevels: []level{
			{
				q: 2.5714285714285716,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2)},
					{simple.Node(3), simple.Node(4), simple.Node(5)},
				},
			},
			{
				q: -0.857142857142857,
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
					{simple.Node(5)},
				},
			},
		},
	},
	{
		name:   "repulsion",
		layers: []layer{{g: repulsion, edgeWeight: -1, weight: -1}},
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2),
					1: linksTo(3, 4, 5),
				},
				want: 9.0, tol: 1e-10,
			},
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0),
					1: linksTo(1),
					2: linksTo(2),
					3: linksTo(3),
					4: linksTo(4),
					5: linksTo(5),
				},
				want: 3, tol: 1e-14,
			},
		},
		wantLevels: []level{
			{
				q: 9.0,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(1), simple.Node(2)},
					{simple.Node(3), simple.Node(4), simple.Node(5)},
				},
			},
			{
				q: 3.0,
				communities: [][]graph.Node{
					{simple.Node(0)},
					{simple.Node(1)},
					{simple.Node(2)},
					{simple.Node(3)},
					{simple.Node(4)},
					{simple.Node(5)},
				},
			},
		},
	},
	{
		name: "middle_east",
		layers: []layer{
			{g: middleEast.friends, edgeWeight: 1, weight: 1},
			{g: middleEast.enemies, edgeWeight: -1, weight: -1},
		},
		structures: []structure{
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 6),
					1: linksTo(1, 7, 9, 12),
					2: linksTo(2, 8, 11),
					3: linksTo(3, 4, 5, 10),
				},
				want: 33.818057455540355, tol: 1e-9,
			},
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 2, 3, 4, 5, 10),
					1: linksTo(1, 7, 9, 12),
					2: linksTo(6),
					3: linksTo(8, 11),
				},
				want: 30.92749658, tol: 1e-7,
			},
			{
				resolution: 1,
				memberships: []intset{
					0: linksTo(0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12),
				},
				want: 0, tol: 1e-14,
			},
		},
		wantLevels: []level{
			{
				q: 33.818057455540355,
				communities: [][]graph.Node{
					{simple.Node(0), simple.Node(6)},
					{simple.Node(1), simple.Node(7), simple.Node(9), simple.Node(12)},
					{simple.Node(2), simple.Node(8), simple.Node(11)},
					{simple.Node(3), simple.Node(4), simple.Node(5), simple.Node(10)},
				},
			},
			{
				q: 3.8071135430916545,
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
				},
			},
		},
	},
}

func TestCommunityQDirectedMultiplex(t *testing.T) {
	for _, test := range communityDirectedMultiplexQTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

		for _, structure := range test.structures {
			communities := make([][]graph.Node, len(structure.memberships))
			for i, c := range structure.memberships {
				for n := range c {
					communities[i] = append(communities[i], simple.Node(n))
				}
			}
			q := QMultiplex(g, communities, weights, []float64{structure.resolution})
			got := floats.Sum(q)
			if !floats.EqualWithinAbsOrRel(got, structure.want, structure.tol, structure.tol) && !math.IsNaN(structure.want) {
				for _, c := range communities {
					sort.Sort(ordered.ByID(c))
				}
				t.Errorf("unexpected Q value for %q %v: got: %v %.3v want: %v",
					test.name, communities, got, q, structure.want)
			}
		}
	}
}

func TestCommunityDeltaQDirectedMultiplex(t *testing.T) {
tests:
	for _, test := range communityDirectedMultiplexQTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

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
			resolution := []float64{structure.resolution}

			before := QMultiplex(g, communities, weights, resolution)

			// We test exhaustively.
			const all = true

			l := newDirectedMultiplexLocalMover(
				reduceDirectedMultiplex(g, nil, weights),
				communities, weights, resolution, all)
			if l == nil {
				if !math.IsNaN(floats.Sum(before)) {
					t.Errorf("unexpected nil localMover with non-NaN Q graph: Q=%.4v", before)
				}
				continue tests
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
					if !(all && hasNegative(weights)) {
						connected := false
					search:
						for l := 0; l < g.Depth(); l++ {
							if weights[l] < 0 {
								connected = true
								break search
							}
							layer := g.Layer(l)
							for n := range c {
								if layer.HasEdgeBetween(int64(n), target.ID()) {
									connected = true
									break search
								}
							}
						}
						if !connected {
							continue
						}
					}
					migrated[i] = append(migrated[i], target)
					after := QMultiplex(g, migrated, weights, resolution)
					migrated[i] = migrated[i][:len(migrated[i])-1]
					if delta := floats.Sum(after) - floats.Sum(before); delta > want {
						want = delta
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
}

func TestReduceQConsistencyDirectedMultiplex(t *testing.T) {
tests:
	for _, test := range communityDirectedMultiplexQTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

		for _, structure := range test.structures {
			if math.IsNaN(structure.want) {
				continue tests
			}

			communities := make([][]graph.Node, len(structure.memberships))
			for i, c := range structure.memberships {
				for n := range c {
					communities[i] = append(communities[i], simple.Node(n))
				}
				sort.Sort(ordered.ByID(communities[i]))
			}

			gQ := QMultiplex(g, communities, weights, []float64{structure.resolution})
			gQnull := QMultiplex(g, nil, weights, nil)

			cg0 := reduceDirectedMultiplex(g, nil, weights)
			cg0Qnull := QMultiplex(cg0, cg0.Structure(), weights, nil)
			if !floats.EqualWithinAbsOrRel(floats.Sum(gQnull), floats.Sum(cg0Qnull), structure.tol, structure.tol) {
				t.Errorf("disagreement between null Q from method: %v and function: %v", cg0Qnull, gQnull)
			}
			cg0Q := QMultiplex(cg0, communities, weights, []float64{structure.resolution})
			if !floats.EqualWithinAbsOrRel(floats.Sum(gQ), floats.Sum(cg0Q), structure.tol, structure.tol) {
				t.Errorf("unexpected Q result after initial reduction: got: %v want :%v", cg0Q, gQ)
			}

			cg1 := reduceDirectedMultiplex(cg0, communities, weights)
			cg1Q := QMultiplex(cg1, cg1.Structure(), weights, []float64{structure.resolution})
			if !floats.EqualWithinAbsOrRel(floats.Sum(gQ), floats.Sum(cg1Q), structure.tol, structure.tol) {
				t.Errorf("unexpected Q result after second reduction: got: %v want :%v", cg1Q, gQ)
			}
		}
	}
}

var localDirectedMultiplexMoveTests = []struct {
	name       string
	layers     []layer
	structures []moveStructures
}{
	{
		name:   "blondel",
		layers: []layer{{g: blondel, weight: 1}, {g: blondel, weight: 0.5}},
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

func TestMoveLocalDirectedMultiplex(t *testing.T) {
	for _, test := range localDirectedMultiplexMoveTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

		for _, structure := range test.structures {
			communities := make([][]graph.Node, len(structure.memberships))
			for i, c := range structure.memberships {
				for n := range c {
					communities[i] = append(communities[i], simple.Node(n))
				}
				sort.Sort(ordered.ByID(communities[i]))
			}

			r := reduceDirectedMultiplex(reduceDirectedMultiplex(g, nil, weights), communities, weights)

			l := newDirectedMultiplexLocalMover(r, r.communities, weights, []float64{structure.resolution}, true)
			for _, n := range structure.targetNodes {
				dQ, dst, src := l.deltaQ(n)
				if dQ > 0 {
					before := floats.Sum(QMultiplex(r, l.communities, weights, []float64{structure.resolution}))
					l.move(dst, src)
					after := floats.Sum(QMultiplex(r, l.communities, weights, []float64{structure.resolution}))
					want := after - before
					if !floats.EqualWithinAbsOrRel(dQ, want, structure.tol, structure.tol) {
						t.Errorf("unexpected deltaQ: got: %v want: %v", dQ, want)
					}
				}
			}
		}
	}
}

func TestLouvainDirectedMultiplex(t *testing.T) {
	const louvainIterations = 20

	for _, test := range communityDirectedMultiplexQTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

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
			got   *ReducedDirectedMultiplex
			bestQ = math.Inf(-1)
		)
		// Modularize is randomised so we do this to
		// ensure the level tests are consistent.
		src := rand.New(rand.NewSource(1))
		for i := 0; i < louvainIterations; i++ {
			r := ModularizeMultiplex(g, weights, nil, true, src).(*ReducedDirectedMultiplex)
			if q := floats.Sum(QMultiplex(r, nil, weights, nil)); q > bestQ || math.IsNaN(q) {
				bestQ = q
				got = r

				if math.IsNaN(q) {
					// Don't try again for non-connected case.
					break
				}
			}

			var qs []float64
			for p := r; p != nil; p = p.Expanded().(*ReducedDirectedMultiplex) {
				qs = append(qs, floats.Sum(QMultiplex(p, nil, weights, nil)))
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
			continue
		}

		var levels []level
		for p := got; p != nil; p = p.Expanded().(*ReducedDirectedMultiplex) {
			var communities [][]graph.Node
			if p.parent != nil {
				communities = p.parent.Communities()
				for _, c := range communities {
					sort.Sort(ordered.ByID(c))
				}
				sort.Sort(ordered.BySliceIDs(communities))
			} else {
				communities = reduceDirectedMultiplex(g, nil, weights).Communities()
			}
			q := floats.Sum(QMultiplex(p, nil, weights, nil))
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
}

func TestNonContiguousDirectedMultiplex(t *testing.T) {
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
		ModularizeMultiplex(DirectedLayers{g}, nil, nil, true, nil)
	}()
}

func TestNonContiguousWeightedDirectedMultiplex(t *testing.T) {
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
		ModularizeMultiplex(DirectedLayers{g}, nil, nil, true, nil)
	}()
}

func BenchmarkLouvainDirectedMultiplex(b *testing.B) {
	src := rand.New(rand.NewSource(1))
	for i := 0; i < b.N; i++ {
		ModularizeMultiplex(DirectedLayers{dupGraphDirected}, nil, nil, true, src)
	}
}

func directedMultiplexFrom(raw []layer) (DirectedLayers, []float64, error) {
	var layers []graph.Directed
	var weights []float64
	for _, l := range raw {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range l.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				w := 1.0
				if l.edgeWeight != 0 {
					w = l.edgeWeight
				}
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: w})
			}
		}
		layers = append(layers, g)
		weights = append(weights, l.weight)
	}
	g, err := NewDirectedLayers(layers...)
	if err != nil {
		return nil, nil, err
	}
	return g, weights, nil
}
