// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dgetrier interface {
	Dgetrfer
	Dgetri(n int, a []float64, lda int, ipiv []int, work []float64, lwork int) bool
}

func DgetriTest(t *testing.T, impl Dgetrier) {
	const tol = 1e-13
	rnd := rand.New(rand.NewSource(1))
	bi := blas64.Implementation()
	for _, test := range []struct {
		n, lda int
	}{
		{5, 0},
		{5, 8},
		{45, 0},
		{45, 50},
		{63, 70},
		{64, 70},
		{65, 0},
		{65, 70},
		{66, 70},
		{150, 0},
		{150, 250},
	} {
		n := test.n
		lda := test.lda
		if lda == 0 {
			lda = n
		}
		// Generate a random well conditioned matrix
		perm := rnd.Perm(n)
		a := make([]float64, n*lda)
		for i := 0; i < n; i++ {
			a[i*lda+perm[i]] = 1
		}
		for i := range a {
			a[i] += 0.01 * rnd.Float64()
		}
		aCopy := make([]float64, len(a))
		copy(aCopy, a)
		ipiv := make([]int, n)
		// Compute LU decomposition.
		impl.Dgetrf(n, n, a, lda, ipiv)
		// Test with various workspace sizes.
		for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
			ainv := make([]float64, len(a))
			copy(ainv, a)

			var lwork int
			switch wl {
			case minimumWork:
				lwork = max(1, n)
			case mediumWork:
				work := make([]float64, 1)
				impl.Dgetri(n, ainv, lda, ipiv, work, -1)
				lwork = max(int(work[0])-2*n, n)
			case optimumWork:
				work := make([]float64, 1)
				impl.Dgetri(n, ainv, lda, ipiv, work, -1)
				lwork = int(work[0])
			}
			work := make([]float64, lwork)

			// Compute inverse.
			ok := impl.Dgetri(n, ainv, lda, ipiv, work, lwork)
			if !ok {
				t.Errorf("Unexpected singular matrix.")
			}

			// Check that A(inv) * A = I.
			ans := make([]float64, len(ainv))
			bi.Dgemm(blas.NoTrans, blas.NoTrans, n, n, n, 1, aCopy, lda, ainv, lda, 0, ans, lda)
			// The tolerance is so high because computing matrix inverses is very unstable.
			dist := distFromIdentity(n, ans, lda)
			if dist > tol {
				t.Errorf("|Inv(A) * A - I|_inf = %v is too large. n = %v, lda = %v", dist, n, lda)
			}
		}
	}
}
