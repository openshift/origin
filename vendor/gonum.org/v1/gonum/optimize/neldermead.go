// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"sort"

	"gonum.org/v1/gonum/floats"
)

// nmIterType is a Nelder-Mead evaluation kind
type nmIterType int

const (
	nmReflected = iota
	nmExpanded
	nmContractedInside
	nmContractedOutside
	nmInitialize
	nmShrink
	nmMajor
)

type nmVertexSorter struct {
	vertices [][]float64
	values   []float64
}

func (n nmVertexSorter) Len() int {
	return len(n.values)
}

func (n nmVertexSorter) Less(i, j int) bool {
	return n.values[i] < n.values[j]
}

func (n nmVertexSorter) Swap(i, j int) {
	n.values[i], n.values[j] = n.values[j], n.values[i]
	n.vertices[i], n.vertices[j] = n.vertices[j], n.vertices[i]
}

// NelderMead is an implementation of the Nelder-Mead simplex algorithm for
// gradient-free nonlinear optimization (not to be confused with Danzig's
// simplex algorithm for linear programming). The implementation follows the
// algorithm described in
//
//  http://epubs.siam.org/doi/pdf/10.1137/S1052623496303470
//
// If an initial simplex is provided, it is used and initLoc is ignored. If
// InitialVertices and InitialValues are both nil, an initial simplex will be
// generated automatically using the initial location as one vertex, and each
// additional vertex as SimplexSize away in one dimension.
//
// If the simplex update parameters (Reflection, etc.)
// are zero, they will be set automatically based on the dimension according to
// the recommendations in
//
//  http://www.webpages.uidaho.edu/~fuchang/res/ANMS.pdf
type NelderMead struct {
	InitialVertices [][]float64
	InitialValues   []float64
	Reflection      float64 // Reflection parameter (>0)
	Expansion       float64 // Expansion parameter (>1)
	Contraction     float64 // Contraction parameter (>0, <1)
	Shrink          float64 // Shrink parameter (>0, <1)
	SimplexSize     float64 // size of auto-constructed initial simplex

	status Status
	err    error

	reflection  float64
	expansion   float64
	contraction float64
	shrink      float64

	vertices [][]float64 // location of the vertices sorted in ascending f
	values   []float64   // function values at the vertices sorted in ascending f
	centroid []float64   // centroid of all but the worst vertex

	fillIdx        int        // index for filling the simplex during initialization and shrinking
	lastIter       nmIterType // Last iteration
	reflectedPoint []float64  // Storage of the reflected point location
	reflectedValue float64    // Value at the last reflection point
}

func (n *NelderMead) Status() (Status, error) {
	return n.status, n.err
}

func (n *NelderMead) Init(dim, tasks int) int {
	n.status = NotTerminated
	n.err = nil
	return 1
}

func (n *NelderMead) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	n.status, n.err = localOptimizer{}.run(n, operation, result, tasks)
	close(operation)
	return
}

func (n *NelderMead) initLocal(loc *Location) (Operation, error) {
	dim := len(loc.X)
	if cap(n.vertices) < dim+1 {
		n.vertices = make([][]float64, dim+1)
	}
	n.vertices = n.vertices[:dim+1]
	for i := range n.vertices {
		n.vertices[i] = resize(n.vertices[i], dim)
	}
	n.values = resize(n.values, dim+1)
	n.centroid = resize(n.centroid, dim)
	n.reflectedPoint = resize(n.reflectedPoint, dim)

	if n.SimplexSize == 0 {
		n.SimplexSize = 0.05
	}

	// Default parameter choices are chosen in a dimension-dependent way
	// from http://www.webpages.uidaho.edu/~fuchang/res/ANMS.pdf
	n.reflection = n.Reflection
	if n.reflection == 0 {
		n.reflection = 1
	}
	n.expansion = n.Expansion
	if n.expansion == 0 {
		n.expansion = 1 + 2/float64(dim)
		if dim == 1 {
			n.expansion = 2
		}
	}
	n.contraction = n.Contraction
	if n.contraction == 0 {
		n.contraction = 0.75 - 1/(2*float64(dim))
		if dim == 1 {
			n.contraction = 0.5
		}
	}
	n.shrink = n.Shrink
	if n.shrink == 0 {
		n.shrink = 1 - 1/float64(dim)
		if dim == 1 {
			n.shrink = 0.5
		}
	}

	if n.InitialVertices != nil {
		// Initial simplex provided. Copy the locations and values, and sort them.
		if len(n.InitialVertices) != dim+1 {
			panic("neldermead: incorrect number of vertices in initial simplex")
		}
		if len(n.InitialValues) != dim+1 {
			panic("neldermead: incorrect number of values in initial simplex")
		}
		for i := range n.InitialVertices {
			if len(n.InitialVertices[i]) != dim {
				panic("neldermead: vertex size mismatch")
			}
			copy(n.vertices[i], n.InitialVertices[i])
		}
		copy(n.values, n.InitialValues)
		sort.Sort(nmVertexSorter{n.vertices, n.values})
		computeCentroid(n.vertices, n.centroid)
		return n.returnNext(nmMajor, loc)
	}

	// No simplex provided. Begin initializing initial simplex. First simplex
	// entry is the initial location, then step 1 in every direction.
	copy(n.vertices[dim], loc.X)
	n.values[dim] = loc.F
	n.fillIdx = 0
	loc.X[n.fillIdx] += n.SimplexSize
	n.lastIter = nmInitialize
	return FuncEvaluation, nil
}

// computeCentroid computes the centroid of all the simplex vertices except the
// final one
func computeCentroid(vertices [][]float64, centroid []float64) {
	dim := len(centroid)
	for i := range centroid {
		centroid[i] = 0
	}
	for i := 0; i < dim; i++ {
		vertex := vertices[i]
		for j, v := range vertex {
			centroid[j] += v
		}
	}
	for i := range centroid {
		centroid[i] /= float64(dim)
	}
}

func (n *NelderMead) iterateLocal(loc *Location) (Operation, error) {
	dim := len(loc.X)
	switch n.lastIter {
	case nmInitialize:
		n.values[n.fillIdx] = loc.F
		copy(n.vertices[n.fillIdx], loc.X)
		n.fillIdx++
		if n.fillIdx == dim {
			// Successfully finished building initial simplex.
			sort.Sort(nmVertexSorter{n.vertices, n.values})
			computeCentroid(n.vertices, n.centroid)
			return n.returnNext(nmMajor, loc)
		}
		copy(loc.X, n.vertices[dim])
		loc.X[n.fillIdx] += n.SimplexSize
		return FuncEvaluation, nil
	case nmMajor:
		// Nelder Mead iterations start with Reflection step
		return n.returnNext(nmReflected, loc)
	case nmReflected:
		n.reflectedValue = loc.F
		switch {
		case loc.F >= n.values[0] && loc.F < n.values[dim-1]:
			n.replaceWorst(loc.X, loc.F)
			return n.returnNext(nmMajor, loc)
		case loc.F < n.values[0]:
			return n.returnNext(nmExpanded, loc)
		default:
			if loc.F < n.values[dim] {
				return n.returnNext(nmContractedOutside, loc)
			}
			return n.returnNext(nmContractedInside, loc)
		}
	case nmExpanded:
		if loc.F < n.reflectedValue {
			n.replaceWorst(loc.X, loc.F)
		} else {
			n.replaceWorst(n.reflectedPoint, n.reflectedValue)
		}
		return n.returnNext(nmMajor, loc)
	case nmContractedOutside:
		if loc.F <= n.reflectedValue {
			n.replaceWorst(loc.X, loc.F)
			return n.returnNext(nmMajor, loc)
		}
		n.fillIdx = 1
		return n.returnNext(nmShrink, loc)
	case nmContractedInside:
		if loc.F < n.values[dim] {
			n.replaceWorst(loc.X, loc.F)
			return n.returnNext(nmMajor, loc)
		}
		n.fillIdx = 1
		return n.returnNext(nmShrink, loc)
	case nmShrink:
		copy(n.vertices[n.fillIdx], loc.X)
		n.values[n.fillIdx] = loc.F
		n.fillIdx++
		if n.fillIdx != dim+1 {
			return n.returnNext(nmShrink, loc)
		}
		sort.Sort(nmVertexSorter{n.vertices, n.values})
		computeCentroid(n.vertices, n.centroid)
		return n.returnNext(nmMajor, loc)
	default:
		panic("unreachable")
	}
}

// returnNext updates the location based on the iteration type and the current
// simplex, and returns the next operation.
func (n *NelderMead) returnNext(iter nmIterType, loc *Location) (Operation, error) {
	n.lastIter = iter
	switch iter {
	case nmMajor:
		// Fill loc with the current best point and value,
		// and command a convergence check.
		copy(loc.X, n.vertices[0])
		loc.F = n.values[0]
		return MajorIteration, nil
	case nmReflected, nmExpanded, nmContractedOutside, nmContractedInside:
		// x_new = x_centroid + scale * (x_centroid - x_worst)
		var scale float64
		switch iter {
		case nmReflected:
			scale = n.reflection
		case nmExpanded:
			scale = n.reflection * n.expansion
		case nmContractedOutside:
			scale = n.reflection * n.contraction
		case nmContractedInside:
			scale = -n.contraction
		}
		dim := len(loc.X)
		floats.SubTo(loc.X, n.centroid, n.vertices[dim])
		floats.Scale(scale, loc.X)
		floats.Add(loc.X, n.centroid)
		if iter == nmReflected {
			copy(n.reflectedPoint, loc.X)
		}
		return FuncEvaluation, nil
	case nmShrink:
		// x_shrink = x_best + delta * (x_i + x_best)
		floats.SubTo(loc.X, n.vertices[n.fillIdx], n.vertices[0])
		floats.Scale(n.shrink, loc.X)
		floats.Add(loc.X, n.vertices[0])
		return FuncEvaluation, nil
	default:
		panic("unreachable")
	}
}

// replaceWorst removes the worst location in the simplex and adds the new
// {x, f} pair maintaining sorting.
func (n *NelderMead) replaceWorst(x []float64, f float64) {
	dim := len(x)
	if f >= n.values[dim] {
		panic("increase in simplex value")
	}
	copy(n.vertices[dim], x)
	n.values[dim] = f

	// Sort the newly-added value.
	for i := dim - 1; i >= 0; i-- {
		if n.values[i] < f {
			break
		}
		n.vertices[i], n.vertices[i+1] = n.vertices[i+1], n.vertices[i]
		n.values[i], n.values[i+1] = n.values[i+1], n.values[i]
	}

	// Update the location of the centroid. Only one point has been replaced, so
	// subtract the worst point and add the new one.
	floats.AddScaled(n.centroid, -1/float64(dim), n.vertices[dim])
	floats.AddScaled(n.centroid, 1/float64(dim), x)
}

func (*NelderMead) Needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{false, false}
}
