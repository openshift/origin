// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import (
	"iter"
	"slices"
)

type transpose[NodeID comparable] struct {
	Graph Graph[NodeID]
	preds map[NodeID][]NodeID
}

// transpose returns a graph like g but with all edges reversed. Node IDs are
// identical to the underlying graph.
//
// Transpose preserves compactness.
func Transpose[NodeID comparable](g Graph[NodeID]) Graph[NodeID] {
	if g, ok := g.(transpose[NodeID]); ok {
		// Transpose(Transpose(g)) == g
		return g.Graph
	}

	preds := make(map[NodeID][]NodeID)
	for nid := range g.Nodes() {
		for succ := range g.Out(nid) {
			preds[succ] = append(preds[succ], nid)
		}
	}
	return transpose[NodeID]{g, preds}
}

func (t transpose[NodeID]) NumNodes() int {
	return len(t.preds)
}

func (t transpose[NodeID]) Nodes() iter.Seq[NodeID] {
	return t.Graph.Nodes()
}

func (t transpose[NodeID]) Out(n NodeID) iter.Seq[NodeID] {
	return slices.Values(t.preds[n])
}

//lint:ignore U1000 False positive in Staticcheck 2026.1 and older.
func (t transpose[NodeID]) unwrapPreservingNodes() Graph[NodeID] {
	return t.Graph
}
