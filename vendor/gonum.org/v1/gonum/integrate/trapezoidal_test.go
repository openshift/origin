// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/integrate/testquad"
)

func TestTrapezoidal(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for i, test := range []struct {
		integral testquad.Integral
		n        int
		tol      float64
	}{
		{integral: testquad.Constant(0), n: 2, tol: 0},
		{integral: testquad.Constant(0), n: 10, tol: 0},
		{integral: testquad.Poly(0), n: 2, tol: 1e-14},
		{integral: testquad.Poly(0), n: 10, tol: 1e-14},
		{integral: testquad.Poly(1), n: 2, tol: 1e-14},
		{integral: testquad.Poly(1), n: 10, tol: 1e-14},
		{integral: testquad.Poly(2), n: 1e5, tol: 1e-8},
		{integral: testquad.Poly(3), n: 1e5, tol: 1e-8},
		{integral: testquad.Poly(4), n: 1e5, tol: 1e-7},
		{integral: testquad.Poly(5), n: 1e5, tol: 1e-7},
		{integral: testquad.Sin(), n: 1e5, tol: 1e-11},
		{integral: testquad.XExpMinusX(), n: 1e5, tol: 1e-10},
		{integral: testquad.Sqrt(), n: 1e5, tol: 1e-8},
		{integral: testquad.ExpOverX2Plus1(), n: 1e5, tol: 1e-10},
	} {
		n := test.n
		a := test.integral.A
		b := test.integral.B

		x := jitterSpan(n, a, b, rnd)
		y := make([]float64, n)
		for i, xi := range x {
			y[i] = test.integral.F(xi)
		}

		got := Trapezoidal(x, y)

		want := test.integral.Value
		diff := math.Abs(got - want)
		if diff > test.tol {
			t.Errorf("Test #%d: %v, n=%v: unexpected result; got=%v want=%v diff=%v",
				i, test.integral.Name, n, got, want, diff)
		}
	}
}

func jitterSpan(n int, a, b float64, rnd *rand.Rand) []float64 {
	dx := (b - a) / float64(n-1)
	x := make([]float64, n)
	x[0] = a
	for i := 1; i < n-1; i++ {
		// Set x[i] to its regular location.
		x[i] = a + float64(i)*dx
		// Generate a random number in [-1,1).
		jitter := 2*rnd.Float64() - 1
		// Jitter x[i] without crossing over its neighbors.
		x[i] += 0.4 * jitter * dx
	}
	x[n-1] = b
	return x
}
