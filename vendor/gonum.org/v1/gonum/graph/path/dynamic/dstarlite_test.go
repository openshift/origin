// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dynamic

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/path/internal/testgraphs"
	"gonum.org/v1/gonum/graph/simple"
)

var (
	debug   = flag.Bool("debug", false, "write path progress for failing dynamic case tests")
	vdebug  = flag.Bool("vdebug", false, "write path progress for all dynamic case tests (requires test.v)")
	maxWide = flag.Int("maxwidth", 5, "maximum width grid to dump for debugging")
)

func TestDStarLiteNullHeuristic(t *testing.T) {
	for _, test := range testgraphs.ShortestPathTests {
		// Skip zero-weight cycles.
		if strings.HasPrefix(test.Name, "zero-weight") {
			continue
		}

		g := test.Graph()
		for _, e := range test.Edges {
			g.SetWeightedEdge(e)
		}

		var (
			d *DStarLite

			panicked bool
		)
		func() {
			defer func() {
				panicked = recover() != nil
			}()
			d = NewDStarLite(test.Query.From(), test.Query.To(), g.(graph.Graph), path.NullHeuristic, simple.NewWeightedDirectedGraph(0, math.Inf(1)))
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

		p, weight := d.Path()

		if !math.IsInf(weight, 1) && p[0].ID() != test.Query.From().ID() {
			t.Fatalf("%q: unexpected from node ID: got:%d want:%d", test.Name, p[0].ID(), test.Query.From().ID())
		}
		if weight != test.Weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
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
	}
}

var dynamicDStarLiteTests = []struct {
	g          *testgraphs.Grid
	radius     float64
	all        bool
	diag, unit bool
	remember   []bool
	modify     func(*testgraphs.LimitedVisionGrid)

	heuristic func(dx, dy float64) float64

	s, t graph.Node

	want        []graph.Node
	weight      float64
	wantedPaths map[int64][]graph.Node
}{
	{
		// This is the example shown in figures 6 and 7 of doi:10.1109/tro.2004.838026.
		g: testgraphs.NewGridFrom(
			"...",
			".*.",
			".*.",
			".*.",
			"...",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		unit:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(3),
		t: simple.Node(14),

		want: []graph.Node{
			simple.Node(3),
			simple.Node(6),
			simple.Node(9),
			simple.Node(13),
			simple.Node(14),
		},
		weight: 4,
	},
	{
		// This is a small example that has the property that the first corner
		// may be taken incorrectly at 90° or correctly at 45° because the
		// calculated rhs values of 12 and 17 are tied when moving from node
		// 16, and the grid is small enough to examine by a dump.
		g: testgraphs.NewGridFrom(
			".....",
			"...*.",
			"**.*.",
			"...*.",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(15),
		t: simple.Node(14),

		want: []graph.Node{
			simple.Node(15),
			simple.Node(16),
			simple.Node(12),
			simple.Node(7),
			simple.Node(3),
			simple.Node(9),
			simple.Node(14),
		},
		weight: 7.242640687119285,
		wantedPaths: map[int64][]graph.Node{
			12: {simple.Node(12), simple.Node(7), simple.Node(3), simple.Node(9), simple.Node(14)},
		},
	},
	{
		// This is the example shown in figure 2 of doi:10.1109/tro.2004.838026
		// with the exception that diagonal edge weights are calculated with the hypot
		// function instead of a step count and only allowing information to be known
		// from exploration.
		g: testgraphs.NewGridFrom(
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"....*.*...........",
			"*****.***.........",
			"......*...........",
			"......***.........",
			"......*...........",
			"......*...........",
			"......*...........",
			"*****.*...........",
			"......*...........",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(253),
		t: simple.Node(122),

		want: []graph.Node{
			simple.Node(253),
			simple.Node(254),
			simple.Node(255),
			simple.Node(256),
			simple.Node(239),
			simple.Node(221),
			simple.Node(203),
			simple.Node(185),
			simple.Node(167),
			simple.Node(149),
			simple.Node(131),
			simple.Node(113),
			simple.Node(96),

			// The following section depends
			// on map iteration order.
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,

			simple.Node(122),
		},
		weight: 21.242640687119287,
	},
	{
		// This is the example shown in figure 2 of doi:10.1109/tro.2004.838026
		// with the exception that diagonal edge weights are calculated with the hypot
		// function instead of a step count, not closing the exit and only allowing
		// information to be known from exploration.
		g: testgraphs.NewGridFrom(
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"....*.*...........",
			"*****.***.........",
			"..................", // Keep open.
			"......***.........",
			"......*...........",
			"......*...........",
			"......*...........",
			"*****.*...........",
			"......*...........",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(253),
		t: simple.Node(122),

		want: []graph.Node{
			simple.Node(253),
			simple.Node(254),
			simple.Node(255),
			simple.Node(256),
			simple.Node(239),
			simple.Node(221),
			simple.Node(203),
			simple.Node(185),
			simple.Node(167),
			simple.Node(150),
			simple.Node(151),
			simple.Node(152),

			// The following section depends
			// on map iteration order.
			nil,
			nil,
			nil,
			nil,
			nil,

			simple.Node(122),
		},
		weight: 18.656854249492383,
	},
	{
		// This is the example shown in figure 2 of doi:10.1109/tro.2004.838026
		// with the exception that diagonal edge weights are calculated with the hypot
		// function instead of a step count, the exit is closed at a distance and
		// information is allowed to be known from exploration.
		g: testgraphs.NewGridFrom(
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"....*.*...........",
			"*****.***.........",
			"........*.........",
			"......***.........",
			"......*...........",
			"......*...........",
			"......*...........",
			"*****.*...........",
			"......*...........",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(253),
		t: simple.Node(122),

		want: []graph.Node{
			simple.Node(253),
			simple.Node(254),
			simple.Node(255),
			simple.Node(256),
			simple.Node(239),
			simple.Node(221),
			simple.Node(203),
			simple.Node(185),
			simple.Node(167),
			simple.Node(150),
			simple.Node(151),
			simple.Node(150),
			simple.Node(131),
			simple.Node(113),
			simple.Node(96),

			// The following section depends
			// on map iteration order.
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,

			simple.Node(122),
		},
		weight: 24.07106781186548,
	},
	{
		// This is the example shown in figure 2 of doi:10.1109/tro.2004.838026
		// with the exception that diagonal edge weights are calculated with the hypot
		// function instead of a step count.
		g: testgraphs.NewGridFrom(
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"..................",
			"....*.*...........",
			"*****.***.........",
			"......*...........", // Forget this wall.
			"......***.........",
			"......*...........",
			"......*...........",
			"......*...........",
			"*****.*...........",
			"......*...........",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{true},

		modify: func(l *testgraphs.LimitedVisionGrid) {
			all := l.Grid.AllVisible
			l.Grid.AllVisible = false
			for _, n := range graph.NodesOf(l.Nodes()) {
				id := n.ID()
				l.Known[id] = l.Grid.Node(id) == nil
			}
			l.Grid.AllVisible = all

			const (
				wallRow = 8
				wallCol = 6
			)
			l.Known[l.NodeAt(wallRow, wallCol).ID()] = false

			// Check we have a correctly modified representation.
			for _, u := range graph.NodesOf(l.Nodes()) {
				uid := u.ID()
				for _, v := range graph.NodesOf(l.Nodes()) {
					vid := v.ID()
					if l.HasEdgeBetween(uid, vid) != l.Grid.HasEdgeBetween(uid, vid) {
						ur, uc := l.RowCol(uid)
						vr, vc := l.RowCol(vid)
						if (ur == wallRow && uc == wallCol) || (vr == wallRow && vc == wallCol) {
							if !l.HasEdgeBetween(uid, vid) {
								panic(fmt.Sprintf("expected to believe edge between %v (%d,%d) and %v (%d,%d) is passable",
									u, v, ur, uc, vr, vc))
							}
							continue
						}
						panic(fmt.Sprintf("disagreement about edge between %v (%d,%d) and %v (%d,%d): got:%t want:%t",
							u, v, ur, uc, vr, vc, l.HasEdgeBetween(uid, vid), l.Grid.HasEdgeBetween(uid, vid)))
					}
				}
			}
		},

		heuristic: func(dx, dy float64) float64 {
			return math.Max(math.Abs(dx), math.Abs(dy))
		},

		s: simple.Node(253),
		t: simple.Node(122),

		want: []graph.Node{
			simple.Node(253),
			simple.Node(254),
			simple.Node(255),
			simple.Node(256),
			simple.Node(239),
			simple.Node(221),
			simple.Node(203),
			simple.Node(185),
			simple.Node(167),
			simple.Node(149),
			simple.Node(131),
			simple.Node(113),
			simple.Node(96),

			// The following section depends
			// on map iteration order.
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,
			nil,

			simple.Node(122),
		},
		weight: 21.242640687119287,
	},
	{
		g: testgraphs.NewGridFrom(
			"*..*",
			"**.*",
			"**.*",
			"**.*",
		),
		radius:   1,
		all:      true,
		diag:     false,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Hypot(dx, dy)
		},

		s: simple.Node(1),
		t: simple.Node(14),

		want: []graph.Node{
			simple.Node(1),
			simple.Node(2),
			simple.Node(6),
			simple.Node(10),
			simple.Node(14),
		},
		weight: 4,
	},
	{
		g: testgraphs.NewGridFrom(
			"*..*",
			"**.*",
			"**.*",
			"**.*",
		),
		radius:   1.5,
		all:      true,
		diag:     true,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Hypot(dx, dy)
		},

		s: simple.Node(1),
		t: simple.Node(14),

		want: []graph.Node{
			simple.Node(1),
			simple.Node(6),
			simple.Node(10),
			simple.Node(14),
		},
		weight: math.Sqrt2 + 2,
	},
	{
		g: testgraphs.NewGridFrom(
			"...",
			".*.",
			".*.",
			".*.",
			".*.",
		),
		radius:   1,
		all:      true,
		diag:     false,
		remember: []bool{false, true},

		heuristic: func(dx, dy float64) float64 {
			return math.Hypot(dx, dy)
		},

		s: simple.Node(6),
		t: simple.Node(14),

		want: []graph.Node{
			simple.Node(6),
			simple.Node(9),
			simple.Node(12),
			simple.Node(9),
			simple.Node(6),
			simple.Node(3),
			simple.Node(0),
			simple.Node(1),
			simple.Node(2),
			simple.Node(5),
			simple.Node(8),
			simple.Node(11),
			simple.Node(14),
		},
		weight: 12,
	},
}

func TestDStarLiteDynamic(t *testing.T) {
	for i, test := range dynamicDStarLiteTests {
		for _, remember := range test.remember {
			l := &testgraphs.LimitedVisionGrid{
				Grid:         test.g,
				VisionRadius: test.radius,
				Location:     test.s,
			}
			if remember {
				l.Known = make(map[int64]bool)
			}

			l.Grid.AllVisible = test.all

			l.Grid.AllowDiagonal = test.diag
			l.Grid.UnitEdgeWeight = test.unit

			if test.modify != nil {
				test.modify(l)
			}

			got := []graph.Node{test.s}
			l.MoveTo(test.s)

			heuristic := func(a, b graph.Node) float64 {
				ax, ay := l.XY(a.ID())
				bx, by := l.XY(b.ID())
				return test.heuristic(ax-bx, ay-by)
			}

			world := simple.NewWeightedDirectedGraph(0, math.Inf(1))
			d := NewDStarLite(test.s, test.t, l, heuristic, world)
			var (
				dp  *dumper
				buf bytes.Buffer
			)
			_, c := l.Grid.Dims()
			if c <= *maxWide && (*debug || *vdebug) {
				dp = &dumper{
					w: &buf,

					dStarLite: d,
					grid:      l,
				}
			}

			dp.dump(true)
			dp.printEdges("Initial world knowledge: %s\n\n", simpleWeightedEdgesOf(l, graph.EdgesOf(world.Edges())))
			for d.Step() {
				changes, _ := l.MoveTo(d.Here())
				got = append(got, l.Location)
				d.UpdateWorld(changes)
				dp.dump(true)
				if wantedPath, ok := test.wantedPaths[l.Location.ID()]; ok {
					gotPath, _ := d.Path()
					if !samePath(gotPath, wantedPath) {
						t.Errorf("unexpected intermediate path estimation for test %d %s memory:\ngot: %v\nwant:%v",
							i, memory(remember), gotPath, wantedPath)
					}
				}
				dp.printEdges("Edges changing after last step:\n%s\n\n", simpleWeightedEdgesOf(l, changes))
			}

			if weight := weightOf(got, l.Grid); !samePath(got, test.want) || weight != test.weight {
				t.Errorf("unexpected path for test %d %s memory got weight:%v want weight:%v:\ngot: %v\nwant:%v",
					i, memory(remember), weight, test.weight, got, test.want)
				b, err := l.Render(got)
				t.Errorf("path taken (err:%v):\n%s", err, b)
				if c <= *maxWide && (*debug || *vdebug) {
					t.Error(buf.String())
				}
			} else if c <= *maxWide && *vdebug {
				t.Logf("Test %d:\n%s", i, buf.String())
			}
		}
	}
}

type memory bool

func (m memory) String() string {
	if m {
		return "with"
	}
	return "without"
}

// samePath compares two paths for equality ignoring nodes that are nil.
func samePath(a, b []graph.Node) bool {
	if len(a) != len(b) {
		return false
	}
	for i, e := range a {
		if e == nil || b[i] == nil {
			continue
		}
		if e.ID() != b[i].ID() {
			return false
		}
	}
	return true
}

// weightOf return the weight of the path in g.
func weightOf(path []graph.Node, g graph.Weighted) float64 {
	var w float64
	if len(path) > 1 {
		for p, n := range path[1:] {
			ew, ok := g.Weight(path[p].ID(), n.ID())
			if !ok {
				return math.Inf(1)
			}
			w += ew
		}
	}
	return w
}

// simpleWeightedEdgesOf returns the weighted edges in g corresponding to the given edges.
func simpleWeightedEdgesOf(g graph.Weighted, edges []graph.Edge) []simple.WeightedEdge {
	w := make([]simple.WeightedEdge, len(edges))
	for i, e := range edges {
		w[i].F = e.From()
		w[i].T = e.To()
		ew, _ := g.Weight(e.From().ID(), e.To().ID())
		w[i].W = ew
	}
	return w
}
