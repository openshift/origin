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

type Ztrsmer interface {
	Ztrsm(side blas.Side, uplo blas.Uplo, transA blas.Transpose, diag blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int)
}

func ZtrsmTest(t *testing.T, impl Ztrsmer) {
	for _, side := range []blas.Side{blas.Left, blas.Right} {
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
				for _, diag := range []blas.Diag{blas.Unit, blas.NonUnit} {
					name := sideString(side) + "-" + uploString(uplo) + "-" + transString(trans) + "-" + diagString(diag)
					t.Run(name, func(t *testing.T) {
						for _, m := range []int{0, 1, 2, 3, 4, 5} {
							for _, n := range []int{0, 1, 2, 3, 4, 5} {
								ztrsmTest(t, impl, side, uplo, trans, diag, m, n)
							}
						}
					})
				}
			}
		}
	}
}

func ztrsmTest(t *testing.T, impl Ztrsmer, side blas.Side, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, m, n int) {
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
				// Set some elements of A to 0 and 1 to cover special cases in Ztrsm.
				if nA > 2 {
					if uplo == blas.Upper {
						a[nA-2] = 1
						a[nA-1] = 0
					} else {
						a[(nA-2)*lda] = 1
						a[(nA-1)*lda] = 0
					}
				}
				// Create a copy of A for checking that Ztrsm
				// does not modify its triangle opposite to uplo.
				aCopy := make([]complex128, len(a))
				copy(aCopy, a)
				// Create a dense representation of A for
				// computing the right-hand side matrix using zmm.
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

				// Allocate the right-hand side matrix B and fill it with random numbers.
				b := make([]complex128, m*ldb)
				for i := range b {
					b[i] = rndComplex128(rnd)
				}
				// Set some elements of B to 0 to cover special cases in Ztrsm.
				if m > 1 && n > 1 {
					b[0] = 0
					b[(m-1)*ldb+n-1] = 0
				}
				bCopy := make([]complex128, len(b))
				copy(bCopy, b)

				// Compute the solution matrix X using Ztrsm.
				// X is overwritten on B.
				impl.Ztrsm(side, uplo, trans, diag, m, n, alpha, a, lda, b, ldb)
				x := b

				prefix := fmt.Sprintf("m=%v,n=%v,lda=%v,ldb=%v,alpha=%v", m, n, lda, ldb, alpha)

				if !zsame(a, aCopy) {
					t.Errorf("%v: unexpected modification of A", prefix)
					continue
				}

				// Compute the left-hand side matrix of op(A)*X=alpha*B or X*op(A)=alpha*B
				// using an internal Zgemm implementation.
				var lhs []complex128
				if side == blas.Left {
					lhs = zmm(trans, blas.NoTrans, m, n, m, 1, aTri, lda, x, ldb, 0, b, ldb)
				} else {
					lhs = zmm(blas.NoTrans, trans, m, n, n, 1, x, ldb, aTri, lda, 0, b, ldb)
				}

				// Compute the right-hand side matrix alpha*B.
				rhs := bCopy
				for i := 0; i < m; i++ {
					for j := 0; j < n; j++ {
						rhs[i*ldb+j] *= alpha
					}
				}

				if !zEqualApprox(lhs, rhs, tol) {
					t.Errorf("%v: unexpected result", prefix)
				}
			}
		}
	}
}
