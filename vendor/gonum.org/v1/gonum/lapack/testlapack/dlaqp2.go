// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dlaqp2er interface {
	Dlapmter
	Dlaqp2(m, n, offset int, a []float64, lda int, jpvt []int, tau, vn1, vn2, work []float64)
}

func Dlaqp2Test(t *testing.T, impl Dlaqp2er) {
	for ti, test := range []struct {
		m, n, offset int
	}{
		{m: 4, n: 3, offset: 0},
		{m: 4, n: 3, offset: 2},
		{m: 4, n: 3, offset: 4},
		{m: 3, n: 4, offset: 0},
		{m: 3, n: 4, offset: 1},
		{m: 3, n: 4, offset: 2},
		{m: 8, n: 3, offset: 0},
		{m: 8, n: 3, offset: 4},
		{m: 8, n: 3, offset: 8},
		{m: 3, n: 8, offset: 0},
		{m: 3, n: 8, offset: 1},
		{m: 3, n: 8, offset: 2},
		{m: 10, n: 10, offset: 0},
		{m: 10, n: 10, offset: 5},
		{m: 10, n: 10, offset: 10},
	} {
		m := test.m
		n := test.n
		jpiv := make([]int, n)

		for _, extra := range []int{0, 11} {
			a := zeros(m, n, n+extra)
			c := 1
			for i := 0; i < m; i++ {
				for j := 0; j < n; j++ {
					a.Data[i*a.Stride+j] = float64(c)
					c++
				}
			}
			aCopy := cloneGeneral(a)
			for j := range jpiv {
				jpiv[j] = j
			}

			tau := make([]float64, n)
			vn1 := columnNorms(m, n, a.Data, a.Stride)
			vn2 := columnNorms(m, n, a.Data, a.Stride)
			work := make([]float64, n)

			impl.Dlaqp2(m, n, test.offset, a.Data, a.Stride, jpiv, tau, vn1, vn2, work)

			prefix := fmt.Sprintf("Case %v (offset=%d,m=%v,n=%v,extra=%v)", ti, test.offset, m, n, extra)
			if !generalOutsideAllNaN(a) {
				t.Errorf("%v: out-of-range write to A", prefix)
			}

			if test.offset == m {
				continue
			}

			mo := m - test.offset
			q := constructQ("QR", mo, n, a.Data[test.offset*a.Stride:], a.Stride, tau)
			// Check that Q is orthogonal.
			if !isOrthogonal(q) {
				t.Errorf("Case %v, Q not orthogonal", ti)
			}

			// Check that A * P = Q * R
			r := blas64.General{
				Rows:   mo,
				Cols:   n,
				Stride: n,
				Data:   make([]float64, mo*n),
			}
			for i := 0; i < mo; i++ {
				for j := i; j < n; j++ {
					r.Data[i*n+j] = a.Data[(test.offset+i)*a.Stride+j]
				}
			}
			got := nanGeneral(mo, n, n)
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, r, 0, got)

			want := aCopy
			impl.Dlapmt(true, want.Rows, want.Cols, want.Data, want.Stride, jpiv)
			want.Rows = mo
			want.Data = want.Data[test.offset*want.Stride:]
			if !equalApproxGeneral(got, want, 1e-12) {
				t.Errorf("Case %v,  Q*R != A*P\nQ*R=%v\nA*P=%v", ti, got, want)
			}
		}
	}
}
