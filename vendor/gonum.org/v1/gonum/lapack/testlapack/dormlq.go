// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/floats"
)

type Dormlqer interface {
	Dorml2er
	Dormlq(side blas.Side, trans blas.Transpose, m, n, k int, a []float64, lda int, tau, c []float64, ldc int, work []float64, lwork int)
}

func DormlqTest(t *testing.T, impl Dormlqer) {
	rnd := rand.New(rand.NewSource(1))
	for _, side := range []blas.Side{blas.Left, blas.Right} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans} {
			for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
				for _, test := range []struct {
					common, adim, cdim, lda, ldc int
				}{
					{0, 0, 0, 0, 0},
					{6, 7, 8, 0, 0},
					{6, 8, 7, 0, 0},
					{7, 6, 8, 0, 0},
					{7, 8, 6, 0, 0},
					{8, 6, 7, 0, 0},
					{8, 7, 6, 0, 0},
					{100, 200, 300, 0, 0},
					{100, 300, 200, 0, 0},
					{200, 100, 300, 0, 0},
					{200, 300, 100, 0, 0},
					{300, 100, 200, 0, 0},
					{300, 200, 100, 0, 0},
					{100, 200, 300, 400, 500},
					{100, 300, 200, 400, 500},
					{200, 100, 300, 400, 500},
					{200, 300, 100, 400, 500},
					{300, 100, 200, 400, 500},
					{300, 200, 100, 400, 500},
					{100, 200, 300, 500, 400},
					{100, 300, 200, 500, 400},
					{200, 100, 300, 500, 400},
					{200, 300, 100, 500, 400},
					{300, 100, 200, 500, 400},
					{300, 200, 100, 500, 400},
				} {
					var ma, na, mc, nc int
					if side == blas.Left {
						ma = test.adim
						na = test.common
						mc = test.common
						nc = test.cdim
					} else {
						ma = test.adim
						na = test.common
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
					// Compute random C matrix
					ldc := test.ldc
					if ldc == 0 {
						ldc = nc
					}
					c := make([]float64, mc*ldc)
					for i := range c {
						c[i] = rnd.Float64()
					}

					// Compute LQ
					k := min(ma, na)
					tau := make([]float64, k)
					work := make([]float64, 1)
					impl.Dgelqf(ma, na, a, lda, tau, work, -1)
					work = make([]float64, int(work[0]))
					impl.Dgelqf(ma, na, a, lda, tau, work, len(work))

					cCopy := make([]float64, len(c))
					copy(cCopy, c)
					ans := make([]float64, len(c))
					copy(ans, cCopy)

					var nw int
					if side == blas.Left {
						nw = nc
					} else {
						nw = mc
					}
					work = make([]float64, max(1, nw))
					impl.Dorml2(side, trans, mc, nc, k, a, lda, tau, ans, ldc, work)

					var lwork int
					switch wl {
					case minimumWork:
						lwork = nw
					case optimumWork:
						impl.Dormlq(side, trans, mc, nc, k, a, lda, tau, c, ldc, work, -1)
						lwork = int(work[0])
					case mediumWork:
						work := make([]float64, 1)
						impl.Dormlq(side, trans, mc, nc, k, a, lda, tau, c, ldc, work, -1)
						lwork = (int(work[0]) + nw) / 2
					}
					lwork = max(1, lwork)
					work = make([]float64, lwork)

					impl.Dormlq(side, trans, mc, nc, k, a, lda, tau, c, ldc, work, lwork)
					if !floats.EqualApprox(c, ans, 1e-13) {
						t.Errorf("Dormqr and Dorm2r results mismatch")
					}
				}
			}
		}
	}
}
