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

type Dlauu2er interface {
	Dlauu2(uplo blas.Uplo, n int, a []float64, lda int)
}

func Dlauu2Test(t *testing.T, impl Dlauu2er) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		name := "Upper"
		if uplo == blas.Lower {
			name = "Lower"
		}
		t.Run(name, func(t *testing.T) {
			ns := []int{0, 1, 2, 3, 4, 5, 10, 25}
			dlauuTest(t, impl.Dlauu2, uplo, ns)
		})
	}
}

func dlauuTest(t *testing.T, dlauu func(blas.Uplo, int, []float64, int), uplo blas.Uplo, ns []int) {
	const tol = 1e-13

	bi := blas64.Implementation()
	rnd := rand.New(rand.NewSource(1))

	for _, n := range ns {
		for _, lda := range []int{max(1, n), n + 11} {
			prefix := fmt.Sprintf("n=%v,lda=%v", n, lda)

			// Allocate n×n matrix A and fill it with random numbers.
			// Only its uplo triangle will be used below.
			a := make([]float64, n*lda)
			for i := range a {
				a[i] = rnd.NormFloat64()
			}
			// Create a copy of A.
			aCopy := make([]float64, len(a))
			copy(aCopy, a)

			// Compute U*U^T or L^T*L using Dlauu?.
			dlauu(uplo, n, a, lda)

			if n == 0 {
				continue
			}

			// * Check that the triangle of A opposite to uplo has not been modified.
			// * Convert the result of Dlauu? into a dense symmetric matrix.
			// * Zero out the triangle in aCopy opposite to uplo.
			if uplo == blas.Upper {
				if !sameLowerTri(n, aCopy, lda, a, lda) {
					t.Errorf("%v: unexpected modification in lower triangle", prefix)
					continue
				}
				for i := 1; i < n; i++ {
					for j := 0; j < i; j++ {
						a[i*lda+j] = a[j*lda+i]
						aCopy[i*lda+j] = 0
					}
				}
			} else {
				if !sameUpperTri(n, aCopy, lda, a, lda) {
					t.Errorf("%v: unexpected modification in upper triangle", prefix)
					continue
				}
				for i := 0; i < n-1; i++ {
					for j := i + 1; j < n; j++ {
						a[i*lda+j] = a[j*lda+i]
						aCopy[i*lda+j] = 0
					}
				}
			}

			// Compute U*U^T or L^T*L using Dgemm with U and L
			// represented as dense triangular matrices.
			ldwant := n
			want := make([]float64, n*ldwant)
			if uplo == blas.Upper {
				// Use aCopy as a dense representation of the upper triangular U.
				u := aCopy
				ldu := lda
				// Compute U * U^T and store the result into want.
				bi.Dgemm(blas.NoTrans, blas.Trans, n, n, n,
					1, u, ldu, u, ldu, 0, want, ldwant)
			} else {
				// Use aCopy as a dense representation of the lower triangular L.
				l := aCopy
				ldl := lda
				// Compute L^T * L and store the result into want.
				bi.Dgemm(blas.Trans, blas.NoTrans, n, n, n,
					1, l, ldl, l, ldl, 0, want, ldwant)
			}
			if !equalApprox(n, n, a, lda, want, tol) {
				t.Errorf("%v: unexpected result", prefix)
			}
		}
	}
}
