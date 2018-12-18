// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gen

import (
	"errors"
	"fmt"
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/stat/sampleuv"
)

// NavigableSmallWorld constructs an N-dimensional grid with guaranteed local connectivity
// and random long-range connectivity in the destination, dst. The dims parameters specifies
// the length of each of the N dimensions, p defines the Manhattan distance between local
// nodes, and q defines the number of out-going long-range connections from each node. Long-
// range connections are made with a probability proportional to |d(u,v)|^-r where d is the
// Manhattan distance between non-local nodes.
//
// The algorithm is essentially as described on p4 of http://www.cs.cornell.edu/home/kleinber/swn.pdf.
func NavigableSmallWorld(dst GraphBuilder, dims []int, p, q int, r float64, src rand.Source) (err error) {
	if p < 1 {
		return fmt.Errorf("gen: bad local distance: p=%v", p)
	}
	if q < 0 {
		return fmt.Errorf("gen: bad distant link count: q=%v", q)
	}
	if r < 0 {
		return fmt.Errorf("gen: bad decay constant: r=%v", r)
	}

	n := 1
	for _, d := range dims {
		n *= d
	}
	for i := 0; i < n; i++ {
		if !dst.Has(int64(i)) {
			dst.AddNode(simple.Node(i))
		}
	}

	hasEdge := dst.HasEdgeBetween
	d, isDirected := dst.(graph.Directed)
	if isDirected {
		hasEdge = d.HasEdgeFromTo
	}

	locality := make([]int, len(dims))
	for i := range locality {
		locality[i] = p*2 + 1
	}
	iterateOver(dims, func(u []int) {
		uid := idFrom(u, dims)
		iterateOver(locality, func(delta []int) {
			d := manhattanDelta(u, delta, dims, -p)
			if d == 0 || d > p {
				return
			}
			vid := idFromDelta(u, delta, dims, -p)
			e := simple.Edge{F: simple.Node(uid), T: simple.Node(vid)}
			if uid > vid {
				e.F, e.T = e.T, e.F
			}
			if !hasEdge(e.From().ID(), e.To().ID()) {
				dst.SetEdge(e)
			}
			if !isDirected {
				return
			}
			e.F, e.T = e.T, e.F
			if !hasEdge(e.From().ID(), e.To().ID()) {
				dst.SetEdge(e)
			}
		})
	})

	defer func() {
		r := recover()
		if r != nil {
			if r != "depleted distribution" {
				panic(r)
			}
			err = errors.New("depleted distribution")
		}
	}()
	w := make([]float64, n)
	ws := sampleuv.NewWeighted(w, src)
	iterateOver(dims, func(u []int) {
		uid := idFrom(u, dims)
		iterateOver(dims, func(v []int) {
			d := manhattanBetween(u, v)
			if d <= p {
				return
			}
			w[idFrom(v, dims)] = math.Pow(float64(d), -r)
		})
		ws.ReweightAll(w)
		for i := 0; i < q; i++ {
			vid, ok := ws.Take()
			if !ok {
				panic("depleted distribution")
			}
			e := simple.Edge{F: simple.Node(uid), T: simple.Node(vid)}
			if !isDirected && uid > vid {
				e.F, e.T = e.T, e.F
			}
			if !hasEdge(e.From().ID(), e.To().ID()) {
				dst.SetEdge(e)
			}
		}
		for i := range w {
			w[i] = 0
		}
	})

	return nil
}

// iterateOver performs an iteration over all dimensions of dims, calling fn
// for each state. The elements of state must not be mutated by fn.
func iterateOver(dims []int, fn func(state []int)) {
	iterator(0, dims, make([]int, len(dims)), fn)
}

func iterator(d int, dims, state []int, fn func(state []int)) {
	if d >= len(dims) {
		fn(state)
		return
	}
	for i := 0; i < dims[d]; i++ {
		state[d] = i
		iterator(d+1, dims, state, fn)
	}
}

// manhattanBetween returns the Manhattan distance between a and b.
func manhattanBetween(a, b []int) int {
	if len(a) != len(b) {
		panic("gen: unexpected dimension")
	}
	var d int
	for i, v := range a {
		d += abs(v - b[i])
	}
	return d
}

// manhattanDelta returns the Manhattan norm of delta+translate. If a
// translated by delta+translate is out of the range given by dims,
// zero is returned.
func manhattanDelta(a, delta, dims []int, translate int) int {
	if len(a) != len(dims) {
		panic("gen: unexpected dimension")
	}
	if len(delta) != len(dims) {
		panic("gen: unexpected dimension")
	}
	var d int
	for i, v := range delta {
		v += translate
		t := a[i] + v
		if t < 0 || t >= dims[i] {
			return 0
		}
		d += abs(v)
	}
	return d
}

// idFrom returns a node id for the slice n over the given dimensions.
func idFrom(n, dims []int) int {
	s := 1
	var id int
	for d, m := range dims {
		p := n[d]
		if p < 0 || p >= m {
			panic("gen: element out of range")
		}
		id += p * s
		s *= m
	}
	return id
}

// idFromDelta returns a node id for the slice base plus the delta over the given
// dimensions and applying the translation.
func idFromDelta(base, delta, dims []int, translate int) int {
	s := 1
	var id int
	for d, m := range dims {
		n := base[d] + delta[d] + translate
		if n < 0 || n >= m {
			panic("gen: element out of range")
		}
		id += n * s
		s *= m
	}
	return id
}
