// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/lapack"
)

type Dlansyer interface {
	Dlanger
	Dlansy(norm lapack.MatrixNorm, uplo blas.Uplo, n int, a []float64, lda int, work []float64) float64
}

func DlansyTest(t *testing.T, impl Dlansyer) {
	rnd := rand.New(rand.NewSource(1))
	for _, norm := range []lapack.MatrixNorm{lapack.MaxAbs, lapack.MaxColumnSum, lapack.MaxRowSum, lapack.Frobenius} {
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			for _, test := range []struct {
				n, lda int
			}{
				{1, 0},
				{3, 0},

				{1, 10},
				{3, 10},
			} {
				for trial := 0; trial < 100; trial++ {
					n := test.n
					lda := test.lda
					if lda == 0 {
						lda = n
					}
					// Allocate n×n matrix A and fill it.
					// Only the uplo triangle of A will be used below
					// to represent a symmetric matrix.
					a := make([]float64, lda*n)
					if trial == 0 {
						// In the first trial fill the matrix
						// with predictable integers.
						for i := range a {
							a[i] = float64(i)
						}
					} else {
						// Otherwise fill it with random numbers.
						for i := range a {
							a[i] = rnd.NormFloat64()
						}
					}

					// Create a dense representation of the symmetric matrix
					// stored in the uplo triangle of A.
					aDense := make([]float64, n*n)
					if uplo == blas.Upper {
						for i := 0; i < n; i++ {
							for j := i; j < n; j++ {
								v := a[i*lda+j]
								aDense[i*n+j] = v
								aDense[j*n+i] = v
							}
						}
					} else {
						for i := 0; i < n; i++ {
							for j := 0; j <= i; j++ {
								v := a[i*lda+j]
								aDense[i*n+j] = v
								aDense[j*n+i] = v
							}
						}
					}

					work := make([]float64, n)
					// Compute the norm of the symmetric matrix A.
					got := impl.Dlansy(norm, uplo, n, a, lda, work)
					// Compute the reference norm value using Dlange
					// and the dense representation of A.
					want := impl.Dlange(norm, n, n, aDense, n, work)
					if math.Abs(want-got) > 1e-14 {
						t.Errorf("Norm mismatch. norm = %c, upper = %v, n = %v, lda = %v, want %v, got %v.",
							norm, uplo == blas.Upper, n, lda, got, want)
					}
				}
			}
		}
	}
}
