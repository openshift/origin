// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package gen provides random graph generation functions.
package gen

import "github.com/gonum/graph"

// GraphBuilder is a graph that can have nodes and edges added.
type GraphBuilder interface {
	Has(graph.Node) bool
	HasEdgeBetween(x, y graph.Node) bool
	graph.Builder
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
