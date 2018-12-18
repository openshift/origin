// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"
)

type Zdotuer interface {
	Zdotu(n int, x []complex128, incX int, y []complex128, incY int) complex128
}

func ZdotuTest(t *testing.T, impl Zdotuer) {
	for tc, test := range []struct {
		x, y []complex128

		want    complex128 // Result when both increments have the same sign.
		wantRev complex128 // Result when the increments have opposite sign.
	}{
		{
			x:       nil,
			y:       nil,
			want:    0,
			wantRev: 0,
		},
		{
			x:       []complex128{1 + 2i},
			y:       []complex128{30 + 31i},
			want:    -32 + 91i,
			wantRev: -32 + 91i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i},
			y:       []complex128{30 + 31i, 33 + 34i},
			want:    -69 + 325i,
			wantRev: -69 + 313i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i},
			want:    -111 + 726i,
			wantRev: -111 + 678i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i},
			want:    -158 + 1318i,
			wantRev: -158 + 1198i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i},
			want:    -210 + 2125i,
			wantRev: -210 + 1885i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i},
			want:    -267 + 3171i,
			wantRev: -267 + 2751i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i},
			want:    -329 + 4480i,
			wantRev: -329 + 3808i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i},
			want:    -396 + 6076i,
			wantRev: -396 + 5068i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i},
			want:    -468 + 7983i,
			wantRev: -468 + 6543i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i},
			want:    -545 + 10225i,
			wantRev: -545 + 8245i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i},
			want:    -627 + 12826i,
			wantRev: -627 + 10186i,
		},
		{
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
			want:    -714 + 15810i,
			wantRev: -714 + 12378i,
		},
	} {
		n := len(test.x)
		if len(test.y) != n {
			panic("bad test")
		}
		for _, inc := range allPairs([]int{-7, -3, 1, 13}, []int{-11, -5, 1, 17}) {
			incX := inc[0]
			incY := inc[1]

			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			y := makeZVector(test.y, incY)
			yCopy := make([]complex128, len(y))
			copy(yCopy, y)

			want := test.want
			if incX*incY < 0 {
				want = test.wantRev
			}

			got := impl.Zdotu(n, x, incX, y, incY)

			prefix := fmt.Sprintf("Case %v (incX=%v,incY=%v):", tc, incX, incY)

			if !zsame(x, xCopy) {
				t.Errorf("%v: unexpected modification of x", prefix)
			}
			if !zsame(y, yCopy) {
				t.Errorf("%v: unexpected modification of y", prefix)
			}

			if got != want {
				t.Errorf("%v: unexpected result. want %v, got %v", prefix, want, got)
			}
		}
	}
}
