// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

type Dpbtrser interface {
	Dpbtrs(uplo blas.Uplo, n, kd, nrhs int, ab []float64, ldab int, b []float64, ldb int)

	Dpbtrfer
}

// DpbtrsTest tests Dpbtrs by comparing the computed and known, generated solutions of
// a linear system with a random symmetric positive definite band matrix.
func DpbtrsTest(t *testing.T, impl Dpbtrser) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 65, 100, 129} {
		for _, kd := range []int{0, (n + 1) / 4, (3*n - 1) / 4, (5*n + 1) / 4} {
			for _, nrhs := range []int{0, 1, 2, 5} {
				for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
					for _, ldab := range []int{kd + 1, kd + 1 + 3} {
						for _, ldb := range []int{max(1, nrhs), nrhs + 4} {
							dpbtrsTest(t, impl, rnd, uplo, n, kd, nrhs, ldab, ldb)
						}
					}
				}
			}
		}
	}
}

func dpbtrsTest(t *testing.T, impl Dpbtrser, rnd *rand.Rand, uplo blas.Uplo, n, kd, nrhs int, ldab, ldb int) {
	const tol = 1e-12

	name := fmt.Sprintf("uplo=%v,n=%v,kd=%v,nrhs=%v,ldab=%v,ldb=%v", string(uplo), n, kd, nrhs, ldab, ldb)

	// Generate a random symmetric positive definite band matrix.
	ab := randSymBand(uplo, n, kd, ldab, rnd)

	// Compute the Cholesky decomposition of A.
	abFac := make([]float64, len(ab))
	copy(abFac, ab)
	ok := impl.Dpbtrf(uplo, n, kd, abFac, ldab)
	if !ok {
		t.Fatalf("%v: bad test matrix, Dpbtrs failed", name)
	}
	abFacCopy := make([]float64, len(abFac))
	copy(abFacCopy, abFac)

	// Generate a random solution.
	xWant := make([]float64, n*ldb)
	for i := range xWant {
		xWant[i] = rnd.NormFloat64()
	}

	// Compute the corresponding right-hand side.
	bi := blas64.Implementation()
	b := make([]float64, len(xWant))
	if n > 0 {
		for j := 0; j < nrhs; j++ {
			bi.Dsbmv(uplo, n, kd, 1, ab, ldab, xWant[j:], ldb, 0, b[j:], ldb)
		}
	}

	// Solve  Uᵀ * U * X = B  or  L * Lᵀ * X = B.
	impl.Dpbtrs(uplo, n, kd, nrhs, abFac, ldab, b, ldb)
	xGot := b

	// Check that the Cholesky factorization matrix has not been modified.
	if !floats.Equal(abFac, abFacCopy) {
		t.Errorf("%v: unexpected modification of ab", name)
	}

	// Compute and check the max-norm difference between the computed and generated solutions.
	var diff float64
	for i := 0; i < n; i++ {
		for j := 0; j < nrhs; j++ {
			diff = math.Max(diff, math.Abs(xWant[i*ldb+j]-xGot[i*ldb+j]))
		}
	}
	if diff > tol {
		t.Errorf("%v: unexpected result, diff=%v", name, diff)
	}
}
