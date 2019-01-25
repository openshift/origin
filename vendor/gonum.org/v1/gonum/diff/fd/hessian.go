// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"math"
	"sync"

	"gonum.org/v1/gonum/mat"
)

// Hessian approximates the Hessian matrix of the multivariate function f
// at the location x. That is
//  H_{i,j} = ∂^2 f(x)/∂x_i ∂x_j
// If dst is not nil, the resulting H will be stored in-place into dst
// and returned, otherwise a new matrix will be allocated first. Finite difference
// formula and other options are specified by settings. If settings is nil,
// the Hessian will be estimated using the Forward formula and a default step size.
//
// Hessian panics if the size of dst and x is not equal, or if the derivative
// order of the formula is not 1.
func Hessian(dst *mat.SymDense, f func(x []float64) float64, x []float64, settings *Settings) *mat.SymDense {
	n := len(x)
	if dst == nil {
		dst = mat.NewSymDense(n, nil)
	} else {
		if n2 := dst.Symmetric(); n2 != n {
			panic("hessian: dst size mismatch")
		}
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				dst.SetSym(i, j, 0)
			}
		}
	}

	// Default settings.
	formula := Forward
	step := math.Sqrt(formula.Step) // Use the sqrt because taking derivatives of derivatives.
	var originValue float64
	var originKnown, concurrent bool

	// Use user settings if provided.
	if settings != nil {
		if !settings.Formula.isZero() {
			formula = settings.Formula
			step = math.Sqrt(formula.Step)
			checkFormula(formula)
			if formula.Derivative != 1 {
				panic(badDerivOrder)
			}
		}
		if settings.Step != 0 {
			if settings.Step < 0 {
				panic(negativeStep)
			}
			step = settings.Step
		}
		originKnown = settings.OriginKnown
		originValue = settings.OriginValue
		concurrent = settings.Concurrent
	}

	evals := n * (n + 1) / 2 * len(formula.Stencil) * len(formula.Stencil)
	for _, pt := range formula.Stencil {
		if pt.Loc == 0 {
			evals -= n * (n + 1) / 2
			break
		}
	}

	nWorkers := computeWorkers(concurrent, evals)
	if nWorkers == 1 {
		hessianSerial(dst, f, x, formula.Stencil, step, originKnown, originValue)
		return dst
	}
	hessianConcurrent(dst, nWorkers, evals, f, x, formula.Stencil, step, originKnown, originValue)
	return dst
}

func hessianSerial(dst *mat.SymDense, f func(x []float64) float64, x []float64, stencil []Point, step float64, originKnown bool, originValue float64) {
	n := len(x)
	xCopy := make([]float64, n)
	fo := func() float64 {
		// Copy x in case it is modified during the call.
		copy(xCopy, x)
		return f(x)
	}
	is2 := 1 / (step * step)
	origin := getOrigin(originKnown, originValue, fo, stencil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			var hess float64
			for _, pti := range stencil {
				for _, ptj := range stencil {
					var v float64
					if pti.Loc == 0 && ptj.Loc == 0 {
						v = origin
					} else {
						// Copying the data anew has two benefits. First, it
						// avoids floating point issues where adding and then
						// subtracting the step don't return to the exact same
						// location. Secondly, it protects against the function
						// modifying the input data.
						copy(xCopy, x)
						xCopy[i] += pti.Loc * step
						xCopy[j] += ptj.Loc * step
						v = f(xCopy)
					}
					hess += v * pti.Coeff * ptj.Coeff * is2
				}
			}
			dst.SetSym(i, j, hess)
		}
	}
}

func hessianConcurrent(dst *mat.SymDense, nWorkers, evals int, f func(x []float64) float64, x []float64, stencil []Point, step float64, originKnown bool, originValue float64) {
	n := dst.Symmetric()
	type run struct {
		i, j       int
		iIdx, jIdx int
		result     float64
	}

	send := make(chan run, evals)
	ans := make(chan run, evals)

	var originWG sync.WaitGroup
	hasOrigin := usesOrigin(stencil)
	if hasOrigin {
		originWG.Add(1)
		// Launch worker to compute the origin.
		go func() {
			defer originWG.Done()
			xCopy := make([]float64, len(x))
			copy(xCopy, x)
			originValue = f(xCopy)
		}()
	}

	var workerWG sync.WaitGroup
	// Launch workers.
	for i := 0; i < nWorkers; i++ {
		workerWG.Add(1)
		go func(send <-chan run, ans chan<- run) {
			defer workerWG.Done()
			xCopy := make([]float64, len(x))
			for r := range send {
				if stencil[r.iIdx].Loc == 0 && stencil[r.jIdx].Loc == 0 {
					originWG.Wait()
					r.result = originValue
				} else {
					// See hessianSerial for comment on the copy.
					copy(xCopy, x)
					xCopy[r.i] += stencil[r.iIdx].Loc * step
					xCopy[r.j] += stencil[r.jIdx].Loc * step
					r.result = f(xCopy)
				}
				ans <- r
			}
		}(send, ans)
	}

	// Launch the distributor, which sends all of runs.
	go func(send chan<- run) {
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				for iIdx := range stencil {
					for jIdx := range stencil {
						send <- run{
							i: i, j: j, iIdx: iIdx, jIdx: jIdx,
						}
					}
				}
			}
		}
		close(send)
		// Wait for all the workers to quit, then close the ans channel.
		workerWG.Wait()
		close(ans)
	}(send)

	is2 := 1 / (step * step)
	// Read in the results.
	for r := range ans {
		v := r.result * stencil[r.iIdx].Coeff * stencil[r.jIdx].Coeff * is2
		v += dst.At(r.i, r.j)
		dst.SetSym(r.i, r.j, v)
	}
}
