// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/mat"
)

// PageRank returns the PageRank weights for nodes of the directed graph g
// using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
// If g is a graph.WeightedDirected, an edge-weighted PageRank is calculated.
func PageRank(g graph.Directed, damp, tol float64) map[int64]float64 {
	if g, ok := g.(graph.WeightedDirected); ok {
		return edgeWeightedPageRank(g, damp, tol)
	}
	return pageRank(g, damp, tol)
}

// PageRankSparse returns the PageRank weights for nodes of the sparse directed
// graph g using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
// If g is a graph.WeightedDirected, an edge-weighted PageRank is calculated.
func PageRankSparse(g graph.Directed, damp, tol float64) map[int64]float64 {
	if g, ok := g.(graph.WeightedDirected); ok {
		return edgeWeightedPageRankSparse(g, damp, tol)
	}
	return pageRankSparse(g, damp, tol)
}

// edgeWeightedPageRank returns the PageRank weights for nodes of the weighted directed graph g
// using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
func edgeWeightedPageRank(g graph.WeightedDirected, damp, tol float64) map[int64]float64 {
	// edgeWeightedPageRank is implemented according to "How Google Finds Your Needle
	// in the Web's Haystack" with the modification that
	// the columns of hyperlink matrix H are calculated with edge weights.
	//
	// G.I^k = alpha.H.I^k + alpha.A.I^k + (1-alpha).1/n.1.I^k
	//
	// http://www.ams.org/samplings/feature-column/fcarc-pagerank

	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}

	m := mat.NewDense(len(nodes), len(nodes), nil)
	dangling := damp / float64(len(nodes))
	for j, u := range nodes {
		to := graph.NodesOf(g.From(u.ID()))
		var z float64
		for _, v := range to {
			if w, ok := g.Weight(u.ID(), v.ID()); ok {
				z += w
			}
		}
		if z != 0 {
			for _, v := range to {
				if w, ok := g.Weight(u.ID(), v.ID()); ok {
					m.Set(indexOf[v.ID()], j, (w*damp)/z)
				}
			}
		} else {
			for i := range nodes {
				m.Set(i, j, dangling)
			}
		}
	}

	matrix := m.RawMatrix().Data
	dt := (1 - damp) / float64(len(nodes))
	for i := range matrix {
		matrix[i] += dt
	}

	last := make([]float64, len(nodes))
	for i := range last {
		last[i] = 1
	}
	lastV := mat.NewVecDense(len(nodes), last)

	vec := make([]float64, len(nodes))
	var sum float64
	for i := range vec {
		r := rand.NormFloat64()
		sum += r
		vec[i] = r
	}
	f := 1 / sum
	for i := range vec {
		vec[i] *= f
	}
	v := mat.NewVecDense(len(nodes), vec)

	for {
		lastV, v = v, lastV
		v.MulVec(m, lastV)
		if normDiff(vec, last) < tol {
			break
		}
	}

	ranks := make(map[int64]float64, len(nodes))
	for i, r := range v.RawVector().Data {
		ranks[nodes[i].ID()] = r
	}

	return ranks
}

// edgeWeightedPageRankSparse returns the PageRank weights for nodes of the sparse weighted directed
// graph g using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
func edgeWeightedPageRankSparse(g graph.WeightedDirected, damp, tol float64) map[int64]float64 {
	// edgeWeightedPageRankSparse is implemented according to "How Google Finds Your Needle
	// in the Web's Haystack" with the modification that
	// the columns of hyperlink matrix H are calculated with edge weights.
	//
	// G.I^k = alpha.H.I^k + alpha.A.I^k + (1-alpha).1/n.1.I^k
	//
	// http://www.ams.org/samplings/feature-column/fcarc-pagerank

	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}

	m := make(rowCompressedMatrix, len(nodes))
	var dangling compressedRow
	df := damp / float64(len(nodes))
	for j, u := range nodes {
		to := graph.NodesOf(g.From(u.ID()))
		var z float64
		for _, v := range to {
			if w, ok := g.Weight(u.ID(), v.ID()); ok {
				z += w
			}
		}
		if z != 0 {
			for _, v := range to {
				if w, ok := g.Weight(u.ID(), v.ID()); ok {
					m.addTo(indexOf[v.ID()], j, (w*damp)/z)
				}
			}
		} else {
			dangling.addTo(j, df)
		}
	}

	last := make([]float64, len(nodes))
	for i := range last {
		last[i] = 1
	}
	lastV := mat.NewVecDense(len(nodes), last)

	vec := make([]float64, len(nodes))
	var sum float64
	for i := range vec {
		r := rand.NormFloat64()
		sum += r
		vec[i] = r
	}
	f := 1 / sum
	for i := range vec {
		vec[i] *= f
	}
	v := mat.NewVecDense(len(nodes), vec)

	dt := (1 - damp) / float64(len(nodes))
	for {
		lastV, v = v, lastV

		m.mulVecUnitary(v, lastV)          // First term of the G matrix equation;
		with := dangling.dotUnitary(lastV) // Second term;
		away := onesDotUnitary(dt, lastV)  // Last term.

		floats.AddConst(with+away, v.RawVector().Data)
		if normDiff(vec, last) < tol {
			break
		}
	}

	ranks := make(map[int64]float64, len(nodes))
	for i, r := range v.RawVector().Data {
		ranks[nodes[i].ID()] = r
	}

	return ranks
}

// pageRank returns the PageRank weights for nodes of the directed graph g
// using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
func pageRank(g graph.Directed, damp, tol float64) map[int64]float64 {
	// pageRank is implemented according to "How Google Finds Your Needle
	// in the Web's Haystack".
	//
	// G.I^k = alpha.S.I^k + (1-alpha).1/n.1.I^k
	//
	// http://www.ams.org/samplings/feature-column/fcarc-pagerank

	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}

	m := mat.NewDense(len(nodes), len(nodes), nil)
	dangling := damp / float64(len(nodes))
	for j, u := range nodes {
		to := graph.NodesOf(g.From(u.ID()))
		f := damp / float64(len(to))
		for _, v := range to {
			m.Set(indexOf[v.ID()], j, f)
		}
		if len(to) == 0 {
			for i := range nodes {
				m.Set(i, j, dangling)
			}
		}
	}
	matrix := m.RawMatrix().Data
	dt := (1 - damp) / float64(len(nodes))
	for i := range matrix {
		matrix[i] += dt
	}

	last := make([]float64, len(nodes))
	for i := range last {
		last[i] = 1
	}
	lastV := mat.NewVecDense(len(nodes), last)

	vec := make([]float64, len(nodes))
	var sum float64
	for i := range vec {
		r := rand.NormFloat64()
		sum += r
		vec[i] = r
	}
	f := 1 / sum
	for i := range vec {
		vec[i] *= f
	}
	v := mat.NewVecDense(len(nodes), vec)

	for {
		lastV, v = v, lastV
		v.MulVec(m, lastV)
		if normDiff(vec, last) < tol {
			break
		}
	}

	ranks := make(map[int64]float64, len(nodes))
	for i, r := range v.RawVector().Data {
		ranks[nodes[i].ID()] = r
	}

	return ranks
}

// pageRankSparse returns the PageRank weights for nodes of the sparse directed
// graph g using the given damping factor and terminating when the 2-norm of the
// vector difference between iterations is below tol. The returned map is
// keyed on the graph node IDs.
func pageRankSparse(g graph.Directed, damp, tol float64) map[int64]float64 {
	// pageRankSparse is implemented according to "How Google Finds Your Needle
	// in the Web's Haystack".
	//
	// G.I^k = alpha.H.I^k + alpha.A.I^k + (1-alpha).1/n.1.I^k
	//
	// http://www.ams.org/samplings/feature-column/fcarc-pagerank

	nodes := graph.NodesOf(g.Nodes())
	indexOf := make(map[int64]int, len(nodes))
	for i, n := range nodes {
		indexOf[n.ID()] = i
	}

	m := make(rowCompressedMatrix, len(nodes))
	var dangling compressedRow
	df := damp / float64(len(nodes))
	for j, u := range nodes {
		to := graph.NodesOf(g.From(u.ID()))
		f := damp / float64(len(to))
		for _, v := range to {
			m.addTo(indexOf[v.ID()], j, f)
		}
		if len(to) == 0 {
			dangling.addTo(j, df)
		}
	}

	last := make([]float64, len(nodes))
	for i := range last {
		last[i] = 1
	}
	lastV := mat.NewVecDense(len(nodes), last)

	vec := make([]float64, len(nodes))
	var sum float64
	for i := range vec {
		r := rand.NormFloat64()
		sum += r
		vec[i] = r
	}
	f := 1 / sum
	for i := range vec {
		vec[i] *= f
	}
	v := mat.NewVecDense(len(nodes), vec)

	dt := (1 - damp) / float64(len(nodes))
	for {
		lastV, v = v, lastV

		m.mulVecUnitary(v, lastV)          // First term of the G matrix equation;
		with := dangling.dotUnitary(lastV) // Second term;
		away := onesDotUnitary(dt, lastV)  // Last term.

		floats.AddConst(with+away, v.RawVector().Data)
		if normDiff(vec, last) < tol {
			break
		}
	}

	ranks := make(map[int64]float64, len(nodes))
	for i, r := range v.RawVector().Data {
		ranks[nodes[i].ID()] = r
	}

	return ranks
}

// rowCompressedMatrix implements row-compressed
// matrix/vector multiplication.
type rowCompressedMatrix []compressedRow

// addTo adds the value v to the matrix element at (i,j). Repeated
// calls to addTo with the same column index will result in
// non-unique element representation.
func (m rowCompressedMatrix) addTo(i, j int, v float64) { m[i].addTo(j, v) }

// mulVecUnitary multiplies the receiver by the src vector, storing
// the result in dst. It assumes src and dst are the same length as m
// and that both have unitary vector increments.
func (m rowCompressedMatrix) mulVecUnitary(dst, src *mat.VecDense) {
	dMat := dst.RawVector().Data
	for i, r := range m {
		dMat[i] = r.dotUnitary(src)
	}
}

// compressedRow implements a simplified scatter-based Ddot.
type compressedRow []sparseElement

// addTo adds the value v to the vector element at j. Repeated
// calls to addTo with the same vector index will result in
// non-unique element representation.
func (r *compressedRow) addTo(j int, v float64) {
	*r = append(*r, sparseElement{index: j, value: v})
}

// dotUnitary performs a simplified scatter-based Ddot operations on
// v and the receiver. v must have a unitary vector increment.
func (r compressedRow) dotUnitary(v *mat.VecDense) float64 {
	var sum float64
	vec := v.RawVector().Data
	for _, e := range r {
		sum += vec[e.index] * e.value
	}
	return sum
}

// sparseElement is a sparse vector or matrix element.
type sparseElement struct {
	index int
	value float64
}

// onesDotUnitary performs the equivalent of a Ddot of v with
// a ones vector of equal length. v must have a unitary vector
// increment.
func onesDotUnitary(alpha float64, v *mat.VecDense) float64 {
	var sum float64
	for _, f := range v.RawVector().Data {
		sum += alpha * f
	}
	return sum
}

// normDiff returns the 2-norm of the difference between x and y.
// This is a cut down version of gonum/floats.Distance.
func normDiff(x, y []float64) float64 {
	var sum float64
	for i, v := range x {
		d := v - y[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}
