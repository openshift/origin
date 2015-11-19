// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path_test

import (
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/internal"
	"github.com/gonum/graph/path"
)

func TestFloydWarshall(t *testing.T) {
	for _, test := range shortestPathTests {
		g := test.g()
		for _, e := range test.edges {
			g.SetEdge(e, e.Cost)
		}

		pt, ok := path.FloydWarshall(g.(graph.Graph))
		if test.hasNegativeCycle {
			if ok {
				t.Errorf("%q: expected negative cycle", test.name)
			}
			continue
		}
		if !ok {
			t.Fatalf("%q: unexpected negative cycle", test.name)
		}

		// Check all random paths returned are OK.
		for i := 0; i < 10; i++ {
			p, weight, unique := pt.Between(test.query.From(), test.query.To())
			if weight != test.weight {
				t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
					test.name, weight, test.weight)
			}
			if weight := pt.Weight(test.query.From(), test.query.To()); weight != test.weight {
				t.Errorf("%q: unexpected weight from Weight: got:%f want:%f",
					test.name, weight, test.weight)
			}
			if unique != test.unique {
				t.Errorf("%q: unexpected number of paths: got: unique=%t want: unique=%t",
					test.name, unique, test.unique)
			}

			var got []int
			for _, n := range p {
				got = append(got, n.ID())
			}
			ok := len(got) == 0 && len(test.want) == 0
			for _, sp := range test.want {
				if reflect.DeepEqual(got, sp) {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("%q: unexpected shortest path:\ngot: %v\nwant from:%v",
					test.name, p, test.want)
			}
		}

		np, weight, unique := pt.Between(test.none.From(), test.none.To())
		if np != nil || !math.IsInf(weight, 1) || unique != false {
			t.Errorf("%q: unexpected path:\ngot: path=%v weight=%f unique=%t\nwant:path=<nil> weight=+Inf unique=false",
				test.name, np, weight, unique)
		}

		paths, weight := pt.AllBetween(test.query.From(), test.query.To())
		if weight != test.weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
				test.name, weight, test.weight)
		}

		var got [][]int
		if len(paths) != 0 {
			got = make([][]int, len(paths))
		}
		for i, p := range paths {
			for _, v := range p {
				got[i] = append(got[i], v.ID())
			}
		}
		sort.Sort(internal.BySliceValues(got))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("testing %q: unexpected shortest paths:\ngot: %v\nwant:%v",
				test.name, got, test.want)
		}

		nps, weight := pt.AllBetween(test.none.From(), test.none.To())
		if nps != nil || !math.IsInf(weight, 1) {
			t.Errorf("%q: unexpected path:\ngot: paths=%v weight=%f\nwant:path=<nil> weight=+Inf",
				test.name, nps, weight)
		}
	}
}
