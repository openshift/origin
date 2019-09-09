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

type Zherker interface {
	Zherk(uplo blas.Uplo, trans blas.Transpose, n, k int, alpha float64, a []complex128, lda int, beta float64, c []complex128, ldc int)
}

func ZherkTest(t *testing.T, impl Zherker) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.ConjTrans} {
			name := uploString(uplo) + "-" + transString(trans)
			t.Run(name, func(t *testing.T) {
				for _, n := range []int{0, 1, 2, 3, 4, 5} {
					for _, k := range []int{0, 1, 2, 3, 4, 5, 7} {
						zherkTest(t, impl, uplo, trans, n, k)
					}
				}
			})
		}
	}
}

func zherkTest(t *testing.T, impl Zherker, uplo blas.Uplo, trans blas.Transpose, n, k int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	rowA, colA := n, k
	if trans == blas.ConjTrans {
		rowA, colA = k, n
	}
	for _, lda := range []int{max(1, colA), colA + 2} {
		for _, ldc := range []int{max(1, n), n + 4} {
			for _, alpha := range []float64{0, 1, 0.7} {
				for _, beta := range []float64{0, 1, 1.3} {
					// Allocate the matrix A and fill it with random numbers.
					a := make([]complex128, rowA*lda)
					for i := range a {
						a[i] = rndComplex128(rnd)
					}
					// Create a copy of A for checking that
					// Zherk does not modify A.
					aCopy := make([]complex128, len(a))
					copy(aCopy, a)

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
					// Zherk does not modify its triangle
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
						want = zmm(blas.NoTrans, blas.ConjTrans, n, n, k, complex(alpha, 0), a, lda, a, lda, complex(beta, 0), cHer, ldc)
					} else {
						want = zmm(blas.ConjTrans, blas.NoTrans, n, n, k, complex(alpha, 0), a, lda, a, lda, complex(beta, 0), cHer, ldc)
					}

					// Compute the result using Zherk.
					impl.Zherk(uplo, trans, n, k, alpha, a, lda, beta, c, ldc)

					prefix := fmt.Sprintf("n=%v,k=%v,lda=%v,ldc=%v,alpha=%v,beta=%v", n, k, lda, ldc, alpha, beta)

					if !zsame(a, aCopy) {
						t.Errorf("%v: unexpected modification of A", prefix)
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
