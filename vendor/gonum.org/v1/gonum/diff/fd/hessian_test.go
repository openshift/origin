// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"testing"

	"gonum.org/v1/gonum/mat"
)

type HessianTester interface {
	Func(x []float64) float64
	Grad(grad, x []float64)
	Hess(dst mat.MutableSymmetric, x []float64)
}

var hessianTestCases = []struct {
	h        HessianTester
	x        []float64
	settings *Settings
	tol      float64
}{
	{
		h:   Watson{},
		x:   []float64{0.2, 0.3, 0.1, 0.4},
		tol: 1e-3,
	},
	{
		h:   Watson{},
		x:   []float64{2, 3, 1, 4},
		tol: 1e-3,
		settings: &Settings{
			Step:    1e-5,
			Formula: Central,
		},
	},
	{
		h:   Watson{},
		x:   []float64{2, 3, 1},
		tol: 1e-3,
		settings: &Settings{
			OriginKnown: true,
			OriginValue: 7606.529501201192,
		},
	},
	{
		h:   ConstFunc(5),
		x:   []float64{1, 9},
		tol: 1e-16,
	},
	{
		h:   LinearFunc{w: []float64{10, 6, -1}, c: 5},
		x:   []float64{3, 1, 8},
		tol: 1e-6,
	},
	{
		h: QuadFunc{
			a: mat.NewSymDense(3, []float64{
				10, 2, 1,
				2, 5, -3,
				1, -3, 6,
			}),
			b: mat.NewVecDense(3, []float64{3, -2, -1}),
			c: 5,
		},
		x:   []float64{-1.6, -3, 2},
		tol: 1e-6,
	},
}

func TestHessian(t *testing.T) {
	for cas, test := range hessianTestCases {
		n := len(test.x)
		got := Hessian(nil, test.h.Func, test.x, test.settings)
		want := mat.NewSymDense(n, nil)
		test.h.Hess(want, test.x)
		if !mat.EqualApprox(got, want, test.tol) {
			t.Errorf("Cas %d: Hessian mismatch\ngot=\n%0.4v\nwant=\n%0.4v\n", cas, mat.Formatted(got), mat.Formatted(want))
		}

		// Test that concurrency works.
		settings := test.settings
		if settings == nil {
			settings = &Settings{}
		}
		settings.Concurrent = true
		got2 := Hessian(nil, test.h.Func, test.x, settings)
		if !mat.EqualApprox(got, got2, 1e-5) {
			t.Errorf("Cas %d: Hessian mismatch concurrent\ngot=\n%0.6v\nwant=\n%0.6v\n", cas, mat.Formatted(got2), mat.Formatted(got))
		}
	}
}
