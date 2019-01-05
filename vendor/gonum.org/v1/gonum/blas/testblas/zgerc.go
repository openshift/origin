// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"
)

type Zgercer interface {
	Zgerc(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int)
}

func ZgercTest(t *testing.T, impl Zgercer) {
	for tc, test := range []struct {
		alpha complex128
		x     []complex128
		incX  int
		y     []complex128
		incY  int
		a     []complex128

		want []complex128
	}{
		{
			incX:  1,
			incY:  1,
			alpha: 1 + 2i,
		},
		{
			incX:  1,
			incY:  1,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				10 + 9i, 6 + 17i, 3 + 10i, 6 + 7i,
				3 + 4i, 11 + 16i, 5 + 14i, 11 + 18i,
				18 + 6i, 4 + 1i, 13 + 2i, 14 + 3i},
			want: []complex128{
				-185 + 534i, 26 + 277i, 118 + 485i, -289 + 592i,
				435 + 913i, 371 + 316i, 761 + 461i, 395 + 1131i,
				84 + 888i, 204 + 361i, 491 + 608i, -24 + 1037i},
		},
		{
			incX:  7,
			incY:  13,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				10 + 9i, 6 + 17i, 3 + 10i, 6 + 7i,
				3 + 4i, 11 + 16i, 5 + 14i, 11 + 18i,
				18 + 6i, 4 + 1i, 13 + 2i, 14 + 3i},
			want: []complex128{
				-185 + 534i, 26 + 277i, 118 + 485i, -289 + 592i,
				435 + 913i, 371 + 316i, 761 + 461i, 395 + 1131i,
				84 + 888i, 204 + 361i, 491 + 608i, -24 + 1037i},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 1 + 2i,
			x:     []complex128{10 + 18i, 18 + 15i, 1 + 13i},
			y:     []complex128{19 + 12i, 5 + 16i, 4 + 8i, 15 + 12i},
			a: []complex128{
				10 + 9i, 6 + 17i, 3 + 10i, 6 + 7i,
				3 + 4i, 11 + 16i, 5 + 14i, 11 + 18i,
				18 + 6i, 4 + 1i, 13 + 2i, 14 + 3i},
			want: []complex128{
				-185 + 534i, 26 + 277i, 118 + 485i, -289 + 592i,
				435 + 913i, 371 + 316i, 761 + 461i, 395 + 1131i,
				84 + 888i, 204 + 361i, 491 + 608i, -24 + 1037i},
		},
		{
			incX:  1,
			incY:  1,
			alpha: 1 + 2i,
			x:     []complex128{5 + 16i, 12 + 19i, 9 + 7i, 2 + 4i},
			y:     []complex128{18 + 7i, 20 + 15i, 12 + 14i},
			a: []complex128{
				11 + 4i, 17 + 18i, 7 + 13i,
				14 + 20i, 14 + 10i, 7 + 5i,
				7 + 17i, 10 + 6i, 11 + 13i,
				7 + 6i, 19 + 16i, 8 + 8i,
			},
			want: []complex128{
				-293 + 661i, -133 + 943i, 47 + 703i,
				-153 + 976i, 139 + 1260i, 297 + 885i,
				92 + 502i, 285 + 581i, 301 + 383i,
				-45 + 192i, 19 + 266i, 48 + 188i,
			},
		},
		{
			incX:  7,
			incY:  13,
			alpha: 1 + 2i,
			x:     []complex128{5 + 16i, 12 + 19i, 9 + 7i, 2 + 4i},
			y:     []complex128{18 + 7i, 20 + 15i, 12 + 14i},
			a: []complex128{
				11 + 4i, 17 + 18i, 7 + 13i,
				14 + 20i, 14 + 10i, 7 + 5i,
				7 + 17i, 10 + 6i, 11 + 13i,
				7 + 6i, 19 + 16i, 8 + 8i,
			},
			want: []complex128{
				-293 + 661i, -133 + 943i, 47 + 703i,
				-153 + 976i, 139 + 1260i, 297 + 885i,
				92 + 502i, 285 + 581i, 301 + 383i,
				-45 + 192i, 19 + 266i, 48 + 188i,
			},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 1 + 2i,
			x:     []complex128{2 + 4i, 9 + 7i, 12 + 19i, 5 + 16i},
			y:     []complex128{12 + 14i, 20 + 15i, 18 + 7i},
			a: []complex128{
				11 + 4i, 17 + 18i, 7 + 13i,
				14 + 20i, 14 + 10i, 7 + 5i,
				7 + 17i, 10 + 6i, 11 + 13i,
				7 + 6i, 19 + 16i, 8 + 8i,
			},
			want: []complex128{
				-293 + 661i, -133 + 943i, 47 + 703i,
				-153 + 976i, 139 + 1260i, 297 + 885i,
				92 + 502i, 285 + 581i, 301 + 383i,
				-45 + 192i, 19 + 266i, 48 + 188i,
			},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 0,
			x:     []complex128{2 + 4i, 9 + 7i, 12 + 19i, 5 + 16i},
			y:     []complex128{12 + 14i, 20 + 15i, 18 + 7i},
			a: []complex128{
				11 + 4i, 17 + 18i, 7 + 13i,
				14 + 20i, 14 + 10i, 7 + 5i,
				7 + 17i, 10 + 6i, 11 + 13i,
				7 + 6i, 19 + 16i, 8 + 8i,
			},
			want: []complex128{
				11 + 4i, 17 + 18i, 7 + 13i,
				14 + 20i, 14 + 10i, 7 + 5i,
				7 + 17i, 10 + 6i, 11 + 13i,
				7 + 6i, 19 + 16i, 8 + 8i,
			},
		},
	} {
		m := len(test.x)
		n := len(test.y)
		incX := test.incX
		incY := test.incY

		for _, lda := range []int{max(1, n), n + 20} {
			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			y := makeZVector(test.y, incY)
			yCopy := make([]complex128, len(y))
			copy(yCopy, y)

			a := makeZGeneral(test.a, m, n, lda)
			want := makeZGeneral(test.want, m, n, lda)

			impl.Zgerc(m, n, test.alpha, x, incX, y, incY, a, lda)

			if !zsame(x, xCopy) {
				t.Errorf("Case %v: unexpected modification of x", tc)
			}
			if !zsame(y, yCopy) {
				t.Errorf("Case %v: unexpected modification of y", tc)
			}
			if !zsame(want, a) {
				t.Errorf("Case %v: unexpected result\nwant %v\ngot %v", tc, want, a)
			}
		}
	}
}
