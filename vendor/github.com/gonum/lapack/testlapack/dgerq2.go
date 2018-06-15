// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
)

type Dgerq2er interface {
	Dgerq2(m, n int, a []float64, lda int, tau []float64, work []float64)
}

func Dgerq2Test(t *testing.T, impl Dgerq2er) {
	rnd := rand.New(rand.NewSource(1))
	for c, test := range []struct {
		m, n, lda int
	}{
		{1, 1, 0},
		{2, 2, 0},
		{3, 2, 0},
		{2, 3, 0},
		{1, 12, 0},
		{2, 6, 0},
		{3, 4, 0},
		{4, 3, 0},
		{6, 2, 0},
		{12, 1, 0},
		{1, 1, 20},
		{2, 2, 20},
		{3, 2, 20},
		{2, 3, 20},
		{1, 12, 20},
		{2, 6, 20},
		{3, 4, 20},
		{4, 3, 20},
		{6, 2, 20},
		{12, 1, 20},
	} {
		n := test.n
		m := test.m
		lda := test.lda
		if lda == 0 {
			lda = test.n
		}
		a := make([]float64, m*lda)
		for i := range a {
			a[i] = rnd.Float64()
		}
		aCopy := make([]float64, len(a))
		k := min(m, n)
		tau := make([]float64, k)
		for i := range tau {
			tau[i] = rnd.Float64()
		}
		work := make([]float64, m)
		for i := range work {
			work[i] = rnd.Float64()
		}
		copy(aCopy, a)
		impl.Dgerq2(m, n, a, lda, tau, work)

		// Test that the RQ factorization has completed successfully. Compute
		// Q based on the vectors.
		q := constructQ("RQ", m, n, a, lda, tau)

		// Check that q is orthonormal
		for i := 0; i < q.Rows; i++ {
			nrm := blas64.Nrm2(q.Cols, blas64.Vector{Inc: 1, Data: q.Data[i*q.Stride:]})
			if math.IsNaN(nrm) || math.Abs(nrm-1) > 1e-14 {
				t.Errorf("Case %v, q not normal", c)
			}
			for j := 0; j < i; j++ {
				dot := blas64.Dot(q.Cols, blas64.Vector{Inc: 1, Data: q.Data[i*q.Stride:]}, blas64.Vector{Inc: 1, Data: q.Data[j*q.Stride:]})
				if math.IsNaN(dot) || math.Abs(dot) > 1e-14 {
					t.Errorf("Case %v, q not orthogonal", c)
				}
			}
		}
		// Check that A = R * Q
		r := blas64.General{
			Rows:   m,
			Cols:   n,
			Stride: n,
			Data:   make([]float64, m*n),
		}
		for i := 0; i < m; i++ {
			off := m - n
			for j := max(0, i-off); j < n; j++ {
				r.Data[i*r.Stride+j] = a[i*lda+j]
			}
		}

		got := blas64.General{
			Rows:   m,
			Cols:   n,
			Stride: lda,
			Data:   make([]float64, m*lda),
		}
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, r, q, 0, got)
		want := blas64.General{
			Rows:   m,
			Cols:   n,
			Stride: lda,
			Data:   aCopy,
		}
		if !equalApproxGeneral(got, want, 1e-14) {
			t.Errorf("Case %d, R*Q != a\ngot: %+v\nwant:%+v", c, got, want)
		}
	}
}
