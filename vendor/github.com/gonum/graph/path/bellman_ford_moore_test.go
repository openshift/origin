// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package path

import (
	"math"
	"reflect"
	"testing"

	"github.com/gonum/graph"
	"github.com/gonum/graph/path/internal/testgraphs"
)

func TestBellmanFordFrom(t *testing.T) {
	for _, test := range testgraphs.ShortestPathTests {
		g := test.Graph()
		for _, e := range test.Edges {
			g.SetEdge(e)
		}

		pt, ok := BellmanFordFrom(test.Query.From(), g.(graph.Graph))
		if test.HasNegativeCycle {
			if ok {
				t.Errorf("%q: expected negative cycle", test.Name)
			}
			continue
		}
		if !ok {
			t.Fatalf("%q: unexpected negative cycle", test.Name)
		}

		if pt.From().ID() != test.Query.From().ID() {
			t.Fatalf("%q: unexpected from node ID: got:%d want:%d", pt.From().ID(), test.Query.From().ID())
		}

		p, weight := pt.To(test.Query.To())
		if weight != test.Weight {
			t.Errorf("%q: unexpected weight from Between: got:%f want:%f",
				test.Name, weight, test.Weight)
		}
		if weight := pt.WeightTo(test.Query.To()); weight != test.Weight {
			t.Errorf("%q: unexpected weight from Weight: got:%f want:%f",
				test.Name, weight, test.Weight)
		}

		var got []int
		for _, n := range p {
			got = append(got, n.ID())
		}
		ok = len(got) == 0 && len(test.WantPaths) == 0
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

		np, weight := pt.To(test.NoPathFor.To())
		if pt.From().ID() == test.NoPathFor.From().ID() && (np != nil || !math.IsInf(weight, 1)) {
			t.Errorf("%q: unexpected path:\ngot: path=%v weight=%f\nwant:path=<nil> weight=+Inf",
				test.Name, np, weight)
		}
	}
}
