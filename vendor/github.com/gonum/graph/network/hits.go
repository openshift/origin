// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"github.com/gonum/floats"
	"github.com/gonum/graph"
)

// HubAuthority is a Hyperlink-Induced Topic Search hub-authority score pair.
type HubAuthority struct {
	Hub       float64
	Authority float64
}

// HITS returns the Hyperlink-Induced Topic Search hub-authority scores for
// nodes of the directed graph g. HITS terminates when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
func HITS(g graph.Directed, tol float64) map[int]HubAuthority {
	nodes := g.Nodes()

	// Make a topological copy of g with dense node IDs.
	indexOf := make(map[int]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}
	nodesLinkingTo := make([][]int, len(nodes))
	nodesLinkedFrom := make([][]int, len(nodes))
	for i, n := range nodes {
		for _, u := range g.To(n) {
			nodesLinkingTo[i] = append(nodesLinkingTo[i], indexOf[u.ID()])
		}
		for _, v := range g.From(n) {
			nodesLinkedFrom[i] = append(nodesLinkedFrom[i], indexOf[v.ID()])
		}
	}
	indexOf = nil

	w := make([]float64, 4*len(nodes))
	auth := w[:len(nodes)]
	hub := w[len(nodes) : 2*len(nodes)]
	for i := range nodes {
		auth[i] = 1
		hub[i] = 1
	}
	deltaAuth := w[2*len(nodes) : 3*len(nodes)]
	deltaHub := w[3*len(nodes):]

	var norm float64
	for {
		norm = 0
		for v := range nodes {
			var a float64
			for _, u := range nodesLinkingTo[v] {
				a += hub[u]
			}
			deltaAuth[v] = auth[v]
			auth[v] = a
			norm += a * a
		}
		norm = math.Sqrt(norm)

		for i := range auth {
			auth[i] /= norm
			deltaAuth[i] -= auth[i]
		}

		norm = 0
		for u := range nodes {
			var h float64
			for _, v := range nodesLinkedFrom[u] {
				h += auth[v]
			}
			deltaHub[u] = hub[u]
			hub[u] = h
			norm += h * h
		}
		norm = math.Sqrt(norm)

		for i := range hub {
			hub[i] /= norm
			deltaHub[i] -= hub[i]
		}

		if floats.Norm(deltaAuth, 2) < tol && floats.Norm(deltaHub, 2) < tol {
			break
		}
	}

	hubAuth := make(map[int]HubAuthority, len(nodes))
	for i, n := range nodes {
		hubAuth[n.ID()] = HubAuthority{Hub: hub[i], Authority: auth[i]}
	}

	return hubAuth
}
