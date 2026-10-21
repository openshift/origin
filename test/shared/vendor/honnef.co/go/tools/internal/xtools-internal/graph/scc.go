// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import "slices"

// SCCs computes the strongly connected components of the graph g.
func SCCs[NodeID comparable](g Graph[NodeID]) [][]NodeID {
	// Use Kosaraju's algorithm. Tarjan is overkill here.

	// Forward pass
	S := Postorder(g)

	// Reverse pass
	gt := Transpose(g)
	seen := make(map[NodeID]bool)
	var scc []NodeID
	var sccs [][]NodeID
	var rvisit func(NodeID)
	rvisit = func(u NodeID) {
		if !seen[u] {
			seen[u] = true
			scc = append(scc, u)
			for v := range gt.Out(u) {
				rvisit(v)
			}
		}
	}
	for _, root := range slices.Backward(S) {
		if !seen[root] {
			scc = nil
			rvisit(root)
			sccs = append(sccs, scc)
		}
	}
	return sccs
}
