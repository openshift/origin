// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package graph

import "slices"

// ShortestPath returns a shortest path from src to dst in g.
// It returns the path as a slice of nodes starting with src and ending with dst.
// If no path is found, it returns nil.
func ShortestPath[NodeID comparable](g Graph[NodeID], src, dst NodeID) []NodeID {
	if src == dst {
		return []NodeID{src}
	}

	pred := make(map[NodeID]NodeID)
	queue := []NodeID{src}
	// Mark src as seen.
	pred[src] = src

	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]

		if n == dst {
			// Reconstruct path
			var path []NodeID
			for curr := dst; curr != src; curr = pred[curr] {
				path = append(path, curr)
			}
			path = append(path, src)
			slices.Reverse(path)
			return path
		}

		for v := range g.Out(n) {
			if _, seen := pred[v]; !seen {
				pred[v] = n
				queue = append(queue, v)
			}
		}
	}
	return nil
}
