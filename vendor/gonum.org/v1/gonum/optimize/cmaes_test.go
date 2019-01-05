// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"errors"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/functions"
)

type cmaTestCase struct {
	dim      int
	problem  Problem
	method   *CmaEsChol
	initX    []float64
	settings *Settings
	good     func(result *Result, err error, concurrent int) error
}

func cmaTestCases() []cmaTestCase {
	localMinMean := []float64{2.2, -2.2}
	s := mat.NewSymDense(2, []float64{0.01, 0, 0, 0.01})
	var localMinChol mat.Cholesky
	localMinChol.Factorize(s)
	return []cmaTestCase{
		{
			// Test that can find a small value.
			dim: 10,
			problem: Problem{
				Func: functions.ExtendedRosenbrock{}.Func,
			},
			method: &CmaEsChol{
				StopLogDet: math.NaN(),
			},
			settings: &Settings{
				FunctionThreshold: 0.01,
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != FunctionThreshold {
					return errors.New("result not function threshold")
				}
				if result.F > 0.01 {
					return errors.New("result not sufficiently small")
				}
				return nil
			},
		},
		{
			// Test that can stop when the covariance gets small.
			// For this case, also test that it is really at a minimum.
			dim: 2,
			problem: Problem{
				Func: functions.ExtendedRosenbrock{}.Func,
			},
			method: &CmaEsChol{},
			settings: &Settings{
				FunctionThreshold: math.Inf(-1),
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != MethodConverge {
					return errors.New("result not method converge")
				}
				if result.F > 1e-12 {
					return errors.New("minimum not found")
				}
				return nil
			},
		},
		{
			// Test that population works properly and it stops after a certain
			// number of iterations.
			dim: 3,
			problem: Problem{
				Func: functions.ExtendedRosenbrock{}.Func,
			},
			method: &CmaEsChol{
				Population: 100,
				ForgetBest: true, // Otherwise may get an update at the end.
			},
			settings: &Settings{
				FunctionThreshold: math.Inf(-1),
				MajorIterations:   10,
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != IterationLimit {
					return errors.New("result not iteration limit")
				}
				threshLower := 10
				threshUpper := 10
				if concurrent != 0 {
					// Could have one more from final update.
					threshUpper++
				}
				if result.MajorIterations < threshLower || result.MajorIterations > threshUpper {
					return errors.New("wrong number of iterations")
				}
				return nil
			},
		},
		{
			// Test that work stops with some number of function evaluations.
			dim: 5,
			problem: Problem{
				Func: functions.ExtendedRosenbrock{}.Func,
			},
			method: &CmaEsChol{
				Population: 100,
			},
			settings: &Settings{
				FunctionThreshold: math.Inf(-1),
				FuncEvaluations:   250, // Somewhere in the middle of an iteration.
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != FunctionEvaluationLimit {
					return errors.New("result not function evaluations")
				}
				threshLower := 250
				threshUpper := 251
				if concurrent != 0 {
					threshUpper = threshLower + concurrent
				}
				if result.FuncEvaluations < threshLower {
					return errors.New("too few function evaluations")
				}
				if result.FuncEvaluations > threshUpper {
					return errors.New("too many function evaluations")
				}
				return nil
			},
		},
		{
			// Test that the global minimum is found with the right initialization.
			dim: 2,
			problem: Problem{
				Func: functions.Rastrigin{}.Func,
			},
			method: &CmaEsChol{
				Population: 100, // Increase the population size to reduce noise.
			},
			settings: &Settings{
				FunctionThreshold: math.Inf(-1),
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != MethodConverge {
					return errors.New("result not method converge")
				}
				if !floats.EqualApprox(result.X, []float64{0, 0}, 1e-6) {
					return errors.New("global minimum not found")
				}
				return nil
			},
		},
		{
			// Test that a local minimum is found (with a different initialization).
			dim: 2,
			problem: Problem{
				Func: functions.Rastrigin{}.Func,
			},
			initX: localMinMean,
			method: &CmaEsChol{
				Population:   100, // Increase the population size to reduce noise.
				InitCholesky: &localMinChol,
				ForgetBest:   true, // So that if it accidentally finds a better place we still converge to the minimum.
			},
			settings: &Settings{
				FunctionThreshold: math.Inf(-1),
			},
			good: func(result *Result, err error, concurrent int) error {
				if result.Status != MethodConverge {
					return errors.New("result not method converge")
				}
				if !floats.EqualApprox(result.X, []float64{2, -2}, 3e-2) {
					return errors.New("local minimum not found")
				}
				return nil
			},
		},
	}
}

func TestCmaEsChol(t *testing.T) {
	for i, test := range cmaTestCases() {
		src := rand.New(rand.NewSource(1))
		method := test.method
		method.Src = src
		initX := test.initX
		if initX == nil {
			initX = make([]float64, test.dim)
		}
		// Run and check that the expected termination occurs.
		result, err := Minimize(test.problem, initX, test.settings, method)
		if testErr := test.good(result, err, test.settings.Concurrent); testErr != nil {
			t.Errorf("cas %d: %v", i, testErr)
		}

		// Run a second time to make sure there are no residual effects
		result, err = Minimize(test.problem, initX, test.settings, method)
		if testErr := test.good(result, err, test.settings.Concurrent); testErr != nil {
			t.Errorf("cas %d second: %v", i, testErr)
		}

		// Test the problem in parallel.
		test.settings.Concurrent = 5
		result, err = Minimize(test.problem, initX, test.settings, method)
		if testErr := test.good(result, err, test.settings.Concurrent); testErr != nil {
			t.Errorf("cas %d concurrent: %v", i, testErr)
		}
		test.settings.Concurrent = 0
	}
}
