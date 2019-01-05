// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"
)

type Zscaler interface {
	Zscal(n int, alpha complex128, x []complex128, incX int)
}

func ZscalTest(t *testing.T, impl Zscaler) {
	for tc, test := range []struct {
		alpha complex128
		x     []complex128
		want  []complex128
	}{
		{
			alpha: 2 + 5i,
			x:     nil,
			want:  nil,
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i},
			want:  []complex128{-8 + 9i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i},
			want:  []complex128{-8 + 9i, -14 + 23i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i, -50 + 107i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i, -50 + 107i, -56 + 121i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i, -50 + 107i, -56 + 121i, -62 + 135i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i, -50 + 107i, -56 + 121i, -62 + 135i, -68 + 149i},
		},
		{
			alpha: 2 + 5i,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			want:  []complex128{-8 + 9i, -14 + 23i, -20 + 37i, -26 + 51i, -32 + 65i, -38 + 79i, -44 + 93i, -50 + 107i, -56 + 121i, -62 + 135i, -68 + 149i, -74 + 163i},
		},
		{
			alpha: 0,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			want:  []complex128{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	} {
		n := len(test.x)
		if len(test.want) != n {
			panic("bad test")
		}
		for _, incX := range []int{-3, -1, 1, 2, 4, 7, 10} {
			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			want := makeZVector(test.want, incX)

			impl.Zscal(n, test.alpha, x, incX)

			prefix := fmt.Sprintf("Case %v (n=%v,incX=%v):", tc, n, incX)

			if incX < 0 {
				if !zsame(x, xCopy) {
					t.Errorf("%v: unexpected modification of x\nwant %v\ngot %v", prefix, want, x)
				}
				continue
			}
			if !zsame(x, want) {
				t.Errorf("%v: unexpected result:\nwant: %v\ngot: %v", prefix, want, x)
			}
		}
	}
}
