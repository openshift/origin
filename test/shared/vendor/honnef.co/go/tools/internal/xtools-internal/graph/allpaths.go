// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

// AllPaths returns the set of nodes that are part of at least one path from src to dst.
func AllPaths[NodeID comparable](g Graph[NodeID], src, dst NodeID) map[NodeID]bool {
	// We intersect the forward closure of 'src' with
	// the reverse closure of 'dst'. This is not the most
	// efficient implementation, but it's the clearest,
	// and the previous one had bugs.

	fwd := Reachable(g, src)
	rev := Reachable(Transpose(g), dst)

	// Intersection
	for n := range fwd {
		if !rev[n] {
			delete(fwd, n)
		}
	}
	return fwd
}
