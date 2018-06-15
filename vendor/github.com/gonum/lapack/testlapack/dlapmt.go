// Copyright ©2017 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"github.com/gonum/blas/blas64"
)

type Dlapmter interface {
	Dlapmt(forward bool, m, n int, x []float64, ldx int, k []int)
}

func DlapmtTest(t *testing.T, impl Dlapmter) {
	for ti, test := range []struct {
		forward bool
		k       []int

		want blas64.General
	}{
		{
			forward: true, k: []int{0, 1, 2},
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
			forward: false, k: []int{0, 1, 2},
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
			forward: true, k: []int{1, 2, 0},
			want: blas64.General{
				Rows:   4,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					2, 3, 1,
					5, 6, 4,
					8, 9, 7,
					11, 12, 10,
				},
			},
		},
		{
			forward: false, k: []int{1, 2, 0},
			want: blas64.General{
				Rows:   4,
				Cols:   3,
				Stride: 3,
				Data: []float64{
					3, 1, 2,
					6, 4, 5,
					9, 7, 8,
					12, 10, 11,
				},
			},
		},
	} {
		m := test.want.Rows
		n := test.want.Cols
		if len(test.k) != n {
			panic("bad length of k")
		}

		for _, extra := range []int{0, 11} {
			x := zeros(m, n, n+extra)
			c := 1
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					x.Data[i*x.Stride+j] = float64(c)
					c++
				}
			}

			k := make([]int, len(test.k))
			copy(k, test.k)

			impl.Dlapmt(test.forward, m, n, x.Data, x.Stride, k)

			prefix := fmt.Sprintf("Case %v (forward=%t,m=%v,n=%v,extra=%v)", ti, test.forward, m, n, extra)
			if !generalOutsideAllNaN(x) {
				t.Errorf("%v: out-of-range write to X", prefix)
			}

			if !equalApproxGeneral(x, test.want, 0) {
				t.Errorf("%v: unexpected X\n%v\n%v", prefix, x, test.want)
			}
		}
	}
}
