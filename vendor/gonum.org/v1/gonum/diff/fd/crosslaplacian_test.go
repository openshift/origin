// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

type CrossLaplacianTester interface {
	Func(x, y []float64) float64
	CrossLaplacian(x, y []float64) float64
}

type WrapperCL struct {
	Tester HessianTester
}

func (WrapperCL) constructZ(x, y []float64) []float64 {
	z := make([]float64, len(x)+len(y))
	copy(z, x)
	copy(z[len(x):], y)
	return z
}

func (w WrapperCL) Func(x, y []float64) float64 {
	z := w.constructZ(x, y)
	return w.Tester.Func(z)
}

func (w WrapperCL) CrossLaplacian(x, y []float64) float64 {
	z := w.constructZ(x, y)
	hess := mat.NewSymDense(len(z), nil)
	w.Tester.Hess(hess, z)
	// The CrossLaplacian is the trace of the off-diagonal block of the Hessian.
	var l float64
	for i := 0; i < len(x); i++ {
		l += hess.At(i, i+len(x))
	}
	return l
}

func TestCrossLaplacian(t *testing.T) {
	for cas, test := range []struct {
		l        CrossLaplacianTester
		x, y     []float64
		settings *Settings
		tol      float64
	}{
		{
			l:   WrapperCL{Watson{}},
			x:   []float64{0.2, 0.3},
			y:   []float64{0.1, 0.4},
			tol: 1e-3,
		},
		{
			l:   WrapperCL{Watson{}},
			x:   []float64{2, 3, 1},
			y:   []float64{1, 4, 1},
			tol: 1e-3,
		},
		{
			l:   WrapperCL{ConstFunc(6)},
			x:   []float64{2, -3, 1},
			y:   []float64{1, 4, -5},
			tol: 1e-6,
		},
		{
			l:   WrapperCL{LinearFunc{w: []float64{10, 6, -1, 5}, c: 5}},
			x:   []float64{3, 1},
			y:   []float64{8, 6},
			tol: 1e-6,
		},
		{
			l: WrapperCL{QuadFunc{
				a: mat.NewSymDense(4, []float64{
					10, 2, 1, 9,
					2, 5, -3, 4,
					1, -3, 6, 2,
					9, 4, 2, -14,
				}),
				b: mat.NewVecDense(4, []float64{3, -2, -1, 4}),
				c: 5,
			}},
			x:   []float64{-1.6, -3},
			y:   []float64{1.8, 3.4},
			tol: 1e-6,
		},
	} {
		got := CrossLaplacian(test.l.Func, test.x, test.y, test.settings)
		want := test.l.CrossLaplacian(test.x, test.y)
		if !floats.EqualWithinAbsOrRel(got, want, test.tol, test.tol) {
			t.Errorf("Cas %d: CrossLaplacian mismatch serial. got %v, want %v", cas, got, want)
		}

		// Test that concurrency works.
		settings := test.settings
		if settings == nil {
			settings = &Settings{}
		}
		settings.Concurrent = true
		got2 := CrossLaplacian(test.l.Func, test.x, test.y, settings)
		if !floats.EqualWithinAbsOrRel(got, got2, 1e-6, 1e-6) {
			t.Errorf("Cas %d: Laplacian mismatch. got %v, want %v", cas, got2, got)
		}
	}
}
