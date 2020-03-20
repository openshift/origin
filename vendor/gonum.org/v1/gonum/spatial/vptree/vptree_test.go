// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vptree

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
	wpData = []Comparable{
		Point{2, 3},
		Point{5, 4},
		Point{9, 6},
		Point{4, 7},
		Point{8, 1},
		Point{7, 2},
	}
)

var newTests = []struct {
	data   []Comparable
	effort int
}{
	{data: wpData, effort: 0},
	{data: wpData, effort: 1},
	{data: wpData, effort: 2},
	{data: wpData, effort: 4},
	{data: wpData, effort: 8},
}

func TestNew(t *testing.T) {
	for i, test := range newTests {
		var tree *Tree
		var err error
		var panicked bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
				}
			}()
			tree, err = New(test.data, test.effort, rand.NewSource(1))
		}()
		if panicked {
			t.Errorf("unexpected panic for test %d", i)
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for test %d: %v", i, err)
			continue
		}

		if !tree.Root.isVPTree() {
			t.Errorf("tree %d is not vp-tree", i)
		}

		if t.Failed() && *genDot && tree.Len() <= *dotLimit {
			err := dotFile(tree, fmt.Sprintf("TestNew%d", i), "")
			if err != nil {
				t.Fatalf("failed to write DOT file: %v", err)
			}
		}
	}
}

type compFn func(v, radius float64) bool

func closer(v, radius float64) bool  { return v <= radius }
func further(v, radius float64) bool { return v >= radius }

func (n *Node) isVPTree() bool {
	if n == nil {
		return true
	}
	if !n.Closer.isPartitioned(n.Point, closer, n.Radius) {
		return false
	}
	if !n.Further.isPartitioned(n.Point, further, n.Radius) {
		return false
	}
	return n.Closer.isVPTree() && n.Further.isVPTree()
}

func (n *Node) isPartitioned(vp Comparable, fn compFn, radius float64) bool {
	if n == nil {
		return true
	}
	if n.Closer != nil && !fn(vp.Distance(n.Closer.Point), radius) {
		return false
	}
	if n.Further != nil && !fn(vp.Distance(n.Further.Point), radius) {
		return false
	}
	return n.Closer.isPartitioned(vp, fn, radius) && n.Further.isPartitioned(vp, fn, radius)
}

func nearest(q Comparable, p []Comparable) (Comparable, float64) {
	min := q.Distance(p[0])
	var r int
	for i := 1; i < len(p); i++ {
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

	var randData []Comparable
	for i := 0; i < setSize; i++ {
		p := make(Point, dims)
		for j := 0; j < dims; j++ {
			p[j] = (max-min)*rnd.Float64() + min
		}
		randData = append(randData, p)
	}
	tree, err := New(randData, 10, rand.NewSource(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	tree, err := New(wpData, 3, rand.NewSource(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, q := range append([]Comparable{
		Point{4, 6},
		// Point{7, 5}, // Omitted because it is ambiguously finds [9 6] or [5 4].
		Point{8, 7},
		Point{6, -5},
		Point{1e5, 1e5},
		Point{1e5, -1e5},
		Point{-1e5, 1e5},
		Point{-1e5, -1e5},
		Point{1e5, 0},
		Point{0, -1e5},
		Point{0, 1e5},
		Point{-1e5, 0},
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

func nearestN(n int, q Comparable, p []Comparable) []ComparableDist {
	nk := NewNKeeper(n)
	for i := 0; i < len(p); i++ {
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
	data := append([]Comparable{
		Point{4, 6},
		Point{7, 5}, // OK here because we collect N.
		Point{8, 7},
		Point{6, -5},
		Point{1e5, 1e5},
		Point{1e5, -1e5},
		Point{-1e5, 1e5},
		Point{-1e5, -1e5},
		Point{1e5, 0},
		Point{0, -1e5},
		Point{0, 1e5},
		Point{-1e5, 0}},
		wpData[:len(wpData)-1]...)

	tree, err := New(wpData, 3, rand.NewSource(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	tree, err := New(wpData, 3, rand.NewSource(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	tree, err := New(wpData, 3, rand.NewSource(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got []Point
	fn := func(c Comparable, _ int) (done bool) {
		got = append(got, c.(Point))
		return
	}
	killed := tree.Do(fn)

	want := make([]Point, len(wpData))
	for i, p := range wpData {
		want[i] = p.(Point)
	}
	sort.Sort(lexical(got))
	sort.Sort(lexical(want))

	if !reflect.DeepEqual(got, want) {
		t.Errorf("unexpected result from tree iteration: got:%v want:%v", got, want)
	}
	if killed {
		t.Error("tree iteration unexpectedly killed")
	}
}

type lexical []Point

func (c lexical) Len() int { return len(c) }
func (c lexical) Less(i, j int) bool {
	a, b := c[i], c[j]
	l := len(a)
	if len(b) < l {
		l = len(b)
	}
	for k, v := range a[:l] {
		if v < b[k] {
			return true
		}
		if v > b[k] {
			return false
		}
	}
	return len(a) < len(b)
}
func (c lexical) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

func BenchmarkNew(b *testing.B) {
	for _, effort := range []int{0, 10, 100} {
		b.Run(fmt.Sprintf("New:%d", effort), func(b *testing.B) {
			rnd := rand.New(rand.NewSource(1))
			p := make([]Comparable, 1e5)
			for i := range p {
				p[i] = Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := New(p, effort, rand.NewSource(1))
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func Benchmark(b *testing.B) {
	var r Comparable
	var d float64
	queryBenchmarks := []struct {
		name string
		fn   func(data []Comparable, tree *Tree, rnd *rand.Rand) func(*testing.B)
	}{
		{
			name: "NearestBrute", fn: func(data []Comparable, _ *Tree, rnd *rand.Rand) func(b *testing.B) {
				return func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						r, d = nearest(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, data)
					}
					if r == nil {
						b.Error("unexpected nil result")
					}
					if math.IsNaN(d) {
						b.Error("unexpected NaN result")
					}
				}
			},
		},
		{
			name: "NearestBruteN10", fn: func(data []Comparable, _ *Tree, rnd *rand.Rand) func(b *testing.B) {
				return func(b *testing.B) {
					var r []ComparableDist
					for i := 0; i < b.N; i++ {
						r = nearestN(10, Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}, data)
					}
					if len(r) != 10 {
						b.Error("unexpected result length", len(r))
					}
				}
			},
		},
		{
			name: "Nearest", fn: func(_ []Comparable, tree *Tree, rnd *rand.Rand) func(b *testing.B) {
				return func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						r, d = tree.Nearest(Point{rnd.Float64(), rnd.Float64(), rnd.Float64()})
					}
					if r == nil {
						b.Error("unexpected nil result")
					}
					if math.IsNaN(d) {
						b.Error("unexpected NaN result")
					}
				}
			},
		},
		{
			name: "NearestSetN10", fn: func(_ []Comparable, tree *Tree, rnd *rand.Rand) func(b *testing.B) {
				return func(b *testing.B) {
					nk := NewNKeeper(10)
					for i := 0; i < b.N; i++ {
						tree.NearestSet(nk, Point{rnd.Float64(), rnd.Float64(), rnd.Float64()})
						if nk.Len() != 10 {
							b.Error("unexpected result length")
						}
						nk.Heap = nk.Heap[:1]
						nk.Heap[0] = ComparableDist{Dist: inf}
					}
				}
			},
		},
	}

	for _, effort := range []int{0, 3, 10, 30, 100, 300} {
		rnd := rand.New(rand.NewSource(1))
		data := make([]Comparable, 1e5)
		for i := range data {
			data[i] = Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
		}
		tree, err := New(data, effort, rand.NewSource(1))
		if err != nil {
			b.Errorf("unexpected error for effort=%d: %v", effort, err)
			continue
		}

		if !tree.Root.isVPTree() {
			b.Fatal("tree is not vantage point tree")
		}

		for i := 0; i < 1e3; i++ {
			q := Point{rnd.Float64(), rnd.Float64(), rnd.Float64()}
			gotP, gotD := tree.Nearest(q)
			wantP, wantD := nearest(q, data)
			if !reflect.DeepEqual(gotP, wantP) {
				b.Errorf("unexpected result for query %.3f: got:%.3f want:%.3f", q, gotP, wantP)
			}
			if gotD != wantD {
				b.Errorf("unexpected distance for query %.3f: got:%v want:%v", q, gotD, wantD)
			}
		}

		if b.Failed() && *genDot && tree.Len() <= *dotLimit {
			err := dotFile(tree, "TestBenches", "")
			if err != nil {
				b.Fatalf("failed to write DOT file: %v", err)
			}
			return
		}

		for _, bench := range queryBenchmarks {
			if strings.Contains(bench.name, "Brute") && effort != 0 {
				continue
			}
			b.Run(fmt.Sprintf("%s:%d", bench.name, effort), bench.fn(data, tree, rnd))
		}
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
		c := fmt.Sprintf("%d[label = \"<Closer> |<Elem> %.3f/%.3f|<Further>\"];",
			id, n.Point, n.Radius)
		if n.Closer != nil {
			c += fmt.Sprintf("\n\t\tedge [arrowhead=normal]; \"%d\":Closer -> \"%d\":Elem [label=%.3f];",
				id, uintptr(unsafe.Pointer(n.Closer)), n.Point.Distance(n.Closer.Point))
			follow(n.Closer)
		}
		if n.Further != nil {
			c += fmt.Sprintf("\n\t\tedge [arrowhead=normal]; \"%d\":Further -> \"%d\":Elem [label=%.3f];",
				id, uintptr(unsafe.Pointer(n.Further)), n.Point.Distance(n.Further.Point))
			follow(n.Further)
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
