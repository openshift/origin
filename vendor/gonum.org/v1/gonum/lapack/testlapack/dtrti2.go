// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

type Dtrti2er interface {
	Dtrti2(uplo blas.Uplo, diag blas.Diag, n int, a []float64, lda int)
}

func Dtrti2Test(t *testing.T, impl Dtrti2er) {
	const tol = 1e-14
	for _, test := range []struct {
		a    []float64
		n    int
		uplo blas.Uplo
		diag blas.Diag
		ans  []float64
	}{
		{
			a: []float64{
				2, 3, 4,
				0, 5, 6,
				8, 0, 8},
			n:    3,
			uplo: blas.Upper,
			diag: blas.NonUnit,
			ans: []float64{
				0.5, -0.3, -0.025,
				0, 0.2, -0.15,
				8, 0, 0.125,
			},
		},
		{
			a: []float64{
				5, 3, 4,
				0, 7, 6,
				10, 0, 8},
			n:    3,
			uplo: blas.Upper,
			diag: blas.Unit,
			ans: []float64{
				5, -3, 14,
				0, 7, -6,
				10, 0, 8,
			},
		},
		{
			a: []float64{
				2, 0, 0,
				3, 5, 0,
				4, 6, 8},
			n:    3,
			uplo: blas.Lower,
			diag: blas.NonUnit,
			ans: []float64{
				0.5, 0, 0,
				-0.3, 0.2, 0,
				-0.025, -0.15, 0.125,
			},
		},
		{
			a: []float64{
				1, 0, 0,
				3, 1, 0,
				4, 6, 1},
			n:    3,
			uplo: blas.Lower,
			diag: blas.Unit,
			ans: []float64{
				1, 0, 0,
				-3, 1, 0,
				14, -6, 1,
			},
		},
	} {
		impl.Dtrti2(test.uplo, test.diag, test.n, test.a, test.n)
		if !floats.EqualApprox(test.ans, test.a, tol) {
			t.Errorf("Matrix inverse mismatch. Want %v, got %v.", test.ans, test.a)
		}
	}
	rnd := rand.New(rand.NewSource(1))
	bi := blas64.Implementation()
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
			for _, test := range []struct {
				n, lda int
			}{
				{1, 0},
				{2, 0},
				{3, 0},
				{1, 5},
				{2, 5},
				{3, 5},
			} {
				n := test.n
				lda := test.lda
				if lda == 0 {
					lda = n
				}
				// Allocate n×n matrix A and fill it with random numbers.
				a := make([]float64, n*lda)
				for i := range a {
					a[i] = rnd.Float64()
				}
				for i := 0; i < n; i++ {
					// This keeps the matrices well conditioned.
					a[i*lda+i] += float64(n)
				}
				aCopy := make([]float64, len(a))
				copy(aCopy, a)
				// Compute the inverse of the uplo triangle.
				impl.Dtrti2(uplo, diag, n, a, lda)
				// Zero out the opposite triangle.
				if uplo == blas.Upper {
					for i := 1; i < n; i++ {
						for j := 0; j < i; j++ {
							aCopy[i*lda+j] = 0
							a[i*lda+j] = 0
						}
					}
				} else {
					for i := 0; i < n; i++ {
						for j := i + 1; j < n; j++ {
							aCopy[i*lda+j] = 0
							a[i*lda+j] = 0
						}
					}
				}
				if diag == blas.Unit {
					// Set the diagonal of A^{-1} and A explicitly to 1.
					for i := 0; i < n; i++ {
						a[i*lda+i] = 1
						aCopy[i*lda+i] = 1
					}
				}
				// Compute A^{-1} * A and store the result in ans.
				ans := make([]float64, len(a))
				bi.Dgemm(blas.NoTrans, blas.NoTrans, n, n, n, 1, a, lda, aCopy, lda, 0, ans, lda)
				// Check that ans is close to the identity matrix.
				dist := distFromIdentity(n, ans, lda)
				if dist > tol {
					t.Errorf("|inv(A) * A - I| = %v. Upper = %v, unit = %v, ans = %v", dist, uplo == blas.Upper, diag == blas.Unit, ans)
				}
			}
		}
	}
}
