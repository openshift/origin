// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"
)

type Zdscaler interface {
	Zdscal(n int, alpha float64, x []complex128, incX int)
}

func ZdscalTest(t *testing.T, impl Zdscaler) {
	for tc, test := range []struct {
		alpha float64
		x     []complex128
		want  []complex128
	}{
		{
			alpha: 3,
			x:     nil,
			want:  nil,
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i},
			want:  []complex128{3 + 6i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i},
			want:  []complex128{3 + 6i, 9 + 12i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i, 45 + 48i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i, 45 + 48i, 51 + 54i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i, 45 + 48i, 51 + 54i, 57 + 60i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i, 45 + 48i, 51 + 54i, 57 + 60i, 63 + 66i},
		},
		{
			alpha: 3,
			x:     []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			want:  []complex128{3 + 6i, 9 + 12i, 15 + 18i, 21 + 24i, 27 + 30i, 33 + 36i, 39 + 42i, 45 + 48i, 51 + 54i, 57 + 60i, 63 + 66i, 69 + 72i},
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

			impl.Zdscal(n, test.alpha, x, incX)

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
