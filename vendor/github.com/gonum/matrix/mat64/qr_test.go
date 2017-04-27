// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"math"

	"github.com/gonum/floats"

	"gopkg.in/check.v1"
)

func isUpperTriangular(a *Dense) bool {
	rows, cols := a.Dims()
	for c := 0; c < cols-1; c++ {
		for r := c + 1; r < rows; r++ {
			if math.Abs(a.At(r, c)) > 1e-14 {
				return false
			}
		}
	}
	return true
}

func isOrthogonal(a *Dense) bool {
	rows, cols := a.Dims()
	col1 := make([]float64, rows)
	col2 := make([]float64, rows)
	for i := 0; i < cols-1; i++ {
		for j := i + 1; j < cols; j++ {
			a.Col(col1, i)
			a.Col(col2, j)
			dot := floats.Dot(col1, col2)
			if math.Abs(dot) > 1e-14 {
				return false
			}
		}
	}
	return true
}

func (s *S) TestQRD(c *check.C) {
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
	} {

		a := NewDense(flatten(test.a))
		qf := QR(DenseCopyOf(a))
		r := qf.R()
		q := qf.Q()

		rows, cols := a.Dims()
		newA := NewDense(rows, cols, nil)
		newA.Mul(q, r)

		c.Check(isOrthogonal(q), check.Equals, true, check.Commentf("Test %v: Q not orthogonal", test.name))
		c.Check(isUpperTriangular(r), check.Equals, true, check.Commentf("Test %v: R not upper triangular", test.name))
		c.Check(a.EqualsApprox(newA, 1e-13), check.Equals, true, check.Commentf("Test %v: Q*R != A", test.name))
	}
}
