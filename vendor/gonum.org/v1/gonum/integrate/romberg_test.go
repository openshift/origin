// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/integrate/testquad"
)

func TestRomberg(t *testing.T) {
	for i, test := range []struct {
		integral testquad.Integral
		n        int
		tol      float64
	}{
		{integral: testquad.Constant(0), n: 3, tol: 0},
		{integral: testquad.Constant(0), n: 1<<5 + 1, tol: 0},
		{integral: testquad.Poly(0), n: 3, tol: 1e-14},
		{integral: testquad.Poly(0), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Poly(1), n: 3, tol: 1e-14},
		{integral: testquad.Poly(1), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Poly(2), n: 3, tol: 1e-14},
		{integral: testquad.Poly(2), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Poly(3), n: 3, tol: 1e-14},
		{integral: testquad.Poly(3), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Poly(4), n: 5, tol: 1e-14},
		{integral: testquad.Poly(4), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Poly(5), n: 5, tol: 1e-14},
		{integral: testquad.Poly(5), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Sin(), n: 1<<3 + 1, tol: 1e-10},
		{integral: testquad.Sin(), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.XExpMinusX(), n: 1<<3 + 1, tol: 1e-9},
		{integral: testquad.XExpMinusX(), n: 1<<5 + 1, tol: 1e-14},
		{integral: testquad.Sqrt(), n: 1<<10 + 1, tol: 1e-5},
		{integral: testquad.ExpOverX2Plus1(), n: 1<<4 + 1, tol: 1e-7},
		{integral: testquad.ExpOverX2Plus1(), n: 1<<6 + 1, tol: 1e-14},
	} {
		n := test.n
		a := test.integral.A
		b := test.integral.B

		x := make([]float64, n)
		floats.Span(x, a, b)

		y := make([]float64, n)
		for i, xi := range x {
			y[i] = test.integral.F(xi)
		}

		dx := (b - a) / float64(n-1)
		got := Romberg(y, dx)

		want := test.integral.Value
		diff := math.Abs(got - want)
		if diff > test.tol {
			t.Errorf("Test #%d: %v, n=%v: unexpected result; got=%v want=%v diff=%v",
				i, test.integral.Name, n, got, want, diff)
		}
	}
}
