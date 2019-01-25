// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this code is governed by a BSD-style
// license that can be found in the LICENSE file

package sampleuv

import (
	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

// Weighted provides sampling without replacement from a collection of items with
// non-uniform probability.
type Weighted struct {
	weights []float64
	// heap is a weight heap.
	//
	// It keeps a heap-organised sum of remaining
	// index weights that are available to be taken
	// from.
	//
	// Each element holds the sum of weights for
	// the corresponding index, plus the sum of
	// of its children's weights; the children
	// of an element i can be found at positions
	// 2*(i+1)-1 and 2*(i+1). The root of the
	// weight heap is at element 0.
	//
	// See comments in container/heap for an
	// explanation of the layout of a heap.
	heap []float64
	rnd  *rand.Rand
}

// NewWeighted returns a Weighted for the weights w. If src is nil, rand.Rand is
// used as the random number generator.
//
// Note that sampling from weights with a high variance or overall low absolute
// value sum may result in problems with numerical stability.
func NewWeighted(w []float64, src rand.Source) Weighted {
	s := Weighted{
		weights: make([]float64, len(w)),
		heap:    make([]float64, len(w)),
	}
	if src != nil {
		s.rnd = rand.New(src)
	}
	s.ReweightAll(w)
	return s
}

// Len returns the number of items held by the Weighted, including items
// already taken.
func (s Weighted) Len() int { return len(s.weights) }

// Take returns an index from the Weighted with probability proportional
// to the weight of the item. The weight of the item is then set to zero.
// Take returns false if there are no items remaining.
func (s Weighted) Take() (idx int, ok bool) {
	const small = 1e-12
	if floats.EqualWithinAbsOrRel(s.heap[0], 0, small, small) {
		return -1, false
	}

	var r float64
	if s.rnd == nil {
		r = s.heap[0] * rand.Float64()
	} else {
		r = s.heap[0] * s.rnd.Float64()
	}
	i := 1
	last := -1
	left := len(s.weights)
	for {
		if r -= s.weights[i-1]; r <= 0 {
			break // Fall within item i-1.
		}
		i <<= 1 // Move to left child.
		if d := s.heap[i-1]; r > d {
			r -= d
			// If enough r to pass left child
			// move to right child state will
			// be caught at break above.
			i++
		}
		if i == last || left < 0 {
			// No progression.
			return -1, false
		}
		last = i
		left--
	}

	w, idx := s.weights[i-1], i-1

	s.weights[i-1] = 0
	for i > 0 {
		s.heap[i-1] -= w
		// The following condition is necessary to
		// handle floating point error. If we see
		// a heap value below zero, we know we need
		// to rebuild it.
		if s.heap[i-1] < 0 {
			s.reset()
			return idx, true
		}
		i >>= 1
	}

	return idx, true
}

// Reweight sets the weight of item idx to w.
func (s Weighted) Reweight(idx int, w float64) {
	w, s.weights[idx] = s.weights[idx]-w, w
	idx++
	for idx > 0 {
		s.heap[idx-1] -= w
		idx >>= 1
	}
}

// ReweightAll sets the weight of all items in the Weighted. ReweightAll
// panics if len(w) != s.Len.
func (s Weighted) ReweightAll(w []float64) {
	if len(w) != s.Len() {
		panic("floats: length of the slices do not match")
	}
	copy(s.weights, w)
	s.reset()
}

func (s Weighted) reset() {
	copy(s.heap, s.weights)
	for i := len(s.heap) - 1; i > 0; i-- {
		// Sometimes 1-based counting makes sense.
		s.heap[((i+1)>>1)-1] += s.heap[i]
	}
}
