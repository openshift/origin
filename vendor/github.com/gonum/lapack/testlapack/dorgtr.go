// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/floats"
)

type Dorgtrer interface {
	Dorgtr(uplo blas.Uplo, n int, a []float64, lda int, tau, work []float64, lwork int)
	Dsytrder
}

func DorgtrTest(t *testing.T, impl Dorgtrer) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
			for _, test := range []struct {
				n, lda int
			}{
				{1, 0},
				{2, 0},
				{3, 0},
				{6, 0},
				{33, 0},
				{100, 0},

				{1, 3},
				{2, 5},
				{3, 7},
				{6, 10},
				{33, 50},
				{100, 120},
			} {
				n := test.n
				lda := test.lda
				if lda == 0 {
					lda = n
				}
				a := make([]float64, n*lda)
				for i := range a {
					a[i] = rnd.NormFloat64()
				}
				aCopy := make([]float64, len(a))
				copy(aCopy, a)

				d := make([]float64, n)
				e := make([]float64, n-1)
				tau := make([]float64, n-1)
				work := make([]float64, 1)
				impl.Dsytrd(uplo, n, a, lda, d, e, tau, work, -1)
				work = make([]float64, int(work[0]))
				impl.Dsytrd(uplo, n, a, lda, d, e, tau, work, len(work))

				var lwork int
				switch wl {
				case minimumWork:
					lwork = max(1, n-1)
				case mediumWork:
					work := make([]float64, 1)
					impl.Dorgtr(uplo, n, a, lda, tau, work, -1)
					lwork = (int(work[0]) + n - 1) / 2
					lwork = max(1, lwork)
				case optimumWork:
					work := make([]float64, 1)
					impl.Dorgtr(uplo, n, a, lda, tau, work, -1)
					lwork = int(work[0])
				}
				work = nanSlice(lwork)

				impl.Dorgtr(uplo, n, a, lda, tau, work, len(work))

				q := blas64.General{
					Rows:   n,
					Cols:   n,
					Stride: lda,
					Data:   a,
				}
				tri := blas64.General{
					Rows:   n,
					Cols:   n,
					Stride: n,
					Data:   make([]float64, n*n),
				}
				for i := 0; i < n; i++ {
					tri.Data[i*tri.Stride+i] = d[i]
					if i != n-1 {
						tri.Data[i*tri.Stride+i+1] = e[i]
						tri.Data[(i+1)*tri.Stride+i] = e[i]
					}
				}

				aMat := blas64.General{
					Rows:   n,
					Cols:   n,
					Stride: n,
					Data:   make([]float64, n*n),
				}
				if uplo == blas.Upper {
					for i := 0; i < n; i++ {
						for j := i; j < n; j++ {
							v := aCopy[i*lda+j]
							aMat.Data[i*aMat.Stride+j] = v
							aMat.Data[j*aMat.Stride+i] = v
						}
					}
				} else {
					for i := 0; i < n; i++ {
						for j := 0; j <= i; j++ {
							v := aCopy[i*lda+j]
							aMat.Data[i*aMat.Stride+j] = v
							aMat.Data[j*aMat.Stride+i] = v
						}
					}
				}

				tmp := blas64.General{Rows: n, Cols: n, Stride: n, Data: make([]float64, n*n)}
				blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, aMat, q, 0, tmp)

				ans := blas64.General{Rows: n, Cols: n, Stride: n, Data: make([]float64, n*n)}
				blas64.Gemm(blas.Trans, blas.NoTrans, 1, q, tmp, 0, ans)

				if !floats.EqualApprox(ans.Data, tri.Data, 1e-13) {
					t.Errorf("Recombination mismatch. n = %v, isUpper = %v", n, uplo == blas.Upper)
				}
			}
		}
	}
}
