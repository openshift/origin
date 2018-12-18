// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"sync"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

type JacobianSettings struct {
	Formula     Formula
	OriginValue []float64
	Step        float64
	Concurrent  bool
}

// Jacobian approximates the Jacobian matrix of a vector-valued function f at
// the location x and stores the result in-place into dst.
//
// Finite difference formula and other options are specified by settings. If
// settings is nil, the Jacobian will be estimated using the Forward formula and
// a default step size.
//
// The Jacobian matrix J is the matrix of all first-order partial derivatives of f.
// If f maps an n-dimensional vector x to an m-dimensional vector y = f(x), J is
// an m×n matrix whose elements are given as
//  J_{i,j} = ∂f_i/∂x_j,
// or expanded out
//      [ ∂f_1/∂x_1 ... ∂f_1/∂x_n ]
//      [     .  .          .     ]
//  J = [     .      .      .     ]
//      [     .          .  .     ]
//      [ ∂f_m/∂x_1 ... ∂f_m/∂x_n ]
//
// dst must be non-nil, the number of its columns must equal the length of x, and
// the derivative order of the formula must be 1, otherwise Jacobian will panic.
func Jacobian(dst *mat.Dense, f func(y, x []float64), x []float64, settings *JacobianSettings) {
	n := len(x)
	if n == 0 {
		panic("jacobian: x has zero length")
	}
	m, c := dst.Dims()
	if c != n {
		panic("jacobian: mismatched matrix size")
	}

	// Default settings.
	formula := Forward
	step := formula.Step
	var originValue []float64
	var concurrent bool

	// Use user settings if provided.
	if settings != nil {
		if !settings.Formula.isZero() {
			formula = settings.Formula
			step = formula.Step
			checkFormula(formula)
			if formula.Derivative != 1 {
				panic(badDerivOrder)
			}
		}
		if settings.Step != 0 {
			step = settings.Step
		}
		originValue = settings.OriginValue
		if originValue != nil && len(originValue) != m {
			panic("jacobian: mismatched OriginValue slice length")
		}
		concurrent = settings.Concurrent
	}

	evals := n * len(formula.Stencil)
	for _, pt := range formula.Stencil {
		if pt.Loc == 0 {
			evals -= n - 1
			break
		}
	}

	nWorkers := computeWorkers(concurrent, evals)
	if nWorkers == 1 {
		jacobianSerial(dst, f, x, originValue, formula, step)
		return
	}
	jacobianConcurrent(dst, f, x, originValue, formula, step, nWorkers)
}

func jacobianSerial(dst *mat.Dense, f func([]float64, []float64), x, origin []float64, formula Formula, step float64) {
	m, n := dst.Dims()
	xcopy := make([]float64, n)
	y := make([]float64, m)
	col := make([]float64, m)
	for j := 0; j < n; j++ {
		for i := range col {
			col[i] = 0
		}
		for _, pt := range formula.Stencil {
			if pt.Loc == 0 {
				if origin == nil {
					origin = make([]float64, m)
					copy(xcopy, x)
					f(origin, xcopy)
				}
				floats.AddScaled(col, pt.Coeff, origin)
			} else {
				copy(xcopy, x)
				xcopy[j] += pt.Loc * step
				f(y, xcopy)
				floats.AddScaled(col, pt.Coeff, y)
			}
		}
		dst.SetCol(j, col)
	}
	dst.Scale(1/step, dst)
}

func jacobianConcurrent(dst *mat.Dense, f func([]float64, []float64), x, origin []float64, formula Formula, step float64, nWorkers int) {
	m, n := dst.Dims()
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			dst.Set(i, j, 0)
		}
	}

	var (
		wg sync.WaitGroup
		mu = make([]sync.Mutex, n) // Guard access to individual columns.
	)
	worker := func(jobs <-chan jacJob) {
		defer wg.Done()
		xcopy := make([]float64, n)
		y := make([]float64, m)
		yVec := mat.NewVecDense(m, y)
		var col mat.VecDense
		for job := range jobs {
			copy(xcopy, x)
			xcopy[job.j] += job.pt.Loc * step
			f(y, xcopy)
			col.ColViewOf(dst, job.j)
			mu[job.j].Lock()
			col.AddScaledVec(&col, job.pt.Coeff, yVec)
			mu[job.j].Unlock()
		}
	}
	jobs := make(chan jacJob, nWorkers)
	for i := 0; i < nWorkers; i++ {
		wg.Add(1)
		go worker(jobs)
	}
	var hasOrigin bool
	for _, pt := range formula.Stencil {
		if pt.Loc == 0 {
			hasOrigin = true
			continue
		}
		for j := 0; j < n; j++ {
			jobs <- jacJob{j, pt}
		}
	}
	close(jobs)
	if hasOrigin && origin == nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			origin = make([]float64, m)
			xcopy := make([]float64, n)
			copy(xcopy, x)
			f(origin, xcopy)
		}()
	}
	wg.Wait()

	if hasOrigin {
		// The formula evaluated at x, we need to add scaled origin to
		// all columns of dst. Iterate again over all Formula points
		// because we don't forbid repeated locations.

		originVec := mat.NewVecDense(m, origin)
		for _, pt := range formula.Stencil {
			if pt.Loc != 0 {
				continue
			}
			var col mat.VecDense
			for j := 0; j < n; j++ {
				col.ColViewOf(dst, j)
				col.AddScaledVec(&col, pt.Coeff, originVec)
			}
		}
	}

	dst.Scale(1/step, dst)
}

type jacJob struct {
	j  int
	pt Point
}
