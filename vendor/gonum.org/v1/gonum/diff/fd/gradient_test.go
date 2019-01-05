// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Rosenbrock struct {
	nDim int
}

func (r Rosenbrock) F(x []float64) (sum float64) {
	deriv := make([]float64, len(x))
	return r.FDf(x, deriv)
}

func (r Rosenbrock) FDf(x []float64, deriv []float64) (sum float64) {
	for i := range deriv {
		deriv[i] = 0
	}

	for i := 0; i < len(x)-1; i++ {
		sum += math.Pow(1-x[i], 2) + 100*math.Pow(x[i+1]-math.Pow(x[i], 2), 2)
	}
	for i := 0; i < len(x)-1; i++ {
		deriv[i] += -1 * 2 * (1 - x[i])
		deriv[i] += 2 * 100 * (x[i+1] - math.Pow(x[i], 2)) * (-2 * x[i])
	}
	for i := 1; i < len(x); i++ {
		deriv[i] += 2 * 100 * (x[i] - math.Pow(x[i-1], 2))
	}

	return sum
}

func TestGradient(t *testing.T) {
	rand.Seed(1)
	for i, test := range []struct {
		nDim    int
		tol     float64
		formula Formula
	}{
		{
			nDim:    2,
			tol:     2e-4,
			formula: Forward,
		},
		{
			nDim:    2,
			tol:     1e-6,
			formula: Central,
		},
		{
			nDim:    40,
			tol:     2e-4,
			formula: Forward,
		},
		{
			nDim:    40,
			tol:     1e-5,
			formula: Central,
		},
	} {
		x := make([]float64, test.nDim)
		for i := range x {
			x[i] = rand.Float64()
		}
		xcopy := make([]float64, len(x))
		copy(xcopy, x)

		r := Rosenbrock{len(x)}
		trueGradient := make([]float64, len(x))
		r.FDf(x, trueGradient)

		// Try with gradient nil.
		gradient := Gradient(nil, r.F, x, &Settings{
			Formula: test.formula,
		})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch in serial with nil. Want: %v, Got: %v.", i, trueGradient, gradient)
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %v: x modified during call to gradient in serial with nil.", i)
		}

		// Try with provided gradient.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, &Settings{
			Formula: test.formula,
		})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch in serial. Want: %v, Got: %v.", i, trueGradient, gradient)
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %v: x modified during call to gradient in serial with non-nil.", i)
		}

		// Try with known value.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, &Settings{
			Formula:     test.formula,
			OriginKnown: true,
			OriginValue: r.F(x),
		})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch with known origin in serial. Want: %v, Got: %v.", i, trueGradient, gradient)
		}

		// Try with concurrent evaluation.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, &Settings{
			Formula:    test.formula,
			Concurrent: true,
		})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch with unknown origin in parallel. Want: %v, Got: %v.", i, trueGradient, gradient)
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %v: x modified during call to gradient in parallel", i)
		}

		// Try with concurrent evaluation with origin known.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, &Settings{
			Formula:     test.formula,
			Concurrent:  true,
			OriginKnown: true,
			OriginValue: r.F(x),
		})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch with known origin in parallel. Want: %v, Got: %v.", i, trueGradient, gradient)
		}

		// Try with nil settings.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, nil)
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch with default settings. Want: %v, Got: %v.", i, trueGradient, gradient)
		}

		// Try with zero-valued settings.
		for i := range gradient {
			gradient[i] = rand.Float64()
		}
		Gradient(gradient, r.F, x, &Settings{})
		if !floats.EqualApprox(gradient, trueGradient, test.tol) {
			t.Errorf("Case %v: gradient mismatch with zero settings. Want: %v, Got: %v.", i, trueGradient, gradient)
		}
	}
}

func Panics(fun func()) (b bool) {
	defer func() {
		err := recover()
		if err != nil {
			b = true
		}
	}()
	fun()
	return
}

func TestGradientPanics(t *testing.T) {
	// Test that it panics
	if !Panics(func() {
		Gradient([]float64{0.0}, func(x []float64) float64 { return x[0] * x[0] }, []float64{0.0, 0.0}, nil)
	}) {
		t.Errorf("Gradient did not panic with length mismatch")
	}
}
