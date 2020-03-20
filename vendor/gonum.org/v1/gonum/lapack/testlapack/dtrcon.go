// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dtrconer interface {
	Dtrcon(norm lapack.MatrixNorm, uplo blas.Uplo, diag blas.Diag, n int, a []float64, lda int, work []float64, iwork []int) float64

	Dtrtri(uplo blas.Uplo, diag blas.Diag, n int, a []float64, lda int) bool
	Dlantr(norm lapack.MatrixNorm, uplo blas.Uplo, diag blas.Diag, m, n int, a []float64, lda int, work []float64) float64
}

func DtrconTest(t *testing.T, impl Dtrconer) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, lda := range []int{max(1, n), n + 3} {
					for _, mattype := range []int{0, 1, 2} {
						dtrconTest(t, impl, rnd, uplo, diag, n, lda, mattype)
					}
				}
			}
		}
	}
}

func dtrconTest(t *testing.T, impl Dtrconer, rnd *rand.Rand, uplo blas.Uplo, diag blas.Diag, n, lda, mattype int) {
	const ratioThresh = 10

	a := make([]float64, max(0, (n-1)*lda+n))
	for i := range a {
		a[i] = rnd.Float64()
	}
	switch mattype {
	default:
		panic("bad mattype")
	case 0:
		// Matrix filled with consecutive integer values.
		// For lapack.MaxRowSum norm (infinity-norm) these matrices
		// sometimes lead to a slightly inaccurate estimate of the condition
		// number.
		c := 2.0
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				a[i*lda+j] = c
				c += 1
			}
		}
	case 1:
		// Identity matrix.
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					a[i*lda+j] = 0
				}
			}
		} else {
			for i := 0; i < n; i++ {
				for j := 0; j < i; j++ {
					a[i*lda+j] = 0
				}
			}
		}
		if diag == blas.NonUnit {
			for i := 0; i < n; i++ {
				a[i*lda+i] = 1
			}
		}
	case 2:
		// Matrix filled with random values uniformly in [-1,1).
		// These matrices often lead to a slightly inaccurate estimate
		// of the condition number.
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				a[i*lda+j] = 2*rnd.Float64() - 1
			}
		}
	}
	aCopy := make([]float64, len(a))
	copy(aCopy, a)

	// Compute the inverse A^{-1}.
	aInv := make([]float64, len(a))
	copy(aInv, a)
	ok := impl.Dtrtri(uplo, diag, n, aInv, lda)
	if !ok {
		t.Fatalf("uplo=%v,diag=%v,n=%v,lda=%v,mattype=%v: bad matrix, Dtrtri failed", string(uplo), string(diag), n, lda, mattype)
	}

	work := make([]float64, 3*n)
	iwork := make([]int, n)
	for _, norm := range []lapack.MatrixNorm{lapack.MaxColumnSum, lapack.MaxRowSum} {
		name := fmt.Sprintf("norm=%v,uplo=%v,diag=%v,n=%v,lda=%v,mattype=%v", string(norm), string(uplo), string(diag), n, lda, mattype)

		// Compute the norm of A and A^{-1}.
		aNorm := impl.Dlantr(norm, uplo, diag, n, n, a, lda, work)
		aInvNorm := impl.Dlantr(norm, uplo, diag, n, n, aInv, lda, work)

		// Compute a good estimate of the condition number
		//  rcondWant := 1/(norm(A) * norm(inv(A)))
		rcondWant := 1.0
		if aNorm > 0 && aInvNorm > 0 {
			rcondWant = 1 / aNorm / aInvNorm
		}

		// Compute an estimate of rcond using Dtrcon.
		rcondGot := impl.Dtrcon(norm, uplo, diag, n, a, lda, work, iwork)
		if !floats.Equal(a, aCopy) {
			t.Errorf("%v: unexpected modification of a", name)
		}

		ratio := rCondTestRatio(rcondGot, rcondWant)
		if ratio >= ratioThresh {
			t.Errorf("%v: unexpected value of rcond; got=%v, want=%v (ratio=%v)",
				name, rcondGot, rcondWant, ratio)
		}
	}
}
