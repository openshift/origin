// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import "gonum.org/v1/gonum/graph"

// GraphBuilder is a graph that can have nodes and edges added.
type GraphBuilder interface {
	HasEdgeBetween(xid, yid int64) bool
	graph.Builder
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
