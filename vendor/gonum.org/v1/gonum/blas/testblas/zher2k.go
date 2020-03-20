// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"math/cmplx"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/blas"
)

type Zher2ker interface {
	Zher2k(uplo blas.Uplo, trans blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta float64, c []complex128, ldc int)
}

func Zher2kTest(t *testing.T, impl Zher2ker) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.ConjTrans} {
			name := uploString(uplo) + "-" + transString(trans)
			t.Run(name, func(t *testing.T) {
				for _, n := range []int{0, 1, 2, 3, 4, 5} {
					for _, k := range []int{0, 1, 2, 3, 4, 5, 7} {
						zher2kTest(t, impl, uplo, trans, n, k)
					}
				}
			})
		}
	}
}

func zher2kTest(t *testing.T, impl Zher2ker, uplo blas.Uplo, trans blas.Transpose, n, k int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	row, col := n, k
	if trans == blas.ConjTrans {
		row, col = k, n
	}
	for _, lda := range []int{max(1, col), col + 2} {
		for _, ldb := range []int{max(1, col), col + 3} {
			for _, ldc := range []int{max(1, n), n + 4} {
				for _, alpha := range []complex128{0, 1, complex(0.7, -0.9)} {
					for _, beta := range []float64{0, 1, 1.3} {
						// Allocate the matrix A and fill it with random numbers.
						a := make([]complex128, row*lda)
						for i := range a {
							a[i] = rndComplex128(rnd)
						}
						// Create a copy of A for checking that
						// Zher2k does not modify A.
						aCopy := make([]complex128, len(a))
						copy(aCopy, a)

						// Allocate the matrix B and fill it with random numbers.
						b := make([]complex128, row*ldb)
						for i := range b {
							b[i] = rndComplex128(rnd)
						}
						// Create a copy of B for checking that
						// Zher2k does not modify B.
						bCopy := make([]complex128, len(b))
						copy(bCopy, b)

						// Allocate the matrix C and fill it with random numbers.
						c := make([]complex128, n*ldc)
						for i := range c {
							c[i] = rndComplex128(rnd)
						}
						if (alpha == 0 || k == 0) && beta == 1 {
							// In case of a quick return
							// zero out the diagonal.
							for i := 0; i < n; i++ {
								c[i*ldc+i] = complex(real(c[i*ldc+i]), 0)
							}
						}
						// Create a copy of C for checking that
						// Zher2k does not modify its triangle
						// opposite to uplo.
						cCopy := make([]complex128, len(c))
						copy(cCopy, c)
						// Create a copy of C expanded into a
						// full hermitian matrix for computing
						// the expected result using zmm.
						cHer := make([]complex128, len(c))
						copy(cHer, c)
						if uplo == blas.Upper {
							for i := 0; i < n; i++ {
								cHer[i*ldc+i] = complex(real(cHer[i*ldc+i]), 0)
								for j := i + 1; j < n; j++ {
									cHer[j*ldc+i] = cmplx.Conj(cHer[i*ldc+j])
								}
							}
						} else {
							for i := 0; i < n; i++ {
								for j := 0; j < i; j++ {
									cHer[j*ldc+i] = cmplx.Conj(cHer[i*ldc+j])
								}
								cHer[i*ldc+i] = complex(real(cHer[i*ldc+i]), 0)
							}
						}

						// Compute the expected result using an internal Zgemm implementation.
						var want []complex128
						if trans == blas.NoTrans {
							//  C = alpha*A*Bᴴ + conj(alpha)*B*Aᴴ + beta*C
							tmp := zmm(blas.NoTrans, blas.ConjTrans, n, n, k, alpha, a, lda, b, ldb, complex(beta, 0), cHer, ldc)
							want = zmm(blas.NoTrans, blas.ConjTrans, n, n, k, cmplx.Conj(alpha), b, ldb, a, lda, 1, tmp, ldc)
						} else {
							//  C = alpha*Aᴴ*B + conj(alpha)*Bᴴ*A + beta*C
							tmp := zmm(blas.ConjTrans, blas.NoTrans, n, n, k, alpha, a, lda, b, ldb, complex(beta, 0), cHer, ldc)
							want = zmm(blas.ConjTrans, blas.NoTrans, n, n, k, cmplx.Conj(alpha), b, ldb, a, lda, 1, tmp, ldc)
						}

						// Compute the result using Zher2k.
						impl.Zher2k(uplo, trans, n, k, alpha, a, lda, b, ldb, beta, c, ldc)

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

						// Check that the diagonal of C has only real elements.
						hasRealDiag := true
						for i := 0; i < n; i++ {
							if imag(c[i*ldc+i]) != 0 {
								hasRealDiag = false
								break
							}
						}
						if !hasRealDiag {
							t.Errorf("%v: diagonal of C has imaginary elements\ngot=%v", prefix, c)
							continue
						}

						// Expand C into a full hermitian matrix
						// for comparison with the result from zmm.
						if uplo == blas.Upper {
							for i := 0; i < n-1; i++ {
								for j := i + 1; j < n; j++ {
									c[j*ldc+i] = cmplx.Conj(c[i*ldc+j])
								}
							}
						} else {
							for i := 1; i < n; i++ {
								for j := 0; j < i; j++ {
									c[j*ldc+i] = cmplx.Conj(c[i*ldc+j])
								}
							}
						}
						if !zEqualApprox(c, want, tol) {
							t.Errorf("%v: unexpected result\nwant=%v\ngot= %v", prefix, want, c)
						}
					}
				}
			}
		}
	}
}
