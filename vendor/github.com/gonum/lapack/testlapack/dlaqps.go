// Copyright ©2017 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
)

type Dlaqpser interface {
	Dlapmter
	Dlaqps(m, n, offset, nb int, a []float64, lda int, jpvt []int, tau, vn1, vn2, auxv, f []float64, ldf int) (kb int)
}

func DlaqpsTest(t *testing.T, impl Dlaqpser) {
	for ti, test := range []struct {
		m, n, nb, offset int
	}{
		{m: 4, n: 3, nb: 2, offset: 0},
		{m: 4, n: 3, nb: 1, offset: 2},
		{m: 3, n: 4, nb: 2, offset: 0},
		{m: 3, n: 4, nb: 1, offset: 2},
		{m: 8, n: 3, nb: 2, offset: 0},
		{m: 8, n: 3, nb: 1, offset: 4},
		{m: 3, n: 8, nb: 2, offset: 0},
		{m: 3, n: 8, nb: 1, offset: 1},
		{m: 10, n: 10, nb: 3, offset: 0},
		{m: 10, n: 10, nb: 2, offset: 5},
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
			auxv := make([]float64, test.nb)
			f := zeros(test.n, test.nb, n)

			kb := impl.Dlaqps(m, n, test.offset, test.nb, a.Data, a.Stride, jpiv, tau, vn1, vn2, auxv, f.Data, f.Stride)

			prefix := fmt.Sprintf("Case %v (offset=%t,m=%v,n=%v,extra=%v)", ti, test.offset, m, n, extra)
			if !generalOutsideAllNaN(a) {
				t.Errorf("%v: out-of-range write to A", prefix)
			}

			if test.offset == m {
				continue
			}

			mo := m - test.offset
			q := constructQ("QR", mo, kb, a.Data[test.offset*a.Stride:], a.Stride, tau)
			// Check that q is orthonormal
			for i := 0; i < mo; i++ {
				nrm := blas64.Nrm2(mo, blas64.Vector{Inc: 1, Data: q.Data[i*mo:]})
				if math.Abs(nrm-1) > 1e-13 {
					t.Errorf("Case %v, q not normal", ti)
				}
				for j := 0; j < i; j++ {
					dot := blas64.Dot(mo, blas64.Vector{Inc: 1, Data: q.Data[i*mo:]}, blas64.Vector{Inc: 1, Data: q.Data[j*mo:]})
					if math.Abs(dot) > 1e-14 {
						t.Errorf("Case %v, q not orthogonal", ti)
					}
				}
			}

			// Check that A * P = Q * R
			r := blas64.General{
				Rows:   mo,
				Cols:   kb,
				Stride: kb,
				Data:   make([]float64, mo*kb),
			}
			for i := 0; i < mo; i++ {
				for j := i; j < kb; j++ {
					r.Data[i*kb+j] = a.Data[(test.offset+i)*a.Stride+j]
				}
			}
			got := nanGeneral(mo, kb, kb)
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, r, 0, got)

			want := aCopy
			impl.Dlapmt(true, want.Rows, want.Cols, want.Data, want.Stride, jpiv)
			want.Rows = mo
			want.Cols = kb
			want.Data = want.Data[test.offset*want.Stride:]
			if !equalApproxGeneral(got, want, 1e-12) {
				t.Errorf("Case %v,  Q*R != A*P\nQ*R=%v\nA*P=%v", ti, got, want)
			}
		}
	}
}
