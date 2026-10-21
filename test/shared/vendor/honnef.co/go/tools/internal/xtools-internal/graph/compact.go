// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import "iter"

// A CompactGraph is a Graph with nodes that are compactly numbered from [0,
// NumNodes()).
//
// Compactly numbered graphs are useful for many graph algorithms, and many
// graph representations are naturally compact.
//
// To compact an arbitrary graph, use [Compact].
type CompactGraph interface {
	Graph[int]
	IsCompact()
}

// A nodePreserving graph is a transformation of another graph that preserves
// node IDs.
type nodePreserving interface {
	Graph[int]
	unwrapPreservingNodes() Graph[int]
}

// Compact takes a Graph with arbitrary NodeIDs and returns a compact graph.
//
// If g implements [CompactGraph], it assumes g is already compact and simply
// returns g and an identity mapping.
func Compact[NodeID comparable](g Graph[NodeID]) (CompactGraph, *Index[NodeID]) {
	// If it's already compact, simply return it.
	if gc, ok := g.(CompactGraph); ok {
		// The above assertion ensures NodeID is int, so we know this type
		// assertion will always succeed.
		return gc, any(NewIdentityIndex(gc.NumNodes())).(*Index[NodeID])
	}

	// If it's a transformation and the underlying graph is compact, we can use
	// an identity index. Though we still need to build a compactGraph to
	// satisfy the CompactGraph interface.
	g2, _ := g.(nodePreserving)
	for g2 != nil {
		unwrapped := g2.unwrapPreservingNodes()
		if gc, ok := unwrapped.(CompactGraph); ok {
			index := any(NewIdentityIndex(gc.NumNodes())).(*Index[NodeID])
			cg := compactGraph[NodeID]{g, index}
			return &cg, index
		}
		g2, _ = unwrapped.(nodePreserving)
	}

	// Nope, just build an index.
	cg := compactGraph[NodeID]{g, NewIndex(g.Nodes())}
	return &cg, cg.m
}

type compactGraph[NodeID comparable] struct {
	g Graph[NodeID]
	m *Index[NodeID]
}

func (g *compactGraph[NodeID]) Nodes() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range g.g.NumNodes() {
			if !yield(i) {
				break
			}
		}
	}
}

func (g *compactGraph[NodeID]) NumNodes() int {
	return g.g.NumNodes()
}

func (g *compactGraph[NodeID]) Out(node int) iter.Seq[int] {
	id := g.m.Value(node)
	return func(yield func(int) bool) {
		for nid := range g.g.Out(id) {
			if !yield(g.m.Index(nid)) {
				break
			}
		}
	}
}

func (g *compactGraph[NodeID]) IsCompact() {}
