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
			a := make([]float64, m*lda)
			for i := range a {
				a[i] = rnd.Float64()
			}
			aCopy := make([]float64, len(a))
			copy(aCopy, a)
			jpvt := make([]int, n)
			for j := range jpvt {
				switch free {
				case all:
					jpvt[j] = -1
				case some:
					jpvt[j] = rnd.Intn(2) - 1
				case none:
					jpvt[j] = 0
				default:
					panic("bad freedom")
				}
			}
			k := min(m, n)
			tau := make([]float64, k)
			for i := range tau {
				tau[i] = rnd.Float64()
			}
			work := make([]float64, 1)
			impl.Dgeqp3(m, n, a, lda, jpvt, tau, work, -1)
			lwork := int(work[0])
			work = make([]float64, lwork)
			for i := range work {
				work[i] = rnd.Float64()
			}
			impl.Dgeqp3(m, n, a, lda, jpvt, tau, work, lwork)

			// Test that the QR factorization has completed successfully. Compute
			// Q based on the vectors.
			q := constructQ("QR", m, n, a, lda, tau)

			// Check that q is orthonormal
			for i := 0; i < m; i++ {
				nrm := blas64.Nrm2(m, blas64.Vector{Inc: 1, Data: q.Data[i*m:]})
				if math.Abs(nrm-1) > 1e-13 {
					t.Errorf("Case %v, q not normal", c)
				}
				for j := 0; j < i; j++ {
					dot := blas64.Dot(m, blas64.Vector{Inc: 1, Data: q.Data[i*m:]}, blas64.Vector{Inc: 1, Data: q.Data[j*m:]})
					if math.Abs(dot) > 1e-14 {
						t.Errorf("Case %v, q not orthogonal", c)
					}
				}
			}
			// Check that A * P = Q * R
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
			got := nanGeneral(m, n, lda)
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, r, 0, got)

			want := blas64.General{Rows: m, Cols: n, Stride: lda, Data: aCopy}
			impl.Dlapmt(true, want.Rows, want.Cols, want.Data, want.Stride, jpvt)
			if !equalApproxGeneral(got, want, 1e-13) {
				t.Errorf("Case %v,  Q*R != A*P\nQ*R=%v\nA*P=%v", c, got, want)
			}
		}
	}
}
