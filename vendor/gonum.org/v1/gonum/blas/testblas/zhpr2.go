// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Zhpr2er interface {
	Zhpr2(uplo blas.Uplo, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, ap []complex128)
}

func Zhpr2Test(t *testing.T, impl Zhpr2er) {
	for tc, test := range zher2TestCases {
		n := len(test.x)
		incX := test.incX
		incY := test.incY
		for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			y := makeZVector(test.y, incY)
			yCopy := make([]complex128, len(y))
			copy(yCopy, y)

			ap := zPack(uplo, n, test.a, n)
			impl.Zhpr2(uplo, n, test.alpha, x, incX, y, incY, ap)
			a := zUnpackAsHermitian(uplo, n, ap)

			if !zsame(x, xCopy) {
				t.Errorf("Case %v (uplo=%v,incX=%v,incY=%v): unexpected modification of x", tc, uplo, incX, incY)
			}
			if !zsame(y, yCopy) {
				t.Errorf("Case %v (uplo=%v,incX=%v,incY=%v): unexpected modification of y", tc, uplo, incX, incY)
			}
			if !zsame(test.want, a) {
				t.Errorf("Case %v (uplo=%v,incX=%v,incY=%v): unexpected result\nwant: %v\ngot:  %v", tc, uplo, incX, incY, test.want, a)
			}
		}
	}
}
