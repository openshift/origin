// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import "math"

// MoreThuente is a Linesearcher that finds steps that satisfy both the
// sufficient decrease and curvature conditions (the strong Wolfe conditions).
//
// References:
//  - More, J.J. and D.J. Thuente: Line Search Algorithms with Guaranteed Sufficient
//    Decrease. ACM Transactions on Mathematical Software 20(3) (1994), 286-307
type MoreThuente struct {
	// DecreaseFactor is the constant factor in the sufficient decrease
	// (Armijo) condition.
	// It must be in the interval [0, 1). The default value is 0.
	DecreaseFactor float64
	// CurvatureFactor is the constant factor in the Wolfe conditions. Smaller
	// values result in a more exact line search.
	// A set value must be in the interval (0, 1). If it is zero, it will be
	// defaulted to 0.9.
	CurvatureFactor float64
	// StepTolerance sets the minimum acceptable width for the linesearch
	// interval. If the relative interval length is less than this value,
	// ErrLinesearcherFailure is returned.
	// It must be non-negative. If it is zero, it will be defaulted to 1e-10.
	StepTolerance float64

	// MinimumStep is the minimum step that the linesearcher will take.
	// It must be non-negative and less than MaximumStep. Defaults to no
	// minimum (a value of 0).
	MinimumStep float64
	// MaximumStep is the maximum step that the linesearcher will take.
	// It must be greater than MinimumStep. If it is zero, it will be defaulted
	// to 1e20.
	MaximumStep float64

	bracketed bool    // Indicates if a minimum has been bracketed.
	fInit     float64 // Function value at step = 0.
	gInit     float64 // Derivative value at step = 0.

	// When stage is 1, the algorithm updates the interval given by x and y
	// so that it contains a minimizer of the modified function
	//  psi(step) = f(step) - f(0) - DecreaseFactor * step * f'(0).
	// When stage is 2, the interval is updated so that it contains a minimizer
	// of f.
	stage int

	step         float64    // Current step.
	lower, upper float64    // Lower and upper bounds on the next step.
	x            float64    // Endpoint of the interval with a lower function value.
	fx, gx       float64    // Data at x.
	y            float64    // The other endpoint.
	fy, gy       float64    // Data at y.
	width        [2]float64 // Width of the interval at two previous iterations.
}

const (
	mtMinGrowthFactor float64 = 1.1
	mtMaxGrowthFactor float64 = 4
)

func (mt *MoreThuente) Init(f, g float64, step float64) Operation {
	// Based on the original Fortran code that is available, for example, from
	//  http://ftp.mcs.anl.gov/pub/MINPACK-2/csrch/
	// as part of
	//  MINPACK-2 Project. November 1993.
	//  Argonne National Laboratory and University of Minnesota.
	//  Brett M. Averick, Richard G. Carter, and Jorge J. Moré.

	if g >= 0 {
		panic("morethuente: initial derivative is non-negative")
	}
	if step <= 0 {
		panic("morethuente: invalid initial step")
	}

	if mt.CurvatureFactor == 0 {
		mt.CurvatureFactor = 0.9
	}
	if mt.StepTolerance == 0 {
		mt.StepTolerance = 1e-10
	}
	if mt.MaximumStep == 0 {
		mt.MaximumStep = 1e20
	}

	if mt.MinimumStep < 0 {
		panic("morethuente: minimum step is negative")
	}
	if mt.MaximumStep <= mt.MinimumStep {
		panic("morethuente: maximum step is not greater than minimum step")
	}
	if mt.DecreaseFactor < 0 || mt.DecreaseFactor >= 1 {
		panic("morethuente: invalid decrease factor")
	}
	if mt.CurvatureFactor <= 0 || mt.CurvatureFactor >= 1 {
		panic("morethuente: invalid curvature factor")
	}
	if mt.StepTolerance <= 0 {
		panic("morethuente: step tolerance is not positive")
	}

	if step < mt.MinimumStep {
		step = mt.MinimumStep
	}
	if step > mt.MaximumStep {
		step = mt.MaximumStep
	}

	mt.bracketed = false
	mt.stage = 1
	mt.fInit = f
	mt.gInit = g

	mt.x, mt.fx, mt.gx = 0, f, g
	mt.y, mt.fy, mt.gy = 0, f, g

	mt.lower = 0
	mt.upper = step + mtMaxGrowthFactor*step

	mt.width[0] = mt.MaximumStep - mt.MinimumStep
	mt.width[1] = 2 * mt.width[0]

	mt.step = step
	return FuncEvaluation | GradEvaluation
}

func (mt *MoreThuente) Iterate(f, g float64) (Operation, float64, error) {
	if mt.stage == 0 {
		panic("morethuente: Init has not been called")
	}

	gTest := mt.DecreaseFactor * mt.gInit
	fTest := mt.fInit + mt.step*gTest

	if mt.bracketed {
		if mt.step <= mt.lower || mt.step >= mt.upper || mt.upper-mt.lower <= mt.StepTolerance*mt.upper {
			// step contains the best step found (see below).
			return NoOperation, mt.step, ErrLinesearcherFailure
		}
	}
	if mt.step == mt.MaximumStep && f <= fTest && g <= gTest {
		return NoOperation, mt.step, ErrLinesearcherBound
	}
	if mt.step == mt.MinimumStep && (f > fTest || g >= gTest) {
		return NoOperation, mt.step, ErrLinesearcherFailure
	}

	// Test for convergence.
	if f <= fTest && math.Abs(g) <= mt.CurvatureFactor*(-mt.gInit) {
		mt.stage = 0
		return MajorIteration, mt.step, nil
	}

	if mt.stage == 1 && f <= fTest && g >= 0 {
		mt.stage = 2
	}

	if mt.stage == 1 && f <= mt.fx && f > fTest {
		// Lower function value but the decrease is not sufficient .

		// Compute values and derivatives of the modified function at step, x, y.
		fm := f - mt.step*gTest
		fxm := mt.fx - mt.x*gTest
		fym := mt.fy - mt.y*gTest
		gm := g - gTest
		gxm := mt.gx - gTest
		gym := mt.gy - gTest
		// Update x, y and step.
		mt.nextStep(fxm, gxm, fym, gym, fm, gm)
		// Recover values and derivates of the non-modified function at x and y.
		mt.fx = fxm + mt.x*gTest
		mt.fy = fym + mt.y*gTest
		mt.gx = gxm + gTest
		mt.gy = gym + gTest
	} else {
		// Update x, y and step.
		mt.nextStep(mt.fx, mt.gx, mt.fy, mt.gy, f, g)
	}

	if mt.bracketed {
		// Monitor the length of the bracketing interval. If the interval has
		// not been reduced sufficiently after two steps, use bisection to
		// force its length to zero.
		width := mt.y - mt.x
		if math.Abs(width) >= 2.0/3*mt.width[1] {
			mt.step = mt.x + 0.5*width
		}
		mt.width[0], mt.width[1] = math.Abs(width), mt.width[0]
	}

	if mt.bracketed {
		mt.lower = math.Min(mt.x, mt.y)
		mt.upper = math.Max(mt.x, mt.y)
	} else {
		mt.lower = mt.step + mtMinGrowthFactor*(mt.step-mt.x)
		mt.upper = mt.step + mtMaxGrowthFactor*(mt.step-mt.x)
	}

	// Force the step to be in [MinimumStep, MaximumStep].
	mt.step = math.Max(mt.MinimumStep, math.Min(mt.step, mt.MaximumStep))

	if mt.bracketed {
		if mt.step <= mt.lower || mt.step >= mt.upper || mt.upper-mt.lower <= mt.StepTolerance*mt.upper {
			// If further progress is not possible, set step to the best step
			// obtained during the search.
			mt.step = mt.x
		}
	}

	return FuncEvaluation | GradEvaluation, mt.step, nil
}

// nextStep computes the next safeguarded step and updates the interval that
// contains a step that satisfies the sufficient decrease and curvature
// conditions.
func (mt *MoreThuente) nextStep(fx, gx, fy, gy, f, g float64) {
	x := mt.x
	y := mt.y
	step := mt.step

	gNeg := g < 0
	if gx < 0 {
		gNeg = !gNeg
	}

	var next float64
	var bracketed bool
	switch {
	case f > fx:
		// A higher function value. The minimum is bracketed between x and step.
		// We want the next step to be closer to x because the function value
		// there is lower.

		theta := 3*(fx-f)/(step-x) + gx + g
		s := math.Max(math.Abs(gx), math.Abs(g))
		s = math.Max(s, math.Abs(theta))
		gamma := s * math.Sqrt((theta/s)*(theta/s)-(gx/s)*(g/s))
		if step < x {
			gamma *= -1
		}
		p := gamma - gx + theta
		q := gamma - gx + gamma + g
		r := p / q
		stpc := x + r*(step-x)
		stpq := x + gx/((fx-f)/(step-x)+gx)/2*(step-x)

		if math.Abs(stpc-x) < math.Abs(stpq-x) {
			// The cubic step is closer to x than the quadratic step.
			// Take the cubic step.
			next = stpc
		} else {
			// If f is much larger than fx, then the quadratic step may be too
			// close to x. Therefore heuristically take the average of the
			// cubic and quadratic steps.
			next = stpc + (stpq-stpc)/2
		}
		bracketed = true

	case gNeg:
		// A lower function value and derivatives of opposite sign. The minimum
		// is bracketed between x and step. If we choose a step that is far
		// from step, the next iteration will also likely fall in this case.

		theta := 3*(fx-f)/(step-x) + gx + g
		s := math.Max(math.Abs(gx), math.Abs(g))
		s = math.Max(s, math.Abs(theta))
		gamma := s * math.Sqrt((theta/s)*(theta/s)-(gx/s)*(g/s))
		if step > x {
			gamma *= -1
		}
		p := gamma - g + theta
		q := gamma - g + gamma + gx
		r := p / q
		stpc := step + r*(x-step)
		stpq := step + g/(g-gx)*(x-step)

		if math.Abs(stpc-step) > math.Abs(stpq-step) {
			// The cubic step is farther from x than the quadratic step.
			// Take the cubic step.
			next = stpc
		} else {
			// Take the quadratic step.
			next = stpq
		}
		bracketed = true

	case math.Abs(g) < math.Abs(gx):
		// A lower function value, derivatives of the same sign, and the
		// magnitude of the derivative decreases. Extrapolate function values
		// at x and step so that the next step lies between step and y.

		theta := 3*(fx-f)/(step-x) + gx + g
		s := math.Max(math.Abs(gx), math.Abs(g))
		s = math.Max(s, math.Abs(theta))
		gamma := s * math.Sqrt(math.Max(0, (theta/s)*(theta/s)-(gx/s)*(g/s)))
		if step > x {
			gamma *= -1
		}
		p := gamma - g + theta
		q := gamma + gx - g + gamma
		r := p / q
		var stpc float64
		switch {
		case r < 0 && gamma != 0:
			stpc = step + r*(x-step)
		case step > x:
			stpc = mt.upper
		default:
			stpc = mt.lower
		}
		stpq := step + g/(g-gx)*(x-step)

		if mt.bracketed {
			// We are extrapolating so be cautious and take the step that
			// is closer to step.
			if math.Abs(stpc-step) < math.Abs(stpq-step) {
				next = stpc
			} else {
				next = stpq
			}
			// Modify next if it is close to or beyond y.
			if step > x {
				next = math.Min(step+2.0/3*(y-step), next)
			} else {
				next = math.Max(step+2.0/3*(y-step), next)
			}
		} else {
			// Minimum has not been bracketed so take the larger step...
			if math.Abs(stpc-step) > math.Abs(stpq-step) {
				next = stpc
			} else {
				next = stpq
			}
			// ...but within reason.
			next = math.Max(mt.lower, math.Min(next, mt.upper))
		}

	default:
		// A lower function value, derivatives of the same sign, and the
		// magnitude of the derivative does not decrease. The function seems to
		// decrease rapidly in the direction of the step.

		switch {
		case mt.bracketed:
			theta := 3*(f-fy)/(y-step) + gy + g
			s := math.Max(math.Abs(gy), math.Abs(g))
			s = math.Max(s, math.Abs(theta))
			gamma := s * math.Sqrt((theta/s)*(theta/s)-(gy/s)*(g/s))
			if step > y {
				gamma *= -1
			}
			p := gamma - g + theta
			q := gamma - g + gamma + gy
			r := p / q
			next = step + r*(y-step)
		case step > x:
			next = mt.upper
		default:
			next = mt.lower
		}
	}

	if f > fx {
		// x is still the best step.
		mt.y = step
		mt.fy = f
		mt.gy = g
	} else {
		// step is the new best step.
		if gNeg {
			mt.y = x
			mt.fy = fx
			mt.gy = gx
		}
		mt.x = step
		mt.fx = f
		mt.gx = g
	}
	mt.bracketed = bracketed
	mt.step = next
}
