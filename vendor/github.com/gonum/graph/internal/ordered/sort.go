// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ordered provides common sort ordering types.
package ordered

import "github.com/gonum/graph"

// ByID implements the sort.Interface sorting a slice of graph.Node
// by ID.
type ByID []graph.Node

func (n ByID) Len() int           { return len(n) }
func (n ByID) Less(i, j int) bool { return n[i].ID() < n[j].ID() }
func (n ByID) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }

// BySliceValues implements the sort.Interface sorting a slice of
// []int lexically by the values of the []int.
type BySliceValues [][]int

func (c BySliceValues) Len() int { return len(c) }
func (c BySliceValues) Less(i, j int) bool {
	a, b := c[i], c[j]
	l := len(a)
	if len(b) < l {
		l = len(b)
	}
	for k, v := range a[:l] {
		if v < b[k] {
			return true
		}
		if v > b[k] {
			return false
		}
	}
	return len(a) < len(b)
}
func (c BySliceValues) Swap(i, j int) { c[i], c[j] = c[j], c[i] }

// BySliceIDs implements the sort.Interface sorting a slice of
// []graph.Node lexically by the IDs of the []graph.Node.
type BySliceIDs [][]graph.Node

func (c BySliceIDs) Len() int { return len(c) }
func (c BySliceIDs) Less(i, j int) bool {
	a, b := c[i], c[j]
	l := len(a)
	if len(b) < l {
		l = len(b)
	}
	for k, v := range a[:l] {
		if v.ID() < b[k].ID() {
			return true
		}
		if v.ID() > b[k].ID() {
			return false
		}
	}
	return len(a) < len(b)
}
func (c BySliceIDs) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
