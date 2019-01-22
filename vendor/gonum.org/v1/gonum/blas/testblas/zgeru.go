// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"
)

type Zgeruer interface {
	Zgeru(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int)
}

func ZgeruTest(t *testing.T, impl Zgeruer) {
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
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  7,
			incY:  13,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  1,
			incY:  13,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  1,
			incY:  -13,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{19 + 12i, 5 + 16i, 4 + 8i, 15 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  7,
			incY:  1,
			alpha: 1 + 2i,
			x:     []complex128{1 + 13i, 18 + 15i, 10 + 18i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  -7,
			incY:  1,
			alpha: 1 + 2i,
			x:     []complex128{10 + 18i, 18 + 15i, 1 + 13i},
			y:     []complex128{15 + 12i, 4 + 8i, 5 + 16i, 19 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 1 + 2i,
			x:     []complex128{10 + 18i, 18 + 15i, 1 + 13i},
			y:     []complex128{19 + 12i, 5 + 16i, 4 + 8i, 15 + 12i},
			a: []complex128{
				4 + 7i, 4 + 7i, 12 + 3i, 9 + 10i,
				3 + 3i, 1 + 2i, 17 + 17i, 9 + 18i,
				14 + 12i, 9 + 16i, 1 + 1i, 9 + 1i,
			},
			want: []complex128{
				-551 - 68i, -216 - 133i, -353 - 322i, -646 - 5i,
				-789 + 624i, -455 + 110i, -859 + 80i, -831 + 843i,
				-832 + 270i, -399 - 40i, -737 - 225i, -941 + 411i,
			},
		},
		{
			incX:  1,
			incY:  1,
			alpha: 1 + 2i,
			x:     []complex128{5 + 16i, 12 + 19i, 9 + 7i, 2 + 4i},
			y:     []complex128{18 + 7i, 20 + 15i, 12 + 14i},
			a: []complex128{
				8 + 17i, 2 + 2i, 8 + 17i,
				1 + 10i, 10 + 15i, 4 + 18i,
				11 + 3i, 15 + 7i, 12 + 15i,
				20 + 10i, 8 + 13i, 19 + 10i,
			},
			want: []complex128{
				-660 + 296i, -928 + 117i, -680 - 49i,
				-768 + 602i, -1155 + 485i, -910 + 170i,
				-254 + 418i, -460 + 432i, -398 + 245i,
				-144 + 112i, -232 + 83i, -165 + 22i,
			},
		},
		{
			incX:  7,
			incY:  13,
			alpha: 1 + 2i,
			x:     []complex128{5 + 16i, 12 + 19i, 9 + 7i, 2 + 4i},
			y:     []complex128{18 + 7i, 20 + 15i, 12 + 14i},
			a: []complex128{
				8 + 17i, 2 + 2i, 8 + 17i,
				1 + 10i, 10 + 15i, 4 + 18i,
				11 + 3i, 15 + 7i, 12 + 15i,
				20 + 10i, 8 + 13i, 19 + 10i,
			},
			want: []complex128{
				-660 + 296i, -928 + 117i, -680 - 49i,
				-768 + 602i, -1155 + 485i, -910 + 170i,
				-254 + 418i, -460 + 432i, -398 + 245i,
				-144 + 112i, -232 + 83i, -165 + 22i,
			},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 1 + 2i,
			x:     []complex128{2 + 4i, 9 + 7i, 12 + 19i, 5 + 16i},
			y:     []complex128{12 + 14i, 20 + 15i, 18 + 7i},
			a: []complex128{
				8 + 17i, 2 + 2i, 8 + 17i,
				1 + 10i, 10 + 15i, 4 + 18i,
				11 + 3i, 15 + 7i, 12 + 15i,
				20 + 10i, 8 + 13i, 19 + 10i,
			},
			want: []complex128{
				-660 + 296i, -928 + 117i, -680 - 49i,
				-768 + 602i, -1155 + 485i, -910 + 170i,
				-254 + 418i, -460 + 432i, -398 + 245i,
				-144 + 112i, -232 + 83i, -165 + 22i,
			},
		},
		{
			incX:  -7,
			incY:  -13,
			alpha: 0,
			x:     []complex128{5 + 16i, 12 + 19i, 9 + 7i, 2 + 4i},
			y:     []complex128{18 + 7i, 20 + 15i, 12 + 14i},
			a: []complex128{
				8 + 17i, 2 + 2i, 8 + 17i,
				1 + 10i, 10 + 15i, 4 + 18i,
				11 + 3i, 15 + 7i, 12 + 15i,
				20 + 10i, 8 + 13i, 19 + 10i,
			},
			want: []complex128{
				8 + 17i, 2 + 2i, 8 + 17i,
				1 + 10i, 10 + 15i, 4 + 18i,
				11 + 3i, 15 + 7i, 12 + 15i,
				20 + 10i, 8 + 13i, 19 + 10i,
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

			impl.Zgeru(m, n, test.alpha, x, incX, y, incY, a, lda)

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
