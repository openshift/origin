// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

var zherTestCases = []struct {
	alpha float64
	x     []complex128
	a     []complex128

	want    []complex128
	wantRev []complex128 // Result when incX is negative.
}{
	{
		alpha: 1,
	},
	{
		alpha: 3,
		x: []complex128{
			0 - 3i,
			6 + 10i,
			-2 - 7i,
		},
		a: []complex128{
			-2 + 3i, -3 - 11i, 0 + 4i,
			-3 + 11i, -6 + 3i, 7 + 2i,
			0 - 4i, 7 - 2i, 18 + 3i,
		},
		want: []complex128{
			25 + 0i, -93 - 65i, 63 + 22i,
			-93 + 65i, 402 + 0i, -239 + 68i,
			63 - 22i, -239 - 68i, 177 + 0i},
		wantRev: []complex128{
			157 + 0i, -249 - 77i, 63 - 14i,
			-249 + 77i, 402 + 0i, -83 + 56i,
			63 + 14i, -83 - 56i, 45 + 0i,
		},
	},
	{
		alpha: 3,
		x: []complex128{
			-6 + 2i,
			-2 - 4i,
			0 + 0i,
			0 + 7i,
		},
		a: []complex128{
			2 + 3i, -9 + 7i, 3 + 11i, 10 - 1i,
			-9 - 7i, 16 + 3i, -5 + 2i, -7 - 5i,
			3 - 11i, -5 - 2i, 14 + 3i, 2 - 1i,
			10 + 1i, -7 + 5i, 2 + 1i, 18 + 3i,
		},
		want: []complex128{
			122 + 0i, 3 - 77i, 3 + 11i, 52 + 125i,
			3 + 77i, 76 + 0i, -5 + 2i, -91 + 37i,
			3 - 11i, -5 - 2i, 14 + 0i, 2 - 1i,
			52 - 125i, -91 - 37i, 2 + 1i, 165 + 0i,
		},
		wantRev: []complex128{
			149 + 0i, -9 + 7i, -81 - 31i, 52 - 127i,
			-9 - 7i, 16 + 0i, -5 + 2i, -7 - 5i,
			-81 + 31i, -5 - 2i, 74 + 0i, 14 + 83i,
			52 + 127i, -7 + 5i, 14 - 83i, 138 + 0i,
		},
	},
	{
		alpha: 0,
		x: []complex128{
			-6 + 2i,
			-2 - 4i,
			0 + 0i,
			0 + 7i,
		},
		a: []complex128{
			2 + 0i, -9 + 7i, 3 + 11i, 10 - 1i,
			-9 - 7i, 16 + 0i, -5 + 2i, -7 - 5i,
			3 - 11i, -5 - 2i, 14 + 0i, 2 - 1i,
			10 + 1i, -7 + 5i, 2 + 1i, 18 + 0i,
		},
		want: []complex128{
			2 + 0i, -9 + 7i, 3 + 11i, 10 - 1i,
			-9 - 7i, 16 + 0i, -5 + 2i, -7 - 5i,
			3 - 11i, -5 - 2i, 14 + 0i, 2 - 1i,
			10 + 1i, -7 + 5i, 2 + 1i, 18 + 0i,
		},
		wantRev: []complex128{
			2 + 0i, -9 + 7i, 3 + 11i, 10 - 1i,
			-9 - 7i, 16 + 0i, -5 + 2i, -7 - 5i,
			3 - 11i, -5 - 2i, 14 + 0i, 2 - 1i,
			10 + 1i, -7 + 5i, 2 + 1i, 18 + 0i,
		},
	},
}

type Zherer interface {
	Zher(uplo blas.Uplo, n int, alpha float64, x []complex128, incX int, a []complex128, lda int)
}

func ZherTest(t *testing.T, impl Zherer) {
	for tc, test := range zherTestCases {
		n := len(test.x)
		for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
			for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
				for _, lda := range []int{max(1, n), n + 11} {
					x := makeZVector(test.x, incX)
					xCopy := make([]complex128, len(x))
					copy(xCopy, x)

					a := makeZGeneral(test.a, n, n, lda)

					var want []complex128
					if incX > 0 {
						want = makeZGeneral(test.want, n, n, lda)
					} else {
						want = makeZGeneral(test.wantRev, n, n, lda)
					}

					if uplo == blas.Upper {
						for i := 0; i < n; i++ {
							for j := 0; j < i; j++ {
								a[i*lda+j] = znan
								want[i*lda+j] = znan
							}
						}
					} else {
						for i := 0; i < n; i++ {
							for j := i + 1; j < n; j++ {
								a[i*lda+j] = znan
								want[i*lda+j] = znan
							}
						}
					}

					impl.Zher(uplo, n, test.alpha, x, incX, a, lda)

					if !zsame(x, xCopy) {
						t.Errorf("Case %v (uplo=%v,incX=%v,lda=%v,alpha=%v): unexpected modification of x", tc, uplo, incX, test.alpha, lda)
					}
					if !zsame(want, a) {
						t.Errorf("Case %v (uplo=%v,incX=%v,lda=%v,alpha=%v): unexpected result\nwant: %v\ngot:  %v", tc, uplo, incX, lda, test.alpha, want, a)
					}
				}
			}
		}
	}
}
