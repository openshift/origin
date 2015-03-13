// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package search_test

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/concrete"
	"github.com/gonum/graph/search"
)

func TestSimpleAStar(t *testing.T) {
	tg, err := concrete.GenerateTileGraph("▀  ▀\n▀▀ ▀\n▀▀ ▀\n▀▀ ▀")
	if err != nil {
		t.Fatal("Couldn't generate tilegraph")
	}

	path, cost, _ := search.AStar(concrete.Node(1), concrete.Node(14), tg, nil, nil)
	if math.Abs(cost-4) > 1e-5 {
		t.Errorf("A* reports incorrect cost for simple tilegraph search")
	}

	if path == nil {
		t.Fatalf("A* fails to find path for for simple tilegraph search")
	} else {
		correctPath := []int{1, 2, 6, 10, 14}
		if len(path) != len(correctPath) {
			t.Fatalf("Astar returns wrong length path for simple tilegraph search")
		}
		for i, node := range path {
			if node.ID() != correctPath[i] {
				t.Errorf("Astar returns wrong path at step", i, "got:", node, "actual:", correctPath[i])
			}
		}
	}
}

func TestBiggerAStar(t *testing.T) {
	tg := concrete.NewTileGraph(3, 3, true)

	path, cost, _ := search.AStar(concrete.Node(0), concrete.Node(8), tg, nil, nil)

	if math.Abs(cost-4) > 1e-5 || !search.IsPath(path, tg) {
		t.Error("Non-optimal or impossible path found for 3x3 grid")
	}

	tg = concrete.NewTileGraph(1000, 1000, true)
	path, cost, _ = search.AStar(concrete.Node(0), concrete.Node(999*1000+999), tg, nil, nil)
	if !search.IsPath(path, tg) || cost != 1998 {
		t.Error("Non-optimal or impossible path found for 100x100 grid; cost:", cost, "path:\n"+tg.PathString(path))
	}
}

func TestObstructedAStar(t *testing.T) {
	tg := concrete.NewTileGraph(10, 10, true)

	// Creates a partial "wall" down the middle row with a gap down the left side
	tg.SetPassability(4, 1, false)
	tg.SetPassability(4, 2, false)
	tg.SetPassability(4, 3, false)
	tg.SetPassability(4, 4, false)
	tg.SetPassability(4, 5, false)
	tg.SetPassability(4, 6, false)
	tg.SetPassability(4, 7, false)
	tg.SetPassability(4, 8, false)
	tg.SetPassability(4, 9, false)

	rows, cols := tg.Dimensions()
	path, cost1, expanded := search.AStar(concrete.Node(5), tg.CoordsToNode(rows-1, cols-1), tg, nil, nil)

	if !search.IsPath(path, tg) {
		t.Error("Path doesn't exist in obstructed graph")
	}

	ManhattanHeuristic := func(n1, n2 graph.Node) float64 {
		id1, id2 := n1.ID(), n2.ID()
		r1, c1 := tg.IDToCoords(id1)
		r2, c2 := tg.IDToCoords(id2)

		return math.Abs(float64(r1)-float64(r2)) + math.Abs(float64(c1)-float64(c2))
	}

	path, cost2, expanded2 := search.AStar(concrete.Node(5), tg.CoordsToNode(rows-1, cols-1), tg, nil, ManhattanHeuristic)
	if !search.IsPath(path, tg) {
		t.Error("Path doesn't exist when using heuristic on obstructed graph")
	}

	if math.Abs(cost1-cost2) > 1e-5 {
		t.Error("Cost when using admissible heuristic isn't approximately equal to cost without it")
	}

	if expanded2 > expanded {
		t.Error("Using admissible, consistent heuristic expanded more nodes than null heuristic (possible, but unlikely -- suggests an error somewhere)")
	}

}

func TestNoPathAStar(t *testing.T) {
	tg := concrete.NewTileGraph(5, 5, true)

	// Creates a "wall" down the middle row
	tg.SetPassability(2, 0, false)
	tg.SetPassability(2, 1, false)
	tg.SetPassability(2, 2, false)
	tg.SetPassability(2, 3, false)
	tg.SetPassability(2, 4, false)

	rows, _ := tg.Dimensions()
	path, _, _ := search.AStar(tg.CoordsToNode(0, 2), tg.CoordsToNode(rows-1, 2), tg, nil, nil)

	if len(path) > 0 { // Note that a nil slice will return len of 0, this won't panic
		t.Error("A* finds path where none exists")
	}
}

func TestSmallAStar(t *testing.T) {
	gg := newSmallGonumGraph()
	heur := newSmallHeuristic()
	if ok, edge, goal := monotonic(gg, heur); !ok {
		t.Fatalf("non-monotonic heuristic.  edge: %v goal: %v", edge, goal)
	}
	for _, start := range gg.NodeList() {
		// get reference paths by Dijkstra
		dPaths, dCosts := search.Dijkstra(start, gg, nil)
		// assert that AStar finds each path
		for goalID, dPath := range dPaths {
			exp := fmt.Sprintln(dPath, dCosts[goalID])
			aPath, aCost, _ := search.AStar(start, concrete.Node(goalID), gg, nil, heur)
			got := fmt.Sprintln(aPath, aCost)
			if got != exp {
				t.Error("expected", exp, "got", got)
			}
		}
	}
}

func ExampleBreadthFirstSearch() {
	g := concrete.NewDirectedGraph()
	var n0, n1, n2, n3 concrete.Node = 0, 1, 2, 3
	g.AddDirectedEdge(concrete.Edge{n0, n1}, 1)
	g.AddDirectedEdge(concrete.Edge{n0, n2}, 1)
	g.AddDirectedEdge(concrete.Edge{n2, n3}, 1)
	path, v := search.BreadthFirstSearch(n0, n3, g)
	fmt.Println("path:", path)
	fmt.Println("nodes visited:", v)
	// Output:
	// path: [0 2 3]
	// nodes visited: 4
}

func newSmallGonumGraph() *concrete.Graph {
	eds := []struct{ n1, n2, edgeCost int }{
		{1, 2, 7},
		{1, 3, 9},
		{1, 6, 14},
		{2, 3, 10},
		{2, 4, 15},
		{3, 4, 11},
		{3, 6, 2},
		{4, 5, 7},
		{5, 6, 9},
	}
	g := concrete.NewGraph()
	for n := concrete.Node(1); n <= 6; n++ {
		g.AddNode(n)
	}
	for _, ed := range eds {
		e := concrete.Edge{
			concrete.Node(ed.n1),
			concrete.Node(ed.n2),
		}
		g.AddUndirectedEdge(e, float64(ed.edgeCost))
	}
	return g
}

func newSmallHeuristic() func(n1, n2 graph.Node) float64 {
	nds := []struct{ id, x, y int }{
		{1, 0, 6},
		{2, 1, 0},
		{3, 8, 7},
		{4, 16, 0},
		{5, 17, 6},
		{6, 9, 8},
	}
	return func(n1, n2 graph.Node) float64 {
		i1 := n1.ID() - 1
		i2 := n2.ID() - 1
		dx := nds[i2].x - nds[i1].x
		dy := nds[i2].y - nds[i1].y
		return math.Hypot(float64(dx), float64(dy))
	}
}

type costEdgeListGraph interface {
	graph.Coster
	graph.EdgeListGraph
}

func monotonic(g costEdgeListGraph, heur func(n1, n2 graph.Node) float64) (bool, graph.Edge, graph.Node) {
	for _, goal := range g.NodeList() {
		for _, edge := range g.EdgeList() {
			head := edge.Head()
			tail := edge.Tail()
			if heur(head, goal) > g.Cost(edge)+heur(tail, goal) {
				return false, edge, goal
			}
		}
	}
	return true, nil, nil
}

// Test for correct result on a small graph easily solvable by hand
func TestDijkstraSmall(t *testing.T) {
	g := newSmallGonumGraph()
	paths, lens := search.Dijkstra(concrete.Node(1), g, nil)
	s := fmt.Sprintln(len(paths), len(lens))
	for i := 1; i <= 6; i++ {
		s += fmt.Sprintln(paths[i], lens[i])
	}
	if s != `6 6
[1] 0
[1 2] 7
[1 3] 9
[1 3 4] 20
[1 3 6 5] 20
[1 3 6] 11
` {
		t.Fatal(s)
	}
}

func TestIsPath(t *testing.T) {
	dg := concrete.NewDirectedGraph()
	if !search.IsPath(nil, dg) {
		t.Error("IsPath returns false on nil path")
	}
	p := []graph.Node{concrete.Node(0)}
	if search.IsPath(p, dg) {
		t.Error("IsPath returns true on nonexistant node")
	}
	dg.AddNode(p[0])
	if !search.IsPath(p, dg) {
		t.Error("IsPath returns false on single-length path with existing node")
	}
	p = append(p, concrete.Node(1))
	dg.AddNode(p[1])
	if search.IsPath(p, dg) {
		t.Error("IsPath returns true on bad path of length 2")
	}
	dg.AddDirectedEdge(concrete.Edge{p[0], p[1]}, 1)
	if !search.IsPath(p, dg) {
		t.Error("IsPath returns false on correct path of length 2")
	}
	p[0], p[1] = p[1], p[0]
	if search.IsPath(p, dg) {
		t.Error("IsPath erroneously returns true for a reverse path")
	}
	p = []graph.Node{p[1], p[0], concrete.Node(2)}
	dg.AddDirectedEdge(concrete.Edge{p[1], p[2]}, 1)
	if !search.IsPath(p, dg) {
		t.Error("IsPath does not find a correct path for path > 2 nodes")
	}
	ug := concrete.NewGraph()
	ug.AddUndirectedEdge(concrete.Edge{p[1], p[0]}, 1)
	ug.AddUndirectedEdge(concrete.Edge{p[1], p[2]}, 1)
	if !search.IsPath(p, ug) {
		t.Error("IsPath does not correctly account for undirected behavior")
	}
}

type interval struct{ start, end int }

var tarjanTests = []struct {
	g []set

	ambiguousOrder []interval
	want           [][]int
}{

	{
		g: []set{
			0: linksTo(1),
			1: linksTo(2, 7),
			2: linksTo(3, 6),
			3: linksTo(4),
			4: linksTo(2, 5),
			6: linksTo(3, 5),
			7: linksTo(0, 6),
		},

		want: [][]int{
			{5},
			{2, 3, 4, 6},
			{0, 1, 7},
		},
	},
	{
		g: []set{
			0: linksTo(1, 2, 3),
			1: linksTo(2),
			2: linksTo(3),
			3: linksTo(1),
		},

		want: [][]int{
			{1, 2, 3},
			{0},
		},
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(0, 2),
			2: linksTo(1),
		},

		want: [][]int{
			{0, 1, 2},
		},
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(2, 3),
			2: linksTo(4, 5),
			3: linksTo(4, 5),
			4: linksTo(6),
			5: nil,
			6: nil,
		},

		// Node pairs (2, 3) and (4, 5) are not
		// relatively orderable within each pair.
		ambiguousOrder: []interval{
			{0, 3}, // This includes node 6 since it only needs to be before 4 in topo sort.
			{3, 5},
		},
		want: [][]int{
			{6}, {5}, {4}, {3}, {2}, {1}, {0},
		},
	},
	{
		g: []set{
			0: linksTo(1),
			1: linksTo(2, 3, 4),
			2: linksTo(0, 3),
			3: linksTo(4),
			4: linksTo(3),
		},

		// SCCs are not relatively ordable.
		ambiguousOrder: []interval{
			{0, 2},
		},
		want: [][]int{
			{3, 4}, {0, 1, 2},
		},
	},
}

func TestTarjan(t *testing.T) {
	for i, test := range tarjanTests {
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			g.AddNode(concrete.Node(u))
			for v := range e {
				if !g.NodeExists(concrete.Node(v)) {
					g.AddNode(concrete.Node(v))
				}
				g.AddDirectedEdge(concrete.Edge{H: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		gotSCCs := search.Tarjan(g)
		// tarjan.strongconnect does range iteration over maps,
		// so sort SCC members to ensure consistent ordering.
		gotIDs := make([][]int, len(gotSCCs))
		for i, scc := range gotSCCs {
			gotIDs[i] = make([]int, len(scc))
			for j, id := range scc {
				gotIDs[i][j] = id.ID()
			}
			sort.Ints(gotIDs[i])
		}
		for _, iv := range test.ambiguousOrder {
			sort.Sort(byComponentLengthOrStart(test.want[iv.start:iv.end]))
			sort.Sort(byComponentLengthOrStart(gotIDs[iv.start:iv.end]))
		}
		if !reflect.DeepEqual(gotIDs, test.want) {
			t.Errorf("unexpected Tarjan scc result for %d:\n\tgot:%v\n\twant:%v", i, gotIDs, test.want)
		}
	}
}

// set is an integer set.
type set map[int]struct{}

func linksTo(i ...int) set {
	if len(i) == 0 {
		return nil
	}
	s := make(set)
	for _, v := range i {
		s[v] = struct{}{}
	}
	return s
}

type byComponentLengthOrStart [][]int

func (c byComponentLengthOrStart) Len() int { return len(c) }
func (c byComponentLengthOrStart) Less(i, j int) bool {
	return len(c[i]) < len(c[j]) || (len(c[i]) == len(c[j]) && c[i][0] < c[j][0])
}
func (c byComponentLengthOrStart) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
