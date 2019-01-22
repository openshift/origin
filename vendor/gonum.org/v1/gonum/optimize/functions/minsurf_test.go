// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functions

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/floats"
)

func TestMinimalSurface(t *testing.T) {
	for _, size := range [][2]int{
		{20, 30},
		{30, 30},
		{50, 40},
	} {
		f := NewMinimalSurface(size[0], size[1])
		x0 := f.InitX()
		grad := make([]float64, len(x0))
		f.Grad(grad, x0)
		fdGrad := fd.Gradient(nil, f.Func, x0, &fd.Settings{Formula: fd.Central})

		// Test that the numerical and analytical gradients agree.
		dist := floats.Distance(grad, fdGrad, math.Inf(1))
		if dist > 1e-9 {
			t.Errorf("grid %v x %v: numerical and analytical gradient do not match. |fdGrad - grad|_∞ = %v",
				size[0], size[1], dist)
		}

		// Test that the gradient at the minimum is small enough.
		// In some sense this test is not completely correct because ExactX
		// returns the exact solution to the continuous problem projected on the
		// grid, not the exact solution to the discrete problem which we are
		// solving. This is the reason why a relatively loose tolerance 1e-4
		// must be used.
		xSol := f.ExactX()
		f.Grad(grad, xSol)
		norm := floats.Norm(grad, math.Inf(1))
		if norm > 1e-4 {
			t.Errorf("grid %v x %v: gradient at the minimum not small enough. |grad|_∞ = %v",
				size[0], size[1], norm)
		}
	}
}
