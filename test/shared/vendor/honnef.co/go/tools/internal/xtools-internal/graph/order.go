// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import "slices"

// Postorder returns the sequence of nodes in the spanning DAG of g, in
// postorder.
//
// For rootless subgraphs, it breaks cycles by starting at the lowest numbered
// node.
//
// This algorithm runs in O(V + E) time and O(V + E) space.
func Postorder[NodeID comparable](g Graph[NodeID]) []NodeID {
	cg, nodeMap := Compact(g)

	numNodes := cg.NumNodes()
	if numNodes == 0 {
		return nil
	}

	result := make([]NodeID, 0, numNodes)
	visited := newBitset(numNodes)
	onStack := newBitset(numNodes)

	// visit performs a Depth-First Search.
	var visit func(u int)
	visit = func(u int) {
		if !visited.add(u) {
			return
		}
		onStack.add(u)

		for v := range cg.Out(u) {
			if onStack.contains(v) {
				// Cycle detected (back-edge).
				// To resolve, we simply skip processing this edge further in the
				// current recursion, effectively "breaking" the cycle at this point.
				continue
			}
			visit(v)
		}

		onStack.remove(u)
		// Post-order: add to result after all descendants are processed.
		result = append(result, nodeMap.Value(u))
	}

	// Visit every node in ascending order to ensure stability.
	for u := range numNodes {
		visit(u)
	}

	return result
}

// ReversePostorder returns the nodes of the graph in reverse post-order.
//
// If g is a directed acyclic graph (DAG), the result is a topological sort of
// g.
//
// See [Postorder] for how this handles back-edges and cycles.
//
// This algorithm runs in O(V + E) time and O(V + E) space.
func ReversePostorder[NodeID comparable](g Graph[NodeID]) []NodeID {
	result := Postorder(g)
	slices.Reverse(result)
	return result
}

// bitset is a simple fixed-size bitset used to reduce memory overhead.
type bitset []uint64

func newBitset(n int) bitset {
	return make(bitset, (n+63)/64)
}

func (b bitset) add(u int) bool {
	if b.contains(u) {
		return false
	}
	b[u/64] |= 1 << (u % 64)
	return true
}

func (b bitset) remove(u int) {
	b[u/64] &= ^(1 << (u % 64))
}

func (b bitset) contains(u int) bool {
	return b[u/64]&(1<<(u%64)) != 0
}
