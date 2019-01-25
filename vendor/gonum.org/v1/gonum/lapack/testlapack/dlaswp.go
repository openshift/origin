// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/blas/blas64"
)

type Dlaswper interface {
	Dlaswp(n int, a []float64, lda, k1, k2 int, ipiv []int, incX int)
}

func DlaswpTest(t *testing.T, impl Dlaswper) {
	for ti, test := range []struct {
		k1, k2 int
		ipiv   []int
		incX   int

		want blas64.General
	}{
		{
			k1:   0,
			k2:   2,
			ipiv: []int{0, 1, 2},
			incX: 1,
			want: blas64.General{
				Rows:   4,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					1, 2, 3,
					4, 5, 6,
					7, 8, 9,
					10, 11, 12,
				},
			},
		},
		{
			k1:   0,
			k2:   2,
			ipiv: []int{0, 1, 2},
			incX: -1,
			want: blas64.General{
				Rows:   4,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					1, 2, 3,
					4, 5, 6,
					7, 8, 9,
					10, 11, 12,
				},
			},
		},
		{
			k1:   0,
			k2:   2,
			ipiv: []int{1, 2, 3},
			incX: 1,
			want: blas64.General{
				Rows:   5,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					4, 5, 6,
					7, 8, 9,
					10, 11, 12,
					1, 2, 3,
					13, 14, 15,
				},
			},
		},
		{
			k1:   0,
			k2:   2,
			ipiv: []int{1, 2, 3},
			incX: -1,
			want: blas64.General{
				Rows:   5,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					10, 11, 12,
					1, 2, 3,
					4, 5, 6,
					7, 8, 9,
					13, 14, 15,
				},
			},
		},
	} {
		m := test.want.Rows
		n := test.want.Cols
		k1 := test.k1
		k2 := test.k2
		if len(test.ipiv) != k2+1 {
			panic("bad length of ipiv")
		}
		incX := test.incX
		for _, extra := range []int{0, 11} {
			a := zeros(m, n, n+extra)
			c := 1
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					a.Data[i*a.Stride+j] = float64(c)
					c++
				}
			}

			ipiv := make([]int, len(test.ipiv))
			copy(ipiv, test.ipiv)

			impl.Dlaswp(n, a.Data, a.Stride, k1, k2, ipiv, incX)

			prefix := fmt.Sprintf("Case %v (m=%v,n=%v,k1=%v,k2=%v,extra=%v)", ti, m, n, k1, k2, extra)
			if !generalOutsideAllNaN(a) {
				t.Errorf("%v: out-of-range write to A", prefix)
			}

			if !equalApproxGeneral(a, test.want, 0) {
				t.Errorf("%v: unexpected A\n%v\n%v", prefix, a, test.want)
			}
		}
	}
}
