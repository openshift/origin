// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

import (
	"flag"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/exp/rand"
)

var (
	genDot   = flag.Bool("dot", false, "generate dot code for failing trees")
	dotLimit = flag.Int("dotmax", 100, "specify maximum size for tree output for dot format")
)

var (
	// Using example from WP article: https://en.wikipedia.org/w/index.php?title=K-d_tree&oldid=887573572.
	wpData   = Points{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}
	nbWpData = nbPoints{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}
	wpBound  = &Bounding{Point{2, 1}, Point{9, 7}}
)

var newTests = []struct {
	data       Interface
	bounding   bool
	wantBounds *Bounding
}{
	{data: wpData, bounding: false, wantBounds: nil},
	{data: nbWpData, bounding: false, wantBounds: nil},
	{data: wpData, bounding: true, wantBounds: wpBound},
	{data: nbWpData, bounding: true, wantBounds: nil},
}

func TestNew(t *testing.T) {
	for i, test := range newTests {
		var tree *Tree
		var panicked bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			tree = New(test.data, test.bounding)
		}()
		if panicked {
			t.Errorf("unexpected panic for test %d", i)
			continue
		}

		if !tree.Root.isKDTree() {
			t.Errorf("tree %d is not k-d tree", i)
		}

		switch data := test.data.(type) {
		case Points:
			for _, p := range data {
				if !tree.Contains(p) {
					t.Errorf("failed to find point %.3f in test %d", p, i)
				}
			}
		case nbPoints:
			for _, p := range data {
				if !tree.Contains(p) {
					t.Errorf("failed to find point %.3f in test %d", p, i)
				}
			}
		default:
			t.Fatalf("bad test: unknown data type: %T", test.data)
		}

		if !reflect.DeepEqual(tree.Root.Bounding, test.wantBounds) {
			t.Errorf("unexpected bounding box for test %d with data type %T: got:%v want:%v",
				i, test.data, tree.Root.Bounding, test.wantBounds)
		}

		if t.Failed() && *genDot && tree.Len() <= *dotLimit {
			err := dotFile(tree, fmt.Sprintf("TestNew%T", test.data), "")
			if err != nil {
				t.Fatalf("failed to write DOT file: %v", err)
			}
		}
	}
}

var insertTests = []struct {
	data       Interface
	insert     []Comparable
	wantBounds *Bounding
}{
	{
		data:       wpData,
		insert:     []Comparable{Point{0, 0}, Point{10, 10}},
		wantBounds: &Bounding{Point{0, 0}, Point{10, 10}},
	},
	{
		data:       nbWpData,
		insert:     []Comparable{nbPoint{0, 0}, nbPoint{10, 10}},
		wantBounds: nil,
	},
}

func TestInsert(t *testing.T) {
	for i, test := range insertTests {
		tree := New(test.data, true)
		for _, v := range test.insert {
			tree.Insert(v, true)
		}

		if !tree.Root.isKDTree() {
			t.Errorf("tree %d is not k-d tree", i)
		}

		if !reflect.DeepEqual(tree.Root.Bounding, test.wantBounds) {
			t.Errorf("unexpected bounding box for test %d with data type %T: got:%v want:%v",
				i, test.data, tree.Root.Bounding, test.wantBounds)
		}

		if t.Failed() && *genDot && tree.Len() <= *dotLimit {
			err := dotFile(tree, fmt.Sprintf("TestInsert%T", test.data), "")
			if err != nil {
				t.Fatalf("failed to write DOT file: %v", err)
			}
		}
	}
}

type compFn func(float64) bool

func left(v float64) bool  { return v <= 0 }
func right(v float64) bool { return !left(v) }

func (n *Node) isKDTree() bool {
	if n == nil {
		return true
	}
	d := n.Point.Dims()
	// Together these define the property of minimal orthogonal bounding.
	if !(n.isContainedBy(n.Bounding) && n.Bounding.planesHaveCoincidentPointsIn(n, [2][]bool{make([]bool, d), make([]bool, d)})) {
		return false
	}
	if !n.Left.isPartitioned(n.Point, left, n.Plane) {
		return false
	}
	if !n.Right.isPartitioned(n.Point, right, n.Plane) {
		return false
	}
	return n.Left.isKDTree() && n.Right.isKDTree()
}

func (n *Node) isPartitioned(pivot Comparable, fn compFn, plane Dim) bool {
	if n == nil {
		return true
	}
	if n.Left != nil && fn(pivot.Compare(n.Left.Point, plane)) {
		return false
	}
	if n.Right != nil && fn(pivot.Compare(n.Right.Point, plane)) {
		return false
	}
	return n.Left.isPartitioned(pivot, fn, plane) && n.Right.isPartitioned(pivot, fn, plane)
}

func (n *Node) isContainedBy(b *Bounding) bool {
	if n == nil {
		return true
	}
	if !b.Contains(n.Point) {
		return false
	}
	return n.Left.isContainedBy(b) && n.Right.isContainedBy(b)
}

func (b *Bounding) planesHaveCoincidentPointsIn(n *Node, tight [2][]bool) bool {
	if b == nil {
		return true
	}
	if n == nil {
		return true
	}

	b.planesHaveCoincidentPointsIn(n.Left, tight)
	b.planesHaveCoincidentPointsIn(n.Right, tight)

	var ok = true
	for i := range tight {
		for d := 0; d < n.Point.Dims(); d++ {
			if c := n.Point.Compare(b.Min, Dim(d)); c == 0 {
				tight[i][d] = true
			}
			ok = ok && tight[i][d]
		}
	}
	return ok
}

func nearest(q Point, p Points) (Point, float64) {
	min := q.Distance(p[0])
	var r int
	for i := 1; i < p.Len(); i++ {
		d := q.Distance(p[i])
		if d < min {
			min = d
			r = i
		}
	}
	return p[r], min
}

func TestNearestRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	const (
		min = 0.0
		max = 1000.0

		dims    = 4
		setSize = 10000
	)

	var randData Points
	for i := 0; i < setSize; i++ {
		p := make(Point, dims)
		for j := 0; j < dims; j++ {
			p[j] = (max-min)*rnd.Float64() + min
		}
		randData = append(randData, p)
	}
	tree := New(randData, false)

	for i := 0; i < setSize; i++ {
		q := make(Point, dims)
		for j := 0; j < dims; j++ {
			q[j] = (max-min)*rnd.Float64() + min
		}

		got, _ := tree.Nearest(q)
		want, _ := nearest(q, randData)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected result from query %d %.3f: got:%.3f want:%.3f", i, q, got, want)
		}
	}
}

func TestNearest(t *testing.T) {
	tree := New(wpData, false)
	for _, q := range append([]Point{
		{4, 6},
		{7, 5},
		{8, 7},
		{6, -5},
		{1e5, 1e5},
		{1e5, -1e5},
		{-1e5, 1e5},
		{-1e5, -1e5},
		{1e5, 0},
		{0, -1e5},
		{0, 1e5},
		{-1e5, 0},
	}, wpData...) {
		gotP, gotD := tree.Nearest(q)
		wantP, wantD := nearest(q, wpData)
		if !reflect.DeepEqual(gotP, wantP) {
			t.Errorf("unexpected result for query %.3f: got:%.3f want:%.3f", q, gotP, wantP)
		}
		if gotD != wantD {
			t.Errorf("unexpected distance for query %.3f : got:%v want:%v", q, gotD, wantD)
		}
	}
}

func nearestN(n int, q Point, p Points) []ComparableDist {
	nk := NewNKeeper(n)
	for i := 0; i < p.Len(); i++ {
		nk.Keep(ComparableDist{Comparable: p[i], Dist: q.Distance(p[i])})
	}
	if len(nk.Heap) == 1 {
		return nk.Heap
	}
	sort.Sort(nk)
	for i, j := 0, len(nk.Heap)-1; i < j; i, j = i+1, j-1 {
		nk.Heap[i], nk.Heap[j] = nk.Heap[j], nk.Heap[i]
	}
	return nk.Heap
}

func TestNearestSetN(t *testing.T) {
	data := append([]Point{
		{4, 6},
		{7, 5},
		{8, 7},
		{6, -5},
		{1e5, 1e5},
		{1e5, -1e5},
		{-1e5, 1e5},
		{-1e5, -1e5},
		{1e5, 0},
		{0, -1e5},
		{0, 1e5},
		{-1e5, 0}},
		wpData[:len(wpData)-1]...)

	tree := New(wpData, false)
	for k := 1; k <= len(wpData); k++ {
		for _, q := range data {
			wantP := nearestN(k, q, wpData)

			nk := NewNKeeper(k)
			tree.NearestSet(nk, q)

			var max float64
			wantD := make(map[float64]map[string]struct{})
			for _, p := range wantP {
				if p.Dist > max {
					max = p.Dist
				}
				d, ok := wantD[p.Dist]
				if !ok {
					d = make(map[string]struct{})
				}
				d[fmt.Sprint(p.Comparable)] = struct{}{}
				wantD[p.Dist] = d
			}
			gotD := make(map[float64]map[string]struct{})
			for _, p := range nk.Heap {
				if p.Dist > max {
					t.Errorf("unexpected distance for point %.3f: got:%v want:<=%v", p.Comparable, p.Dist, max)
				}
				d, ok := gotD[p.Dist]
				if !ok {
					d = make(map[string]struct{})
				}
				d[fmt.Sprint(p.Comparable)] = struct{}{}
				gotD[p.Dist] = d
			}

			// If the available number of slots does not fit all the coequal furthest points
			// we will fail the check. So remove, but check them minimally here.
			if !reflect.DeepEqual(wantD[max], gotD[max]) {
				// The best we can do at this stage is confirm that there are an equal number of matches at this distance.
				if len(gotD[max]) != len(wantD[max]) {
					t.Errorf("unexpected number of maximal distance points: got:%d want:%d", len(gotD[max]), len(wantD[max]))
				}
				delete(wantD, max)
				delete(gotD, max)
			}

			if !reflect.DeepEqual(gotD, wantD) {
				t.Errorf("unexpected result for k=%d query %.3f: got:%v want:%v", k, q, gotD, wantD)
			}
		}
	}
}

var nearestSetDistTests = []Point{
	{4, 6},
	{7, 5},
	{8, 7},
	{6, -5},
}

func TestNearestSetDist(t *testing.T) {
	tree := New(wpData, false)
	for i, q := range nearestSetDistTests {
		for d := 1.0; d < 100; d += 0.1 {
			dk := NewDistKeeper(d)
			tree.NearestSet(dk, q)

			hits := make(map[string]float64)
			for _, p := range wpData {
				hits[fmt.Sprint(p)] = p.Distance(q)
			}

			for _, p := range dk.Heap {
				var done bool
				if p.Comparable == nil {
					done = true
					continue
				}
				delete(hits, fmt.Sprint(p.Comparable))
				if done {
					t.Error("expectedly finished heap iteration")
					break
				}
				dist := p.Comparable.Distance(q)
				if dist > d {
					t.Errorf("Test %d: query %v found %v expect %.3f <= %.3f", i, q, p, dist, d)
				}
			}

			for p, dist := range hits {
				if dist <= d {
					t.Errorf("Test %d: query %v missed %v expect %.3f > %.3f", i, q, p, dist, d)
				}
			}
		}
	}
}

func TestDo(t *testing.T) {
	tree := New(wpData, false)
	var got Points
	fn := func(c Comparable, _ *Bounding, _ int) (done bool) {
		got = append(got, c.(Point))
		return
	}
	killed := tree.Do(fn)
	if !reflect.DeepEqual(got, wpData) {
		t.Errorf("unexpected result from tree iteration: got:%v want:%v", got, wpData)
	}
	if killed {
		t.Error("tree iteration unexpectedly killed")
	}
}

var doBoundedTests = []struct {
	bounds *Bounding
	want   Points
}{
	{
		bounds: nil,
		want:   wpData,
	},
	{
		bounds: &Bounding{Point{0, 0}, Point{10, 10}},
		want:   wpData,
	},
	{
		bounds: &Bounding{Point{3, 4}, Point{10, 10}},
		want:   Points{Point{5, 4}, Point{4, 7}, Point{9, 6}},
	},
	{
		bounds: &Bounding{Point{3, 3}, Point{10, 10}},
		want:   Points{Point{5, 4}, Point{4, 7}, Point{9, 6}},
	},
	{
		bounds: &Bounding{Point{0, 0}, Point{6, 5}},
		want:   Points{Point{2, 3}, Point{5, 4}},
	},
	{
		bounds: &Bounding{Point{5, 2}, Point{7, 4}},
		want:   Points{Point{5, 4}, Point{7, 2}},
	},
	{
		bounds: &Bounding{Point{2, 2}, Point{7, 4}},
		want:   Points{Point{2, 3}, Point{5, 4}, Point{7, 2}},
	},
	{
		bounds: &Bounding{Point{2, 3}, Point{9, 6}},
		want:   Points{Point{2, 3}, Point{5, 4}, Point{9, 6}},
	},
	{
		bounds: &Bounding{Point{7, 2}, Point{7, 2}},
		want:   Points{Point{7, 2}},
	},
}

func TestDoBounded(t *testing.T) {
	for _, test := range doBoundedTests {
		tree := New(wpData, false)
		var got Points
		fn := func(c Comparable, _ *Bounding, _ int) (done bool) {
			got = append(got, c.(Point))
			return
		}
		killed := tree.DoBounded(test.bounds, fn)
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result from bounded tree iteration: got:%v want:%v", got, test.want)
		}
		if killed {
			t.Error("tree iteration unexpectedly killed")
		}
	}
}

func BenchmarkNew(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	p := make(Points, 1e5)
	for i := range p {
		p[i] = Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(p, false)
	}
}

func BenchmarkNewBounds(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	p := make(Points, 1e5)
	for i := range p {
		p[i] = Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = New(p, true)
	}
}

func BenchmarkInsert(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	t := &Tree{}
	for i := 0; i < b.N; i++ {
		t.Insert(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, false)
	}
}

func BenchmarkInsertBounds(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	t := &Tree{}
	for i := 0; i < b.N; i++ {
		t.Insert(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, true)
	}
}

func Benchmark(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))
	data := make(Points, 1e2)
	for i := range data {
		data[i] = Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
	}
	tree := New(data, true)

	if !tree.Root.isKDTree() {
		b.Fatal("tree is not k-d tree")
	}

	for i := 0; i < 1e3; i++ {
		q := Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
		gotP, gotD := tree.Nearest(q)
		wantP, wantD := nearest(q, data)
		if !reflect.DeepEqual(gotP, wantP) {
			b.Errorf("unexpected result for query %.3f: got:%.3f want:%.3f", q, gotP, wantP)
		}
		if gotD != wantD {
			b.Errorf("unexpected distance for query %.3f : got:%v want:%v", q, gotD, wantD)
		}
	}

	if b.Failed() && *genDot && tree.Len() <= *dotLimit {
		err := dotFile(tree, "TestBenches", "")
		if err != nil {
			b.Fatalf("failed to write DOT file: %v", err)
		}
		return
	}

	var r Comparable
	var d float64
	queryBenchmarks := []struct {
		name string
		fn   func(*testing.B)
	}{
		{
			name: "Nearest", fn: func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					r, d = tree.Nearest(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()})
				}
				if r == nil {
					b.Error("unexpected nil result")
				}
				if math.IsNaN(d) {
					b.Error("unexpected NaN result")
				}
			},
		},
		{
			name: "NearestBrute", fn: func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					r, d = nearest(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, data)
				}
				if r == nil {
					b.Error("unexpected nil result")
				}
				if math.IsNaN(d) {
					b.Error("unexpected NaN result")
				}
			},
		},
		{
			name: "NearestSetN10", fn: func(b *testing.B) {
				nk := NewNKeeper(10)
				for i := 0; i < b.N; i++ {
					tree.NearestSet(nk, Point{rnd.Float64(), rnd.Float64(), rnd.Float64()})
					if nk.Len() != 10 {
						b.Error("unexpected result length")
					}
					nk.Heap = nk.Heap[:1]
					nk.Heap[0] = ComparableDist{Dist: inf}
				}
			},
		},
		{
			name: "NearestBruteN10", fn: func(b *testing.B) {
				var r []ComparableDist
				for i := 0; i < b.N; i++ {
					r = nearestN(10, Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, data)
				}
				if len(r) != 10 {
					b.Error("unexpected result length", len(r))
				}
			},
		},
	}
	for _, bench := range queryBenchmarks {
		b.Run(bench.name, bench.fn)
	}
}

func dot(t *Tree, label string) string {
	if t == nil {
		return ""
	}
	var (
		s      []string
		follow func(*Node)
	)
	follow = func(n *Node) {
		id := uintptr(unsafe.Pointer(n))
		c := fmt.Sprintf("%d[label = \"<Left> |<Elem> %s/%.3f\\n%.3f|<Right>\"];",
			id, n, n.Point.(Point)[n.Plane], *n.Bounding)
		if n.Left != nil {
			c += fmt.Sprintf("\n\t\tedge [arrowhead=normal]; \"%d\":Left -> \"%d\":Elem;",
				id, uintptr(unsafe.Pointer(n.Left)))
			follow(n.Left)
		}
		if n.Right != nil {
			c += fmt.Sprintf("\n\t\tedge [arrowhead=normal]; \"%d\":Right -> \"%d\":Elem;",
				id, uintptr(unsafe.Pointer(n.Right)))
			follow(n.Right)
		}
		s = append(s, c)
	}
	if t.Root != nil {
		follow(t.Root)
	}
	return fmt.Sprintf("digraph %s {\n\tnode [shape=record,height=0.1];\n\t%s\n}\n",
		label,
		strings.Join(s, "\n\t"),
	)
}

func dotFile(t *Tree, label, dotString string) (err error) {
	if t == nil && dotString == "" {
		return
	}
	f, err := os.Create(label + ".dot")
	if err != nil {
		return
	}
	defer f.Close()
	if dotString == "" {
		fmt.Fprint(f, dot(t, label))
	} else {
		fmt.Fprint(f, dotString)
	}
	return
}
