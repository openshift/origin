// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"gonum.org/v1/gonum/floats"
)

// LBFGS implements the limited-memory BFGS method for gradient-based
// unconstrained minimization.
//
// It stores a modified version of the inverse Hessian approximation H
// implicitly from the last Store iterations while the normal BFGS method
// stores and manipulates H directly as a dense matrix. Therefore LBFGS is more
// appropriate than BFGS for large problems as the cost of LBFGS scales as
// O(Store * dim) while BFGS scales as O(dim^2). The "forgetful" nature of
// LBFGS may also make it perform better than BFGS for functions with Hessians
// that vary rapidly spatially.
type LBFGS struct {
	// Linesearcher selects suitable steps along the descent direction.
	// Accepted steps should satisfy the strong Wolfe conditions.
	// If Linesearcher is nil, a reasonable default will be chosen.
	Linesearcher Linesearcher
	// Store is the size of the limited-memory storage.
	// If Store is 0, it will be defaulted to 15.
	Store int

	status Status
	err    error

	ls *LinesearchMethod

	dim  int       // Dimension of the problem
	x    []float64 // Location at the last major iteration
	grad []float64 // Gradient at the last major iteration

	// History
	oldest int         // Index of the oldest element of the history
	y      [][]float64 // Last Store values of y
	s      [][]float64 // Last Store values of s
	rho    []float64   // Last Store values of rho
	a      []float64   // Cache of Hessian updates
}

func (l *LBFGS) Status() (Status, error) {
	return l.status, l.err
}

func (l *LBFGS) Init(dim, tasks int) int {
	l.status = NotTerminated
	l.err = nil
	return 1
}

func (l *LBFGS) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	l.status, l.err = localOptimizer{}.run(l, operation, result, tasks)
	close(operation)
	return
}

func (l *LBFGS) initLocal(loc *Location) (Operation, error) {
	if l.Linesearcher == nil {
		l.Linesearcher = &Bisection{}
	}
	if l.Store == 0 {
		l.Store = 15
	}

	if l.ls == nil {
		l.ls = &LinesearchMethod{}
	}
	l.ls.Linesearcher = l.Linesearcher
	l.ls.NextDirectioner = l

	return l.ls.Init(loc)
}

func (l *LBFGS) iterateLocal(loc *Location) (Operation, error) {
	return l.ls.Iterate(loc)
}

func (l *LBFGS) InitDirection(loc *Location, dir []float64) (stepSize float64) {
	dim := len(loc.X)
	l.dim = dim
	l.oldest = 0

	l.a = resize(l.a, l.Store)
	l.rho = resize(l.rho, l.Store)
	l.y = l.initHistory(l.y)
	l.s = l.initHistory(l.s)

	l.x = resize(l.x, dim)
	copy(l.x, loc.X)

	l.grad = resize(l.grad, dim)
	copy(l.grad, loc.Gradient)

	copy(dir, loc.Gradient)
	floats.Scale(-1, dir)
	return 1 / floats.Norm(dir, 2)
}

func (l *LBFGS) initHistory(hist [][]float64) [][]float64 {
	c := cap(hist)
	if c < l.Store {
		n := make([][]float64, l.Store-c)
		hist = append(hist[:c], n...)
	}
	hist = hist[:l.Store]
	for i := range hist {
		hist[i] = resize(hist[i], l.dim)
		for j := range hist[i] {
			hist[i][j] = 0
		}
	}
	return hist
}

func (l *LBFGS) NextDirection(loc *Location, dir []float64) (stepSize float64) {
	// Uses two-loop correction as described in
	// Nocedal, J., Wright, S.: Numerical Optimization (2nd ed). Springer (2006), chapter 7, page 178.

	if len(loc.X) != l.dim {
		panic("lbfgs: unexpected size mismatch")
	}
	if len(loc.Gradient) != l.dim {
		panic("lbfgs: unexpected size mismatch")
	}
	if len(dir) != l.dim {
		panic("lbfgs: unexpected size mismatch")
	}

	y := l.y[l.oldest]
	floats.SubTo(y, loc.Gradient, l.grad)
	s := l.s[l.oldest]
	floats.SubTo(s, loc.X, l.x)
	sDotY := floats.Dot(s, y)
	l.rho[l.oldest] = 1 / sDotY

	l.oldest = (l.oldest + 1) % l.Store

	copy(l.x, loc.X)
	copy(l.grad, loc.Gradient)
	copy(dir, loc.Gradient)

	// Start with the most recent element and go backward,
	for i := 0; i < l.Store; i++ {
		idx := l.oldest - i - 1
		if idx < 0 {
			idx += l.Store
		}
		l.a[idx] = l.rho[idx] * floats.Dot(l.s[idx], dir)
		floats.AddScaled(dir, -l.a[idx], l.y[idx])
	}

	// Scale the initial Hessian.
	gamma := sDotY / floats.Dot(y, y)
	floats.Scale(gamma, dir)

	// Start with the oldest element and go forward.
	for i := 0; i < l.Store; i++ {
		idx := i + l.oldest
		if idx >= l.Store {
			idx -= l.Store
		}
		beta := l.rho[idx] * floats.Dot(l.y[idx], dir)
		floats.AddScaled(dir, l.a[idx]-beta, l.s[idx])
	}

	// dir contains H^{-1} * g, so flip the direction for minimization.
	floats.Scale(-1, dir)

	return 1
}

func (*LBFGS) Needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{true, false}
}
