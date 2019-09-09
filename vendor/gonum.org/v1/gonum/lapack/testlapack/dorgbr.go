// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dorgbrer interface {
	Dorgbr(vect lapack.GenOrtho, m, n, k int, a []float64, lda int, tau, work []float64, lwork int)
	Dgebrder
}

func DorgbrTest(t *testing.T, impl Dorgbrer) {
	rnd := rand.New(rand.NewSource(1))
	for _, vect := range []lapack.GenOrtho{lapack.GenerateQ, lapack.GeneratePT} {
		for _, test := range []struct {
			m, n, k, lda int
		}{
			{5, 5, 5, 0},
			{5, 5, 3, 0},
			{5, 3, 5, 0},
			{3, 5, 5, 0},
			{3, 4, 5, 0},
			{3, 5, 4, 0},
			{4, 3, 5, 0},
			{4, 5, 3, 0},
			{5, 3, 4, 0},
			{5, 4, 3, 0},

			{5, 5, 5, 10},
			{5, 5, 3, 10},
			{5, 3, 5, 10},
			{3, 5, 5, 10},
			{3, 4, 5, 10},
			{3, 5, 4, 10},
			{4, 3, 5, 10},
			{4, 5, 3, 10},
			{5, 3, 4, 10},
			{5, 4, 3, 10},
		} {
			m := test.m
			n := test.n
			k := test.k
			lda := test.lda
			// Filter out bad tests
			if vect == lapack.GenerateQ {
				if m < n || n < min(m, k) || m < min(m, k) {
					continue
				}
			} else {
				if n < m || m < min(n, k) || n < min(n, k) {
					continue
				}
			}
			// Sizes for Dorgbr.
			var ma, na int
			if vect == lapack.GenerateQ {
				if m >= k {
					ma = m
					na = k
				} else {
					ma = m
					na = m
				}
			} else {
				if n >= k {
					ma = k
					na = n
				} else {
					ma = n
					na = n
				}
			}
			// a eventually needs to store either P or Q, so it must be
			// sufficiently big.
			var a []float64
			if vect == lapack.GenerateQ {
				lda = max(m, lda)
				a = make([]float64, m*lda)
			} else {
				lda = max(n, lda)
				a = make([]float64, n*lda)
			}
			for i := range a {
				a[i] = rnd.NormFloat64()
			}

			nTau := min(ma, na)
			tauP := make([]float64, nTau)
			tauQ := make([]float64, nTau)
			d := make([]float64, nTau)
			e := make([]float64, nTau)
			lwork := -1
			work := make([]float64, 1)
			impl.Dgebrd(ma, na, a, lda, d, e, tauQ, tauP, work, lwork)
			work = make([]float64, int(work[0]))
			lwork = len(work)
			impl.Dgebrd(ma, na, a, lda, d, e, tauQ, tauP, work, lwork)

			aCopy := make([]float64, len(a))
			copy(aCopy, a)

			var tau []float64
			if vect == lapack.GenerateQ {
				tau = tauQ
			} else {
				tau = tauP
			}

			impl.Dorgbr(vect, m, n, k, a, lda, tau, work, -1)
			work = make([]float64, int(work[0]))
			lwork = len(work)
			impl.Dorgbr(vect, m, n, k, a, lda, tau, work, lwork)

			var ans blas64.General
			var nRows, nCols int
			equal := true
			if vect == lapack.GenerateQ {
				nRows = m
				nCols = m
				if m >= k {
					nCols = n
				}
				ans = constructQPBidiagonal(lapack.ApplyQ, ma, na, min(m, k), aCopy, lda, tau)
			} else {
				nRows = n
				if k < n {
					nRows = m
				}
				nCols = n
				ansTmp := constructQPBidiagonal(lapack.ApplyP, ma, na, min(k, n), aCopy, lda, tau)
				// Dorgbr actually computes P^T
				ans = transposeGeneral(ansTmp)
			}
			for i := 0; i < nRows; i++ {
				for j := 0; j < nCols; j++ {
					if !floats.EqualWithinAbsOrRel(a[i*lda+j], ans.Data[i*ans.Stride+j], 1e-8, 1e-8) {
						equal = false
					}
				}
			}
			if !equal {
				t.Errorf("Extracted matrix mismatch. gen = %v, m = %v, n = %v, k = %v", string(vect), m, n, k)
			}
		}
	}
}
