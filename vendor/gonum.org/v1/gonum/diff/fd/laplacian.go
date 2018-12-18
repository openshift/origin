// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import "sync"

// Laplacian computes the Laplacian of the multivariate function f at the location
// x. That is, Laplacian returns
//  ∆ f(x) = ∇ · ∇ f(x) = \sum_i ∂^2 f(x)/∂x_i^2
// The finite difference formula and other options are specified by settings.
// The order of the difference formula must be 2 or Laplacian will panic.
func Laplacian(f func(x []float64) float64, x []float64, settings *Settings) float64 {
	n := len(x)
	if n == 0 {
		panic("laplacian: x has zero length")
	}

	// Default settings.
	formula := Central2nd
	step := formula.Step
	var originValue float64
	var originKnown, concurrent bool

	// Use user settings if provided.
	if settings != nil {
		if !settings.Formula.isZero() {
			formula = settings.Formula
			step = formula.Step
			checkFormula(formula)
			if formula.Derivative != 2 {
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

	evals := n * len(formula.Stencil)
	if usesOrigin(formula.Stencil) {
		evals -= n
	}

	nWorkers := computeWorkers(concurrent, evals)
	if nWorkers == 1 {
		return laplacianSerial(f, x, formula.Stencil, step, originKnown, originValue)
	}
	return laplacianConcurrent(nWorkers, evals, f, x, formula.Stencil, step, originKnown, originValue)
}

func laplacianSerial(f func(x []float64) float64, x []float64, stencil []Point, step float64, originKnown bool, originValue float64) float64 {
	n := len(x)
	xCopy := make([]float64, n)
	fo := func() float64 {
		// Copy x in case it is modified during the call.
		copy(xCopy, x)
		return f(x)
	}
	is2 := 1 / (step * step)
	origin := getOrigin(originKnown, originValue, fo, stencil)
	var laplacian float64
	for i := 0; i < n; i++ {
		for _, pt := range stencil {
			var v float64
			if pt.Loc == 0 {
				v = origin
			} else {
				// Copying the data anew has two benefits. First, it
				// avoids floating point issues where adding and then
				// subtracting the step don't return to the exact same
				// location. Secondly, it protects against the function
				// modifying the input data.
				copy(xCopy, x)
				xCopy[i] += pt.Loc * step
				v = f(xCopy)
			}
			laplacian += v * pt.Coeff * is2
		}
	}
	return laplacian
}

func laplacianConcurrent(nWorkers, evals int, f func(x []float64) float64, x []float64, stencil []Point, step float64, originKnown bool, originValue float64) float64 {
	type run struct {
		i      int
		idx    int
		result float64
	}
	n := len(x)
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
				if stencil[r.idx].Loc == 0 {
					originWG.Wait()
					r.result = originValue
				} else {
					// See laplacianSerial for comment on the copy.
					copy(xCopy, x)
					xCopy[r.i] += stencil[r.idx].Loc * step
					r.result = f(xCopy)
				}
				ans <- r
			}
		}(send, ans)
	}

	// Launch the distributor, which sends all of runs.
	go func(send chan<- run) {
		for i := 0; i < n; i++ {
			for idx := range stencil {
				send <- run{
					i: i, idx: idx,
				}
			}
		}
		close(send)
		// Wait for all the workers to quit, then close the ans channel.
		workerWG.Wait()
		close(ans)
	}(send)

	// Read in the results.
	is2 := 1 / (step * step)
	var laplacian float64
	for r := range ans {
		laplacian += r.result * stencil[r.idx].Coeff * is2
	}
	return laplacian
}
