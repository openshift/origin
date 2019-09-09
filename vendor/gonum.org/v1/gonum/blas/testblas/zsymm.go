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

type Zsymmer interface {
	Zsymm(side blas.Side, uplo blas.Uplo, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int)
}

func ZsymmTest(t *testing.T, impl Zsymmer) {
	for _, side := range []blas.Side{blas.Left, blas.Right} {
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			name := sideString(side) + "-" + uploString(uplo)
			t.Run(name, func(t *testing.T) {
				for _, m := range []int{0, 1, 2, 3, 4, 5} {
					for _, n := range []int{0, 1, 2, 3, 4, 5} {
						zsymmTest(t, impl, side, uplo, m, n)
					}
				}
			})
		}
	}
}

func zsymmTest(t *testing.T, impl Zsymmer, side blas.Side, uplo blas.Uplo, m, n int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	nA := m
	if side == blas.Right {
		nA = n
	}
	for _, lda := range []int{max(1, nA), nA + 2} {
		for _, ldb := range []int{max(1, n), n + 3} {
			for _, ldc := range []int{max(1, n), n + 4} {
				for _, alpha := range []complex128{0, 1, complex(0.7, -0.9)} {
					for _, beta := range []complex128{0, 1, complex(1.3, -1.1)} {
						// Allocate the matrix A and fill it with random numbers.
						a := make([]complex128, nA*lda)
						for i := range a {
							a[i] = rndComplex128(rnd)
						}
						// Create a copy of A for checking that
						// Zsymm does not modify its triangle
						// opposite to uplo.
						aCopy := make([]complex128, len(a))
						copy(aCopy, a)
						// Create a copy of A expanded into a
						// full symmetric matrix for computing
						// the expected result using zmm.
						aSym := make([]complex128, len(a))
						copy(aSym, a)
						if uplo == blas.Upper {
							for i := 0; i < nA-1; i++ {
								for j := i + 1; j < nA; j++ {
									aSym[j*lda+i] = aSym[i*lda+j]
								}
							}
						} else {
							for i := 1; i < nA; i++ {
								for j := 0; j < i; j++ {
									aSym[j*lda+i] = aSym[i*lda+j]
								}
							}
						}

						// Allocate the matrix B and fill it with random numbers.
						b := make([]complex128, m*ldb)
						for i := range b {
							b[i] = rndComplex128(rnd)
						}
						// Create a copy of B for checking that
						// Zsymm does not modify B.
						bCopy := make([]complex128, len(b))
						copy(bCopy, b)

						// Allocate the matrix C and fill it with random numbers.
						c := make([]complex128, m*ldc)
						for i := range c {
							c[i] = rndComplex128(rnd)
						}
						// Create a copy of C for checking that
						// Zsymm does not modify C.
						cCopy := make([]complex128, len(c))
						copy(cCopy, c)

						// Compute the expected result using an internal Zgemm implementation.
						var want []complex128
						if side == blas.Left {
							want = zmm(blas.NoTrans, blas.NoTrans, m, n, m, alpha, aSym, lda, b, ldb, beta, c, ldc)
						} else {
							want = zmm(blas.NoTrans, blas.NoTrans, m, n, n, alpha, b, ldb, aSym, lda, beta, c, ldc)
						}

						// Compute the result using Zsymm.
						impl.Zsymm(side, uplo, m, n, alpha, a, lda, b, ldb, beta, c, ldc)

						prefix := fmt.Sprintf("m=%v,n=%v,lda=%v,ldb=%v,ldc=%v,alpha=%v,beta=%v", m, n, lda, ldb, ldc, alpha, beta)

						if !zsame(a, aCopy) {
							t.Errorf("%v: unexpected modification of A", prefix)
							continue
						}
						if !zsame(b, bCopy) {
							t.Errorf("%v: unexpected modification of B", prefix)
							continue
						}

						if !zEqualApprox(c, want, tol) {
							t.Errorf("%v: unexpected result", prefix)
						}
					}
				}
			}
		}
	}
}
