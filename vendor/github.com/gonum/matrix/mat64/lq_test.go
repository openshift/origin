// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"math"

	"gopkg.in/check.v1"
)

func isLowerTriangular(a *Dense) bool {
	rows, cols := a.Dims()
	for r := 0; r < rows; r++ {
		for c := r + 1; c < cols; c++ {
			if math.Abs(a.At(r, c)) > 1e-14 {
				return false
			}
		}
	}
	return true
}

func (s *S) TestLQD(c *check.C) {
	for _, test := range []struct {
		a    [][]float64
		name string
	}{
		{
			name: "Square",
			a: [][]float64{
				{1.3, 2.4, 8.9},
				{-2.6, 8.7, 9.1},
				{5.6, 5.8, 2.1},
			},
		},
		{
			name: "Skinny",
			a: [][]float64{
				{1.3, 2.4, 8.9},
				{-2.6, 8.7, 9.1},
				{5.6, 5.8, 2.1},
				{19.4, 5.2, -26.1},
			},
		},
		{
			name: "Id",
			a: [][]float64{
				{1, 0, 0},
				{0, 1, 0},
				{0, 0, 1},
			},
		},
		{
			name: "Id",
			a: [][]float64{
				{0, 0, 2},
				{0, 1, 0},
				{3, 0, 0},
			},
		},
		{
			name: "small",
			a: [][]float64{
				{1, 1},
				{1, 2},
			},
		},
	} {
		a := NewDense(flatten(test.a))

		at := new(Dense)
		at.TCopy(a)

		lq := LQ(DenseCopyOf(at))

		rows, cols := a.Dims()

		Q := NewDense(rows, cols, nil)
		for i := 0; i < cols; i++ {
			Q.Set(i, i, 1)
		}
		lq.applyQTo(Q, true)
		l := lq.L()

		lt := NewDense(rows, cols, nil)
		ltview := lt.View(0, 0, cols, cols).(*Dense)
		ltview.TCopy(l)
		lq.applyQTo(lt, true)

		c.Check(isOrthogonal(Q), check.Equals, true, check.Commentf("Test %v: Q not orthogonal", test.name))
		c.Check(a.EqualsApprox(lt, 1e-13), check.Equals, true, check.Commentf("Test %v: Q*R != A", test.name))
		c.Check(isLowerTriangular(l), check.Equals, true,
			check.Commentf("Test %v: L not lower triangular", test.name))

		nrhs := 2
		barr := make([]float64, nrhs*cols)
		for i := range barr {
			barr[i] = float64(i)
		}
		b := NewDense(cols, nrhs, barr)

		x := lq.Solve(b)

		var bProj Dense
		bProj.Mul(at, x)

		c.Check(bProj.EqualsApprox(b, 1e-13), check.Equals, true, check.Commentf("Test %v: A*X != B", test.name))

		qr := QR(DenseCopyOf(a))
		lambda := qr.Solve(DenseCopyOf(x))

		var xCheck Dense
		xCheck.Mul(a, lambda)

		c.Check(xCheck.EqualsApprox(x, 1e-13), check.Equals, true,
			check.Commentf("Test %v: A*lambda != X", test.name))
	}
}
