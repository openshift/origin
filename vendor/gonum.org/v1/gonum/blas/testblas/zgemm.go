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

type Zgemmer interface {
	Zgemm(tA, tB blas.Transpose, m, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int)
}

func ZgemmTest(t *testing.T, impl Zgemmer) {
	for _, tA := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
		for _, tB := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			name := transString(tA) + "-" + transString(tB)
			t.Run(name, func(t *testing.T) {
				for _, m := range []int{0, 1, 2, 5, 10} {
					for _, n := range []int{0, 1, 2, 5, 10} {
						for _, k := range []int{0, 1, 2, 7, 11} {
							zgemmTest(t, impl, tA, tB, m, n, k)
						}
					}
				}
			})
		}
	}
}

func zgemmTest(t *testing.T, impl Zgemmer, tA, tB blas.Transpose, m, n, k int) {
	const tol = 1e-13

	rnd := rand.New(rand.NewSource(1))

	rowA, colA := m, k
	if tA != blas.NoTrans {
		rowA, colA = k, m
	}
	rowB, colB := k, n
	if tB != blas.NoTrans {
		rowB, colB = n, k
	}

	for _, lda := range []int{max(1, colA), colA + 2} {
		for _, ldb := range []int{max(1, colB), colB + 3} {
			for _, ldc := range []int{max(1, n), n + 4} {
				for _, alpha := range []complex128{0, 1, complex(0.7, -0.9)} {
					for _, beta := range []complex128{0, 1, complex(1.3, -1.1)} {
						// Allocate the matrix A and fill it with random numbers.
						a := make([]complex128, rowA*lda)
						for i := range a {
							a[i] = rndComplex128(rnd)
						}
						// Create a copy of A.
						aCopy := make([]complex128, len(a))
						copy(aCopy, a)

						// Allocate the matrix B and fill it with random numbers.
						b := make([]complex128, rowB*ldb)
						for i := range b {
							b[i] = rndComplex128(rnd)
						}
						// Create a copy of B.
						bCopy := make([]complex128, len(b))
						copy(bCopy, b)

						// Allocate the matrix C and fill it with random numbers.
						c := make([]complex128, m*ldc)
						for i := range c {
							c[i] = rndComplex128(rnd)
						}

						// Compute the expected result using an internal Zgemm implementation.
						want := zmm(tA, tB, m, n, k, alpha, a, lda, b, ldb, beta, c, ldc)

						// Compute a result using Zgemm.
						impl.Zgemm(tA, tB, m, n, k, alpha, a, lda, b, ldb, beta, c, ldc)

						prefix := fmt.Sprintf("m=%v,n=%v,k=%v,lda=%v,ldb=%v,ldc=%v,alpha=%v,beta=%v", m, n, k, lda, ldb, ldc, alpha, beta)

						if !zsame(a, aCopy) {
							t.Errorf("%v: unexpected modification of A", prefix)
							continue
						}
						if !zsame(b, bCopy) {
							t.Errorf("%v: unexpected modification of B", prefix)
							continue
						}

						if !zEqualApprox(c, want, tol) {
							t.Errorf("%v: unexpected result,\nwant=%v\ngot =%v\n", prefix, want, c)
						}
					}
				}
			}
		}
	}
}
