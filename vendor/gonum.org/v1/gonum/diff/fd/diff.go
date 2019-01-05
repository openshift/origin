// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"math"
	"runtime"
)

// A Point is a stencil location in a finite difference formula.
type Point struct {
	Loc   float64
	Coeff float64
}

// Formula represents a finite difference formula on a regularly spaced grid
// that approximates the derivative of order k of a function f at x as
//  d^k f(x) ≈ (1 / Step^k) * \sum_i Coeff_i * f(x + Step * Loc_i).
// Step must be positive, or the finite difference formula will panic.
type Formula struct {
	// Stencil is the set of sampling Points which are used to estimate the
	// derivative. The locations will be scaled by Step and are relative to x.
	Stencil    []Point
	Derivative int     // The order of the approximated derivative.
	Step       float64 // Default step size for the formula.
}

func (f Formula) isZero() bool {
	return f.Stencil == nil && f.Derivative == 0 && f.Step == 0
}

// Settings is the settings structure for computing finite differences.
type Settings struct {
	// Formula is the finite difference formula used
	// for approximating the derivative.
	// Zero value indicates a default formula.
	Formula Formula
	// Step is the distance between points of the stencil.
	// If equal to 0, formula's default step will be used.
	Step float64

	OriginKnown bool    // Flag that the value at the origin x is known.
	OriginValue float64 // Value at the origin (only used if OriginKnown is true).

	Concurrent bool // Should the function calls be executed concurrently.
}

// Forward represents a first-order accurate forward approximation
// to the first derivative.
var Forward = Formula{
	Stencil:    []Point{{Loc: 0, Coeff: -1}, {Loc: 1, Coeff: 1}},
	Derivative: 1,
	Step:       2e-8,
}

// Forward2nd represents a first-order accurate forward approximation
// to the second derivative.
var Forward2nd = Formula{
	Stencil:    []Point{{Loc: 0, Coeff: 1}, {Loc: 1, Coeff: -2}, {Loc: 2, Coeff: 1}},
	Derivative: 2,
	Step:       1e-4,
}

// Backward represents a first-order accurate backward approximation
// to the first derivative.
var Backward = Formula{
	Stencil:    []Point{{Loc: -1, Coeff: -1}, {Loc: 0, Coeff: 1}},
	Derivative: 1,
	Step:       2e-8,
}

// Backward2nd represents a first-order accurate forward approximation
// to the second derivative.
var Backward2nd = Formula{
	Stencil:    []Point{{Loc: 0, Coeff: 1}, {Loc: -1, Coeff: -2}, {Loc: -2, Coeff: 1}},
	Derivative: 2,
	Step:       1e-4,
}

// Central represents a second-order accurate centered approximation
// to the first derivative.
var Central = Formula{
	Stencil:    []Point{{Loc: -1, Coeff: -0.5}, {Loc: 1, Coeff: 0.5}},
	Derivative: 1,
	Step:       6e-6,
}

// Central2nd represents a secord-order accurate centered approximation
// to the second derivative.
var Central2nd = Formula{
	Stencil:    []Point{{Loc: -1, Coeff: 1}, {Loc: 0, Coeff: -2}, {Loc: 1, Coeff: 1}},
	Derivative: 2,
	Step:       1e-4,
}

var negativeStep = "fd: negative step"

// checkFormula checks if the formula is valid, and panics otherwise.
func checkFormula(formula Formula) {
	if formula.Derivative == 0 || formula.Stencil == nil || formula.Step <= 0 {
		panic("fd: bad formula")
	}
}

// computeWorkers returns the desired number of workers given the concurrency
// level and number of evaluations.
func computeWorkers(concurrent bool, evals int) int {
	if !concurrent {
		return 1
	}
	nWorkers := runtime.GOMAXPROCS(0)
	if nWorkers > evals {
		nWorkers = evals
	}
	return nWorkers
}

// usesOrigin returns whether the stencil uses the origin, which is true iff
// one of the locations in the stencil equals 0.
func usesOrigin(stencil []Point) bool {
	for _, pt := range stencil {
		if pt.Loc == 0 {
			return true
		}
	}
	return false
}

// getOrigin returns the value at the origin. It returns originValue if originKnown
// is true. It returns the value returned by f if stencil contains a point with
// zero location, and NaN otherwise.
func getOrigin(originKnown bool, originValue float64, f func() float64, stencil []Point) float64 {
	if originKnown {
		return originValue
	}
	for _, pt := range stencil {
		if pt.Loc == 0 {
			return f()
		}
	}
	return math.NaN()
}

const (
	badDerivOrder = "fd: invalid derivative order"
)
