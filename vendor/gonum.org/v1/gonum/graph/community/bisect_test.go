// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"fmt"
	"log"
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/simple"
)

func ExampleProfile_simple() {
	// Profile calls Modularize which implements the Louvain modularization algorithm.
	// Since this is a randomized algorithm we use a defined random source to ensure
	// consistency between test runs. In practice, results will not differ greatly
	// between runs with different PRNG seeds.
	src := rand.NewSource(1)

	// Create dumbell graph:
	//
	//  0       4
	//  |\     /|
	//  | 2 - 3 |
	//  |/     \|
	//  1       5
	//
	g := simple.NewUndirectedGraph()
	for u, e := range smallDumbell {
		for v := range e {
			g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
		}
	}

	// Get the profile of internal node weight for resolutions
	// between 0.1 and 10 using logarithmic bisection.
	p, err := Profile(ModularScore(g, Weight, 10, src), true, 1e-3, 0.1, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Print out each step with communities ordered.
	for _, d := range p {
		comm := d.Communities()
		for _, c := range comm {
			sort.Sort(ordered.ByID(c))
		}
		sort.Sort(ordered.BySliceIDs(comm))
		fmt.Printf("Low:%.2v High:%.2v Score:%v Communities:%v Q=%.3v\n",
			d.Low, d.High, d.Score, comm, Q(g, comm, d.Low))
	}

	// Output:
	// Low:0.1 High:0.29 Score:14 Communities:[[0 1 2 3 4 5]] Q=0.9
	// Low:0.29 High:2.3 Score:12 Communities:[[0 1 2] [3 4 5]] Q=0.714
	// Low:2.3 High:3.5 Score:4 Communities:[[0 1] [2] [3] [4 5]] Q=-0.31
	// Low:3.5 High:10 Score:0 Communities:[[0] [1] [2] [3] [4] [5]] Q=-0.607
}

var friends, enemies *simple.WeightedUndirectedGraph

func init() {
	friends = simple.NewWeightedUndirectedGraph(0, 0)
	for u, e := range middleEast.friends {
		// Ensure unconnected nodes are included.
		if friends.Node(int64(u)) == nil {
			friends.AddNode(simple.Node(u))
		}
		for v := range e {
			friends.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
		}
	}
	enemies = simple.NewWeightedUndirectedGraph(0, 0)
	for u, e := range middleEast.enemies {
		// Ensure unconnected nodes are included.
		if enemies.Node(int64(u)) == nil {
			enemies.AddNode(simple.Node(u))
		}
		for v := range e {
			enemies.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: -1})
		}
	}
}

func ExampleProfile_multiplex() {
	// Profile calls ModularizeMultiplex which implements the Louvain modularization
	// algorithm. Since this is a randomized algorithm we use a defined random source
	// to ensure consistency between test runs. In practice, results will not differ
	// greatly between runs with different PRNG seeds.
	src := rand.NewSource(1)

	// The undirected graphs, friends and enemies, are the political relationships
	// in the Middle East as described in the Slate article:
	// http://www.slate.com/blogs/the_world_/2014/07/17/the_middle_east_friendship_chart.html
	g, err := NewUndirectedLayers(friends, enemies)
	if err != nil {
		log.Fatal(err)
	}
	weights := []float64{1, -1}

	// Get the profile of internal node weight for resolutions
	// between 0.1 and 10 using logarithmic bisection.
	p, err := Profile(ModularMultiplexScore(g, weights, true, WeightMultiplex, 10, src), true, 1e-3, 0.1, 10)
	if err != nil {
		log.Fatal(err)
	}

	// Print out each step with communities ordered.
	for _, d := range p {
		comm := d.Communities()
		for _, c := range comm {
			sort.Sort(ordered.ByID(c))
		}
		sort.Sort(ordered.BySliceIDs(comm))
		fmt.Printf("Low:%.2v High:%.2v Score:%v Communities:%v Q=%.3v\n",
			d.Low, d.High, d.Score, comm, QMultiplex(g, comm, weights, []float64{d.Low}))
	}

	// Output:
	// Low:0.1 High:0.72 Score:26 Communities:[[0] [1 7 9 12] [2 8 11] [3 4 5 10] [6]] Q=[24.7 1.97]
	// Low:0.72 High:1.1 Score:24 Communities:[[0 6] [1 7 9 12] [2 8 11] [3 4 5 10]] Q=[16.9 14.1]
	// Low:1.1 High:1.2 Score:18 Communities:[[0 2 6 11] [1 7 9 12] [3 4 5 8 10]] Q=[9.16 25.1]
	// Low:1.2 High:1.6 Score:10 Communities:[[0 3 4 5 6 10] [1 7 9 12] [2 8 11]] Q=[10.5 26.7]
	// Low:1.6 High:1.6 Score:8 Communities:[[0 1 6 7 9 12] [2 8 11] [3 4 5 10]] Q=[5.56 39.8]
	// Low:1.6 High:1.8 Score:2 Communities:[[0 2 3 4 5 6 10] [1 7 8 9 11 12]] Q=[-1.82 48.6]
	// Low:1.8 High:2.3 Score:-6 Communities:[[0 2 3 4 5 6 8 10 11] [1 7 9 12]] Q=[-5 57.5]
	// Low:2.3 High:2.4 Score:-10 Communities:[[0 1 2 6 7 8 9 11 12] [3 4 5 10]] Q=[-11.2 79]
	// Low:2.4 High:4.3 Score:-52 Communities:[[0 1 2 3 4 5 6 7 8 9 10 11 12]] Q=[-46.1 117]
	// Low:4.3 High:10 Score:-54 Communities:[[0 1 2 3 4 6 7 8 9 10 11 12] [5]] Q=[-82 254]
}

func TestProfileUndirected(t *testing.T) {
	for _, test := range communityUndirectedQTests {
		g := simple.NewUndirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testProfileUndirected(t, test, g)
	}
}

func TestProfileWeightedUndirected(t *testing.T) {
	for _, test := range communityUndirectedQTests {
		g := simple.NewWeightedUndirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testProfileUndirected(t, test, g)
	}
}

func testProfileUndirected(t *testing.T, test communityUndirectedQTest, g graph.Undirected) {
	fn := ModularScore(g, Weight, 10, nil)
	p, err := Profile(fn, true, 1e-3, 0.1, 10)
	if err != nil {
		t.Errorf("%s: unexpected error: %v", test.name, err)
	}

	const tries = 1000
	for i, d := range p {
		var score float64
		for i := 0; i < tries; i++ {
			score, _ = fn(d.Low)
			if score >= d.Score {
				break
			}
		}
		if score < d.Score {
			t.Errorf("%s: failed to recover low end score: got: %v want: %v", test.name, score, d.Score)
		}
		if i != 0 && d.Score >= p[i-1].Score {
			t.Errorf("%s: not monotonically decreasing: %v -> %v", test.name, p[i-1], d)
		}
	}
}

func TestProfileDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewDirectedGraph()
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetEdge(simple.Edge{F: simple.Node(u), T: simple.Node(v)})
			}
		}

		testProfileDirected(t, test, g)
	}
}

func TestProfileWeightedDirected(t *testing.T) {
	for _, test := range communityDirectedQTests {
		g := simple.NewWeightedDirectedGraph(0, 0)
		for u, e := range test.g {
			// Add nodes that are not defined by an edge.
			if g.Node(int64(u)) == nil {
				g.AddNode(simple.Node(u))
			}
			for v := range e {
				g.SetWeightedEdge(simple.WeightedEdge{F: simple.Node(u), T: simple.Node(v), W: 1})
			}
		}

		testProfileDirected(t, test, g)
	}
}

func testProfileDirected(t *testing.T, test communityDirectedQTest, g graph.Directed) {
	fn := ModularScore(g, Weight, 10, nil)
	p, err := Profile(fn, true, 1e-3, 0.1, 10)
	if err != nil {
		t.Errorf("%s: unexpected error: %v", test.name, err)
	}

	const tries = 1000
	for i, d := range p {
		var score float64
		for i := 0; i < tries; i++ {
			score, _ = fn(d.Low)
			if score >= d.Score {
				break
			}
		}
		if score < d.Score {
			t.Errorf("%s: failed to recover low end score: got: %v want: %v", test.name, score, d.Score)
		}
		if i != 0 && d.Score >= p[i-1].Score {
			t.Errorf("%s: not monotonically decreasing: %v -> %v", test.name, p[i-1], d)
		}
	}
}

func TestProfileUndirectedMultiplex(t *testing.T) {
	for _, test := range communityUndirectedMultiplexQTests {
		g, weights, err := undirectedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

		const all = true

		fn := ModularMultiplexScore(g, weights, all, WeightMultiplex, 10, nil)
		p, err := Profile(fn, true, 1e-3, 0.1, 10)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		const tries = 1000
		for i, d := range p {
			var score float64
			for i := 0; i < tries; i++ {
				score, _ = fn(d.Low)
				if score >= d.Score {
					break
				}
			}
			if score < d.Score {
				t.Errorf("%s: failed to recover low end score: got: %v want: %v", test.name, score, d.Score)
			}
			if i != 0 && d.Score >= p[i-1].Score {
				t.Errorf("%s: not monotonically decreasing: %v -> %v", test.name, p[i-1], d)
			}
		}
	}
}

func TestProfileDirectedMultiplex(t *testing.T) {
	for _, test := range communityDirectedMultiplexQTests {
		g, weights, err := directedMultiplexFrom(test.layers)
		if err != nil {
			t.Errorf("unexpected error creating multiplex: %v", err)
			continue
		}

		const all = true

		fn := ModularMultiplexScore(g, weights, all, WeightMultiplex, 10, nil)
		p, err := Profile(fn, true, 1e-3, 0.1, 10)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}

		const tries = 1000
		for i, d := range p {
			var score float64
			for i := 0; i < tries; i++ {
				score, _ = fn(d.Low)
				if score >= d.Score {
					break
				}
			}
			if score < d.Score {
				t.Errorf("%s: failed to recover low end score: got: %v want: %v", test.name, score, d.Score)
			}
			if i != 0 && d.Score >= p[i-1].Score {
				t.Errorf("%s: not monotonically decreasing: %v -> %v", test.name, p[i-1], d)
			}
		}
	}
}
