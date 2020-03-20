// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dtrtrier interface {
	Dtrtri(uplo blas.Uplo, diag blas.Diag, n int, a []float64, lda int) bool
}

func DtrtriTest(t *testing.T, impl Dtrtrier) {
	const tol = 1e-10
	rnd := rand.New(rand.NewSource(1))
	bi := blas64.Implementation()
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
			for _, test := range []struct {
				n, lda int
			}{
				{3, 0},
				{70, 0},
				{200, 0},
				{3, 5},
				{70, 92},
				{200, 205},
			} {
				n := test.n
				lda := test.lda
				if lda == 0 {
					lda = n
				}
				// Allocate n×n matrix A and fill it with random numbers.
				a := make([]float64, n*lda)
				for i := range a {
					a[i] = rnd.Float64()
				}
				for i := 0; i < n; i++ {
					// This keeps the matrices well conditioned.
					a[i*lda+i] += float64(n)
				}
				aCopy := make([]float64, len(a))
				copy(aCopy, a)
				// Compute the inverse of the uplo triangle.
				impl.Dtrtri(uplo, diag, n, a, lda)
				// Zero out the opposite triangle.
				if uplo == blas.Upper {
					for i := 1; i < n; i++ {
						for j := 0; j < i; j++ {
							aCopy[i*lda+j] = 0
							a[i*lda+j] = 0
						}
					}
				} else {
					for i := 0; i < n; i++ {
						for j := i + 1; j < n; j++ {
							aCopy[i*lda+j] = 0
							a[i*lda+j] = 0
						}
					}
				}
				if diag == blas.Unit {
					// Set the diagonal explicitly to 1.
					for i := 0; i < n; i++ {
						a[i*lda+i] = 1
						aCopy[i*lda+i] = 1
					}
				}
				// Compute A^{-1} * A and store the result in ans.
				ans := make([]float64, len(a))
				bi.Dgemm(blas.NoTrans, blas.NoTrans, n, n, n, 1, a, lda, aCopy, lda, 0, ans, lda)
				// Check that ans is the identity matrix.
				dist := distFromIdentity(n, ans, lda)
				if dist > tol {
					t.Errorf("|inv(A) * A - I| = %v is too large. Upper = %v, unit = %v, n = %v, lda = %v",
						dist, uplo == blas.Upper, diag == blas.Unit, n, lda)
				}
			}
		}
	}
}
