// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dorg2rer interface {
	Dgeqrfer
	Dorg2r(m, n, k int, a []float64, lda int, tau []float64, work []float64)
}

func Dorg2rTest(t *testing.T, impl Dorg2rer) {
	rnd := rand.New(rand.NewSource(1))
	for ti, test := range []struct {
		m, n, k, lda int
	}{
		{3, 3, 0, 0},
		{4, 3, 0, 0},
		{3, 3, 2, 0},
		{4, 3, 2, 0},

		{5, 5, 0, 20},
		{5, 5, 3, 20},
		{10, 5, 0, 20},
		{10, 5, 2, 20},
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

		// Compute the QR decomposition of A.
		tau := make([]float64, min(m, n))
		work := make([]float64, 1)
		impl.Dgeqrf(m, n, a, lda, tau, work, -1)
		work = make([]float64, int(work[0]))
		impl.Dgeqrf(m, n, a, lda, tau, work, len(work))

		// Compute the matrix Q explicitly using the first k elementary reflectors.
		k := test.k
		if k == 0 {
			k = n
		}
		q := constructQK("QR", m, n, k, a, lda, tau)

		// Compute the matrix Q using Dorg2r.
		impl.Dorg2r(m, n, k, a, lda, tau, work)

		// Check that the first n columns of both results match.
		same := true
	loop:
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				if !floats.EqualWithinAbsOrRel(q.Data[i*q.Stride+j], a[i*lda+j], 1e-12, 1e-12) {
					same = false
					break loop
				}
			}
		}
		if !same {
			t.Errorf("Case %v: Q mismatch", ti)
		}
	}
}
