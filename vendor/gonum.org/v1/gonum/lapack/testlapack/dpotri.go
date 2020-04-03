// Copyright ©2019 The Gonum Authors. All rights reserved.
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

type Dpotrier interface {
	Dpotri(uplo blas.Uplo, n int, a []float64, lda int) bool

	Dpotrf(uplo blas.Uplo, n int, a []float64, lda int) bool
}

func DpotriTest(t *testing.T, impl Dpotrier) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		name := "Upper"
		if uplo == blas.Lower {
			name = "Lower"
		}
		t.Run(name, func(t *testing.T) {
			// Include small and large sizes to make sure that both
			// unblocked and blocked paths are taken.
			ns := []int{0, 1, 2, 3, 4, 5, 10, 25, 31, 32, 33, 63, 64, 65, 127, 128, 129}
			const tol = 1e-12

			bi := blas64.Implementation()
			rnd := rand.New(rand.NewSource(1))
			for _, n := range ns {
				for _, lda := range []int{max(1, n), n + 11} {
					prefix := fmt.Sprintf("n=%v,lda=%v", n, lda)

					// Generate a random diagonal matrix D with positive entries.
					d := make([]float64, n)
					Dlatm1(d, 3, 10000, false, 2, rnd)

					// Construct a positive definite matrix A as
					//  A = U * D * Uᵀ
					// where U is a random orthogonal matrix.
					a := make([]float64, n*lda)
					Dlagsy(n, 0, d, a, lda, rnd, make([]float64, 2*n))
					// Create a copy of A.
					aCopy := make([]float64, len(a))
					copy(aCopy, a)
					// Compute the Cholesky factorization of A.
					ok := impl.Dpotrf(uplo, n, a, lda)
					if !ok {
						t.Fatalf("%v: unexpected Cholesky failure", prefix)
					}

					// Compute the inverse inv(A).
					ok = impl.Dpotri(uplo, n, a, lda)
					if !ok {
						t.Errorf("%v: unexpected failure", prefix)
						continue
					}

					// Check that the triangle of A opposite to uplo has not been modified.
					if uplo == blas.Upper && !sameLowerTri(n, aCopy, lda, a, lda) {
						t.Errorf("%v: unexpected modification in lower triangle", prefix)
						continue
					}
					if uplo == blas.Lower && !sameUpperTri(n, aCopy, lda, a, lda) {
						t.Errorf("%v: unexpected modification in upper triangle", prefix)
						continue
					}

					// Change notation for the sake of clarity.
					ainv := a
					ldainv := lda

					// Expand ainv into a full dense matrix so that we can call Dsymm below.
					if uplo == blas.Upper {
						for i := 1; i < n; i++ {
							for j := 0; j < i; j++ {
								ainv[i*ldainv+j] = ainv[j*ldainv+i]
							}
						}
					} else {
						for i := 0; i < n-1; i++ {
							for j := i + 1; j < n; j++ {
								ainv[i*ldainv+j] = ainv[j*ldainv+i]
							}
						}
					}

					// Compute A*inv(A) and store the result into want.
					ldwant := max(1, n)
					want := make([]float64, n*ldwant)
					bi.Dsymm(blas.Left, uplo, n, n, 1, aCopy, lda, ainv, ldainv, 0, want, ldwant)

					// Check that want is close to the identity matrix.
					dist := distFromIdentity(n, want, ldwant)
					if dist > tol {
						t.Errorf("%v: |A * inv(A) - I| = %v is too large", prefix, dist)
					}
				}
			}
		})
	}
}
