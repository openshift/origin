// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/path"
)

func TestBellmanFordFrom(t *testing.T) {
	for _, test := range shortestPathTests {
		g := test.g()
		for _, e := range test.edges {
			g.SetEdge(e, e.Cost)
		}

		pt, ok := path.BellmanFordFrom(test.query.From(), g.(graph.Graph))
		if test.hasNegativeCycle {
			if ok {
				t.Errorf("%q: expected negative cycle", test.name)
			}
			continue
		}
		if !ok {
			t.Fatalf("%q: unexpected negative cycle", test.name)
		}

		if pt.From().ID() != test.query.From().ID() {
			t.Fatalf("%q: unexpected from node ID: got:%d want:%d", pt.From().ID(), test.query.From().ID())
		}

		p, weight := pt.To(test.query.To())
		if weight != test.weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
				test.name, weight, test.weight)
		}
		if weight := pt.WeightTo(test.query.To()); weight != test.weight {
			t.Errorf("%q: unexpected weight from Weight: got:%f want:%f",
				test.name, weight, test.weight)
		}

		var got []int
		for _, n := range p {
			got = append(got, n.ID())
		}
		ok = len(got) == 0 && len(test.want) == 0
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

		np, weight := pt.To(test.none.To())
		if pt.From().ID() == test.none.From().ID() && (np != nil || !math.IsInf(weight, 1)) {
			t.Errorf("%q: unexpected path:\ngot: path=%v weight=%f\nwant:path=<nil> weight=+Inf",
				test.name, np, weight)
		}
	}
}
