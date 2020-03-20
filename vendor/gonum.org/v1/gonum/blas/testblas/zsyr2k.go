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

type Zsyr2ker interface {
	Zsyr2k(uplo blas.Uplo, trans blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int)
}

func Zsyr2kTest(t *testing.T, impl Zsyr2ker) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans} {
			name := uploString(uplo) + "-" + transString(trans)
			t.Run(name, func(t *testing.T) {
				for _, n := range []int{0, 1, 2, 3, 4, 5} {
					for _, k := range []int{0, 1, 2, 3, 4, 5, 7} {
						zsyr2kTest(t, impl, uplo, trans, n, k)
					}
				}
			})
		}
	}
}

func zsyr2kTest(t *testing.T, impl Zsyr2ker, uplo blas.Uplo, trans blas.Transpose, n, k int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	row, col := n, k
	if trans == blas.Trans {
		row, col = k, n
	}
	for _, lda := range []int{max(1, col), col + 2} {
		for _, ldb := range []int{max(1, col), col + 3} {
			for _, ldc := range []int{max(1, n), n + 4} {
				for _, alpha := range []complex128{0, 1, complex(0.7, -0.9)} {
					for _, beta := range []complex128{0, 1, complex(1.3, -1.1)} {
						// Allocate the matrix A and fill it with random numbers.
						a := make([]complex128, row*lda)
						for i := range a {
							a[i] = rndComplex128(rnd)
						}
						// Create a copy of A for checking that
						// Zsyr2k does not modify A.
						aCopy := make([]complex128, len(a))
						copy(aCopy, a)

						// Allocate the matrix B and fill it with random numbers.
						b := make([]complex128, row*ldb)
						for i := range b {
							b[i] = rndComplex128(rnd)
						}
						// Create a copy of B for checking that
						// Zsyr2k does not modify B.
						bCopy := make([]complex128, len(b))
						copy(bCopy, b)

						// Allocate the matrix C and fill it with random numbers.
						c := make([]complex128, n*ldc)
						for i := range c {
							c[i] = rndComplex128(rnd)
						}
						// Create a copy of C for checking that
						// Zsyr2k does not modify its triangle
						// opposite to uplo.
						cCopy := make([]complex128, len(c))
						copy(cCopy, c)
						// Create a copy of C expanded into a
						// full symmetric matrix for computing
						// the expected result using zmm.
						cSym := make([]complex128, len(c))
						copy(cSym, c)
						if uplo == blas.Upper {
							for i := 0; i < n-1; i++ {
								for j := i + 1; j < n; j++ {
									cSym[j*ldc+i] = cSym[i*ldc+j]
								}
							}
						} else {
							for i := 1; i < n; i++ {
								for j := 0; j < i; j++ {
									cSym[j*ldc+i] = cSym[i*ldc+j]
								}
							}
						}

						// Compute the expected result using an internal Zgemm implementation.
						var want []complex128
						if trans == blas.NoTrans {
							//  C = alpha*A*Bᵀ + alpha*B*Aᵀ + beta*C
							tmp := zmm(blas.NoTrans, blas.Trans, n, n, k, alpha, a, lda, b, ldb, beta, cSym, ldc)
							want = zmm(blas.NoTrans, blas.Trans, n, n, k, alpha, b, ldb, a, lda, 1, tmp, ldc)
						} else {
							//  C = alpha*Aᵀ*B + alpha*Bᵀ*A + beta*C
							tmp := zmm(blas.Trans, blas.NoTrans, n, n, k, alpha, a, lda, b, ldb, beta, cSym, ldc)
							want = zmm(blas.Trans, blas.NoTrans, n, n, k, alpha, b, ldb, a, lda, 1, tmp, ldc)
						}

						// Compute the result using Zsyr2k.
						impl.Zsyr2k(uplo, trans, n, k, alpha, a, lda, b, ldb, beta, c, ldc)

						prefix := fmt.Sprintf("n=%v,k=%v,lda=%v,ldb=%v,ldc=%v,alpha=%v,beta=%v", n, k, lda, ldb, ldc, alpha, beta)

						if !zsame(a, aCopy) {
							t.Errorf("%v: unexpected modification of A", prefix)
							continue
						}
						if !zsame(b, bCopy) {
							t.Errorf("%v: unexpected modification of B", prefix)
							continue
						}
						if uplo == blas.Upper && !zSameLowerTri(n, c, ldc, cCopy, ldc) {
							t.Errorf("%v: unexpected modification in lower triangle of C", prefix)
							continue
						}
						if uplo == blas.Lower && !zSameUpperTri(n, c, ldc, cCopy, ldc) {
							t.Errorf("%v: unexpected modification in upper triangle of C", prefix)
							continue
						}

						// Expand C into a full symmetric matrix
						// for comparison with the result from zmm.
						if uplo == blas.Upper {
							for i := 0; i < n-1; i++ {
								for j := i + 1; j < n; j++ {
									c[j*ldc+i] = c[i*ldc+j]
								}
							}
						} else {
							for i := 1; i < n; i++ {
								for j := 0; j < i; j++ {
									c[j*ldc+i] = c[i*ldc+j]
								}
							}
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
