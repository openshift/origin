// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
)

type Dpbtf2er interface {
	Dpbtf2(ul blas.Uplo, n, kd int, ab []float64, ldab int) (ok bool)
	Dpotrfer
}

func Dpbtf2Test(t *testing.T, impl Dpbtf2er) {
	// Test random symmetric banded matrices against the full version.
	rnd := rand.New(rand.NewSource(1))

	for _, n := range []int{5, 10, 20} {
		for _, kb := range []int{0, 1, 3, n - 1} {
			for _, ldoff := range []int{0, 4} {
				for _, ul := range []blas.Uplo{blas.Upper, blas.Lower} {
					ldab := kb + 1 + ldoff
					sym, band := randSymBand(ul, n, ldab, kb, rnd)

					// Compute the Cholesky decomposition of the symmetric matrix.
					ok := impl.Dpotrf(ul, sym.N, sym.Data, sym.Stride)
					if !ok {
						panic("bad test: symmetric cholesky decomp failed")
					}

					// Compute the Cholesky decomposition of the banded matrix.
					ok = impl.Dpbtf2(band.Uplo, band.N, band.K, band.Data, band.Stride)
					if !ok {
						t.Errorf("SymBand cholesky decomp failed")
					}

					// Compare the result to the Symmetric decomposition.
					sb := symBandToSym(ul, band.Data, n, kb, ldab)
					if !equalApproxSymmetric(sym, sb, 1e-10) {
						t.Errorf("chol mismatch banded and sym. n = %v, kb = %v, ldoff = %v", n, kb, ldoff)
					}
				}
			}
		}
	}
}
