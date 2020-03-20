// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"gonum.org/v1/gonum/optimize/functions"
)

func TestMoreThuente(t *testing.T) {
	d := 0.001
	c := 0.001
	ls := &MoreThuente{
		DecreaseFactor:  d,
		CurvatureFactor: c,
	}
	testLinesearcher(t, ls, d, c, true)
}

func TestBisection(t *testing.T) {
	c := 0.1
	ls := &Bisection{
		CurvatureFactor: c,
	}
	testLinesearcher(t, ls, 0, c, true)
}

func TestBacktracking(t *testing.T) {
	d := 0.001
	ls := &Backtracking{
		DecreaseFactor: d,
	}
	testLinesearcher(t, ls, d, 0, false)
}

type funcGrader interface {
	Func([]float64) float64
	Grad([]float64, []float64)
}

type linesearcherTest struct {
	name string
	f    func(float64) float64
	g    func(float64) float64
}

func newLinesearcherTest(name string, fg funcGrader) linesearcherTest {
	grad := make([]float64, 1)
	return linesearcherTest{
		name: name,
		f: func(x float64) float64 {
			return fg.Func([]float64{x})
		},
		g: func(x float64) float64 {
			fg.Grad(grad, []float64{x})
			return grad[0]
		},
	}
}

func testLinesearcher(t *testing.T, ls Linesearcher, decrease, curvature float64, strongWolfe bool) {
	for i, prob := range []linesearcherTest{
		newLinesearcherTest("Concave-to-the-right function", functions.ConcaveRight{}),
		newLinesearcherTest("Concave-to-the-left function", functions.ConcaveLeft{}),
		newLinesearcherTest("Plassmann wiggly function (l=39, beta=0.01)", functions.Plassmann{L: 39, Beta: 0.01}),
		newLinesearcherTest("Yanai-Ozawa-Kaneko function (beta1=0.001, beta2=0.001)", functions.YanaiOzawaKaneko{Beta1: 0.001, Beta2: 0.001}),
		newLinesearcherTest("Yanai-Ozawa-Kaneko function (beta1=0.01, beta2=0.001)", functions.YanaiOzawaKaneko{Beta1: 0.01, Beta2: 0.001}),
		newLinesearcherTest("Yanai-Ozawa-Kaneko function (beta1=0.001, beta2=0.01)", functions.YanaiOzawaKaneko{Beta1: 0.001, Beta2: 0.01}),
	} {
		for _, initStep := range []float64{0.001, 0.1, 1, 10, 1000} {
			prefix := fmt.Sprintf("test %d (%v started from %v)", i, prob.name, initStep)

			f0 := prob.f(0)
			g0 := prob.g(0)
			if g0 >= 0 {
				panic("bad test function")
			}

			op := ls.Init(f0, g0, initStep)
			if !op.isEvaluation() {
				t.Errorf("%v: Linesearcher.Init returned non-evaluating operation %v", prefix, op)
				continue
			}

			var (
				err  error
				k    int
				f, g float64
				step float64
			)
		loop:
			for {
				switch op {
				case MajorIteration:
					if f > f0+step*decrease*g0 {
						t.Errorf("%v: %v found step %v that does not satisfy the sufficient decrease condition",
							prefix, reflect.TypeOf(ls), step)
					}
					if strongWolfe && math.Abs(g) > curvature*(-g0) {
						t.Errorf("%v: %v found step %v that does not satisfy the curvature condition",
							prefix, reflect.TypeOf(ls), step)
					}
					break loop
				case FuncEvaluation:
					f = prob.f(step)
				case GradEvaluation:
					g = prob.g(step)
				case FuncEvaluation | GradEvaluation:
					f = prob.f(step)
					g = prob.g(step)
				default:
					t.Errorf("%v: Linesearcher returned an invalid operation %v", prefix, op)
					break loop
				}

				k++
				if k == 1000 {
					t.Errorf("%v: %v did not finish", prefix, reflect.TypeOf(ls))
					break
				}

				op, step, err = ls.Iterate(f, g)
				if err != nil {
					t.Errorf("%v: %v failed at step %v with %v", prefix, reflect.TypeOf(ls), step, err)
					break
				}
			}
		}
	}
}
