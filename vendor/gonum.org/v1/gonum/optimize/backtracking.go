// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

const (
	defaultBacktrackingContraction = 0.5
	defaultBacktrackingDecrease    = 1e-4
	minimumBacktrackingStepSize    = 1e-20
)

// Backtracking is a Linesearcher that uses backtracking to find a point that
// satisfies the Armijo condition with the given decrease factor. If the Armijo
// condition has not been met, the step size is decreased by ContractionFactor.
//
// The Armijo condition only requires the gradient at the beginning of each
// major iteration (not at successive step locations), and so Backtracking may
// be a good linesearch for functions with expensive gradients. Backtracking is
// not appropriate for optimizers that require the Wolfe conditions to be met,
// such as BFGS.
//
// Both DecreaseFactor and ContractionFactor must be between zero and one, and
// Backtracking will panic otherwise. If either DecreaseFactor or
// ContractionFactor are zero, it will be set to a reasonable default.
type Backtracking struct {
	DecreaseFactor    float64 // Constant factor in the sufficient decrease (Armijo) condition.
	ContractionFactor float64 // Step size multiplier at each iteration (step *= ContractionFactor).

	stepSize float64
	initF    float64
	initG    float64

	lastOp Operation
}

func (b *Backtracking) Init(f, g float64, step float64) Operation {
	if step <= 0 {
		panic("backtracking: bad step size")
	}
	if g >= 0 {
		panic("backtracking: initial derivative is non-negative")
	}

	if b.ContractionFactor == 0 {
		b.ContractionFactor = defaultBacktrackingContraction
	}
	if b.DecreaseFactor == 0 {
		b.DecreaseFactor = defaultBacktrackingDecrease
	}
	if b.ContractionFactor <= 0 || b.ContractionFactor >= 1 {
		panic("backtracking: ContractionFactor must be between 0 and 1")
	}
	if b.DecreaseFactor <= 0 || b.DecreaseFactor >= 1 {
		panic("backtracking: DecreaseFactor must be between 0 and 1")
	}

	b.stepSize = step
	b.initF = f
	b.initG = g

	b.lastOp = FuncEvaluation
	return b.lastOp
}

func (b *Backtracking) Iterate(f, _ float64) (Operation, float64, error) {
	if b.lastOp != FuncEvaluation {
		panic("backtracking: Init has not been called")
	}

	if ArmijoConditionMet(f, b.initF, b.initG, b.stepSize, b.DecreaseFactor) {
		b.lastOp = MajorIteration
		return b.lastOp, b.stepSize, nil
	}
	b.stepSize *= b.ContractionFactor
	if b.stepSize < minimumBacktrackingStepSize {
		b.lastOp = NoOperation
		return b.lastOp, b.stepSize, ErrLinesearcherFailure
	}
	b.lastOp = FuncEvaluation
	return b.lastOp, b.stepSize, nil
}
