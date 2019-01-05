// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Zhprer interface {
	Zhpr(uplo blas.Uplo, n int, alpha float64, x []complex128, incX int, ap []complex128)
}

func ZhprTest(t *testing.T, impl Zhprer) {
	for tc, test := range zherTestCases {
		n := len(test.x)
		for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
			for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
				x := makeZVector(test.x, incX)
				xCopy := make([]complex128, len(x))
				copy(xCopy, x)

				ap := zPack(uplo, n, test.a, n)
				impl.Zhpr(uplo, n, test.alpha, x, incX, ap)
				a := zUnpackAsHermitian(uplo, n, ap)

				var want []complex128
				if incX > 0 {
					want = makeZGeneral(test.want, n, n, max(1, n))
				} else {
					want = makeZGeneral(test.wantRev, n, n, max(1, n))
				}

				if !zsame(x, xCopy) {
					t.Errorf("Case %v (uplo=%v,incX=%v,alpha=%v): unexpected modification of x", tc, uplo, incX, test.alpha)
				}
				if !zsame(want, a) {
					t.Errorf("Case %v (uplo=%v,incX=%v,alpha=%v): unexpected result\nwant: %v\ngot:  %v", tc, uplo, incX, test.alpha, want, a)
				}
			}
		}
	}
}
