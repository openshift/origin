// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dgelqfer interface {
	Dgelq2er
	Dgelqf(m, n int, a []float64, lda int, tau, work []float64, lwork int)
}

func DgelqfTest(t *testing.T, impl Dgelqfer) {
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
		{10, 5, 30},
		{5, 10, 30},
		{10, 10, 30},
		{300, 5, 500},
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
			lda = n
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
			tau[i] = rnd.NormFloat64()
		}

		// Compute the expected result using unblocked LQ algorithm and
		// store it want.
		want := make([]float64, len(a))
		copy(want, a)
		impl.Dgelq2(m, n, want, lda, tau, make([]float64, m))

		for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
			copy(a, aCopy)

			var lwork int
			switch wl {
			case minimumWork:
				lwork = m
			case mediumWork:
				work := make([]float64, 1)
				impl.Dgelqf(m, n, a, lda, tau, work, -1)
				lwork = int(work[0]) - 2*m
			case optimumWork:
				work := make([]float64, 1)
				impl.Dgelqf(m, n, a, lda, tau, work, -1)
				lwork = int(work[0])
			}
			work := make([]float64, lwork)

			// Compute the LQ factorization of A.
			impl.Dgelqf(m, n, a, lda, tau, work, len(work))
			// Compare the result with Dgelq2.
			if !floats.EqualApprox(want, a, tol) {
				t.Errorf("Case %v, workspace type %v, unexpected result", c, wl)
			}
		}
	}
}
