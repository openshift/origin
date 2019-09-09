// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
)

type Ztrmmer interface {
	Ztrmm(side blas.Side, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int)
}

func ZtrmmTest(t *testing.T, impl Ztrmmer) {
	for _, side := range []blas.Side{blas.Left, blas.Right} {
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
				for _, diag := range []blas.Diag{blas.Unit, blas.NonUnit} {
					name := sideString(side) + "-" + uploString(uplo) + "-" + transString(trans) + "-" + diagString(diag)
					t.Run(name, func(t *testing.T) {
						for _, m := range []int{0, 1, 2, 3, 4, 5} {
							for _, n := range []int{0, 1, 2, 3, 4, 5} {
								ztrmmTest(t, impl, side, uplo, trans, diag, m, n)
							}
						}
					})
				}
			}
		}
	}
}

func ztrmmTest(t *testing.T, impl Ztrmmer, side blas.Side, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, m, n int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	nA := m
	if side == blas.Right {
		nA = n
	}
	for _, lda := range []int{max(1, nA), nA + 2} {
		for _, ldb := range []int{max(1, n), n + 3} {
			for _, alpha := range []complex128{0, 1, complex(0.7, -0.9)} {
				// Allocate the matrix A and fill it with random numbers.
				a := make([]complex128, nA*lda)
				for i := range a {
					a[i] = rndComplex128(rnd)
				}
				// Put a zero into A to cover special cases in Ztrmm.
				if nA > 1 {
					if uplo == blas.Upper {
						a[nA-1] = 0
					} else {
						a[(nA-1)*lda] = 0
					}
				}
				// Create a copy of A for checking that Ztrmm
				// does not modify its triangle opposite to
				// uplo.
				aCopy := make([]complex128, len(a))
				copy(aCopy, a)
				// Create a dense representation of A for
				// computing the expected result using zmm.
				aTri := make([]complex128, len(a))
				copy(aTri, a)
				if uplo == blas.Upper {
					for i := 0; i < nA; i++ {
						// Zero out the lower triangle.
						for j := 0; j < i; j++ {
							aTri[i*lda+j] = 0
						}
						if diag == blas.Unit {
							aTri[i*lda+i] = 1
						}
					}
				} else {
					for i := 0; i < nA; i++ {
						if diag == blas.Unit {
							aTri[i*lda+i] = 1
						}
						// Zero out the upper triangle.
						for j := i + 1; j < nA; j++ {
							aTri[i*lda+j] = 0
						}
					}
				}

				// Allocate the matrix B and fill it with random numbers.
				b := make([]complex128, m*ldb)
				for i := range b {
					b[i] = rndComplex128(rnd)
				}
				// Put a zero into B to cover special cases in Ztrmm.
				if m > 0 && n > 0 {
					b[0] = 0
				}

				// Compute the expected result using an internal Zgemm implementation.
				var want []complex128
				if side == blas.Left {
					want = zmm(trans, blas.NoTrans, m, n, m, alpha, aTri, lda, b, ldb, 0, b, ldb)
				} else {
					want = zmm(blas.NoTrans, trans, m, n, n, alpha, b, ldb, aTri, lda, 0, b, ldb)
				}

				// Compute the result using Ztrmm.
				impl.Ztrmm(side, uplo, trans, diag, m, n, alpha, a, lda, b, ldb)

				prefix := fmt.Sprintf("m=%v,n=%v,lda=%v,ldb=%v,alpha=%v", m, n, lda, ldb, alpha)
				if !zsame(a, aCopy) {
					t.Errorf("%v: unexpected modification of A", prefix)
					continue
				}

				if !zEqualApprox(b, want, tol) {
					t.Errorf("%v: unexpected result", prefix)
				}
			}
		}
	}
}
