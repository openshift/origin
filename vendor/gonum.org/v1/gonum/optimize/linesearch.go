// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"gonum.org/v1/gonum/floats"
)

// LinesearchMethod represents an abstract optimization method in which a
// function is optimized through successive line search optimizations.
type LinesearchMethod struct {
	// NextDirectioner specifies the search direction of each linesearch.
	NextDirectioner NextDirectioner
	// Linesearcher performs a linesearch along the search direction.
	Linesearcher Linesearcher

	x   []float64 // Starting point for the current iteration.
	dir []float64 // Search direction for the current iteration.

	first     bool      // Indicator of the first iteration.
	nextMajor bool      // Indicates that MajorIteration must be commanded at the next call to Iterate.
	eval      Operation // Indicator of valid fields in Location.

	lastStep float64   // Step taken from x in the previous call to Iterate.
	lastOp   Operation // Operation returned from the previous call to Iterate.
}

func (ls *LinesearchMethod) Init(loc *Location) (Operation, error) {
	if loc.Gradient == nil {
		panic("linesearch: gradient is nil")
	}

	dim := len(loc.X)
	ls.x = resize(ls.x, dim)
	ls.dir = resize(ls.dir, dim)

	ls.first = true
	ls.nextMajor = false

	// Indicate that all fields of loc are valid.
	ls.eval = FuncEvaluation | GradEvaluation
	if loc.Hessian != nil {
		ls.eval |= HessEvaluation
	}

	ls.lastStep = math.NaN()
	ls.lastOp = NoOperation

	return ls.initNextLinesearch(loc)
}

func (ls *LinesearchMethod) Iterate(loc *Location) (Operation, error) {
	switch ls.lastOp {
	case NoOperation:
		// TODO(vladimir-ch): Either Init has not been called, or the caller is
		// trying to resume the optimization run after Iterate previously
		// returned with an error. Decide what is the proper thing to do. See also #125.

	case MajorIteration:
		// The previous updated location did not converge the full
		// optimization. Initialize a new Linesearch.
		return ls.initNextLinesearch(loc)

	default:
		// Update the indicator of valid fields of loc.
		ls.eval |= ls.lastOp

		if ls.nextMajor {
			ls.nextMajor = false

			// Linesearcher previously finished, and the invalid fields of loc
			// have now been validated. Announce MajorIteration.
			ls.lastOp = MajorIteration
			return ls.lastOp, nil
		}
	}

	// Continue the linesearch.

	f := math.NaN()
	if ls.eval&FuncEvaluation != 0 {
		f = loc.F
	}
	projGrad := math.NaN()
	if ls.eval&GradEvaluation != 0 {
		projGrad = floats.Dot(loc.Gradient, ls.dir)
	}
	op, step, err := ls.Linesearcher.Iterate(f, projGrad)
	if err != nil {
		return ls.error(err)
	}

	switch op {
	case MajorIteration:
		// Linesearch has been finished.

		ls.lastOp = complementEval(loc, ls.eval)
		if ls.lastOp == NoOperation {
			// loc is complete, MajorIteration can be declared directly.
			ls.lastOp = MajorIteration
		} else {
			// Declare MajorIteration on the next call to Iterate.
			ls.nextMajor = true
		}

	case FuncEvaluation, GradEvaluation, FuncEvaluation | GradEvaluation:
		if step != ls.lastStep {
			// We are moving to a new location, and not, say, evaluating extra
			// information at the current location.

			// Compute the next evaluation point and store it in loc.X.
			floats.AddScaledTo(loc.X, ls.x, step, ls.dir)
			if floats.Equal(ls.x, loc.X) {
				// Step size has become so small that the next evaluation point is
				// indistinguishable from the starting point for the current
				// iteration due to rounding errors.
				return ls.error(ErrNoProgress)
			}
			ls.lastStep = step
			ls.eval = NoOperation // Indicate all invalid fields of loc.
		}
		ls.lastOp = op

	default:
		panic("linesearch: Linesearcher returned invalid operation")
	}

	return ls.lastOp, nil
}

func (ls *LinesearchMethod) error(err error) (Operation, error) {
	ls.lastOp = NoOperation
	return ls.lastOp, err
}

// initNextLinesearch initializes the next linesearch using the previous
// complete location stored in loc. It fills loc.X and returns an evaluation
// to be performed at loc.X.
func (ls *LinesearchMethod) initNextLinesearch(loc *Location) (Operation, error) {
	copy(ls.x, loc.X)

	var step float64
	if ls.first {
		ls.first = false
		step = ls.NextDirectioner.InitDirection(loc, ls.dir)
	} else {
		step = ls.NextDirectioner.NextDirection(loc, ls.dir)
	}

	projGrad := floats.Dot(loc.Gradient, ls.dir)
	if projGrad >= 0 {
		return ls.error(ErrNonDescentDirection)
	}

	op := ls.Linesearcher.Init(loc.F, projGrad, step)
	switch op {
	case FuncEvaluation, GradEvaluation, FuncEvaluation | GradEvaluation:
	default:
		panic("linesearch: Linesearcher returned invalid operation")
	}

	floats.AddScaledTo(loc.X, ls.x, step, ls.dir)
	if floats.Equal(ls.x, loc.X) {
		// Step size is so small that the next evaluation point is
		// indistinguishable from the starting point for the current iteration
		// due to rounding errors.
		return ls.error(ErrNoProgress)
	}

	ls.lastStep = step
	ls.eval = NoOperation // Invalidate all fields of loc.

	ls.lastOp = op
	return ls.lastOp, nil
}

// ArmijoConditionMet returns true if the Armijo condition (aka sufficient
// decrease) has been met. Under normal conditions, the following should be
// true, though this is not enforced:
//  - initGrad < 0
//  - step > 0
//  - 0 < decrease < 1
func ArmijoConditionMet(currObj, initObj, initGrad, step, decrease float64) bool {
	return currObj <= initObj+decrease*step*initGrad
}

// StrongWolfeConditionsMet returns true if the strong Wolfe conditions have been met.
// The strong Wolfe conditions ensure sufficient decrease in the function
// value, and sufficient decrease in the magnitude of the projected gradient.
// Under normal conditions, the following should be true, though this is not
// enforced:
//  - initGrad < 0
//  - step > 0
//  - 0 <= decrease < curvature < 1
func StrongWolfeConditionsMet(currObj, currGrad, initObj, initGrad, step, decrease, curvature float64) bool {
	if currObj > initObj+decrease*step*initGrad {
		return false
	}
	return math.Abs(currGrad) < curvature*math.Abs(initGrad)
}

// WeakWolfeConditionsMet returns true if the weak Wolfe conditions have been met.
// The weak Wolfe conditions ensure sufficient decrease in the function value,
// and sufficient decrease in the value of the projected gradient. Under normal
// conditions, the following should be true, though this is not enforced:
//  - initGrad < 0
//  - step > 0
//  - 0 <= decrease < curvature< 1
func WeakWolfeConditionsMet(currObj, currGrad, initObj, initGrad, step, decrease, curvature float64) bool {
	if currObj > initObj+decrease*step*initGrad {
		return false
	}
	return currGrad >= curvature*initGrad
}
