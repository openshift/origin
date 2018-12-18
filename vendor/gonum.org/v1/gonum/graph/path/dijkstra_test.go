// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/path/internal/testgraphs"
	"gonum.org/v1/gonum/graph/traverse"
)

func TestDijkstraFrom(t *testing.T) {
	for _, test := range testgraphs.ShortestPathTests {
		g := test.Graph()
		for _, e := range test.Edges {
			g.SetWeightedEdge(e)
		}

		for _, tg := range []struct {
			typ string
			g   traverse.Graph
		}{
			{"complete", g.(graph.Graph)},
			{"incremental", incremental{g.(graph.Weighted)}},
		} {
			var (
				pt Shortest

				panicked bool
			)
			func() {
				defer func() {
					panicked = recover() != nil
				}()
				pt = DijkstraFrom(test.Query.From(), tg.g)
			}()
			if panicked || test.HasNegativeWeight {
				if !test.HasNegativeWeight {
					t.Errorf("%q %s: unexpected panic", test.Name, tg.typ)
				}
				if !panicked {
					t.Errorf("%q %s: expected panic for negative edge weight", test.Name, tg.typ)
				}
				continue
			}

			if pt.From().ID() != test.Query.From().ID() {
				t.Fatalf("%q %s: unexpected from node ID: got:%d want:%d", test.Name, tg.typ, pt.From().ID(), test.Query.From().ID())
			}

			p, weight := pt.To(test.Query.To().ID())
			if weight != test.Weight {
				t.Errorf("%q %s: unexpected weight from Between: got:%f want:%f",
					test.Name, tg.typ, weight, test.Weight)
			}
			if weight := pt.WeightTo(test.Query.To().ID()); weight != test.Weight {
				t.Errorf("%q %s: unexpected weight from Weight: got:%f want:%f",
					test.Name, tg.typ, weight, test.Weight)
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
				t.Errorf("%q %s: unexpected shortest path:\ngot: %v\nwant from:%v",
					test.Name, tg.typ, p, test.WantPaths)
			}

			np, weight := pt.To(test.NoPathFor.To().ID())
			if pt.From().ID() == test.NoPathFor.From().ID() && (np != nil || !math.IsInf(weight, 1)) {
				t.Errorf("%q %s: unexpected path:\ngot: path=%v weight=%f\nwant:path=<nil> weight=+Inf",
					test.Name, tg.typ, np, weight)
			}
		}
	}
}

type weightedTraverseGraph interface {
	traverse.Graph
	Weighted
}

type incremental struct {
	weightedTraverseGraph
}

func TestDijkstraAllPaths(t *testing.T) {
	for _, test := range testgraphs.ShortestPathTests {
		g := test.Graph()
		for _, e := range test.Edges {
			g.SetWeightedEdge(e)
		}

		var (
			pt AllShortest

			panicked bool
		)
		func() {
			defer func() {
				panicked = recover() != nil
			}()
			pt = DijkstraAllPaths(g.(graph.Graph))
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

		// Check all random paths returned are OK.
		for i := 0; i < 10; i++ {
			p, weight, unique := pt.Between(test.Query.From().ID(), test.Query.To().ID())
			if weight != test.Weight {
				t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
					test.Name, weight, test.Weight)
			}
			if weight := pt.Weight(test.Query.From().ID(), test.Query.To().ID()); weight != test.Weight {
				t.Errorf("%q: unexpected weight from Weight: got:%f want:%f",
					test.Name, weight, test.Weight)
			}
			if unique != test.HasUniquePath {
				t.Errorf("%q: unexpected number of paths: got: unique=%t want: unique=%t",
					test.Name, unique, test.HasUniquePath)
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

		np, weight, unique := pt.Between(test.NoPathFor.From().ID(), test.NoPathFor.To().ID())
		if np != nil || !math.IsInf(weight, 1) || unique {
			t.Errorf("%q: unexpected path:\ngot: path=%v weight=%f unique=%t\nwant:path=<nil> weight=+Inf unique=false",
				test.Name, np, weight, unique)
		}

		paths, weight := pt.AllBetween(test.Query.From().ID(), test.Query.To().ID())
		if weight != test.Weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
				test.Name, weight, test.Weight)
		}

		var got [][]int64
		if len(paths) != 0 {
			got = make([][]int64, len(paths))
		}
		for i, p := range paths {
			for _, v := range p {
				got[i] = append(got[i], v.ID())
			}
		}
		sort.Sort(ordered.BySliceValues(got))
		if !reflect.DeepEqual(got, test.WantPaths) {
			t.Errorf("testing %q: unexpected shortest paths:\ngot: %v\nwant:%v",
				test.Name, got, test.WantPaths)
		}

		nps, weight := pt.AllBetween(test.NoPathFor.From().ID(), test.NoPathFor.To().ID())
		if nps != nil || !math.IsInf(weight, 1) {
			t.Errorf("%q: unexpected path:\ngot: paths=%v weight=%f\nwant:path=<nil> weight=+Inf",
				test.Name, nps, weight)
		}
	}
}
