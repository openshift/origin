// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"gonum.org/v1/gonum/floats"
)

const (
	initialStepFactor = 1

	quadraticMinimumStepSize = 1e-3
	quadraticMaximumStepSize = 1
	quadraticThreshold       = 1e-12

	firstOrderMinimumStepSize = quadraticMinimumStepSize
	firstOrderMaximumStepSize = quadraticMaximumStepSize
)

// ConstantStepSize is a StepSizer that returns the same step size for
// every iteration.
type ConstantStepSize struct {
	Size float64
}

func (c ConstantStepSize) Init(_ *Location, _ []float64) float64 {
	return c.Size
}

func (c ConstantStepSize) StepSize(_ *Location, _ []float64) float64 {
	return c.Size
}

// QuadraticStepSize estimates the initial line search step size as the minimum
// of a quadratic that interpolates f(x_{k-1}), f(x_k) and ∇f_k⋅p_k.
// This is useful for line search methods that do not produce well-scaled
// descent directions, such as gradient descent or conjugate gradient methods.
// The step size is bounded away from zero.
type QuadraticStepSize struct {
	// Threshold determines that the initial step size should be estimated by
	// quadratic interpolation when the relative change in the objective
	// function is larger than Threshold.  Otherwise the initial step size is
	// set to 2*previous step size.
	// If Threshold is zero, it will be set to 1e-12.
	Threshold float64
	// InitialStepFactor sets the step size for the first iteration to be InitialStepFactor / |g|_∞.
	// If InitialStepFactor is zero, it will be set to one.
	InitialStepFactor float64
	// MinStepSize is the lower bound on the estimated step size.
	// MinStepSize times GradientAbsTol should always be greater than machine epsilon.
	// If MinStepSize is zero, it will be set to 1e-3.
	MinStepSize float64
	// MaxStepSize is the upper bound on the estimated step size.
	// If MaxStepSize is zero, it will be set to 1.
	MaxStepSize float64

	fPrev        float64
	dirPrevNorm  float64
	projGradPrev float64
	xPrev        []float64
}

func (q *QuadraticStepSize) Init(loc *Location, dir []float64) (stepSize float64) {
	if q.Threshold == 0 {
		q.Threshold = quadraticThreshold
	}
	if q.InitialStepFactor == 0 {
		q.InitialStepFactor = initialStepFactor
	}
	if q.MinStepSize == 0 {
		q.MinStepSize = quadraticMinimumStepSize
	}
	if q.MaxStepSize == 0 {
		q.MaxStepSize = quadraticMaximumStepSize
	}
	if q.MaxStepSize <= q.MinStepSize {
		panic("optimize: MinStepSize not smaller than MaxStepSize")
	}

	gNorm := floats.Norm(loc.Gradient, math.Inf(1))
	stepSize = math.Max(q.MinStepSize, math.Min(q.InitialStepFactor/gNorm, q.MaxStepSize))

	q.fPrev = loc.F
	q.dirPrevNorm = floats.Norm(dir, 2)
	q.projGradPrev = floats.Dot(loc.Gradient, dir)
	q.xPrev = resize(q.xPrev, len(loc.X))
	copy(q.xPrev, loc.X)
	return stepSize
}

func (q *QuadraticStepSize) StepSize(loc *Location, dir []float64) (stepSize float64) {
	stepSizePrev := floats.Distance(loc.X, q.xPrev, 2) / q.dirPrevNorm
	projGrad := floats.Dot(loc.Gradient, dir)

	stepSize = 2 * stepSizePrev
	if !floats.EqualWithinRel(q.fPrev, loc.F, q.Threshold) {
		// Two consecutive function values are not relatively equal, so
		// computing the minimum of a quadratic interpolant might make sense

		df := (loc.F - q.fPrev) / stepSizePrev
		quadTest := df - q.projGradPrev
		if quadTest > 0 {
			// There is a chance of approximating the function well by a
			// quadratic only if the finite difference (f_k-f_{k-1})/stepSizePrev
			// is larger than ∇f_{k-1}⋅p_{k-1}

			// Set the step size to the minimizer of the quadratic function that
			// interpolates f_{k-1}, ∇f_{k-1}⋅p_{k-1} and f_k
			stepSize = -q.projGradPrev * stepSizePrev / quadTest / 2
		}
	}
	// Bound the step size to lie in [MinStepSize, MaxStepSize]
	stepSize = math.Max(q.MinStepSize, math.Min(stepSize, q.MaxStepSize))

	q.fPrev = loc.F
	q.dirPrevNorm = floats.Norm(dir, 2)
	q.projGradPrev = projGrad
	copy(q.xPrev, loc.X)
	return stepSize
}

// FirstOrderStepSize estimates the initial line search step size based on the
// assumption that the first-order change in the function will be the same as
// that obtained at the previous iteration. That is, the initial step size s^0_k
// is chosen so that
//   s^0_k ∇f_k⋅p_k = s_{k-1} ∇f_{k-1}⋅p_{k-1}
// This is useful for line search methods that do not produce well-scaled
// descent directions, such as gradient descent or conjugate gradient methods.
type FirstOrderStepSize struct {
	// InitialStepFactor sets the step size for the first iteration to be InitialStepFactor / |g|_∞.
	// If InitialStepFactor is zero, it will be set to one.
	InitialStepFactor float64
	// MinStepSize is the lower bound on the estimated step size.
	// MinStepSize times GradientAbsTol should always be greater than machine epsilon.
	// If MinStepSize is zero, it will be set to 1e-3.
	MinStepSize float64
	// MaxStepSize is the upper bound on the estimated step size.
	// If MaxStepSize is zero, it will be set to 1.
	MaxStepSize float64

	dirPrevNorm  float64
	projGradPrev float64
	xPrev        []float64
}

func (fo *FirstOrderStepSize) Init(loc *Location, dir []float64) (stepSize float64) {
	if fo.InitialStepFactor == 0 {
		fo.InitialStepFactor = initialStepFactor
	}
	if fo.MinStepSize == 0 {
		fo.MinStepSize = firstOrderMinimumStepSize
	}
	if fo.MaxStepSize == 0 {
		fo.MaxStepSize = firstOrderMaximumStepSize
	}
	if fo.MaxStepSize <= fo.MinStepSize {
		panic("optimize: MinStepSize not smaller than MaxStepSize")
	}

	gNorm := floats.Norm(loc.Gradient, math.Inf(1))
	stepSize = math.Max(fo.MinStepSize, math.Min(fo.InitialStepFactor/gNorm, fo.MaxStepSize))

	fo.dirPrevNorm = floats.Norm(dir, 2)
	fo.projGradPrev = floats.Dot(loc.Gradient, dir)
	fo.xPrev = resize(fo.xPrev, len(loc.X))
	copy(fo.xPrev, loc.X)
	return stepSize
}

func (fo *FirstOrderStepSize) StepSize(loc *Location, dir []float64) (stepSize float64) {
	stepSizePrev := floats.Distance(loc.X, fo.xPrev, 2) / fo.dirPrevNorm
	projGrad := floats.Dot(loc.Gradient, dir)

	stepSize = stepSizePrev * fo.projGradPrev / projGrad
	stepSize = math.Max(fo.MinStepSize, math.Min(stepSize, fo.MaxStepSize))

	fo.dirPrevNorm = floats.Norm(dir, 2)
	fo.projGradPrev = floats.Dot(loc.Gradient, dir)
	copy(fo.xPrev, loc.X)
	return stepSize
}
