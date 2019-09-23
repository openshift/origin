// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

const (
	nonpositiveDimension string = "optimize: non-positive input dimension"
	negativeTasks        string = "optimize: negative input number of tasks"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// newLocation allocates a new locatian structure of the appropriate size. It
// allocates memory based on the dimension and the values in Needs.
func newLocation(dim int, method Needser) *Location {
	// TODO(btracey): combine this with Local.
	loc := &Location{
		X: make([]float64, dim),
	}
	if method.Needs().Gradient {
		loc.Gradient = make([]float64, dim)
	}
	if method.Needs().Hessian {
		loc.Hessian = mat.NewSymDense(dim, nil)
	}
	return loc
}

func copyLocation(dst, src *Location) {
	dst.X = resize(dst.X, len(src.X))
	copy(dst.X, src.X)

	dst.F = src.F

	dst.Gradient = resize(dst.Gradient, len(src.Gradient))
	copy(dst.Gradient, src.Gradient)

	if src.Hessian != nil {
		if dst.Hessian == nil || dst.Hessian.Symmetric() != len(src.X) {
			dst.Hessian = mat.NewSymDense(len(src.X), nil)
		}
		dst.Hessian.CopySym(src.Hessian)
	}
}

// getInitLocation checks the validity of initLocation and initOperation and
// returns the initial values as a *Location.
func getInitLocation(dim int, initX []float64, initValues *Location, method Needser) (Operation, *Location) {
	needs := method.Needs()
	loc := newLocation(dim, method)
	if initX == nil {
		if initValues != nil {
			panic("optimize: initValues is non-nil but no initial location specified")
		}
		return NoOperation, loc
	}
	copy(loc.X, initX)
	if initValues == nil {
		return NoOperation, loc
	} else {
		if initValues.X != nil {
			panic("optimize: location specified in InitValues (only use InitX)")
		}
	}
	loc.F = initValues.F
	op := FuncEvaluation
	if initValues.Gradient != nil {
		if len(initValues.Gradient) != dim {
			panic("optimize: initial gradient does not match problem dimension")
		}
		if needs.Gradient {
			copy(loc.Gradient, initValues.Gradient)
			op |= GradEvaluation
		}
	}
	if initValues.Hessian != nil {
		if initValues.Hessian.Symmetric() != dim {
			panic("optimize: initial Hessian does not match problem dimension")
		}
		if needs.Hessian {
			loc.Hessian.CopySym(initValues.Hessian)
			op |= HessEvaluation
		}
	}
	return op, loc
}

func checkOptimization(p Problem, dim int, method Needser, recorder Recorder) error {
	if p.Func == nil {
		panic(badProblem)
	}
	if dim <= 0 {
		panic("optimize: impossible problem dimension")
	}
	if err := p.satisfies(method); err != nil {
		return err
	}
	if p.Status != nil {
		_, err := p.Status()
		if err != nil {
			return err
		}
	}
	if recorder != nil {
		err := recorder.Init()
		if err != nil {
			return err
		}
	}
	return nil
}

// evaluate evaluates the routines specified by the Operation at loc.X, and stores
// the answer into loc. loc.X is copied into x before evaluating in order to
// prevent the routines from modifying it.
func evaluate(p *Problem, loc *Location, op Operation, x []float64) {
	if !op.isEvaluation() {
		panic(fmt.Sprintf("optimize: invalid evaluation %v", op))
	}
	copy(x, loc.X)
	if op&FuncEvaluation != 0 {
		loc.F = p.Func(x)
	}
	if op&GradEvaluation != 0 {
		p.Grad(loc.Gradient, x)
	}
	if op&HessEvaluation != 0 {
		p.Hess(loc.Hessian, x)
	}
}

// updateEvaluationStats updates the statistics based on the operation.
func updateEvaluationStats(stats *Stats, op Operation) {
	if op&FuncEvaluation != 0 {
		stats.FuncEvaluations++
	}
	if op&GradEvaluation != 0 {
		stats.GradEvaluations++
	}
	if op&HessEvaluation != 0 {
		stats.HessEvaluations++
	}
}

// checkLocationConvergence checks if the current optimal location satisfies
// any of the convergence criteria based on the function location.
//
// checkLocationConvergence returns NotTerminated if the Location does not satisfy
// the convergence criteria given by settings. Otherwise a corresponding status is
// returned.
// Unlike checkLimits, checkConvergence is called only at MajorIterations.
func checkLocationConvergence(loc *Location, settings *Settings) Status {
	if math.IsInf(loc.F, -1) {
		return FunctionNegativeInfinity
	}
	if loc.Gradient != nil {
		norm := floats.Norm(loc.Gradient, math.Inf(1))
		if norm < settings.GradientThreshold {
			return GradientThreshold
		}
	}
	if loc.F < settings.FunctionThreshold {
		return FunctionThreshold
	}
	if settings.FunctionConverge != nil {
		return settings.FunctionConverge.FunctionConverged(loc.F)
	}
	return NotTerminated
}

// checkEvaluationLimits checks the optimization limits after an evaluation
// Operation. It checks the number of evaluations (of various kinds) and checks
// the status of the Problem, if applicable.
func checkEvaluationLimits(p *Problem, stats *Stats, settings *Settings) (Status, error) {
	if p.Status != nil {
		status, err := p.Status()
		if err != nil || status != NotTerminated {
			return status, err
		}
	}
	if settings.FuncEvaluations > 0 && stats.FuncEvaluations >= settings.FuncEvaluations {
		return FunctionEvaluationLimit, nil
	}
	if settings.GradEvaluations > 0 && stats.GradEvaluations >= settings.GradEvaluations {
		return GradientEvaluationLimit, nil
	}
	if settings.HessEvaluations > 0 && stats.HessEvaluations >= settings.HessEvaluations {
		return HessianEvaluationLimit, nil
	}
	return NotTerminated, nil
}

// checkIterationLimits checks the limits on iterations affected by MajorIteration.
func checkIterationLimits(loc *Location, stats *Stats, settings *Settings) Status {
	if settings.MajorIterations > 0 && stats.MajorIterations >= settings.MajorIterations {
		return IterationLimit
	}
	if settings.Runtime > 0 && stats.Runtime >= settings.Runtime {
		return RuntimeLimit
	}
	return NotTerminated
}

// performMajorIteration does all of the steps needed to perform a MajorIteration.
// It increments the iteration count, updates the optimal location, and checks
// the necessary convergence criteria.
func performMajorIteration(optLoc, loc *Location, stats *Stats, startTime time.Time, settings *Settings) Status {
	copyLocation(optLoc, loc)
	stats.MajorIterations++
	stats.Runtime = time.Since(startTime)
	status := checkLocationConvergence(optLoc, settings)
	if status != NotTerminated {
		return status
	}
	return checkIterationLimits(optLoc, stats, settings)
}
