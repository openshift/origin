// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dgeqrfer interface {
	Dgeqr2er
	Dgeqrf(m, n int, a []float64, lda int, tau, work []float64, lwork int)
}

func DgeqrfTest(t *testing.T, impl Dgeqrfer) {
	const tol = 1e-12
	rnd := rand.New(rand.NewSource(1))
	for c, test := range []struct {
		m, n, lda int
	}{
		{10, 5, 0},
		{5, 10, 0},
		{10, 10, 0},
		{300, 5, 0},
		{3, 500, 0},
		{200, 200, 0},
		{300, 200, 0},
		{204, 300, 0},
		{1, 3000, 0},
		{3000, 1, 0},
		{10, 5, 20},
		{5, 10, 20},
		{10, 10, 20},
		{300, 5, 400},
		{3, 500, 600},
		{200, 200, 300},
		{300, 200, 300},
		{204, 300, 400},
		{1, 3000, 4000},
		{3000, 1, 4000},
	} {
		m := test.m
		n := test.n
		lda := test.lda
		if lda == 0 {
			lda = test.n
		}

		// Allocate m×n matrix A and fill it with random numbers.
		a := make([]float64, m*lda)
		for i := range a {
			a[i] = rnd.NormFloat64()
		}
		// Store a copy of A for later comparison.
		aCopy := make([]float64, len(a))
		copy(aCopy, a)

		// Allocate a slice for scalar factors of elementary reflectors
		// and fill it with random numbers.
		tau := make([]float64, n)
		for i := 0; i < n; i++ {
			tau[i] = rnd.Float64()
		}

		// Compute the expected result using unblocked QR algorithm and
		// store it in want.
		want := make([]float64, len(a))
		copy(want, a)
		impl.Dgeqr2(m, n, want, lda, tau, make([]float64, n))

		for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
			copy(a, aCopy)

			var lwork int
			switch wl {
			case minimumWork:
				lwork = n
			case mediumWork:
				work := make([]float64, 1)
				impl.Dgeqrf(m, n, a, lda, tau, work, -1)
				lwork = int(work[0]) - 2*n
			case optimumWork:
				work := make([]float64, 1)
				impl.Dgeqrf(m, n, a, lda, tau, work, -1)
				lwork = int(work[0])
			}
			work := make([]float64, lwork)

			// Compute the QR factorization of A.
			impl.Dgeqrf(m, n, a, lda, tau, work, len(work))
			// Compare the result with Dgeqr2.
			if !floats.EqualApprox(want, a, tol) {
				t.Errorf("Case %v, workspace %v, unexpected result.", c, wl)
			}
		}
	}
}
