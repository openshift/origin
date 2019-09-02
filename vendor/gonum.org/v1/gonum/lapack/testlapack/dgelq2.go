// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

type Dgelq2er interface {
	Dgelq2(m, n int, a []float64, lda int, tau, work []float64)
}

func Dgelq2Test(t *testing.T, impl Dgelq2er) {
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
		{1, 12, 0},
		{1, 1, 20},
		{2, 2, 20},
		{3, 2, 20},
		{2, 3, 20},
		{1, 12, 20},
		{2, 6, 20},
		{3, 4, 20},
		{4, 3, 20},
		{6, 2, 20},
		{1, 12, 20},
	} {
		n := test.n
		m := test.m
		lda := test.lda
		if lda == 0 {
			lda = test.n
		}
		k := min(m, n)
		tau := make([]float64, k)
		for i := range tau {
			tau[i] = rnd.Float64()
		}
		work := make([]float64, m)
		for i := range work {
			work[i] = rnd.Float64()
		}
		a := make([]float64, m*lda)
		for i := 0; i < m*lda; i++ {
			a[i] = rnd.Float64()
		}
		aCopy := make([]float64, len(a))
		copy(aCopy, a)
		impl.Dgelq2(m, n, a, lda, tau, work)

		Q := constructQ("LQ", m, n, a, lda, tau)

		// Check that Q is orthogonal.
		if !isOrthogonal(Q) {
			t.Errorf("Case %v: Q not orthogonal", c)
		}

		L := blas64.General{
			Rows:   m,
			Cols:   n,
			Stride: n,
			Data:   make([]float64, m*n),
		}
		for i := 0; i < m; i++ {
			for j := 0; j <= min(i, n-1); j++ {
				L.Data[i*L.Stride+j] = a[i*lda+j]
			}
		}

		ans := blas64.General{
			Rows:   m,
			Cols:   n,
			Stride: lda,
			Data:   make([]float64, m*lda),
		}
		copy(ans.Data, aCopy)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, L, Q, 0, ans)
		if !floats.EqualApprox(aCopy, ans.Data, 1e-14) {
			t.Errorf("Case %v, LQ mismatch. Want %v, got %v.", c, aCopy, ans.Data)
		}
	}
}
