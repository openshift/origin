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

type Dgeqp3er interface {
	Dlapmter
	Dgeqp3(m, n int, a []float64, lda int, jpvt []int, tau, work []float64, lwork int)
}

func Dgeqp3Test(t *testing.T, impl Dgeqp3er) {
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
		{129, 256, 0},
		{256, 129, 0},
		{129, 256, 266},
		{256, 129, 266},
	} {
		n := test.n
		m := test.m
		lda := test.lda
		if lda == 0 {
			lda = test.n
		}
		const (
			all = iota
			some
			none
		)
		for _, free := range []int{all, some, none} {
			// Allocate m×n matrix A and fill it with random numbers.
			a := make([]float64, m*lda)
			for i := range a {
				a[i] = rnd.Float64()
			}
			// Store a copy of A for later comparison.
			aCopy := make([]float64, len(a))
			copy(aCopy, a)
			// Allocate a slice of column pivots.
			jpvt := make([]int, n)
			for j := range jpvt {
				switch free {
				case all:
					// All columns are free.
					jpvt[j] = -1
				case some:
					// Some columns are free, some are leading columns.
					jpvt[j] = rnd.Intn(2) - 1 // -1 or 0
				case none:
					// All columns are leading.
					jpvt[j] = 0
				default:
					panic("bad freedom")
				}
			}
			// Allocate a slice for scalar factors of elementary
			// reflectors and fill it with random numbers. Dgeqp3
			// will overwrite them with valid data.
			k := min(m, n)
			tau := make([]float64, k)
			for i := range tau {
				tau[i] = rnd.Float64()
			}
			// Get optimal workspace size for Dgeqp3.
			work := make([]float64, 1)
			impl.Dgeqp3(m, n, a, lda, jpvt, tau, work, -1)
			lwork := int(work[0])
			work = make([]float64, lwork)
			for i := range work {
				work[i] = rnd.Float64()
			}

			// Compute a QR factorization of A with column pivoting.
			impl.Dgeqp3(m, n, a, lda, jpvt, tau, work, lwork)

			// Compute Q based on the elementary reflectors stored in A.
			q := constructQ("QR", m, n, a, lda, tau)
			// Check that Q is orthogonal.
			if !isOrthogonal(q) {
				t.Errorf("Case %v, Q not orthogonal", c)
			}

			// Copy the upper triangle of A into R.
			r := blas64.General{
				Rows:   m,
				Cols:   n,
				Stride: n,
				Data:   make([]float64, m*n),
			}
			for i := 0; i < m; i++ {
				for j := i; j < n; j++ {
					r.Data[i*n+j] = a[i*lda+j]
				}
			}
			// Compute Q * R.
			got := nanGeneral(m, n, lda)
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, r, 0, got)
			// Compute A * P: rearrange the columns of A based on the permutation in jpvt.
			want := blas64.General{Rows: m, Cols: n, Stride: lda, Data: aCopy}
			impl.Dlapmt(true, want.Rows, want.Cols, want.Data, want.Stride, jpvt)
			// Check that A * P = Q * R.
			if !equalApproxGeneral(got, want, 1e-13) {
				t.Errorf("Case %v,  Q*R != A*P\nQ*R=%v\nA*P=%v", c, got, want)
			}
		}
	}
}
