// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"github.com/gonum/floats"
	"github.com/gonum/graph/concrete"
)

var hitsTests = []struct {
	g   []set
	tol float64

	wantTol float64
	want    map[int]HubAuthority
}{
	{
		// Example graph from http://www.cis.hut.fi/Opinnot/T-61.6020/2008/pagerank_hits.pdf page 8.
		g: []set{
			A: linksTo(B, C, D),
			B: linksTo(C, D),
			C: linksTo(B),
			D: nil,
		},
		tol: 1e-4,

		wantTol: 1e-4,
		want: map[int]HubAuthority{
			A: {Hub: 0.7887, Authority: 0},
			B: {Hub: 0.5774, Authority: 0.4597},
			C: {Hub: 0.2113, Authority: 0.6280},
			D: {Hub: 0, Authority: 0.6280},
		},
	},
}

func TestHITS(t *testing.T) {
	for i, test := range hitsTests {
		g := concrete.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if !g.Has(concrete.Node(u)) {
				g.AddNode(concrete.Node(u))
			}
			for v := range e {
				g.SetEdge(concrete.Edge{F: concrete.Node(u), T: concrete.Node(v)}, 0)
			}
		}
		got := HITS(g, test.tol)
		prec := 1 - int(math.Log10(test.wantTol))
		for n := range test.g {
			if !floats.EqualWithinAbsOrRel(got[n].Hub, test.want[n].Hub, test.wantTol, test.wantTol) {
				t.Errorf("unexpected HITS result for test %d:\ngot: %v\nwant:%v",
					i, orderedHubAuth(got, prec), orderedHubAuth(test.want, prec))
				break
			}
			if !floats.EqualWithinAbsOrRel(got[n].Authority, test.want[n].Authority, test.wantTol, test.wantTol) {
				t.Errorf("unexpected HITS result for test %d:\ngot: %v\nwant:%v",
					i, orderedHubAuth(got, prec), orderedHubAuth(test.want, prec))
				break
			}
		}
	}
}

func orderedHubAuth(w map[int]HubAuthority, prec int) []keyHubAuthVal {
	o := make(orderedHubAuthMap, 0, len(w))
	for k, v := range w {
		o = append(o, keyHubAuthVal{prec: prec, key: k, val: v})
	}
	sort.Sort(o)
	return o
}

type keyHubAuthVal struct {
	prec int
	key  int
	val  HubAuthority
}

func (kv keyHubAuthVal) String() string {
	return fmt.Sprintf("%d:{H:%.*f, A:%.*f}",
		kv.key, kv.prec, kv.val.Hub, kv.prec, kv.val.Authority,
	)
}

type orderedHubAuthMap []keyHubAuthVal

func (o orderedHubAuthMap) Len() int           { return len(o) }
func (o orderedHubAuthMap) Less(i, j int) bool { return o[i].key < o[j].key }
func (o orderedHubAuthMap) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
