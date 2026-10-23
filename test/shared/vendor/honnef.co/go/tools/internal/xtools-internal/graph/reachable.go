// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

// Reachable returns the set of nodes reachable from the given roots.
func Reachable[NodeID comparable](g Graph[NodeID], roots ...NodeID) map[NodeID]bool {
	seen := make(map[NodeID]bool)
	var visit func(node NodeID)
	visit = func(node NodeID) {
		if !seen[node] {
			seen[node] = true
			for e := range g.Out(node) {
				visit(e)
			}
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return seen
}
