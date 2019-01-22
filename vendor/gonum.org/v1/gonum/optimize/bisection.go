// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import "math"

const (
	defaultBisectionCurvature = 0.9
)

// Bisection is a Linesearcher that uses a bisection to find a point that
// satisfies the strong Wolfe conditions with the given curvature factor and
// a decrease factor of zero.
type Bisection struct {
	// CurvatureFactor is the constant factor in the curvature condition.
	// Smaller values result in a more exact line search.
	// A set value must be in the interval (0, 1), otherwise Init will panic.
	// If it is zero, it will be defaulted to 0.9.
	CurvatureFactor float64

	minStep  float64
	maxStep  float64
	currStep float64

	initF float64
	minF  float64
	maxF  float64
	lastF float64

	initGrad float64

	lastOp Operation
}

func (b *Bisection) Init(f, g float64, step float64) Operation {
	if step <= 0 {
		panic("bisection: bad step size")
	}
	if g >= 0 {
		panic("bisection: initial derivative is non-negative")
	}

	if b.CurvatureFactor == 0 {
		b.CurvatureFactor = defaultBisectionCurvature
	}
	if b.CurvatureFactor <= 0 || b.CurvatureFactor >= 1 {
		panic("bisection: CurvatureFactor not between 0 and 1")
	}

	b.minStep = 0
	b.maxStep = math.Inf(1)
	b.currStep = step

	b.initF = f
	b.minF = f
	b.maxF = math.NaN()

	b.initGrad = g

	// Only evaluate the gradient when necessary.
	b.lastOp = FuncEvaluation
	return b.lastOp
}

func (b *Bisection) Iterate(f, g float64) (Operation, float64, error) {
	if b.lastOp != FuncEvaluation && b.lastOp != GradEvaluation {
		panic("bisection: Init has not been called")
	}
	minF := b.initF
	if b.maxF < minF {
		minF = b.maxF
	}
	if b.minF < minF {
		minF = b.minF
	}
	if b.lastOp == FuncEvaluation {
		// See if the function value is good enough to make progress. If it is,
		// evaluate the gradient. If not, set it to the upper bound if the bound
		// has not yet been found, otherwise iterate toward the minimum location.
		if f <= minF {
			b.lastF = f
			b.lastOp = GradEvaluation
			return b.lastOp, b.currStep, nil
		}
		if math.IsInf(b.maxStep, 1) {
			b.maxStep = b.currStep
			b.maxF = f
			return b.nextStep((b.minStep + b.maxStep) / 2)
		}
		if b.minF <= b.maxF {
			b.maxStep = b.currStep
			b.maxF = f
		} else {
			b.minStep = b.currStep
			b.minF = f
		}
		return b.nextStep((b.minStep + b.maxStep) / 2)
	}
	f = b.lastF
	// The function value was lower. Check if this location is sufficient to
	// converge the linesearch, otherwise iterate.
	if StrongWolfeConditionsMet(f, g, minF, b.initGrad, b.currStep, 0, b.CurvatureFactor) {
		b.lastOp = MajorIteration
		return b.lastOp, b.currStep, nil
	}
	if math.IsInf(b.maxStep, 1) {
		// The function value is lower. If the gradient is positive, an upper bound
		// of the minimum been found. If the gradient is negative, search farther
		// in that direction.
		if g > 0 {
			b.maxStep = b.currStep
			b.maxF = f
			return b.nextStep((b.minStep + b.maxStep) / 2)
		}
		b.minStep = b.currStep
		b.minF = f
		return b.nextStep(b.currStep * 2)
	}
	// The interval has been bounded, and we have found a new lowest value. Use
	// the gradient to decide which direction.
	if g < 0 {
		b.minStep = b.currStep
		b.minF = f
	} else {
		b.maxStep = b.currStep
		b.maxF = f
	}
	return b.nextStep((b.minStep + b.maxStep) / 2)
}

// nextStep checks if the new step is equal to the old step.
// This can happen if min and max are the same, or if the step size is infinity,
// both of which indicate the minimization must stop. If the steps are different,
// it sets the new step size and returns the evaluation type and the step. If the steps
// are the same, it returns an error.
func (b *Bisection) nextStep(step float64) (Operation, float64, error) {
	if b.currStep == step {
		b.lastOp = NoOperation
		return b.lastOp, b.currStep, ErrLinesearcherFailure
	}
	b.currStep = step
	b.lastOp = FuncEvaluation
	return b.lastOp, b.currStep, nil
}
