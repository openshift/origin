// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Zgemver interface {
	Zgemv(trans blas.Transpose, m, n int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int)
}

func ZgemvTest(t *testing.T, impl Zgemver) {
	for tc, test := range []struct {
		trans blas.Transpose
		alpha complex128
		a     []complex128
		x     []complex128
		beta  complex128
		y     []complex128

		want      []complex128
		wantXNeg  []complex128
		wantYNeg  []complex128
		wantXYNeg []complex128
	}{
		{
			trans: blas.NoTrans,
			alpha: 1 + 2i,
			beta:  3 + 4i,
		},
		{
			trans: blas.NoTrans,
			alpha: 1 + 2i,
			a: []complex128{
				9 + 5i, -2 + 6i, 5 + 1i, 9 + 2i, 10 + 4i,
				0 - 7i, 9 - 9i, 5 + 3i, -8 - 1i, 7 - 7i,
				10 - 7i, -1 + 3i, 2 + 2i, 7 + 6i, 9 + 1i,
				10 + 0i, 8 - 6i, 4 - 6i, -2 - 10i, -5 + 0i,
			},
			x: []complex128{
				4 - 9i,
				8 + 5i,
				-2 - 10i,
				2 - 4i,
				-6 + 6i,
			},
			beta: 3 + 4i,
			y: []complex128{
				-2 + 3i,
				10 + 5i,
				-8 - 5i,
				-8 + 7i,
			},
			want: []complex128{
				101 - 116i,
				58 + 166i,
				126 - 242i,
				336 - 75i,
			},
			wantXNeg: []complex128{
				98 + 128i,
				374 - 252i,
				-113 + 205i,
				-60 - 312i,
			},
			wantYNeg: []complex128{
				370 - 63i,
				140 - 140i,
				44 + 64i,
				67 - 128i,
			},
			wantXYNeg: []complex128{
				-26 - 300i,
				-99 + 307i,
				360 - 354i,
				64 + 116i,
			},
		},
		{
			trans: blas.Trans,
			alpha: 1 + 2i,
			a: []complex128{
				9 + 5i, -2 + 6i, 5 + 1i, 9 + 2i, 10 + 4i,
				0 - 7i, 9 - 9i, 5 + 3i, -8 - 1i, 7 - 7i,
				10 - 7i, -1 + 3i, 2 + 2i, 7 + 6i, 9 + 1i,
				10 + 0i, 8 - 6i, 4 - 6i, -2 - 10i, -5 + 0i,
			},
			x: []complex128{
				4 - 9i,
				8 + 5i,
				-2 - 10i,
				2 - 4i,
			},
			beta: 3 + 4i,
			y: []complex128{
				8 - 6i,
				-8 - 2i,
				9 + 5i,
				4 - 1i,
				6 - 4i,
			},
			want: []complex128{
				580 - 137i,
				221 + 311i,
				149 + 115i,
				443 - 208i,
				517 + 143i,
			},
			wantXNeg: []complex128{
				387 + 152i,
				109 - 433i,
				225 - 53i,
				-246 + 44i,
				13 + 20i,
			},
			wantYNeg: []complex128{
				531 + 145i,
				411 - 259i,
				149 + 115i,
				253 + 362i,
				566 - 139i,
			},
			wantXYNeg: []complex128{
				27 + 22i,
				-278 - 7i,
				225 - 53i,
				141 - 382i,
				373 + 150i,
			},
		},
		{
			trans: blas.ConjTrans,
			alpha: 1 + 2i,
			a: []complex128{
				9 + 5i, -2 + 6i, 5 + 1i, 9 + 2i, 10 + 4i,
				0 - 7i, 9 - 9i, 5 + 3i, -8 - 1i, 7 - 7i,
				10 - 7i, -1 + 3i, 2 + 2i, 7 + 6i, 9 + 1i,
				10 + 0i, 8 - 6i, 4 - 6i, -2 - 10i, -5 + 0i,
			},
			x: []complex128{
				4 - 9i,
				8 + 5i,
				-2 - 10i,
				2 - 4i,
			},
			beta: 3 + 4i,
			y: []complex128{
				8 - 6i,
				-8 - 2i,
				9 + 5i,
				4 - 1i,
				6 - 4i,
			},
			want: []complex128{
				472 - 133i,
				-253 + 23i,
				217 + 131i,
				229 - 316i,
				187 - 97i,
			},
			wantXNeg: []complex128{
				289 + 276i,
				499 + 47i,
				237 + 91i,
				54 + 504i,
				251 + 196i,
			},
			wantYNeg: []complex128{
				201 - 95i,
				197 - 367i,
				217 + 131i,
				-221 + 74i,
				458 - 135i,
			},
			wantXYNeg: []complex128{
				265 + 198i,
				22 + 453i,
				237 + 91i,
				531 + 98i,
				275 + 274i,
			},
		},
		{
			trans: blas.ConjTrans,
			alpha: 1 + 2i,
			a: []complex128{
				9 + 5i, -2 + 6i, 5 + 1i, 9 + 2i, 10 + 4i,
				0 - 7i, 9 - 9i, 5 + 3i, -8 - 1i, 7 - 7i,
				10 - 7i, -1 + 3i, 2 + 2i, 7 + 6i, 9 + 1i,
				10 + 0i, 8 - 6i, 4 - 6i, -2 - 10i, -5 + 0i,
			},
			x: []complex128{
				4 - 9i,
				8 + 5i,
				-2 - 10i,
				2 - 4i,
			},
			beta: 0,
			y: []complex128{
				8 - 6i,
				-8 - 2i,
				9 + 5i,
				4 - 1i,
				6 - 4i,
			},
			want: []complex128{
				424 - 147i,
				-237 + 61i,
				210 + 80i,
				213 - 329i,
				153 - 109i,
			},
			wantXNeg: []complex128{
				241 + 262i,
				515 + 85i,
				230 + 40i,
				38 + 491i,
				217 + 184i,
			},
			wantYNeg: []complex128{
				153 - 109i,
				213 - 329i,
				210 + 80i,
				-237 + 61i,
				424 - 147i,
			},
			wantXYNeg: []complex128{
				217 + 184i,
				38 + 491i,
				230 + 40i,
				515 + 85i,
				241 + 262i,
			},
		},
		{
			trans: blas.ConjTrans,
			alpha: 0,
			a: []complex128{
				9 + 5i, -2 + 6i, 5 + 1i, 9 + 2i, 10 + 4i,
				0 - 7i, 9 - 9i, 5 + 3i, -8 - 1i, 7 - 7i,
				10 - 7i, -1 + 3i, 2 + 2i, 7 + 6i, 9 + 1i,
				10 + 0i, 8 - 6i, 4 - 6i, -2 - 10i, -5 + 0i,
			},
			x: []complex128{
				4 - 9i,
				8 + 5i,
				-2 - 10i,
				2 - 4i,
			},
			beta: 3 + 4i,
			y: []complex128{
				8 - 6i,
				-8 - 2i,
				9 + 5i,
				4 - 1i,
				6 - 4i,
			},
			want: []complex128{
				48 + 14i,
				-16 - 38i,
				7 + 51i,
				16 + 13i,
				34 + 12i,
			},
			wantXNeg: []complex128{
				48 + 14i,
				-16 - 38i,
				7 + 51i,
				16 + 13i,
				34 + 12i,
			},
			wantYNeg: []complex128{
				48 + 14i,
				-16 - 38i,
				7 + 51i,
				16 + 13i,
				34 + 12i,
			},
			wantXYNeg: []complex128{
				48 + 14i,
				-16 - 38i,
				7 + 51i,
				16 + 13i,
				34 + 12i,
			},
		},
	} {
		var m, n int
		switch test.trans {
		case blas.NoTrans:
			m = len(test.y)
			n = len(test.x)
		case blas.Trans, blas.ConjTrans:
			m = len(test.x)
			n = len(test.y)
		}
		for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
			for _, incY := range []int{-11, -2, -1, 1, 2, 7} {
				for _, lda := range []int{max(1, n), n + 11} {
					alpha := test.alpha

					a := makeZGeneral(test.a, m, n, lda)
					aCopy := make([]complex128, len(a))
					copy(aCopy, a)

					x := makeZVector(test.x, incX)
					xCopy := make([]complex128, len(x))
					copy(xCopy, x)

					y := makeZVector(test.y, incY)

					impl.Zgemv(test.trans, m, n, alpha, a, lda, x, incX, test.beta, y, incY)

					if !zsame(x, xCopy) {
						t.Errorf("Case %v (incX=%v,incY=%v,lda=%v): unexpected modification of x", tc, incX, incY, lda)
					}
					if !zsame(a, aCopy) {
						t.Errorf("Case %v (incX=%v,incY=%v,lda=%v): unexpected modification of A", tc, incX, incY, lda)
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
						t.Errorf("Case %v (incX=%v,incY=%v,lda=%v): unexpected result\nwant %v\ngot  %v", tc, incX, incY, lda, want, y)
					}
				}
			}
		}
	}
}
