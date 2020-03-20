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
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dpbconer interface {
	Dpbcon(uplo blas.Uplo, n, kd int, ab []float64, ldab int, anorm float64, work []float64, iwork []int) float64

	Dpbtrser
	Dlanger
	Dlansber
}

// DpbconTest tests Dpbcon by generating a random symmetric band matrix A and
// checking that the estimated condition number is not too different from the
// condition number computed via the explicit inverse of A.
func DpbconTest(t *testing.T, impl Dpbconer) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, kd := range []int{0, (n + 1) / 4, (3*n - 1) / 4, (5*n + 1) / 4} {
			for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
				for _, ldab := range []int{kd + 1, kd + 1 + 3} {
					dpbconTest(t, impl, uplo, n, kd, ldab, rnd)
				}
			}
		}
	}
}

func dpbconTest(t *testing.T, impl Dpbconer, uplo blas.Uplo, n, kd, ldab int, rnd *rand.Rand) {
	const ratioThresh = 10

	name := fmt.Sprintf("uplo=%v,n=%v,kd=%v,ldab=%v", string(uplo), n, kd, ldab)

	// Generate a random symmetric positive definite band matrix.
	ab := randSymBand(uplo, n, kd, ldab, rnd)

	// Compute the Cholesky decomposition of A.
	abFac := make([]float64, len(ab))
	copy(abFac, ab)
	ok := impl.Dpbtrf(uplo, n, kd, abFac, ldab)
	if !ok {
		t.Fatalf("%v: bad test matrix, Dpbtrf failed", name)
	}

	// Compute the norm of A.
	work := make([]float64, 3*n)
	aNorm := impl.Dlansb(lapack.MaxColumnSum, uplo, n, kd, ab, ldab, work)

	// Compute an estimate of rCond.
	iwork := make([]int, n)
	abFacCopy := make([]float64, len(abFac))
	copy(abFacCopy, abFac)
	rCondGot := impl.Dpbcon(uplo, n, kd, abFac, ldab, aNorm, work, iwork)

	if !floats.Equal(abFac, abFacCopy) {
		t.Errorf("%v: unexpected modification of ab", name)
	}

	// Form the inverse of A to compute a good estimate of the condition number
	//  rCondWant := 1/(norm(A) * norm(inv(A)))
	lda := max(1, n)
	aInv := make([]float64, n*lda)
	for i := 0; i < n; i++ {
		aInv[i*lda+i] = 1
	}
	impl.Dpbtrs(uplo, n, kd, n, abFac, ldab, aInv, lda)
	aInvNorm := impl.Dlange(lapack.MaxColumnSum, n, n, aInv, lda, work)
	rCondWant := 1.0
	if aNorm > 0 && aInvNorm > 0 {
		rCondWant = 1 / aNorm / aInvNorm
	}

	ratio := rCondTestRatio(rCondGot, rCondWant)
	if ratio >= ratioThresh {
		t.Errorf("%v: unexpected value of rcond. got=%v, want=%v (ratio=%v)", name, rCondGot, rCondWant, ratio)
	}
}

// rCondTestRatio returns a test ratio to compare two values of the reciprocal
// of the condition number.
//
// This function corresponds to DGET06 in Reference LAPACK.
func rCondTestRatio(rcond, rcondc float64) float64 {
	const eps = dlamchE
	switch {
	case rcond > 0 && rcondc > 0:
		return math.Max(rcond, rcondc)/math.Min(rcond, rcondc) - (1 - eps)
	case rcond > 0:
		return rcond / eps
	case rcondc > 0:
		return rcondc / eps
	default:
		return 0
	}
}
