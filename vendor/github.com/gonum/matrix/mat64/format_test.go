// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"fmt"
	"math"

	"gopkg.in/check.v1"
)

type fm struct {
	Matrix
	margin int
}

func (m fm) Format(fs fmt.State, c rune) {
	if c == 'v' && fs.Flag('#') {
		fmt.Fprintf(fs, "%#v", m.Matrix)
		return
	}
	Format(m.Matrix, m.margin, '.', fs, c)
}

func (s *S) TestFormat(c *check.C) {
	type rp struct {
		format string
		output string
	}
	sqrt := func(_, _ int, v float64) float64 { return math.Sqrt(v) }
	for i, test := range []struct {
		m   fm
		rep []rp
	}{
		// Dense matrix representation
		{
			fm{Matrix: NewDense(3, 3, []float64{0, 0, 0, 0, 0, 0, 0, 0, 0})},
			[]rp{
				{"%v", "⎡0  0  0⎤\n⎢0  0  0⎥\n⎣0  0  0⎦"},
				{"%#f", "⎡.  .  .⎤\n⎢.  .  .⎥\n⎣.  .  .⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:3, Cols:3, Stride:3, Data:[]float64{0, 0, 0, 0, 0, 0, 0, 0, 0}}, capRows:3, capCols:3}"},
				{"%s", "%!s(*mat64.Dense=Dims(3, 3))"},
			},
		},
		{
			fm{Matrix: NewDense(3, 3, []float64{1, 1, 1, 1, 1, 1, 1, 1, 1})},
			[]rp{
				{"%v", "⎡1  1  1⎤\n⎢1  1  1⎥\n⎣1  1  1⎦"},
				{"%#f", "⎡1  1  1⎤\n⎢1  1  1⎥\n⎣1  1  1⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:3, Cols:3, Stride:3, Data:[]float64{1, 1, 1, 1, 1, 1, 1, 1, 1}}, capRows:3, capCols:3}"},
			},
		},
		{
			fm{Matrix: NewDense(3, 3, []float64{1, 0, 0, 0, 1, 0, 0, 0, 1})},
			[]rp{
				{"%v", "⎡1  0  0⎤\n⎢0  1  0⎥\n⎣0  0  1⎦"},
				{"%#f", "⎡1  .  .⎤\n⎢.  1  .⎥\n⎣.  .  1⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:3, Cols:3, Stride:3, Data:[]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}}, capRows:3, capCols:3}"},
			},
		},
		{
			fm{Matrix: NewDense(2, 3, []float64{1, 2, 3, 4, 5, 6})},
			[]rp{
				{"%v", "⎡1  2  3⎤\n⎣4  5  6⎦"},
				{"%#f", "⎡1  2  3⎤\n⎣4  5  6⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:2, Cols:3, Stride:3, Data:[]float64{1, 2, 3, 4, 5, 6}}, capRows:2, capCols:3}"},
			},
		},
		{
			fm{Matrix: NewDense(3, 2, []float64{1, 2, 3, 4, 5, 6})},
			[]rp{
				{"%v", "⎡1  2⎤\n⎢3  4⎥\n⎣5  6⎦"},
				{"%#f", "⎡1  2⎤\n⎢3  4⎥\n⎣5  6⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:3, Cols:2, Stride:2, Data:[]float64{1, 2, 3, 4, 5, 6}}, capRows:3, capCols:2}"},
			},
		},
		{
			func() fm {
				m := NewDense(2, 3, []float64{0, 1, 2, 3, 4, 5})
				m.Apply(sqrt, m)
				return fm{Matrix: m}
			}(),
			[]rp{
				{"%v", "⎡                 0                   1  1.4142135623730951⎤\n⎣1.7320508075688772                   2    2.23606797749979⎦"},
				{"%.2f", "⎡0.00  1.00  1.41⎤\n⎣1.73  2.00  2.24⎦"},
				{"%#f", "⎡                 .                   1  1.4142135623730951⎤\n⎣1.7320508075688772                   2    2.23606797749979⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:2, Cols:3, Stride:3, Data:[]float64{0, 1, 1.4142135623730951, 1.7320508075688772, 2, 2.23606797749979}}, capRows:2, capCols:3}"},
			},
		},
		{
			func() fm {
				m := NewDense(3, 2, []float64{0, 1, 2, 3, 4, 5})
				m.Apply(sqrt, m)
				return fm{Matrix: m}
			}(),
			[]rp{
				{"%v", "⎡                 0                   1⎤\n⎢1.4142135623730951  1.7320508075688772⎥\n⎣                 2    2.23606797749979⎦"},
				{"%.2f", "⎡0.00  1.00⎤\n⎢1.41  1.73⎥\n⎣2.00  2.24⎦"},
				{"%#f", "⎡                 .                   1⎤\n⎢1.4142135623730951  1.7320508075688772⎥\n⎣                 2    2.23606797749979⎦"},
				{"%#v", "&mat64.Dense{mat:blas64.General{Rows:3, Cols:2, Stride:2, Data:[]float64{0, 1, 1.4142135623730951, 1.7320508075688772, 2, 2.23606797749979}}, capRows:3, capCols:2}"},
			},
		},
		{
			func() fm {
				m := NewDense(1, 10, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
				return fm{Matrix: m, margin: 3}
			}(),
			[]rp{
				{"%v", "Dims(1, 10)\n[ 1   2   3  ...  ...   8   9  10]"},
			},
		},
		{
			func() fm {
				m := NewDense(10, 1, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
				return fm{Matrix: m, margin: 3}
			}(),
			[]rp{
				{"%v", "Dims(10, 1)\n⎡ 1⎤\n⎢ 2⎥\n⎢ 3⎥\n .\n .\n .\n⎢ 8⎥\n⎢ 9⎥\n⎣10⎦"},
			},
		},
		{
			func() fm {
				m := NewDense(10, 10, nil)
				for i := 0; i < 10; i++ {
					m.Set(i, i, 1)
				}
				return fm{Matrix: m, margin: 3}
			}(),
			[]rp{
				{"%v", "Dims(10, 10)\n⎡1  0  0  ...  ...  0  0  0⎤\n⎢0  1  0            0  0  0⎥\n⎢0  0  1            0  0  0⎥\n .\n .\n .\n⎢0  0  0            1  0  0⎥\n⎢0  0  0            0  1  0⎥\n⎣0  0  0  ...  ...  0  0  1⎦"},
			},
		},
	} {
		for _, rp := range test.rep {
			c.Check(fmt.Sprintf(rp.format, test.m), check.Equals, rp.output, check.Commentf("Test %d", i))
		}
	}
}
