// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dsytrder interface {
	Dsytrd(uplo blas.Uplo, n int, a []float64, lda int, d, e, tau, work []float64, lwork int)

	Dorgqr(m, n, k int, a []float64, lda int, tau, work []float64, lwork int)
	Dorgql(m, n, k int, a []float64, lda int, tau, work []float64, lwork int)
}

func DsytrdTest(t *testing.T, impl Dsytrder) {
	const tol = 1e-13
	rnd := rand.New(rand.NewSource(1))
	for tc, test := range []struct {
		n, lda int
	}{
		{1, 0},
		{2, 0},
		{3, 0},
		{4, 0},
		{10, 0},
		{50, 0},
		{100, 0},
		{150, 0},
		{300, 0},

		{1, 3},
		{2, 3},
		{3, 7},
		{4, 9},
		{10, 20},
		{50, 70},
		{100, 120},
		{150, 170},
		{300, 320},
	} {
		for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
			for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
				n := test.n
				lda := test.lda
				if lda == 0 {
					lda = n
				}
				a := randomGeneral(n, n, lda, rnd)
				for i := 1; i < n; i++ {
					for j := 0; j < i; j++ {
						a.Data[i*a.Stride+j] = a.Data[j*a.Stride+i]
					}
				}
				aCopy := cloneGeneral(a)

				d := nanSlice(n)
				e := nanSlice(n - 1)
				tau := nanSlice(n - 1)

				var lwork int
				switch wl {
				case minimumWork:
					lwork = 1
				case mediumWork:
					work := make([]float64, 1)
					impl.Dsytrd(uplo, n, a.Data, a.Stride, d, e, tau, work, -1)
					lwork = (int(work[0]) + 1) / 2
					lwork = max(1, lwork)
				case optimumWork:
					work := make([]float64, 1)
					impl.Dsytrd(uplo, n, a.Data, a.Stride, d, e, tau, work, -1)
					lwork = int(work[0])
				}
				work := make([]float64, lwork)

				impl.Dsytrd(uplo, n, a.Data, a.Stride, d, e, tau, work, lwork)

				prefix := fmt.Sprintf("Case #%v: uplo=%v,n=%v,lda=%v,work=%v",
					tc, uplo, n, lda, wl)

				if !generalOutsideAllNaN(a) {
					t.Errorf("%v: out-of-range write to A", prefix)
				}

				// Extract Q by doing what Dorgtr does.
				q := cloneGeneral(a)
				if uplo == blas.Upper {
					for j := 0; j < n-1; j++ {
						for i := 0; i < j; i++ {
							q.Data[i*q.Stride+j] = q.Data[i*q.Stride+j+1]
						}
						q.Data[(n-1)*q.Stride+j] = 0
					}
					for i := 0; i < n-1; i++ {
						q.Data[i*q.Stride+n-1] = 0
					}
					q.Data[(n-1)*q.Stride+n-1] = 1
					if n > 1 {
						work = make([]float64, n-1)
						impl.Dorgql(n-1, n-1, n-1, q.Data, q.Stride, tau, work, len(work))
					}
				} else {
					for j := n - 1; j > 0; j-- {
						q.Data[j] = 0
						for i := j + 1; i < n; i++ {
							q.Data[i*q.Stride+j] = q.Data[i*q.Stride+j-1]
						}
					}
					q.Data[0] = 1
					for i := 1; i < n; i++ {
						q.Data[i*q.Stride] = 0
					}
					if n > 1 {
						work = make([]float64, n-1)
						impl.Dorgqr(n-1, n-1, n-1, q.Data[q.Stride+1:], q.Stride, tau, work, len(work))
					}
				}
				if !isOrthogonal(q) {
					t.Errorf("%v: Q not orthogonal", prefix)
				}

				// Contruct symmetric tridiagonal T from d and e.
				tMat := zeros(n, n, n)
				for i := 0; i < n; i++ {
					tMat.Data[i*tMat.Stride+i] = d[i]
				}
				if uplo == blas.Upper {
					for j := 1; j < n; j++ {
						tMat.Data[(j-1)*tMat.Stride+j] = e[j-1]
						tMat.Data[j*tMat.Stride+j-1] = e[j-1]
					}
				} else {
					for j := 0; j < n-1; j++ {
						tMat.Data[(j+1)*tMat.Stride+j] = e[j]
						tMat.Data[j*tMat.Stride+j+1] = e[j]
					}
				}

				// Compute Q^T * A * Q.
				tmp := zeros(n, n, n)
				blas64.Gemm(blas.Trans, blas.NoTrans, 1, q, aCopy, 0, tmp)
				got := zeros(n, n, n)
				blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, tmp, q, 0, got)

				// Compare with T.
				if !equalApproxGeneral(got, tMat, tol) {
					t.Errorf("%v: Q^T*A*Q != T", prefix)
				}
			}
		}
	}
}
