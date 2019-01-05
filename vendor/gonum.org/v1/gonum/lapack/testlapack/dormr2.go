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

type Dormr2er interface {
	Dgerqf(m, n int, a []float64, lda int, tau, work []float64, lwork int)
	Dormr2(side blas.Side, trans blas.Transpose, m, n, k int, a []float64, lda int, tau, c []float64, ldc int, work []float64)
}

func Dormr2Test(t *testing.T, impl Dormr2er) {
	rnd := rand.New(rand.NewSource(1))
	for _, side := range []blas.Side{blas.Left, blas.Right} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans} {
			for _, test := range []struct {
				common, adim, cdim, lda, ldc int
			}{
				{3, 4, 5, 0, 0},
				{3, 5, 4, 0, 0},
				{4, 3, 5, 0, 0},
				{4, 5, 3, 0, 0},
				{5, 3, 4, 0, 0},
				{5, 4, 3, 0, 0},
				{3, 4, 5, 6, 20},
				{3, 5, 4, 6, 20},
				{4, 3, 5, 6, 20},
				{4, 5, 3, 6, 20},
				{5, 3, 4, 6, 20},
				{5, 4, 3, 6, 20},
				{3, 4, 5, 20, 6},
				{3, 5, 4, 20, 6},
				{4, 3, 5, 20, 6},
				{4, 5, 3, 20, 6},
				{5, 3, 4, 20, 6},
				{5, 4, 3, 20, 6},
			} {
				ma := test.adim
				na := test.common
				var mc, nc int
				if side == blas.Left {
					mc = test.common
					nc = test.cdim
				} else {
					mc = test.cdim
					nc = test.common
				}

				// Generate a random matrix
				lda := test.lda
				if lda == 0 {
					lda = na
				}
				a := make([]float64, ma*lda)
				for i := range a {
					a[i] = rnd.Float64()
				}
				ldc := test.ldc
				if ldc == 0 {
					ldc = nc
				}
				// Compute random C matrix
				c := make([]float64, mc*ldc)
				for i := range c {
					c[i] = rnd.Float64()
				}

				// Compute RQ
				k := min(ma, na)
				tau := make([]float64, k)
				work := make([]float64, 1)
				impl.Dgerqf(ma, na, a, lda, tau, work, -1)
				work = make([]float64, int(work[0]))
				impl.Dgerqf(ma, na, a, lda, tau, work, len(work))

				// Build Q from result
				q := constructQ("RQ", ma, na, a, lda, tau)

				cMat := blas64.General{
					Rows:   mc,
					Cols:   nc,
					Stride: ldc,
					Data:   make([]float64, len(c)),
				}
				copy(cMat.Data, c)
				cMatCopy := blas64.General{
					Rows:   cMat.Rows,
					Cols:   cMat.Cols,
					Stride: cMat.Stride,
					Data:   make([]float64, len(cMat.Data)),
				}
				copy(cMatCopy.Data, cMat.Data)
				switch {
				default:
					panic("bad test")
				case side == blas.Left && trans == blas.NoTrans:
					blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, cMatCopy, 0, cMat)
				case side == blas.Left && trans == blas.Trans:
					blas64.Gemm(blas.Trans, blas.NoTrans, 1, q, cMatCopy, 0, cMat)
				case side == blas.Right && trans == blas.NoTrans:
					blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, cMatCopy, q, 0, cMat)
				case side == blas.Right && trans == blas.Trans:
					blas64.Gemm(blas.NoTrans, blas.Trans, 1, cMatCopy, q, 0, cMat)
				}
				// Do Dorm2r ard compare
				if side == blas.Left {
					work = make([]float64, nc)
				} else {
					work = make([]float64, mc)
				}
				aCopy := make([]float64, len(a))
				copy(aCopy, a)
				tauCopy := make([]float64, len(tau))
				copy(tauCopy, tau)
				impl.Dormr2(side, trans, mc, nc, k, a[(ma-k)*lda:], lda, tau, c, ldc, work)
				if !floats.Equal(a, aCopy) {
					t.Errorf("a changed in call")
				}
				if !floats.Equal(tau, tauCopy) {
					t.Errorf("tau changed in call")
				}
				if !floats.EqualApprox(cMat.Data, c, 1e-14) {
					t.Errorf("Multiplication mismatch.\n Want %v \n got %v.", cMat.Data, c)
				}
			}
		}
	}
}
