// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas/blas64"
)

func TestDlagsy(t *testing.T) {
	const tol = 1e-14
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, lda := range []int{0, 2*n + 1} {
			if lda == 0 {
				lda = max(1, n)
			}
			// D is the identity matrix I.
			d := make([]float64, n)
			for i := range d {
				d[i] = 1
			}
			// Allocate an n×n symmetric matrix A and fill it with NaNs.
			a := nanSlice(n * lda)
			work := make([]float64, 2*n)
			// Compute A = U * D * Uᵀ where U is a random orthogonal matrix.
			Dlagsy(n, 0, d, a, lda, rnd, work)
			// A should be the identity matrix because
			//  A = U * D * Uᵀ = U * I * Uᵀ = U * Uᵀ = I.
			dist := distFromIdentity(n, a, lda)
			if dist > tol {
				t.Errorf("Case n=%v,lda=%v: |A-I|=%v is too large", n, lda, dist)
			}
		}
	}
}

func TestDlagge(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, lda := range []int{0, 2*n + 1} {
			if lda == 0 {
				lda = max(1, n)
			}
			d := make([]float64, n)
			for i := range d {
				d[i] = 1
			}
			a := blas64.General{
				Rows:   n,
				Cols:   n,
				Stride: lda,
				Data:   nanSlice(n * lda),
			}
			work := make([]float64, a.Rows+a.Cols)

			Dlagge(a.Rows, a.Cols, 0, 0, d, a.Data, a.Stride, rnd, work)

			if !isOrthogonal(a) {
				t.Errorf("Case n=%v,lda=%v: unexpected result", n, lda)
			}
		}
	}

}

func TestRandomOrthogonal(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for n := 1; n <= 20; n++ {
		q := randomOrthogonal(n, rnd)
		if !isOrthogonal(q) {
			t.Errorf("Case n=%v: Q not orthogonal", n)
		}
	}
}
