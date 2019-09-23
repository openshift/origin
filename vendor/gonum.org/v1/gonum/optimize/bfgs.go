// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// BFGS implements the Broyden–Fletcher–Goldfarb–Shanno optimization method. It
// is a quasi-Newton method that performs successive rank-one updates to an
// estimate of the inverse Hessian of the objective function. It exhibits
// super-linear convergence when in proximity to a local minimum. It has memory
// cost that is O(n^2) relative to the input dimension.
type BFGS struct {
	// Linesearcher selects suitable steps along the descent direction.
	// Accepted steps should satisfy the strong Wolfe conditions.
	// If Linesearcher == nil, an appropriate default is chosen.
	Linesearcher Linesearcher

	ls *LinesearchMethod

	status Status
	err    error

	dim  int
	x    mat.VecDense // Location of the last major iteration.
	grad mat.VecDense // Gradient at the last major iteration.
	s    mat.VecDense // Difference between locations in this and the previous iteration.
	y    mat.VecDense // Difference between gradients in this and the previous iteration.
	tmp  mat.VecDense

	invHess *mat.SymDense

	first bool // Indicator of the first iteration.
}

func (b *BFGS) Status() (Status, error) {
	return b.status, b.err
}

func (b *BFGS) Init(dim, tasks int) int {
	b.status = NotTerminated
	b.err = nil
	return 1
}

func (b *BFGS) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	b.status, b.err = localOptimizer{}.run(b, operation, result, tasks)
	close(operation)
	return
}

func (b *BFGS) initLocal(loc *Location) (Operation, error) {
	if b.Linesearcher == nil {
		b.Linesearcher = &Bisection{}
	}
	if b.ls == nil {
		b.ls = &LinesearchMethod{}
	}
	b.ls.Linesearcher = b.Linesearcher
	b.ls.NextDirectioner = b

	return b.ls.Init(loc)
}

func (b *BFGS) iterateLocal(loc *Location) (Operation, error) {
	return b.ls.Iterate(loc)
}

func (b *BFGS) InitDirection(loc *Location, dir []float64) (stepSize float64) {
	dim := len(loc.X)
	b.dim = dim
	b.first = true

	x := mat.NewVecDense(dim, loc.X)
	grad := mat.NewVecDense(dim, loc.Gradient)
	b.x.CloneVec(x)
	b.grad.CloneVec(grad)

	b.y.Reset()
	b.s.Reset()
	b.tmp.Reset()

	if b.invHess == nil || cap(b.invHess.RawSymmetric().Data) < dim*dim {
		b.invHess = mat.NewSymDense(dim, nil)
	} else {
		b.invHess = mat.NewSymDense(dim, b.invHess.RawSymmetric().Data[:dim*dim])
	}
	// The values of the inverse Hessian are initialized in the first call to
	// NextDirection.

	// Initial direction is just negative of the gradient because the Hessian
	// is an identity matrix.
	d := mat.NewVecDense(dim, dir)
	d.ScaleVec(-1, grad)
	return 1 / mat.Norm(d, 2)
}

func (b *BFGS) NextDirection(loc *Location, dir []float64) (stepSize float64) {
	dim := b.dim
	if len(loc.X) != dim {
		panic("bfgs: unexpected size mismatch")
	}
	if len(loc.Gradient) != dim {
		panic("bfgs: unexpected size mismatch")
	}
	if len(dir) != dim {
		panic("bfgs: unexpected size mismatch")
	}

	x := mat.NewVecDense(dim, loc.X)
	grad := mat.NewVecDense(dim, loc.Gradient)

	// s = x_{k+1} - x_{k}
	b.s.SubVec(x, &b.x)
	// y = g_{k+1} - g_{k}
	b.y.SubVec(grad, &b.grad)

	sDotY := mat.Dot(&b.s, &b.y)

	if b.first {
		// Rescale the initial Hessian.
		// From: Nocedal, J., Wright, S.: Numerical Optimization (2nd ed).
		//       Springer (2006), page 143, eq. 6.20.
		yDotY := mat.Dot(&b.y, &b.y)
		scale := sDotY / yDotY
		for i := 0; i < dim; i++ {
			for j := i; j < dim; j++ {
				if i == j {
					b.invHess.SetSym(i, i, scale)
				} else {
					b.invHess.SetSym(i, j, 0)
				}
			}
		}
		b.first = false
	}

	if math.Abs(sDotY) != 0 {
		// Update the inverse Hessian according to the formula
		//
		//  B_{k+1}^-1 = B_k^-1
		//             + (s_k^T y_k + y_k^T B_k^-1 y_k) / (s_k^T y_k)^2 * (s_k s_k^T)
		//             - (B_k^-1 y_k s_k^T + s_k y_k^T B_k^-1) / (s_k^T y_k).
		//
		// Note that y_k^T B_k^-1 y_k is a scalar, and that the third term is a
		// rank-two update where B_k^-1 y_k is one vector and s_k is the other.
		yBy := mat.Inner(&b.y, b.invHess, &b.y)
		b.tmp.MulVec(b.invHess, &b.y)
		scale := (1 + yBy/sDotY) / sDotY
		b.invHess.SymRankOne(b.invHess, scale, &b.s)
		b.invHess.RankTwo(b.invHess, -1/sDotY, &b.tmp, &b.s)
	}

	// Update the stored BFGS data.
	b.x.CopyVec(x)
	b.grad.CopyVec(grad)

	// New direction is stored in dir.
	d := mat.NewVecDense(dim, dir)
	d.MulVec(b.invHess, grad)
	d.ScaleVec(-1, d)

	return 1
}

func (*BFGS) Needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{true, false}
}
