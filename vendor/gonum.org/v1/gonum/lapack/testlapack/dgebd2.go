// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"
)

type Dgebd2er interface {
	Dgebd2(m, n int, a []float64, lda int, d, e, tauq, taup, work []float64)
}

func Dgebd2Test(t *testing.T, impl Dgebd2er) {
	rnd := rand.New(rand.NewSource(1))
	for _, test := range []struct {
		m, n, lda int
	}{
		{3, 4, 0},
		{4, 3, 0},
		{3, 4, 10},
		{4, 3, 10},
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
		// Allocate slices for the main and off diagonal.
		nb := min(m, n)
		d := nanSlice(nb)
		e := nanSlice(nb - 1)
		// Allocate slices for scalar factors of elementary reflectors
		// and fill them with NaNs.
		tauP := nanSlice(nb)
		tauQ := nanSlice(nb)
		// Allocate workspace.
		work := nanSlice(max(m, n))

		// Reduce A to upper or lower bidiagonal form by an orthogonal
		// transformation.
		impl.Dgebd2(m, n, a, lda, d, e, tauQ, tauP, work)

		// Check that it holds Qᵀ * A * P = B where B is represented by
		// d and e.
		checkBidiagonal(t, m, n, nb, a, lda, d, e, tauP, tauQ, aCopy)
	}
}
