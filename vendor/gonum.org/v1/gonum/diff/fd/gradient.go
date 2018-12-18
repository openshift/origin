// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import "gonum.org/v1/gonum/floats"

// Gradient estimates the gradient of the multivariate function f at the
// location x. If dst is not nil, the result will be stored in-place into dst
// and returned, otherwise a new slice will be allocated first. Finite
// difference formula and other options are specified by settings. If settings is
// nil, the gradient will be estimated using the Forward formula and a default
// step size.
//
// Gradient panics if the length of dst and x is not equal, or if the derivative
// order of the formula is not 1.
func Gradient(dst []float64, f func([]float64) float64, x []float64, settings *Settings) []float64 {
	if dst == nil {
		dst = make([]float64, len(x))
	}
	if len(dst) != len(x) {
		panic("fd: slice length mismatch")
	}

	// Default settings.
	formula := Forward
	step := formula.Step
	var originValue float64
	var originKnown, concurrent bool

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
		originKnown = settings.OriginKnown
		originValue = settings.OriginValue
		concurrent = settings.Concurrent
	}

	evals := len(formula.Stencil) * len(x)
	nWorkers := computeWorkers(concurrent, evals)

	hasOrigin := usesOrigin(formula.Stencil)
	// Copy x in case it is modified during the call.
	xcopy := make([]float64, len(x))
	if hasOrigin && !originKnown {
		copy(xcopy, x)
		originValue = f(xcopy)
	}

	if nWorkers == 1 {
		for i := range xcopy {
			var deriv float64
			for _, pt := range formula.Stencil {
				if pt.Loc == 0 {
					deriv += pt.Coeff * originValue
					continue
				}
				// Copying the data anew has two benefits. First, it
				// avoids floating point issues where adding and then
				// subtracting the step don't return to the exact same
				// location. Secondly, it protects against the function
				// modifying the input data.
				copy(xcopy, x)
				xcopy[i] += pt.Loc * step
				deriv += pt.Coeff * f(xcopy)
			}
			dst[i] = deriv / step
		}
		return dst
	}

	sendChan := make(chan fdrun, evals)
	ansChan := make(chan fdrun, evals)
	quit := make(chan struct{})
	defer close(quit)

	// Launch workers. Workers receive an index and a step, and compute the answer.
	for i := 0; i < nWorkers; i++ {
		go func(sendChan <-chan fdrun, ansChan chan<- fdrun, quit <-chan struct{}) {
			xcopy := make([]float64, len(x))
			for {
				select {
				case <-quit:
					return
				case run := <-sendChan:
					// See above comment on the copy.
					copy(xcopy, x)
					xcopy[run.idx] += run.pt.Loc * step
					run.result = f(xcopy)
					ansChan <- run
				}
			}
		}(sendChan, ansChan, quit)
	}

	// Launch the distributor. Distributor sends the cases to be computed.
	go func(sendChan chan<- fdrun, ansChan chan<- fdrun) {
		for i := range x {
			for _, pt := range formula.Stencil {
				if pt.Loc == 0 {
					// Answer already known. Send the answer on the answer channel.
					ansChan <- fdrun{
						idx:    i,
						pt:     pt,
						result: originValue,
					}
					continue
				}
				// Answer not known, send the answer to be computed.
				sendChan <- fdrun{
					idx: i,
					pt:  pt,
				}
			}
		}
	}(sendChan, ansChan)

	for i := range dst {
		dst[i] = 0
	}
	// Read in all of the results.
	for i := 0; i < evals; i++ {
		run := <-ansChan
		dst[run.idx] += run.pt.Coeff * run.result
	}
	floats.Scale(1/step, dst)
	return dst
}

type fdrun struct {
	idx    int
	pt     Point
	result float64
}
