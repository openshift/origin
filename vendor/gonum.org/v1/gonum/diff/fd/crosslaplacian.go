// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"math"
	"sync"
)

// CrossLaplacian computes a Laplacian-like quantity for a function of two vectors
// at the locations x and y.
// It computes
//  ∇_y · ∇_x f(x,y) = \sum_i ∂^2 f(x,y)/∂x_i ∂y_i
// The two input vector lengths must be the same.
//
// Finite difference formula and other options are specified by settings. If
// settings is nil, CrossLaplacian will be estimated using the Forward formula and
// a default step size.
//
// CrossLaplacian panics if the two input vectors are not the same length, or if
// the derivative order of the formula is not 1.
func CrossLaplacian(f func(x, y []float64) float64, x, y []float64, settings *Settings) float64 {
	n := len(x)
	if n == 0 {
		panic("crosslaplacian: x has zero length")
	}
	if len(x) != len(y) {
		panic("crosslaplacian: input vector length mismatch")
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

	evals := n * len(formula.Stencil) * len(formula.Stencil)
	if usesOrigin(formula.Stencil) {
		evals -= n
	}

	nWorkers := computeWorkers(concurrent, evals)
	if nWorkers == 1 {
		return crossLaplacianSerial(f, x, y, formula.Stencil, step, originKnown, originValue)
	}
	return crossLaplacianConcurrent(nWorkers, evals, f, x, y, formula.Stencil, step, originKnown, originValue)
}

func crossLaplacianSerial(f func(x, y []float64) float64, x, y []float64, stencil []Point, step float64, originKnown bool, originValue float64) float64 {
	n := len(x)
	xCopy := make([]float64, len(x))
	yCopy := make([]float64, len(y))
	fo := func() float64 {
		// Copy x and y in case they are modified during the call.
		copy(xCopy, x)
		copy(yCopy, y)
		return f(x, y)
	}
	origin := getOrigin(originKnown, originValue, fo, stencil)

	is2 := 1 / (step * step)
	var laplacian float64
	for i := 0; i < n; i++ {
		for _, pty := range stencil {
			for _, ptx := range stencil {
				var v float64
				if ptx.Loc == 0 && pty.Loc == 0 {
					v = origin
				} else {
					// Copying the data anew has two benefits. First, it
					// avoids floating point issues where adding and then
					// subtracting the step don't return to the exact same
					// location. Secondly, it protects against the function
					// modifying the input data.
					copy(yCopy, y)
					copy(xCopy, x)
					yCopy[i] += pty.Loc * step
					xCopy[i] += ptx.Loc * step
					v = f(xCopy, yCopy)
				}
				laplacian += v * ptx.Coeff * pty.Coeff * is2
			}
		}
	}
	return laplacian
}

func crossLaplacianConcurrent(nWorkers, evals int, f func(x, y []float64) float64, x, y []float64, stencil []Point, step float64, originKnown bool, originValue float64) float64 {
	n := len(x)
	type run struct {
		i          int
		xIdx, yIdx int
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
			yCopy := make([]float64, len(y))
			copy(xCopy, x)
			copy(yCopy, y)
			originValue = f(xCopy, yCopy)
		}()
	}

	var workerWG sync.WaitGroup
	// Launch workers.
	for i := 0; i < nWorkers; i++ {
		workerWG.Add(1)
		go func(send <-chan run, ans chan<- run) {
			defer workerWG.Done()
			xCopy := make([]float64, len(x))
			yCopy := make([]float64, len(y))
			for r := range send {
				if stencil[r.xIdx].Loc == 0 && stencil[r.yIdx].Loc == 0 {
					originWG.Wait()
					r.result = originValue
				} else {
					// See crossLaplacianSerial for comment on the copy.
					copy(xCopy, x)
					copy(yCopy, y)
					xCopy[r.i] += stencil[r.xIdx].Loc * step
					yCopy[r.i] += stencil[r.yIdx].Loc * step
					r.result = f(xCopy, yCopy)
				}
				ans <- r
			}
		}(send, ans)
	}

	// Launch the distributor, which sends all of runs.
	go func(send chan<- run) {
		for i := 0; i < n; i++ {
			for xIdx := range stencil {
				for yIdx := range stencil {
					send <- run{
						i: i, xIdx: xIdx, yIdx: yIdx,
					}
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
		laplacian += r.result * stencil[r.xIdx].Coeff * stencil[r.yIdx].Coeff * is2
	}
	return laplacian
}
