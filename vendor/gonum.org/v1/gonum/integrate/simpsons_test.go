// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/integrate/testquad"
)

func TestSimpsons(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for i, test := range []struct {
		integral testquad.Integral
		n        int
		tol      float64
	}{
		{integral: testquad.Constant(0), n: 3, tol: 0},
		{integral: testquad.Constant(0), n: 10, tol: 0},
		{integral: testquad.Poly(0), n: 3, tol: 1e-14},
		{integral: testquad.Poly(0), n: 10, tol: 1e-14},
		{integral: testquad.Poly(1), n: 3, tol: 1e-14},
		{integral: testquad.Poly(1), n: 10, tol: 1e-14},
		{integral: testquad.Poly(2), n: 3, tol: 1e-14},
		{integral: testquad.Poly(2), n: 10, tol: 1e-14},
		{integral: testquad.Poly(3), n: 1e3, tol: 1e-8},
		{integral: testquad.Poly(4), n: 1e3, tol: 1e-8},
		{integral: testquad.Poly(5), n: 1e3, tol: 1e-7},
		{integral: testquad.Sin(), n: 1e2, tol: 1e-8},
		{integral: testquad.XExpMinusX(), n: 1e2, tol: 1e-8},
		{integral: testquad.Sqrt(), n: 1e4, tol: 1e-6},
		{integral: testquad.ExpOverX2Plus1(), n: 1e2, tol: 1e-7},
	} {
		n := test.n
		a := test.integral.A
		b := test.integral.B

		x := jitterSpan(n, a, b, rnd)
		y := make([]float64, n)
		for i, xi := range x {
			y[i] = test.integral.F(xi)
		}

		got := Simpsons(x, y)

		want := test.integral.Value
		diff := math.Abs(got - want)
		if diff > test.tol {
			t.Errorf("Test #%d: %v, n=%v: unexpected result; got=%v want=%v diff=%v",
				i, test.integral.Name, n, got, want, diff)
		}
	}
}
