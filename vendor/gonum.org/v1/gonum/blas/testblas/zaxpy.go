// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"
)

type Zaxpyer interface {
	Zaxpy(n int, alpha complex128, x []complex128, incX int, y []complex128, incY int)
}

func ZaxpyTest(t *testing.T, impl Zaxpyer) {
	for tc, test := range []struct {
		alpha complex128
		x, y  []complex128

		want    []complex128 // Result when both increments have the same sign.
		wantRev []complex128 // Result when the increments have opposite sign.
	}{
		{
			alpha:   0,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
			want:    []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
			wantRev: []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
		},
		{
			alpha:   1,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
			want:    []complex128{31 + 33i, 36 + 38i, 41 + 43i, 46 + 48i, 51 + 53i, 56 + 58i, 61 + 63i, 66 + 68i, 71 + 73i, 76 + 78i, 81 + 83i, 86 + 88i},
			wantRev: []complex128{53 + 55i, 54 + 56i, 55 + 57i, 56 + 58i, 57 + 59i, 58 + 60i, 59 + 61i, 60 + 62i, 61 + 63i, 62 + 64i, 63 + 65i, 64 + 66i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i},
			y:       []complex128{30 + 31i},
			want:    []complex128{19 + 44i},
			wantRev: []complex128{19 + 44i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i},
			y:       []complex128{30 + 31i, 33 + 34i},
			want:    []complex128{19 + 44i, 14 + 67i},
			wantRev: []complex128{11 + 64i, 22 + 47i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i},
			wantRev: []complex128{3 + 84i, 14 + 67i, 25 + 50i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i},
			wantRev: []complex128{-5 + 104i, 6 + 87i, 17 + 70i, 28 + 53i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i},
			wantRev: []complex128{-13 + 124i, -2 + 107i, 9 + 90i, 20 + 73i, 31 + 56i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i},
			wantRev: []complex128{-21 + 144i, -10 + 127i, 1 + 110i, 12 + 93i, 23 + 76i, 34 + 59i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i},
			wantRev: []complex128{-29 + 164i, -18 + 147i, -7 + 130i, 4 + 113i, 15 + 96i, 26 + 79i, 37 + 62i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i, -16 + 205i},
			wantRev: []complex128{-37 + 184i, -26 + 167i, -15 + 150i, -4 + 133i, 7 + 116i, 18 + 99i, 29 + 82i, 40 + 65i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i, -16 + 205i, -21 + 228i},
			wantRev: []complex128{-45 + 204i, -34 + 187i, -23 + 170i, -12 + 153i, -1 + 136i, 10 + 119i, 21 + 102i, 32 + 85i, 43 + 68i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i, -16 + 205i, -21 + 228i, -26 + 251i},
			wantRev: []complex128{-53 + 224i, -42 + 207i, -31 + 190i, -20 + 173i, -9 + 156i, 2 + 139i, 13 + 122i, 24 + 105i, 35 + 88i, 46 + 71i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i, -16 + 205i, -21 + 228i, -26 + 251i, -31 + 274i},
			wantRev: []complex128{-61 + 244i, -50 + 227i, -39 + 210i, -28 + 193i, -17 + 176i, -6 + 159i, 5 + 142i, 16 + 125i, 27 + 108i, 38 + 91i, 49 + 74i},
		},
		{
			alpha:   3 + 7i,
			x:       []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			y:       []complex128{30 + 31i, 33 + 34i, 36 + 37i, 39 + 40i, 42 + 43i, 45 + 46i, 48 + 49i, 51 + 52i, 54 + 55i, 57 + 58i, 60 + 61i, 63 + 64i},
			want:    []complex128{19 + 44i, 14 + 67i, 9 + 90i, 4 + 113i, -1 + 136i, -6 + 159i, -11 + 182i, -16 + 205i, -21 + 228i, -26 + 251i, -31 + 274i, -36 + 297i},
			wantRev: []complex128{-69 + 264i, -58 + 247i, -47 + 230i, -36 + 213i, -25 + 196i, -14 + 179i, -3 + 162i, 8 + 145i, 19 + 128i, 30 + 111i, 41 + 94i, 52 + 77i},
		},
	} {
		n := len(test.x)
		if len(test.y) != n || len(test.want) != n || len(test.wantRev) != n {
			panic("bad test")
		}
		for _, inc := range allPairs([]int{-7, -3, 1, 13}, []int{-11, -5, 1, 17}) {
			incX := inc[0]
			incY := inc[1]

			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			y := makeZVector(test.y, incY)

			var want []complex128
			if incX*incY > 0 {
				want = makeZVector(test.want, incY)
			} else {
				want = makeZVector(test.wantRev, incY)
			}

			impl.Zaxpy(n, test.alpha, x, incX, y, incY)

			prefix := fmt.Sprintf("Case %v (incX=%v,incY=%v):", tc, incX, incY)

			if !zsame(x, xCopy) {
				t.Errorf("%v: unexpected modification of x", prefix)
			}

			if !zsame(y, want) {
				t.Errorf("%v: unexpected y:\nwant %v\ngot %v", prefix, want, y)
			}
		}
	}
}
