// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testgraph provides a set of testing helper functions
// that test gonum graph interface implementations.
package testgraph // import "gonum.org/v1/gonum/graph/testgraph"

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/internal/ordered"
	"gonum.org/v1/gonum/graph/internal/set"
	"gonum.org/v1/gonum/mat"
)

// BUG(kortschak): Edge equality is tested in part with reflect.DeepEqual and
// direct equality of weight values. This means that edges returned by graphs
// must not contain NaN values. Weights returned by the Weight method are
// compared with NaN-awareness, so they may be NaN when there is no edge
// associated with the Weight call.

func isValidIterator(it graph.Iterator) bool {
	return it != nil
}

func checkEmptyIterator(t *testing.T, it graph.Iterator, useEmpty bool) {
	t.Helper()

	if it.Len() != 0 {
		return
	}
	if it != graph.Empty {
		if useEmpty {
			t.Errorf("unexpected empty iterator: got:%T", it)
			return
		}
		// Only log this since we say that a graph should
		// return a graph.Empty when it is empty.
		t.Logf("unexpected empty iterator: got:%T", it)
	}
}

// Edge supports basic edge operations.
type Edge interface {
	// From returns the from node of the edge.
	From() graph.Node

	// To returns the to node of the edge.
	To() graph.Node
}

// WeightedLine is a generalized graph edge that supports all graph
// edge operations except reversal.
type WeightedLine interface {
	Edge

	// ID returns the unique ID for the Line.
	ID() int64

	// Weight returns the weight of the edge.
	Weight() float64
}

// A Builder function returns a graph constructed from the nodes, edges and
// default weights passed in, potentially altering the nodes and edges to
// conform to the requirements of the graph. The graph is returned along with
// the nodes, edges and default weights used to construct the graph.
// The returned edges may be any of graph.Edge, graph.WeightedEdge, graph.Line
// or graph.WeightedLine depending on what the graph requires.
// The client may skip a test case by returning ok=false when the input is not
// a valid graph construction.
type Builder func(nodes []graph.Node, edges []WeightedLine, self, absent float64) (g graph.Graph, n []graph.Node, e []Edge, s, a float64, ok bool)

// edgeLister is a graph that can return all its edges.
type edgeLister interface {
	// Edges returns all the edges of a graph.
	Edges() graph.Edges
}

// weightedEdgeLister is a graph that can return all its weighted edges.
type weightedEdgeLister interface {
	// WeightedEdges returns all the weighted edges of a graph.
	WeightedEdges() graph.WeightedEdges
}

// matrixer is a graph that can return an adjacency matrix.
type matrixer interface {
	// Matrix returns the graph's adjacency matrix.
	Matrix() mat.Matrix
}

// ReturnAllNodes tests the constructed graph for the ability to return all
// the nodes it claims it has used in its construction. This is a check of
// the Nodes method of graph.Graph and the iterator that is returned.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAllNodes(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, want, _, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		it := g.Nodes()
		if !isValidIterator(it) {
			t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
			continue
		}
		checkEmptyIterator(t, it, useEmpty)
		var got []graph.Node
		for it.Next() {
			got = append(got, it.Node())
		}

		sort.Sort(ordered.ByID(got))
		sort.Sort(ordered.ByID(want))

		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected nodes result for test %q:\ngot: %v\nwant:%v", test.name, got, want)
		}
	}
}

// ReturnNodeSlice tests the constructed graph for the ability to return all
// the nodes it claims it has used in its construction using the NodeSlicer
// interface. This is a check of the Nodes method of graph.Graph and the
// iterator that is returned.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnNodeSlice(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, want, _, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		it := g.Nodes()
		if !isValidIterator(it) {
			t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
			continue
		}
		checkEmptyIterator(t, it, useEmpty)
		if it == nil {
			continue
		}
		s, ok := it.(graph.NodeSlicer)
		if !ok {
			t.Errorf("invalid type for test %q: %T cannot return node slicer", test.name, g)
			continue
		}
		got := s.NodeSlice()

		sort.Sort(ordered.ByID(got))
		sort.Sort(ordered.ByID(want))

		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected nodes result for test %q:\ngot: %v\nwant:%v", test.name, got, want)
		}
	}
}

// NodeExistence tests the constructed graph for the ability to correctly
// return the existence of nodes within the graph. This is a check of the
// Node method of graph.Graph.
func NodeExistence(t *testing.T, b Builder) {
	for _, test := range testCases {
		g, want, _, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		seen := set.NewNodes()
		for _, exist := range want {
			seen.Add(exist)
			if g.Node(exist.ID()) == nil {
				t.Errorf("missing node for test %q: %v", test.name, exist)
			}
		}
		for _, ghost := range test.nonexist {
			if g.Node(ghost.ID()) != nil {
				if seen.Has(ghost) {
					// Do not fail nodes that the graph builder says can exist
					// even if the test case input thinks they should not.
					t.Logf("builder has modified non-exist node set: %v is now allowed and present", ghost)
					continue
				}
				t.Errorf("unexpected node for test %q: %v", test.name, ghost)
			}
		}
	}
}

// ReturnAllEdges tests the constructed graph for the ability to return all
// the edges it claims it has used in its construction. This is a check of
// the Edges method of graph.Graph and the iterator that is returned.
// ReturnAllEdges  also checks that the edge end nodes exist within the graph,
// checking the Node method of graph.Graph.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAllEdges(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case edgeLister:
			it := eg.Edges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			for it.Next() {
				e := it.Edge()
				got = append(got, e)
				qe := g.Edge(e.From().ID(), e.To().ID())
				if qe == nil {
					t.Errorf("missing edge for test %q: %v", test.name, e)
				} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
					t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
						test.name, e.From().ID(), e.To().ID(), qe)
				}
				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test %q: %T cannot return edge iterator", test.name, g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// ReturnEdgeSlice tests the constructed graph for the ability to return all
// the edges it claims it has used in its construction using the EdgeSlicer
// interface. This is a check of the Edges method of graph.Graph and the
// iterator that is returned. ReturnEdgeSlice also checks that the edge end
// nodes exist within the graph, checking the Node method of graph.Graph.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnEdgeSlice(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case edgeLister:
			it := eg.Edges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			if it == nil {
				continue
			}
			s, ok := it.(graph.EdgeSlicer)
			if !ok {
				t.Errorf("invalid type for test %q: %T cannot return edge slicer", test.name, g)
				continue
			}
			gotNative := s.EdgeSlice()
			if len(gotNative) != 0 {
				got = make([]Edge, len(gotNative))
			}
			for i, e := range gotNative {
				got[i] = e

				qe := g.Edge(e.From().ID(), e.To().ID())
				if qe == nil {
					t.Errorf("missing edge for test %q: %v", test.name, e)
				} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
					t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
						test.name, e.From().ID(), e.To().ID(), qe)
				}
				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test %T: cannot return edge iterator", g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// ReturnAllLines tests the constructed graph for the ability to return all
// the edges it claims it has used in its construction and then recover all
// the lines that contribute to those edges. This is a check of the Edges
// method of graph.Graph and the iterator that is returned and the graph.Lines
// implementation of those edges. ReturnAllLines also checks that the edge
// end nodes exist within the graph, checking the Node method of graph.Graph.
//
// The edges used within and returned by the Builder function should be
// graph.Line. The edge parameter passed to b will contain only graph.Line.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAllLines(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case edgeLister:
			it := eg.Edges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			for _, e := range graph.EdgesOf(it) {
				qe := g.Edge(e.From().ID(), e.To().ID())
				if qe == nil {
					t.Errorf("missing edge for test %q: %v", test.name, e)
				} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
					t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
						test.name, e.From().ID(), e.To().ID(), qe)
				}

				// FIXME(kortschak): This would not be necessary
				// if graph.WeightedLines (and by symmetry)
				// graph.WeightedEdges also were graph.Lines
				// and graph.Edges.
				switch lit := e.(type) {
				case graph.Lines:
					if !isValidIterator(lit) {
						t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
						continue
					}
					checkEmptyIterator(t, lit, useEmpty)
					for lit.Next() {
						got = append(got, lit.Line())
					}
				case graph.WeightedLines:
					if !isValidIterator(lit) {
						t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
						continue
					}
					checkEmptyIterator(t, lit, useEmpty)
					for lit.Next() {
						got = append(got, lit.WeightedLine())
					}
				default:
					continue
				}

				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test: %T cannot return edge iterator", g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// ReturnAllWeightedEdges tests the constructed graph for the ability to return
// all the edges it claims it has used in its construction. This is a check of
// the Edges method of graph.Graph and the iterator that is returned.
// ReturnAllWeightedEdges also checks that the edge end nodes exist within the
// graph, checking the Node method of graph.Graph.
//
// The edges used within and returned by the Builder function should be
// graph.WeightedEdge. The edge parameter passed to b will contain only
// graph.WeightedEdge.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAllWeightedEdges(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case weightedEdgeLister:
			it := eg.WeightedEdges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			for it.Next() {
				e := it.WeightedEdge()
				got = append(got, e)
				switch g := g.(type) {
				case graph.Weighted:
					qe := g.WeightedEdge(e.From().ID(), e.To().ID())
					if qe == nil {
						t.Errorf("missing edge for test %q: %v", test.name, e)
					} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, e.From().ID(), e.To().ID(), qe)
					}
				default:
					t.Logf("weighted edge lister is not a weighted graph - are you sure?: %T", g)
					qe := g.Edge(e.From().ID(), e.To().ID())
					if qe == nil {
						t.Errorf("missing edge for test %q: %v", test.name, e)
					} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, e.From().ID(), e.To().ID(), qe)
					}
				}
				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test: %T cannot return weighted edge iterator", g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// ReturnWeightedEdgeSlice tests the constructed graph for the ability to
// return all the edges it claims it has used in its construction using the
// WeightedEdgeSlicer interface. This is a check of the Edges method of
// graph.Graph and the iterator that is returned. ReturnWeightedEdgeSlice
// also checks that the edge end nodes exist within the graph, checking
// the Node method of graph.Graph.
//
// The edges used within and returned by the Builder function should be
// graph.WeightedEdge. The edge parameter passed to b will contain only
// graph.WeightedEdge.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnWeightedEdgeSlice(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case weightedEdgeLister:
			it := eg.WeightedEdges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			s, ok := it.(graph.WeightedEdgeSlicer)
			if !ok {
				t.Errorf("invalid type for test %T: cannot return weighted edge slice", g)
				continue
			}
			for _, e := range s.WeightedEdgeSlice() {
				got = append(got, e)
				qe := g.Edge(e.From().ID(), e.To().ID())
				if qe == nil {
					t.Errorf("missing edge for test %q: %v", test.name, e)
				} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
					t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
						test.name, e.From().ID(), e.To().ID(), qe)
				}
				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test: %T cannot return weighted edge iterator", g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// ReturnAllWeightedLines tests the constructed graph for the ability to return
// all the edges it claims it has used in its construction and then recover all
// the lines that contribute to those edges. This is a check of the Edges
// method of graph.Graph and the iterator that is returned and the graph.Lines
// implementation of those edges. ReturnAllWeightedLines also checks that the
// edge end nodes exist within the graph, checking the Node method of
// graph.Graph.
//
// The edges used within and returned by the Builder function should be
// graph.WeightedLine. The edge parameter passed to b will contain only
// graph.WeightedLine.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAllWeightedLines(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, _, want, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		var got []Edge
		switch eg := g.(type) {
		case weightedEdgeLister:
			it := eg.WeightedEdges()
			if !isValidIterator(it) {
				t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
				continue
			}
			checkEmptyIterator(t, it, useEmpty)
			for _, e := range graph.WeightedEdgesOf(it) {
				qe := g.Edge(e.From().ID(), e.To().ID())
				if qe == nil {
					t.Errorf("missing edge for test %q: %v", test.name, e)
				} else if qe.From().ID() != e.From().ID() || qe.To().ID() != e.To().ID() {
					t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
						test.name, e.From().ID(), e.To().ID(), qe)
				}

				// FIXME(kortschak): This would not be necessary
				// if graph.WeightedLines (and by symmetry)
				// graph.WeightedEdges also were graph.Lines
				// and graph.Edges.
				switch lit := e.(type) {
				case graph.Lines:
					if !isValidIterator(lit) {
						t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
						continue
					}
					checkEmptyIterator(t, lit, useEmpty)
					for lit.Next() {
						got = append(got, lit.Line())
					}
				case graph.WeightedLines:
					if !isValidIterator(lit) {
						t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
						continue
					}
					checkEmptyIterator(t, lit, useEmpty)
					for lit.Next() {
						got = append(got, lit.WeightedLine())
					}
				default:
					continue
				}

				if g.Node(e.From().ID()) == nil {
					t.Errorf("missing from node for test %q: %v", test.name, e.From().ID())
				}
				if g.Node(e.To().ID()) == nil {
					t.Errorf("missing to node for test %q: %v", test.name, e.To().ID())
				}
			}

		default:
			t.Errorf("invalid type for test: %T cannot return edge iterator", g)
			continue
		}

		checkEdges(t, test.name, g, got, want)
	}
}

// checkEdges compares got and want for the given graph type.
func checkEdges(t *testing.T, name string, g graph.Graph, got, want []Edge) {
	t.Helper()
	switch g.(type) {
	case graph.Undirected:
		sort.Sort(lexicalUndirectedEdges(got))
		sort.Sort(lexicalUndirectedEdges(want))
		if !undirectedEdgeSetEqual(got, want) {
			t.Errorf("unexpected edges result for test %q:\ngot: %#v\nwant:%#v", name, got, want)
		}
	default:
		sort.Sort(lexicalEdges(got))
		sort.Sort(lexicalEdges(want))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("unexpected edges result for test %q:\ngot: %#v\nwant:%#v", name, got, want)
		}
	}
}

// EdgeExistence tests the constructed graph for the ability to correctly
// return the existence of edges within the graph. This is a check of the
// Edge methods of graph.Graph, the EdgeBetween method of graph.Undirected
// and the EdgeFromTo method of graph.Directed. EdgeExistence also checks
// that the nodes and traversed edges exist within the graph, checking the
// Node, Edge, EdgeBetween and HasEdgeBetween methods of graph.Graph, the
// EdgeBetween method of graph.Undirected and the HasEdgeFromTo method of
// graph.Directed.
func EdgeExistence(t *testing.T, b Builder) {
	for _, test := range testCases {
		g, nodes, edges, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		want := make(map[edge]bool)
		for _, e := range edges {
			want[edge{f: e.From().ID(), t: e.To().ID()}] = true
		}
		for _, x := range nodes {
			for _, y := range nodes {
				between := want[edge{f: x.ID(), t: y.ID()}] || want[edge{f: y.ID(), t: x.ID()}]

				if has := g.HasEdgeBetween(x.ID(), y.ID()); has != between {
					if has {
						t.Errorf("unexpected edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					} else {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
				} else {
					if want[edge{f: x.ID(), t: y.ID()}] {
						e := g.Edge(x.ID(), y.ID())
						if e == nil {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else if e.From().ID() != x.ID() || e.To().ID() != y.ID() {
							t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
								test.name, x.ID(), y.ID(), e)
						}
					}
					if between && !g.HasEdgeBetween(x.ID(), y.ID()) {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
					if g.Node(x.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, x.ID())
					}
					if g.Node(y.ID()) == nil {
						t.Errorf("missing to node for test %q: %v", test.name, y.ID())
					}
				}

				switch g := g.(type) {
				case graph.Directed:
					u := x
					v := y
					if has := g.HasEdgeFromTo(u.ID(), v.ID()); has != want[edge{f: u.ID(), t: v.ID()}] {
						if has {
							t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						} else {
							t.Errorf("missing edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						}
						continue
					}
					// Edge has already been tested above.
					if g.Node(u.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, u.ID())
					}
					if g.Node(v.ID()) == nil {
						t.Errorf("missing to node for test %q: %v", test.name, v.ID())
					}

				case graph.Undirected:
					// HasEdgeBetween is already tested above.
					if between && g.Edge(x.ID(), y.ID()) == nil {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
					if between && g.EdgeBetween(x.ID(), y.ID()) == nil {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
				}

				switch g := g.(type) {
				case graph.WeightedDirected:
					u := x
					v := y
					if has := g.WeightedEdge(u.ID(), v.ID()) != nil; has != want[edge{f: u.ID(), t: v.ID()}] {
						if has {
							t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						} else {
							t.Errorf("missing edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						}
						continue
					}

				case graph.WeightedUndirected:
					// HasEdgeBetween is already tested above.
					if between && g.WeightedEdge(x.ID(), y.ID()) == nil {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
					if between && g.WeightedEdgeBetween(x.ID(), y.ID()) == nil {
						t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
				}
			}
		}
	}
}

// LineExistence tests the constructed graph for the ability to correctly
// return the existence of lines within the graph. This is a check of the
// Line methods of graph.MultiGraph, the EdgeBetween method of graph.Undirected
// and the EdgeFromTo method of graph.Directed. LineExistence also checks
// that the nodes and traversed edges exist within the graph, checking the
// Node, Edge, EdgeBetween and HasEdgeBetween methods of graph.Graph, the
// EdgeBetween method of graph.Undirected and the HasEdgeFromTo method of
// graph.Directed.
func LineExistence(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, nodes, edges, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		switch mg := g.(type) {
		case graph.Multigraph:
			want := make(map[edge]bool)
			for _, e := range edges {
				want[edge{f: e.From().ID(), t: e.To().ID()}] = true
			}
			for _, x := range nodes {
				for _, y := range nodes {
					between := want[edge{f: x.ID(), t: y.ID()}] || want[edge{f: y.ID(), t: x.ID()}]

					if has := g.HasEdgeBetween(x.ID(), y.ID()); has != between {
						if has {
							t.Errorf("unexpected edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						}
					} else {
						if want[edge{f: x.ID(), t: y.ID()}] {
							lit := mg.Lines(x.ID(), y.ID())
							if !isValidIterator(lit) {
								t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
								continue
							}
							checkEmptyIterator(t, lit, useEmpty)
							if lit.Len() == 0 {
								t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
							} else {
								for lit.Next() {
									l := lit.Line()
									if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
										t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
											test.name, x.ID(), y.ID(), l)
									}
								}
							}
						}
						if between && !g.HasEdgeBetween(x.ID(), y.ID()) {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						}
						if g.Node(x.ID()) == nil {
							t.Errorf("missing from node for test %q: %v", test.name, x.ID())
						}
						if g.Node(y.ID()) == nil {
							t.Errorf("missing to node for test %q: %v", test.name, y.ID())
						}
					}

					switch g := g.(type) {
					case graph.DirectedMultigraph:
						u := x
						v := y
						if has := g.HasEdgeFromTo(u.ID(), v.ID()); has != want[edge{f: u.ID(), t: v.ID()}] {
							if has {
								t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
							} else {
								t.Errorf("missing edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
							}
							continue
						}
						// Edge has already been tested above.
						if g.Node(u.ID()) == nil {
							t.Errorf("missing from node for test %q: %v", test.name, u.ID())
						}
						if g.Node(v.ID()) == nil {
							t.Errorf("missing to node for test %q: %v", test.name, v.ID())
						}

					case graph.UndirectedMultigraph:
						// HasEdgeBetween is already tested above.
						lit := g.Lines(x.ID(), y.ID())
						if !isValidIterator(lit) {
							t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
							continue
						}
						checkEmptyIterator(t, lit, useEmpty)
						if between && lit.Len() == 0 {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else {
							for lit.Next() {
								l := lit.Line()
								if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
									t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
										test.name, x.ID(), y.ID(), l)
								}
							}
						}
						lit = g.LinesBetween(x.ID(), y.ID())
						if !isValidIterator(lit) {
							t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
							continue
						}
						checkEmptyIterator(t, lit, useEmpty)
						if between && lit.Len() == 0 {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else {
							for lit.Next() {
								l := lit.Line()
								if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
									t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
										test.name, x.ID(), y.ID(), l)
								}
							}
						}
					}

					switch g := g.(type) {
					case graph.WeightedDirectedMultigraph:
						u := x
						v := y
						lit := g.WeightedLines(u.ID(), v.ID())
						if !isValidIterator(lit) {
							t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
							continue
						}
						checkEmptyIterator(t, lit, useEmpty)
						if has := lit != graph.Empty; has != want[edge{f: u.ID(), t: v.ID()}] {
							if has {
								t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
							} else {
								t.Errorf("missing edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
							}
							continue
						}
						for lit.Next() {
							l := lit.WeightedLine()
							if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
								t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
									test.name, x.ID(), y.ID(), l)
							}
						}
						// Edge has already been tested above.
						if g.Node(u.ID()) == nil {
							t.Errorf("missing from node for test %q: %v", test.name, u.ID())
						}
						if g.Node(v.ID()) == nil {
							t.Errorf("missing to node for test %q: %v", test.name, v.ID())
						}

					case graph.WeightedUndirectedMultigraph:
						// HasEdgeBetween is already tested above.
						lit := g.WeightedLines(x.ID(), y.ID())
						if !isValidIterator(lit) {
							t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
							continue
						}
						checkEmptyIterator(t, lit, useEmpty)
						if between && lit.Len() == 0 {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else {
							for lit.Next() {
								l := lit.WeightedLine()
								if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
									t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
										test.name, x.ID(), y.ID(), l)
								}
							}
						}
						lit = g.WeightedLinesBetween(x.ID(), y.ID())
						if !isValidIterator(lit) {
							t.Errorf("invalid iterator for test %q: got:%#v", test.name, lit)
							continue
						}
						checkEmptyIterator(t, lit, useEmpty)
						if between && lit.Len() == 0 {
							t.Errorf("missing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
						} else {
							for lit.Next() {
								l := lit.WeightedLine()
								if l.From().ID() != x.ID() || l.To().ID() != y.ID() {
									t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
										test.name, x.ID(), y.ID(), l)
								}
							}
						}
					}
				}
			}
		default:
			t.Errorf("invalid type for test: %T not a multigraph", g)
			continue
		}
	}
}

// ReturnAdjacentNodes tests the constructed graph for the ability to correctly
// return the nodes reachable from each node within the graph. This is a check
// of the From method of graph.Graph and the To method of graph.Directed.
// ReturnAdjacentNodes also checks that the nodes and traversed edges exist
// within the graph, checking the Node, Edge, EdgeBetween and HasEdgeBetween
// methods of graph.Graph, the EdgeBetween method of graph.Undirected and the
// HasEdgeFromTo method of graph.Directed.
// If useEmpty is true, graph iterators will be checked for the use of
// graph.Empty if they are empty.
func ReturnAdjacentNodes(t *testing.T, b Builder, useEmpty bool) {
	for _, test := range testCases {
		g, nodes, edges, _, _, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}

		want := make(map[edge]bool)
		for _, e := range edges {
			want[edge{f: e.From().ID(), t: e.To().ID()}] = true
		}
		for _, x := range nodes {
			switch g := g.(type) {
			case graph.Directed:
				// Test forward.
				u := x
				it := g.From(u.ID())
				if !isValidIterator(it) {
					t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
					continue
				}
				checkEmptyIterator(t, it, useEmpty)
				for i := 0; it.Next(); i++ {
					v := it.Node()
					if i == 0 && g.Node(u.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, u.ID())
					}
					if g.Node(v.ID()) == nil {
						t.Errorf("missing to node for test %q: %v", test.name, v.ID())
					}
					qe := g.Edge(u.ID(), v.ID())
					if qe == nil {
						t.Errorf("missing from edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					} else if qe.From().ID() != u.ID() || qe.To().ID() != v.ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, u.ID(), v.ID(), qe)
					}
					if !g.HasEdgeBetween(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
					}
					if !g.HasEdgeFromTo(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					}
					if !want[edge{f: u.ID(), t: v.ID()}] {
						t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					}
				}

				// Test backward.
				v := x
				it = g.To(v.ID())
				if !isValidIterator(it) {
					t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
					continue
				}
				checkEmptyIterator(t, it, useEmpty)
				for i := 0; it.Next(); i++ {
					u := it.Node()
					if i == 0 && g.Node(v.ID()) == nil {
						t.Errorf("missing to node for test %q: %v", test.name, v.ID())
					}
					if g.Node(u.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, u.ID())
					}
					qe := g.Edge(u.ID(), v.ID())
					if qe == nil {
						t.Errorf("missing from edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if qe.From().ID() != u.ID() || qe.To().ID() != v.ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, u.ID(), v.ID(), qe)
					}
					if !g.HasEdgeBetween(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if !g.HasEdgeFromTo(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if !want[edge{f: u.ID(), t: v.ID()}] {
						t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					}
				}

			case graph.Undirected:
				u := x
				it := g.From(u.ID())
				if !isValidIterator(it) {
					t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
					continue
				}
				checkEmptyIterator(t, it, useEmpty)
				for i := 0; it.Next(); i++ {
					v := it.Node()
					if i == 0 && g.Node(u.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, u.ID())
					}
					qe := g.Edge(u.ID(), v.ID())
					if qe == nil {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if qe.From().ID() != u.ID() || qe.To().ID() != v.ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, u.ID(), v.ID(), qe)
					}
					qe = g.EdgeBetween(u.ID(), v.ID())
					if qe == nil {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if qe.From().ID() != u.ID() || qe.To().ID() != v.ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, u.ID(), v.ID(), qe)
					}
					if !g.HasEdgeBetween(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					between := want[edge{f: u.ID(), t: v.ID()}] || want[edge{f: v.ID(), t: u.ID()}]
					if !between {
						t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					}
				}

			default:
				u := x
				it := g.From(u.ID())
				if !isValidIterator(it) {
					t.Errorf("invalid iterator for test %q: got:%#v", test.name, it)
					continue
				}
				checkEmptyIterator(t, it, useEmpty)
				for i := 0; it.Next(); i++ {
					v := it.Node()
					if i == 0 && g.Node(u.ID()) == nil {
						t.Errorf("missing from node for test %q: %v", test.name, u.ID())
					}
					qe := g.Edge(u.ID(), v.ID())
					if qe == nil {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					if qe.From().ID() != u.ID() || qe.To().ID() != v.ID() {
						t.Errorf("inverted edge for test %q query with F=%d T=%d: got:%#v",
							test.name, u.ID(), v.ID(), qe)
					}
					if !g.HasEdgeBetween(u.ID(), v.ID()) {
						t.Errorf("missing from edge for test %q: (%v)--(%v)", test.name, u.ID(), v.ID())
						continue
					}
					between := want[edge{f: u.ID(), t: v.ID()}] || want[edge{f: v.ID(), t: u.ID()}]
					if !between {
						t.Errorf("unexpected edge for test %q: (%v)->(%v)", test.name, u.ID(), v.ID())
					}
				}
			}
		}
	}
}

// Weight tests the constructed graph for the ability to correctly return
// the weight between to nodes, checking the Weight method of graph.Weighted.
//
// The self and absent values returned by the Builder should match the values
// used by the Weight method.
func Weight(t *testing.T, b Builder) {
	for _, test := range testCases {
		g, nodes, _, self, absent, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}
		wg, ok := g.(graph.Weighted)
		if !ok {
			t.Errorf("invalid graph type for test %q: %T is not graph.Weighted", test.name, g)
		}
		_, multi := g.(graph.Multigraph)

		for _, x := range nodes {
			for _, y := range nodes {
				w, ok := wg.Weight(x.ID(), y.ID())
				e := wg.WeightedEdge(x.ID(), y.ID())
				switch {
				case !ok:
					if e != nil {
						t.Errorf("missing edge weight for existing edge for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())
					}
					if !same(w, absent) {
						t.Errorf("unexpected absent weight for test %q: got:%v want:%v", test.name, w, absent)
					}

				case !multi && x.ID() == y.ID():
					if !same(w, self) {
						t.Errorf("unexpected self weight for test %q: got:%v want:%v", test.name, w, self)
					}

				case e == nil:
					t.Errorf("missing edge for existing non-self weight for test %q: (%v)--(%v)", test.name, x.ID(), y.ID())

				case e.Weight() != w:
					t.Errorf("weight mismatch for test %q: edge=%v graph=%v", test.name, e.Weight(), w)
				}
			}
		}
	}
}

// AdjacencyMatrix tests the constructed graph for the ability to correctly
// return an adjacency matrix that matches the weights returned by the graphs
// Weight method.
//
// The self and absent values returned by the Builder should match the values
// used by the Weight method.
func AdjacencyMatrix(t *testing.T, b Builder) {
	for _, test := range testCases {
		g, nodes, _, self, absent, ok := b(test.nodes, test.edges, test.self, test.absent)
		if !ok {
			t.Logf("skipping test case: %q", test.name)
			continue
		}
		wg, ok := g.(graph.Weighted)
		if !ok {
			t.Errorf("invalid graph type for test %q: %T is not graph.Weighted", test.name, g)
		}
		mg, ok := g.(matrixer)
		if !ok {
			t.Errorf("invalid graph type for test %q: %T cannot return adjacency matrix", test.name, g)
		}
		m := mg.Matrix()

		r, c := m.Dims()
		if r != c || r != len(nodes) {
			t.Errorf("dimension mismatch for test %q: r=%d c=%d order=%d", test.name, r, c, len(nodes))
		}

		for _, x := range nodes {
			i := int(x.ID())
			for _, y := range nodes {
				j := int(y.ID())
				w, ok := wg.Weight(x.ID(), y.ID())
				switch {
				case !ok:
					if !same(m.At(i, j), absent) {
						t.Errorf("weight mismatch for test %q: (%v)--(%v) matrix=%v graph=%v", test.name, x.ID(), y.ID(), m.At(i, j), w)
					}
				case x.ID() == y.ID():
					if !same(m.At(i, j), self) {
						t.Errorf("weight mismatch for test %q: (%v)--(%v) matrix=%v graph=%v", test.name, x.ID(), y.ID(), m.At(i, j), w)
					}
				default:
					if !same(m.At(i, j), w) {
						t.Errorf("weight mismatch for test %q: (%v)--(%v) matrix=%v graph=%v", test.name, x.ID(), y.ID(), m.At(i, j), w)
					}
				}
			}
		}
	}
}

// lexicalEdges sorts a collection of edges lexically on the
// keys: from.ID > to.ID > [line.ID] > [weight].
type lexicalEdges []Edge

func (e lexicalEdges) Len() int { return len(e) }
func (e lexicalEdges) Less(i, j int) bool {
	if e[i].From().ID() < e[j].From().ID() {
		return true
	}
	sf := e[i].From().ID() == e[j].From().ID()
	if sf && e[i].To().ID() < e[j].To().ID() {
		return true
	}
	st := e[i].To().ID() == e[j].To().ID()
	li, oki := e[i].(graph.Line)
	lj, okj := e[j].(graph.Line)
	if oki != okj {
		panic(fmt.Sprintf("testgraph: mismatched types %T != %T", e[i], e[j]))
	}
	if !oki {
		return sf && st && lessWeight(e[i], e[j])
	}
	if sf && st && li.ID() < lj.ID() {
		return true
	}
	return sf && st && li.ID() == lj.ID() && lessWeight(e[i], e[j])
}
func (e lexicalEdges) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

// lexicalUndirectedEdges sorts a collection of edges lexically on the
// keys: lo.ID > hi.ID > [line.ID] > [weight].
type lexicalUndirectedEdges []Edge

func (e lexicalUndirectedEdges) Len() int { return len(e) }
func (e lexicalUndirectedEdges) Less(i, j int) bool {
	lidi, hidi, _ := undirectedIDs(e[i])
	lidj, hidj, _ := undirectedIDs(e[j])

	if lidi < lidj {
		return true
	}
	sl := lidi == lidj
	if sl && hidi < hidj {
		return true
	}
	sh := hidi == hidj
	li, oki := e[i].(graph.Line)
	lj, okj := e[j].(graph.Line)
	if oki != okj {
		panic(fmt.Sprintf("testgraph: mismatched types %T != %T", e[i], e[j]))
	}
	if !oki {
		return sl && sh && lessWeight(e[i], e[j])
	}
	if sl && sh && li.ID() < lj.ID() {
		return true
	}
	return sl && sh && li.ID() == lj.ID() && lessWeight(e[i], e[j])
}
func (e lexicalUndirectedEdges) Swap(i, j int) { e[i], e[j] = e[j], e[i] }

func lessWeight(ei, ej Edge) bool {
	wei, oki := ei.(graph.WeightedEdge)
	wej, okj := ej.(graph.WeightedEdge)
	if oki != okj {
		panic(fmt.Sprintf("testgraph: mismatched types %T != %T", ei, ej))
	}
	if !oki {
		return false
	}
	return wei.Weight() < wej.Weight()
}

// undirectedEdgeSetEqual returned whether a pair of undirected edge
// slices sorted by lexicalUndirectedEdges are equal.
func undirectedEdgeSetEqual(a, b []Edge) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	if !undirectedEdgeEqual(a[0], b[0]) {
		return false
	}
	i, j := 0, 0
	for {
		switch {
		case i == len(a)-1 && j == len(b)-1:
			return true

		case i < len(a)-1 && undirectedEdgeEqual(a[i+1], b[j]):
			i++

		case j < len(b)-1 && undirectedEdgeEqual(a[i], b[j+1]):
			j++

		case i < len(a)-1 && j < len(b)-1 && undirectedEdgeEqual(a[i+1], b[j+1]):
			i++
			j++

		default:
			return false
		}
	}
}

// undirectedEdgeEqual returns whether a pair of undirected edges are equal
// after canonicalising from and to IDs by numerical sort order.
func undirectedEdgeEqual(a, b Edge) bool {
	loa, hia, inva := undirectedIDs(a)
	lob, hib, invb := undirectedIDs(b)
	// Use reflect.DeepEqual if the edges are parallel
	// rather anti-parallel.
	if inva == invb {
		return reflect.DeepEqual(a, b)
	}
	if loa != lob || hia != hib {
		return false
	}
	la, oka := a.(graph.Line)
	lb, okb := b.(graph.Line)
	if !oka && !okb {
		return true
	}
	if la.ID() != lb.ID() {
		return false
	}
	wea, oka := a.(graph.WeightedEdge)
	web, okb := b.(graph.WeightedEdge)
	if !oka && !okb {
		return true
	}
	return wea.Weight() == web.Weight()
}

// NodeAdder is a graph.NodeAdder graph.
type NodeAdder interface {
	graph.Graph
	graph.NodeAdder
}

// AddNodes tests whether g correctly implements the graph.NodeAdder interface.
// AddNodes gets a new node from g and adds it to the graph, repeating this pair
// of operations n times. The existence of added nodes is confirmed in the graph.
// AddNodes also checks that re-adding each of the added nodes causes a panic.
func AddNodes(t *testing.T, g NodeAdder, n int) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	var addedNodes []graph.Node
	for i := 0; i < n; i++ {
		node := g.NewNode()
		prev := g.Nodes().Len()
		if g.Node(node.ID()) != nil {
			curr := g.Nodes().Len()
			if curr != prev {
				t.Fatalf("NewNode mutated graph: prev graph order != curr graph order, %d != %d", prev, curr)
			}
			t.Fatalf("NewNode returned existing: %#v", node)
		}
		g.AddNode(node)
		addedNodes = append(addedNodes, node)
		curr := g.Nodes().Len()
		if curr != prev+1 {
			t.Fatalf("AddNode failed to mutate graph: curr graph order != prev graph order+1, %d != %d", curr, prev+1)
		}
		if g.Node(node.ID()) == nil {
			t.Fatalf("AddNode failed to add node to graph trying to add %#v", node)
		}
	}

	sort.Sort(ordered.ByID(addedNodes))
	graphNodes := graph.NodesOf(g.Nodes())
	sort.Sort(ordered.ByID(graphNodes))
	if !reflect.DeepEqual(addedNodes, graphNodes) {
		if n > 20 {
			t.Errorf("unexpected node set after node addition: got len:%v want len:%v", len(graphNodes), len(addedNodes))
		} else {
			t.Errorf("unexpected node set after node addition: got:\n %v\nwant:\n%v", graphNodes, addedNodes)
		}
	}

	it := g.Nodes()
	for it.Next() {
		panicked := panics(func() {
			g.AddNode(it.Node())
		})
		if !panicked {
			t.Fatalf("expected panic adding existing node: %v", it.Node())
		}
	}
}

// AddArbitraryNodes tests whether g correctly implements the AddNode method. Not all
// graph.NodeAdder graphs are expected to implement the semantics of this test.
// AddArbitraryNodes iterates over add, adding each node to the graph. The existence
// of each added node in the graph is confirmed.
func AddArbitraryNodes(t *testing.T, g NodeAdder, add graph.Nodes) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	for add.Next() {
		node := add.Node()
		prev := g.Nodes().Len()
		g.AddNode(node)
		curr := g.Nodes().Len()
		if curr != prev+1 {
			t.Fatalf("AddNode failed to mutate graph: curr graph order != prev graph order+1, %d != %d", curr, prev+1)
		}
		if g.Node(node.ID()) == nil {
			t.Fatalf("AddNode failed to add node to graph trying to add %#v", node)
		}
	}

	add.Reset()
	addedNodes := graph.NodesOf(add)
	sort.Sort(ordered.ByID(addedNodes))
	graphNodes := graph.NodesOf(g.Nodes())
	sort.Sort(ordered.ByID(graphNodes))
	if !reflect.DeepEqual(addedNodes, graphNodes) {
		t.Errorf("unexpected node set after node addition: got:\n %v\nwant:\n%v", graphNodes, addedNodes)
	}

	it := g.Nodes()
	for it.Next() {
		panicked := panics(func() {
			g.AddNode(it.Node())
		})
		if !panicked {
			t.Fatalf("expected panic adding existing node: %v", it.Node())
		}
	}
}

// NodeRemover is a graph.NodeRemover graph.
type NodeRemover interface {
	graph.Graph
	graph.NodeRemover
}

// RemoveNodes tests whether g correctly implements the graph.NodeRemover interface.
// The input graph g must contain a set of nodes with some edges between them.
func RemoveNodes(t *testing.T, g NodeRemover) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	it := g.Nodes()
	first := true
	for it.Next() {
		u := it.Node()

		seen := make(map[edge]struct{})

		// Collect all incident edges.
		var incident []graph.Edge
		to := g.From(u.ID())
		for to.Next() {
			v := to.Node()
			e := g.Edge(u.ID(), v.ID())
			if e == nil {
				t.Fatalf("bad graph: neighbors not connected: u=%#v v=%#v", u, v)
			}
			if _, ok := g.(graph.Undirected); ok {
				seen[edge{f: e.To().ID(), t: e.From().ID()}] = struct{}{}
			}
			seen[edge{f: e.From().ID(), t: e.To().ID()}] = struct{}{}
			incident = append(incident, e)
		}

		// Collect all other edges.
		var others []graph.Edge
		currit := g.Nodes()
		for currit.Next() {
			u := currit.Node()
			to := g.From(u.ID())
			for to.Next() {
				v := to.Node()
				e := g.Edge(u.ID(), v.ID())
				if e == nil {
					t.Fatalf("bad graph: neighbors not connected: u=%#v v=%#v", u, v)
				}
				seen[edge{f: e.From().ID(), t: e.To().ID()}] = struct{}{}
				others = append(others, e)
			}
		}

		if first && len(seen) == 0 {
			t.Fatal("incomplete test: no edges in graph")
		}
		first = false

		prev := g.Nodes().Len()
		g.RemoveNode(u.ID())
		curr := g.Nodes().Len()
		if curr != prev-1 {
			t.Fatalf("RemoveNode failed to mutate graph: curr graph order != prev graph order-1, %d != %d", curr, prev-1)
		}
		if g.Node(u.ID()) != nil {
			t.Fatalf("failed to remove node: %#v", u)
		}

		for _, e := range incident {
			if g.HasEdgeBetween(e.From().ID(), e.To().ID()) {
				t.Fatalf("RemoveNode failed to remove connected edge: %#v", e)
			}
		}

		for _, e := range others {
			if e.From().ID() == u.ID() || e.To().ID() == u.ID() {
				continue
			}
			if g.Edge(e.From().ID(), e.To().ID()) == nil {
				t.Fatalf("RemoveNode %v removed unconnected edge: %#v", u, e)
			}
		}
	}
}

// EdgeAdder is a graph.EdgeAdder graph.
type EdgeAdder interface {
	graph.Graph
	graph.EdgeAdder
}

// AddEdges tests whether g correctly implements the graph.EdgeAdder interface.
// AddEdges creates n pairs of nodes with random IDs in [0,n) and joins edges
// each node in the pair using SetEdge. AddEdges confirms that the end point
// nodes are added to the graph and that the edges are stored in the graph.
// If canLoop is true, self edges may be created. If canSet is true, a second
// call to SetEdge is made for each edge to confirm that the nodes corresponding
// the end points are updated.
func AddEdges(t *testing.T, n int, g EdgeAdder, newNode func(id int64) graph.Node, canLoop, canSetNode bool) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	type altNode struct {
		graph.Node
	}

	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		u := newNode(rnd.Int63n(int64(n)))
		var v graph.Node
		for {
			v = newNode(rnd.Int63n(int64(n)))
			if canLoop || u.ID() != v.ID() {
				break
			}
		}
		e := g.NewEdge(u, v)
		if g.Edge(u.ID(), v.ID()) != nil {
			t.Fatalf("NewEdge returned existing: %#v", e)
		}
		g.SetEdge(e)
		if g.Edge(u.ID(), v.ID()) == nil {
			t.Fatalf("SetEdge failed to add edge: %#v", e)
		}
		if g.Node(u.ID()) == nil {
			t.Fatalf("SetEdge failed to add from node: %#v", u)
		}
		if g.Node(v.ID()) == nil {
			t.Fatalf("SetEdge failed to add to node: %#v", v)
		}

		if !canSetNode {
			continue
		}

		g.SetEdge(g.NewEdge(altNode{u}, altNode{v}))
		if nu := g.Node(u.ID()); nu == u {
			t.Fatalf("SetEdge failed to update from node: u=%#v nu=%#v", u, nu)
		}
		if nv := g.Node(v.ID()); nv == v {
			t.Fatalf("SetEdge failed to update to node: v=%#v nv=%#v", v, nv)
		}
	}
}

// WeightedEdgeAdder is a graph.EdgeAdder graph.
type WeightedEdgeAdder interface {
	graph.Graph
	graph.WeightedEdgeAdder
}

// AddWeightedEdges tests whether g correctly implements the graph.WeightedEdgeAdder
// interface. AddWeightedEdges creates n pairs of nodes with random IDs in [0,n) and
// joins edges each node in the pair using SetWeightedEdge with weight w.
// AddWeightedEdges confirms that the end point nodes are added to the graph and that
// the edges are stored in the graph. If canLoop is true, self edges may be created.
// If canSet is true, a second call to SetWeightedEdge is made for each edge to
// confirm that the nodes corresponding the end points are updated.
func AddWeightedEdges(t *testing.T, n int, g WeightedEdgeAdder, w float64, newNode func(id int64) graph.Node, canLoop, canSetNode bool) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	type altNode struct {
		graph.Node
	}

	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < n; i++ {
		u := newNode(rnd.Int63n(int64(n)))
		var v graph.Node
		for {
			v = newNode(rnd.Int63n(int64(n)))
			if canLoop || u.ID() != v.ID() {
				break
			}
		}
		e := g.NewWeightedEdge(u, v, w)
		if g.Edge(u.ID(), v.ID()) != nil {
			t.Fatalf("NewEdge returned existing: %#v", e)
		}
		g.SetWeightedEdge(e)
		ne := g.Edge(u.ID(), v.ID())
		if ne == nil {
			t.Fatalf("SetWeightedEdge failed to add edge: %#v", e)
		}
		we, ok := ne.(graph.WeightedEdge)
		if !ok {
			t.Fatalf("SetWeightedEdge failed to add weighted edge: %#v", e)
		}
		if we.Weight() != w {
			t.Fatalf("edge weight mismatch: got:%f want:%f", we.Weight(), w)
		}

		if g.Node(u.ID()) == nil {
			t.Fatalf("SetWeightedEdge failed to add from node: %#v", u)
		}
		if g.Node(v.ID()) == nil {
			t.Fatalf("SetWeightedEdge failed to add to node: %#v", v)
		}

		if !canSetNode {
			continue
		}

		g.SetWeightedEdge(g.NewWeightedEdge(altNode{u}, altNode{v}, w))
		if nu := g.Node(u.ID()); nu == u {
			t.Fatalf("SetWeightedEdge failed to update from node: u=%#v nu=%#v", u, nu)
		}
		if nv := g.Node(v.ID()); nv == v {
			t.Fatalf("SetWeightedEdge failed to update to node: v=%#v nv=%#v", v, nv)
		}
	}
}

// NoLoopAddEdges tests whether g panics for self-loop addition. NoLoopAddEdges
// adds n nodes with IDs in [0,n) and creates an edge from the graph with NewEdge.
// NoLoopAddEdges confirms that this does not panic and then adds the edge to the
// graph to ensure that SetEdge will panic when adding a self-loop.
func NoLoopAddEdges(t *testing.T, n int, g EdgeAdder, newNode func(id int64) graph.Node) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	for id := 0; id < n; id++ {
		node := newNode(int64(id))
		e := g.NewEdge(node, node)
		panicked := panics(func() {
			g.SetEdge(e)
		})
		if !panicked {
			t.Errorf("expected panic for self-edge: %#v", e)
		}
	}
}

// NoLoopAddWeightedEdges tests whether g panics for self-loop addition. NoLoopAddWeightedEdges
// adds n nodes with IDs in [0,n) and creates an edge from the graph with NewWeightedEdge.
// NoLoopAddWeightedEdges confirms that this does not panic and then adds the edge to the
// graph to ensure that SetWeightedEdge will panic when adding a self-loop.
func NoLoopAddWeightedEdges(t *testing.T, n int, g WeightedEdgeAdder, w float64, newNode func(id int64) graph.Node) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	for id := 0; id < n; id++ {
		node := newNode(int64(id))
		e := g.NewWeightedEdge(node, node, w)
		panicked := panics(func() {
			g.SetWeightedEdge(e)
		})
		if !panicked {
			t.Errorf("expected panic for self-edge: %#v", e)
		}
	}
}

// LineAdder is a graph.LineAdder multigraph.
type LineAdder interface {
	graph.Multigraph
	graph.LineAdder
}

// AddLines tests whether g correctly implements the graph.LineAdder interface.
// AddLines creates n pairs of nodes with random IDs in [0,n) and joins edges
// each node in the pair using SetLine. AddLines confirms that the end point
// nodes are added to the graph and that the edges are stored in the graph.
// If canSet is true, a second call to SetLine is made for each edge to confirm
// that the nodes corresponding the end points are updated.
func AddLines(t *testing.T, n int, g LineAdder, newNode func(id int64) graph.Node, canSetNode bool) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	type altNode struct {
		graph.Node
	}

	rnd := rand.New(rand.NewSource(1))
	seen := make(set.Int64s)
	for i := 0; i < n; i++ {
		u := newNode(rnd.Int63n(int64(n)))
		v := newNode(rnd.Int63n(int64(n)))
		prev := g.Lines(u.ID(), v.ID())
		l := g.NewLine(u, v)
		if seen.Has(l.ID()) {
			t.Fatalf("NewLine returned an existing line: %#v", l)
		}
		if g.Lines(u.ID(), v.ID()).Len() != prev.Len() {
			t.Fatalf("NewLine added a line: %#v", l)
		}
		g.SetLine(l)
		seen.Add(l.ID())
		if g.Lines(u.ID(), v.ID()).Len() != prev.Len()+1 {
			t.Fatalf("SetLine failed to add line: %#v", l)
		}
		if g.Node(u.ID()) == nil {
			t.Fatalf("SetLine failed to add from node: %#v", u)
		}
		if g.Node(v.ID()) == nil {
			t.Fatalf("SetLine failed to add to node: %#v", v)
		}

		if !canSetNode {
			continue
		}

		g.SetLine(g.NewLine(altNode{u}, altNode{v}))
		if nu := g.Node(u.ID()); nu == u {
			t.Fatalf("SetLine failed to update from node: u=%#v nu=%#v", u, nu)
		}
		if nv := g.Node(v.ID()); nv == v {
			t.Fatalf("SetLine failed to update to node: v=%#v nv=%#v", v, nv)
		}
	}
}

// WeightedLineAdder is a graph.WeightedLineAdder multigraph.
type WeightedLineAdder interface {
	graph.Multigraph
	graph.WeightedLineAdder
}

// AddWeightedLines tests whether g correctly implements the graph.WeightedEdgeAdder
// interface. AddWeightedLines creates n pairs of nodes with random IDs in [0,n) and
// joins edges each node in the pair using SetWeightedLine with weight w.
// AddWeightedLines confirms that the end point nodes are added to the graph and that
// the edges are stored in the graph. If canSet is true, a second call to SetWeightedLine
// is made for each edge to confirm that the nodes corresponding the end points are
// updated.
func AddWeightedLines(t *testing.T, n int, g WeightedLineAdder, w float64, newNode func(id int64) graph.Node, canSetNode bool) {
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	type altNode struct {
		graph.Node
	}

	rnd := rand.New(rand.NewSource(1))
	seen := make(set.Int64s)
	for i := 0; i < n; i++ {
		u := newNode(rnd.Int63n(int64(n)))
		v := newNode(rnd.Int63n(int64(n)))
		prev := g.Lines(u.ID(), v.ID())
		l := g.NewWeightedLine(u, v, w)
		if seen.Has(l.ID()) {
			t.Fatalf("NewWeightedLine returned an existing line: %#v", l)
		}
		if g.Lines(u.ID(), v.ID()).Len() != prev.Len() {
			t.Fatalf("NewWeightedLine added a line: %#v", l)
		}
		g.SetWeightedLine(l)
		seen.Add(l.ID())
		curr := g.Lines(u.ID(), v.ID())
		if curr.Len() != prev.Len()+1 {
			t.Fatalf("SetWeightedLine failed to add line: %#v", l)
		}
		var found bool
		for curr.Next() {
			if curr.Line().ID() == l.ID() {
				found = true
				wl, ok := curr.Line().(graph.WeightedLine)
				if !ok {
					t.Fatalf("SetWeightedLine failed to add weighted line: %#v", l)
				}
				if wl.Weight() != w {
					t.Fatalf("line weight mismatch: got:%f want:%f", wl.Weight(), w)
				}
				break
			}
		}
		if !found {
			t.Fatalf("SetWeightedLine failed to add line: %#v", l)
		}
		if g.Node(u.ID()) == nil {
			t.Fatalf("SetWeightedLine failed to add from node: %#v", u)
		}
		if g.Node(v.ID()) == nil {
			t.Fatalf("SetWeightedLine failed to add to node: %#v", v)
		}

		if !canSetNode {
			continue
		}

		g.SetWeightedLine(g.NewWeightedLine(altNode{u}, altNode{v}, w))
		if nu := g.Node(u.ID()); nu == u {
			t.Fatalf("SetWeightedLine failed to update from node: u=%#v nu=%#v", u, nu)
		}
		if nv := g.Node(v.ID()); nv == v {
			t.Fatalf("SetWeightedLine failed to update to node: v=%#v nv=%#v", v, nv)
		}
	}
}

// EdgeRemover is a graph.EdgeRemover graph.
type EdgeRemover interface {
	graph.Graph
	graph.EdgeRemover
}

// RemoveEdges tests whether g correctly implements the graph.EdgeRemover interface.
// The input graph g must contain a set of nodes with some edges between them.
// RemoveEdges iterates over remove, which must contain edges in g, removing each
// edge. RemoveEdges confirms that the edge is removed, leaving its end-point nodes
// and all other edges in the graph.
func RemoveEdges(t *testing.T, g EdgeRemover, remove graph.Edges) {
	edges := make(map[edge]struct{})
	nodes := g.Nodes()
	for nodes.Next() {
		u := nodes.Node()
		uid := u.ID()
		to := g.From(uid)
		for to.Next() {
			v := to.Node()
			edges[edge{f: u.ID(), t: v.ID()}] = struct{}{}
		}
	}

	for remove.Next() {
		e := remove.Edge()
		if g.Edge(e.From().ID(), e.To().ID()) == nil {
			t.Fatalf("bad tests: missing edge: %#v", e)
		}
		if g.Node(e.From().ID()) == nil {
			t.Fatalf("bad tests: missing from node: %#v", e.From())
		}
		if g.Node(e.To().ID()) == nil {
			t.Fatalf("bad tests: missing to node: %#v", e.To())
		}

		g.RemoveEdge(e.From().ID(), e.To().ID())

		if _, ok := g.(graph.Undirected); ok {
			delete(edges, edge{f: e.To().ID(), t: e.From().ID()})
		}
		delete(edges, edge{f: e.From().ID(), t: e.To().ID()})
		for ge := range edges {
			if g.Edge(ge.f, ge.t) == nil {
				t.Fatalf("unexpected missing edge after removing edge %#v: %#v", e, ge)
			}
		}

		if ne := g.Edge(e.From().ID(), e.To().ID()); ne != nil {
			t.Fatalf("expected nil edge: got:%#v", ne)
		}
		if g.Node(e.From().ID()) == nil {
			t.Fatalf("unexpected deletion of from node: %#v", e.From())
		}
		if g.Node(e.To().ID()) == nil {
			t.Fatalf("unexpected deletion  to node: %#v", e.To())
		}
	}
}

// LineRemover is a graph.EdgeRemove graph.
type LineRemover interface {
	graph.Multigraph
	graph.LineRemover
}

// RemoveLines tests whether g correctly implements the graph.LineRemover interface.
// The input graph g must contain a set of nodes with some lines between them.
// RemoveLines iterates over remove, which must contain lines in g, removing each
// line. RemoveLines confirms that the line is removed, leaving its end-point nodes
// and all other lines in the graph.
func RemoveLines(t *testing.T, g LineRemover, remove graph.Lines) {
	// lines is the set of lines in the graph.
	// The presence of a key indicates that the
	// line should exist in the graph. The value
	// for each key is used to indicate whether
	// it has been found during testing.
	lines := make(map[edge]bool)
	nodes := g.Nodes()
	for nodes.Next() {
		u := nodes.Node()
		uid := u.ID()
		to := g.From(uid)
		for to.Next() {
			v := to.Node()
			lit := g.Lines(u.ID(), v.ID())
			for lit.Next() {
				lines[edge{f: u.ID(), t: v.ID(), id: lit.Line().ID()}] = true
			}
		}
	}

	for remove.Next() {
		l := remove.Line()
		if g.Lines(l.From().ID(), l.To().ID()) == graph.Empty {
			t.Fatalf("bad tests: missing line: %#v", l)
		}
		if g.Node(l.From().ID()) == nil {
			t.Fatalf("bad tests: missing from node: %#v", l.From())
		}
		if g.Node(l.To().ID()) == nil {
			t.Fatalf("bad tests: missing to node: %#v", l.To())
		}

		prev := g.Lines(l.From().ID(), l.To().ID())

		g.RemoveLine(l.From().ID(), l.To().ID(), l.ID())

		if _, ok := g.(graph.Undirected); ok {
			delete(lines, edge{f: l.To().ID(), t: l.From().ID(), id: l.ID()})
		}
		delete(lines, edge{f: l.From().ID(), t: l.To().ID(), id: l.ID()})

		// Mark all lines as not found.
		for gl := range lines {
			lines[gl] = false
		}

		// Mark found lines. This could be done far more efficiently.
		for gl := range lines {
			lit := g.Lines(gl.f, gl.t)
			for lit.Next() {
				lid := lit.Line().ID()
				if lid == gl.id {
					lines[gl] = true
					break
				}
			}
		}
		for gl, found := range lines {
			if !found {
				t.Fatalf("unexpected missing line after removing line %#v: %#v", l, gl)
			}
		}

		if curr := g.Lines(l.From().ID(), l.To().ID()); curr.Len() != prev.Len()-1 {
			t.Fatalf("RemoveLine failed to mutate graph: curr edge size != prev edge size-1, %d != %d", curr.Len(), prev.Len()-1)
		}
		if g.Node(l.From().ID()) == nil {
			t.Fatalf("unexpected deletion of from node: %#v", l.From())
		}
		if g.Node(l.To().ID()) == nil {
			t.Fatalf("unexpected deletion  to node: %#v", l.To())
		}
	}
}

// undirectedIDs returns a numerical sort ordered canonicalisation of the
// IDs of e.
func undirectedIDs(e Edge) (lo, hi int64, inverted bool) {
	lid := e.From().ID()
	hid := e.To().ID()
	if hid < lid {
		inverted = true
		hid, lid = lid, hid
	}
	return lid, hid, inverted
}

type edge struct {
	f, t, id int64
}

func same(a, b float64) bool {
	return (math.IsNaN(a) && math.IsNaN(b)) || a == b
}

func panics(fn func()) (ok bool) {
	defer func() {
		ok = recover() != nil
	}()
	fn()
	return
}

// RandomNodes implements the graph.Nodes interface for a set of random nodes.
type RandomNodes struct {
	n       int
	seed    uint64
	newNode func(int64) graph.Node

	curr int64

	state *rand.Rand
	seen  set.Int64s
	count int
}

var _ graph.Nodes = (*RandomNodes)(nil)

// NewRandomNodes returns a new implicit node iterator containing a set of n nodes
// with IDs generated from a PRNG seeded by the given seed.
// The provided new func maps the id to a graph.Node.
func NewRandomNodes(n int, seed uint64, new func(id int64) graph.Node) *RandomNodes {
	return &RandomNodes{
		n:       n,
		seed:    seed,
		newNode: new,

		state: rand.New(rand.NewSource(seed)),
		seen:  make(set.Int64s),
		count: 0,
	}
}

// Len returns the remaining number of nodes to be iterated over.
func (n *RandomNodes) Len() int {
	return n.n - n.count
}

// Next returns whether the next call of Node will return a valid node.
func (n *RandomNodes) Next() bool {
	if n.count >= n.n {
		return false
	}
	n.count++
	for {
		sign := int64(1)
		if n.state.Float64() < 0.5 {
			sign *= -1
		}
		n.curr = sign * n.state.Int63()
		if !n.seen.Has(n.curr) {
			n.seen.Add(n.curr)
			return true
		}
	}
}

// Node returns the current node of the iterator. Next must have been
// called prior to a call to Node.
func (n *RandomNodes) Node() graph.Node {
	if n.Len() == -1 || n.count == 0 {
		return nil
	}
	return n.newNode(n.curr)
}

// Reset returns the iterator to its initial state.
func (n *RandomNodes) Reset() {
	n.state = rand.New(rand.NewSource(n.seed))
	n.seen = make(set.Int64s)
	n.count = 0
}
