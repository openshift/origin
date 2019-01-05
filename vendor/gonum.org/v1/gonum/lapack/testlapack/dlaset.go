// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
)

type Dlaseter interface {
	Dlaset(uplo blas.Uplo, m, n int, alpha, beta float64, a []float64, lda int)
}

func DlasetTest(t *testing.T, impl Dlaseter) {
	rnd := rand.New(rand.NewSource(1))
	for ti, test := range []struct {
		m, n int
	}{
		{0, 0},
		{1, 1},
		{1, 10},
		{10, 1},
		{2, 2},
		{2, 10},
		{10, 2},
		{11, 11},
		{11, 100},
		{100, 11},
	} {
		m := test.m
		n := test.n
		for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower, blas.All} {
			for _, extra := range []int{0, 10} {
				a := randomGeneral(m, n, n+extra, rnd)
				alpha := 1.0
				beta := 2.0

				impl.Dlaset(uplo, m, n, alpha, beta, a.Data, a.Stride)

				prefix := fmt.Sprintf("Case #%v: m=%v,n=%v,uplo=%v,extra=%v",
					ti, m, n, uplo, extra)
				if !generalOutsideAllNaN(a) {
					t.Errorf("%v: out-of-range write to A", prefix)
				}
				for i := 0; i < min(m, n); i++ {
					if a.Data[i*a.Stride+i] != beta {
						t.Errorf("%v: unexpected diagonal of A", prefix)
					}
				}
				if uplo == blas.Upper || uplo == blas.All {
					for i := 0; i < m; i++ {
						for j := i + 1; j < n; j++ {
							if a.Data[i*a.Stride+j] != alpha {
								t.Errorf("%v: unexpected upper triangle of A", prefix)
							}
						}
					}
				}
				if uplo == blas.Lower || uplo == blas.All {
					for i := 1; i < m; i++ {
						for j := 0; j < min(i, n); j++ {
							if a.Data[i*a.Stride+j] != alpha {
								t.Errorf("%v: unexpected lower triangle of A", prefix)
							}
						}
					}
				}
			}
		}
	}
}
