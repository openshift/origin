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

type Dpotrfer interface {
	Dpotrf(ul blas.Uplo, n int, a []float64, lda int) (ok bool)
}

func DpotrfTest(t *testing.T, impl Dpotrfer) {
	const tol = 1e-13
	rnd := rand.New(rand.NewSource(1))
	bi := blas64.Implementation()
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for tc, test := range []struct {
			n   int
			lda int
		}{
			{1, 0},
			{2, 0},
			{3, 0},
			{10, 0},
			{30, 0},
			{63, 0},
			{65, 0},
			{127, 0},
			{129, 0},
			{500, 0},
			{1, 10},
			{2, 10},
			{3, 10},
			{10, 20},
			{30, 50},
			{63, 100},
			{65, 100},
			{127, 200},
			{129, 200},
			{500, 600},
		} {
			n := test.n

			// Random diagonal matrix D with positive entries.
			d := make([]float64, n)
			Dlatm1(d, 4, 10000, false, 1, rnd)

			// Construct a positive definite matrix A as
			//  A = U * D * U^T
			// where U is a random orthogonal matrix.
			lda := test.lda
			if lda == 0 {
				lda = n
			}
			a := make([]float64, n*lda)
			Dlagsy(n, 0, d, a, lda, rnd, make([]float64, 2*n))

			aCopy := make([]float64, len(a))
			copy(aCopy, a)

			ok := impl.Dpotrf(uplo, n, a, lda)
			if !ok {
				t.Errorf("Case %v: unexpected failure for positive definite matrix", tc)
				continue
			}

			switch uplo {
			case blas.Upper:
				for i := 0; i < n; i++ {
					for j := 0; j < i; j++ {
						a[i*lda+j] = 0
					}
				}
			case blas.Lower:
				for i := 0; i < n; i++ {
					for j := i + 1; j < n; j++ {
						a[i*lda+j] = 0
					}
				}
			default:
				panic("bad uplo")
			}

			ans := make([]float64, len(a))
			switch uplo {
			case blas.Upper:
				// Multiply U^T * U.
				bi.Dsyrk(uplo, blas.Trans, n, n, 1, a, lda, 0, ans, lda)
			case blas.Lower:
				// Multiply L * L^T.
				bi.Dsyrk(uplo, blas.NoTrans, n, n, 1, a, lda, 0, ans, lda)
			}

			match := true
			switch uplo {
			case blas.Upper:
				for i := 0; i < n; i++ {
					for j := i; j < n; j++ {
						if !floats.EqualWithinAbsOrRel(ans[i*lda+j], aCopy[i*lda+j], tol, tol) {
							match = false
						}
					}
				}
			case blas.Lower:
				for i := 0; i < n; i++ {
					for j := 0; j <= i; j++ {
						if !floats.EqualWithinAbsOrRel(ans[i*lda+j], aCopy[i*lda+j], tol, tol) {
							match = false
						}
					}
				}
			}
			if !match {
				t.Errorf("Case %v (uplo=%v,n=%v,lda=%v): unexpected result", tc, uplo, n, lda)
			}

			// Make one element of D negative so that A is not
			// positive definite, and check that Dpotrf fails.
			d[0] *= -1
			Dlagsy(n, 0, d, a, lda, rnd, make([]float64, 2*n))
			ok = impl.Dpotrf(uplo, n, a, lda)
			if ok {
				t.Errorf("Case %v: unexpected success for not positive definite matrix", tc)
			}
		}
	}
}
