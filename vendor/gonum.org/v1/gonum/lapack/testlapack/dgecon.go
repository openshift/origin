// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dgeconer interface {
	Dgecon(norm lapack.MatrixNorm, n int, a []float64, lda int, anorm float64, work []float64, iwork []int) float64

	Dgetrier
	Dlanger
}

func DgeconTest(t *testing.T, impl Dgeconer) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, lda := range []int{max(1, n), n + 3} {
			dgeconTest(t, impl, rnd, n, lda)
		}
	}
}

func dgeconTest(t *testing.T, impl Dgeconer, rnd *rand.Rand, n, lda int) {
	const ratioThresh = 10

	// Generate a random square matrix A with elements uniformly in [-1,1).
	a := make([]float64, max(0, (n-1)*lda+n))
	for i := range a {
		a[i] = 2*rnd.Float64() - 1
	}

	// Allocate work slices.
	iwork := make([]int, n)
	work := make([]float64, max(1, 4*n))

	// Compute the LU factorization of A.
	aFac := make([]float64, len(a))
	copy(aFac, a)
	ipiv := make([]int, n)
	ok := impl.Dgetrf(n, n, aFac, lda, ipiv)
	if !ok {
		t.Fatalf("n=%v,lda=%v: bad matrix, Dgetrf failed", n, lda)
	}
	aFacCopy := make([]float64, len(aFac))
	copy(aFacCopy, aFac)

	// Compute the inverse A^{-1} from the LU factorization.
	aInv := make([]float64, len(aFac))
	copy(aInv, aFac)
	ok = impl.Dgetri(n, aInv, lda, ipiv, work, len(work))
	if !ok {
		t.Fatalf("n=%v,lda=%v: bad matrix, Dgetri failed", n, lda)
	}

	for _, norm := range []lapack.MatrixNorm{lapack.MaxColumnSum, lapack.MaxRowSum} {
		name := fmt.Sprintf("norm%v,n=%v,lda=%v", string(norm), n, lda)

		// Compute the norm of A and A^{-1}.
		aNorm := impl.Dlange(norm, n, n, a, lda, work)
		aInvNorm := impl.Dlange(norm, n, n, aInv, lda, work)

		// Compute a good estimate of the condition number
		//  rcondWant := 1/(norm(A) * norm(inv(A)))
		rcondWant := 1.0
		if aNorm > 0 && aInvNorm > 0 {
			rcondWant = 1 / aNorm / aInvNorm
		}

		// Compute an estimate of rcond using the LU factorization and Dgecon.
		rcondGot := impl.Dgecon(norm, n, aFac, lda, aNorm, work, iwork)
		if !floats.Equal(aFac, aFacCopy) {
			t.Errorf("%v: unexpected modification of aFac", name)
		}

		ratio := rCondTestRatio(rcondGot, rcondWant)
		if ratio >= ratioThresh {
			t.Errorf("%v: unexpected value of rcond; got=%v, want=%v (ratio=%v)",
				name, rcondGot, rcondWant, ratio)
		}
	}
}
