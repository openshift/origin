// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"gopkg.in/check.v1"
)

func (s *S) TestLUD(c *check.C) {
	for _, t := range []struct {
		a *Dense

		l *Dense
		u *Dense

		pivot []int
		sign  int
	}{
		{ // This is a hard coded equivalent of the approach used in the Jama LU test.
			a: NewDense(3, 3, []float64{
				0, 2, 3,
				4, 5, 6,
				7, 8, 9,
			}),

			l: NewDense(3, 3, []float64{
				1, 0, 0,
				0, 1, 0,
				0.5714285714285714, 0.2142857142857144, 1,
			}),
			u: NewDense(3, 3, []float64{
				7, 8, 9,
				0, 2, 3,
				0, 0, 0.2142857142857144,
			}),
			pivot: []int{
				2, // 0 0 1
				0, // 1 0 0
				1, // 0 1 0
			},
			sign: 1,
		},
		{
			a: NewDense(2, 3, []float64{
				0, 2, 3,
				4, 5, 6,
			}),

			l: NewDense(2, 2, []float64{
				1, 0,
				0, 1,
			}),
			u: NewDense(2, 3, []float64{
				4, 5, 6,
				0, 2, 3,
			}),
			pivot: []int{
				1, // 0 1
				0, // 1 0
			},
			sign: -1,
		},
	} {
		lf := LU(DenseCopyOf(t.a))
		if t.pivot != nil {
			c.Check(lf.Pivot, check.DeepEquals, t.pivot)
			c.Check(lf.Sign, check.Equals, t.sign)
		}

		l := lf.L()
		if t.l != nil {
			c.Check(l.Equals(t.l), check.Equals, true)
		}
		u := lf.U()
		if t.u != nil {
			c.Check(u.Equals(t.u), check.Equals, true)
		}

		var got Dense
		got.Mul(l, u)
		c.Check(got.EqualsApprox(pivotRows(DenseCopyOf(t.a), lf.Pivot), 1e-12), check.Equals, true)

		if m, n := t.a.Dims(); m != n {
			continue
		}
		x := lf.Solve(eye())
		t.a.Mul(t.a, x)
		c.Check(t.a.EqualsApprox(eye(), 1e-12), check.Equals, true)
	}
}
