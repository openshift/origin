// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Zhpmver interface {
	Zhpmv(uplo blas.Uplo, n int, alpha complex128, ap []complex128, x []complex128, incX int, beta complex128, y []complex128, incY int)
}

func ZhpmvTest(t *testing.T, impl Zhpmver) {
	for tc, test := range zhemvTestCases {
		uplo := test.uplo
		n := len(test.x)
		alpha := test.alpha
		beta := test.beta
		for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
			for _, incY := range []int{-11, -2, -1, 1, 2, 7} {
				x := makeZVector(test.x, incX)
				xCopy := make([]complex128, len(x))
				copy(xCopy, x)

				y := makeZVector(test.y, incY)

				ap := zPack(uplo, n, test.a, n)
				apCopy := make([]complex128, len(ap))
				copy(apCopy, ap)

				impl.Zhpmv(test.uplo, n, alpha, ap, x, incX, beta, y, incY)

				if !zsame(x, xCopy) {
					t.Errorf("Case %v (incX=%v,incY=%v): unexpected modification of x", tc, incX, incY)
				}
				if !zsame(ap, apCopy) {
					t.Errorf("Case %v (incX=%v,incY=%v): unexpected modification of A", tc, incX, incY)
				}

				var want []complex128
				switch {
				case incX > 0 && incY > 0:
					want = makeZVector(test.want, incY)
				case incX < 0 && incY > 0:
					want = makeZVector(test.wantXNeg, incY)
				case incX > 0 && incY < 0:
					want = makeZVector(test.wantYNeg, incY)
				default:
					want = makeZVector(test.wantXYNeg, incY)
				}
				if !zsame(y, want) {
					t.Errorf("Case %v (incX=%v,incY=%v): unexpected result\nwant %v\ngot  %v", tc, incX, incY, want, y)
				}
			}
		}
	}
}
