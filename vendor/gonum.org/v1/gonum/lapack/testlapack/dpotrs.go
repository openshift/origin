// Copyright ©2018 The Gonum Authors. All rights reserved.
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

type Dpotrser interface {
	Dpotrs(uplo blas.Uplo, n, nrhs int, a []float64, lda int, b []float64, ldb int)

	Dpotrf(uplo blas.Uplo, n int, a []float64, lda int) bool
}

func DpotrsTest(t *testing.T, impl Dpotrser) {
	const tol = 1e-14

	rnd := rand.New(rand.NewSource(1))
	bi := blas64.Implementation()

	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, n := range []int{1, 2, 5} {
			for _, nrhs := range []int{1, 2, 5, 10} {
				for _, ld := range []struct{ a, b int }{
					{n, nrhs},
					{n + 7, nrhs},
					{n, nrhs + 3},
					{n + 7, nrhs + 3},
				} {
					// Construct a random SPD matrix A by first making a symmetric matrix
					// and then ensuring that it is diagonally dominant.
					a := nanGeneral(n, n, ld.a)
					for i := 0; i < n; i++ {
						for j := i; j < n; j++ {
							v := rnd.Float64()
							a.Data[i*a.Stride+j] = v
							a.Data[j*a.Stride+i] = v
						}
					}
					for i := 0; i < n; i++ {
						a.Data[i*a.Stride+i] += float64(n)
					}

					// Generate a random solution X.
					want := nanGeneral(n, nrhs, ld.b)
					for i := 0; i < n; i++ {
						for j := 0; j < nrhs; j++ {
							want.Data[i*want.Stride+j] = rnd.NormFloat64()
						}
					}

					// Compute the right-hand side matrix as A * X.
					b := nanGeneral(n, nrhs, ld.b)
					bi.Dgemm(blas.NoTrans, blas.NoTrans, n, nrhs, n, 1, a.Data, a.Stride, want.Data, want.Stride, 0, b.Data, b.Stride)

					// Compute the Cholesky decomposition of A.
					ok := impl.Dpotrf(uplo, n, a.Data, a.Stride)
					if !ok {
						panic("bad test")
					}

					aCopy := cloneGeneral(a)

					// Solve A * X = B.
					impl.Dpotrs(uplo, n, nrhs, a.Data, a.Stride, b.Data, b.Stride)

					name := fmt.Sprintf("uplo=%v,n=%v,nrhs=%v,lda=%v,ldb=%v", uplo, n, nrhs, a.Stride, b.Stride)

					if !generalOutsideAllNaN(a) {
						t.Errorf("%v: out-of-range modification of A", name)
					}
					if !equalApproxGeneral(a, aCopy, 0) {
						t.Errorf("%v: unexpected modification of A", name)
					}
					if !generalOutsideAllNaN(b) {
						t.Errorf("%v: out-of-range modification of B", name)
					}
					if !equalApproxGeneral(b, want, tol) {
						t.Errorf("%v: unexpected result\ngot  %v\nwant %v", name, b, want)
					}
				}
			}
		}
	}
}
