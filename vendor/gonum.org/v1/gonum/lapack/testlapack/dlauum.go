// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Dlauumer interface {
	Dlauum(uplo blas.Uplo, n int, a []float64, lda int)
}

func DlauumTest(t *testing.T, impl Dlauumer) {
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		name := "Upper"
		if uplo == blas.Lower {
			name = "Lower"
		}
		t.Run(name, func(t *testing.T) {
			// Include small and large sizes to make sure that both
			// unblocked and blocked paths are taken.
			ns := []int{0, 1, 2, 3, 4, 5, 10, 25, 31, 32, 33, 63, 64, 65, 127, 128, 129}
			dlauuTest(t, impl.Dlauum, uplo, ns)
		})
	}
}
