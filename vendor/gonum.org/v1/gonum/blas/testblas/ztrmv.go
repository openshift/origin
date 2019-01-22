// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

var ztrmvTestCases = []struct {
	uplo blas.Uplo
	a    []complex128
	x    []complex128

	// Results with non-unit diagonal.
	want             []complex128
	wantNeg          []complex128
	wantTrans        []complex128
	wantTransNeg     []complex128
	wantConjTrans    []complex128
	wantConjTransNeg []complex128

	// Results with unit diagonal.
	wantUnit             []complex128
	wantUnitNeg          []complex128
	wantUnitTrans        []complex128
	wantUnitTransNeg     []complex128
	wantUnitConjTrans    []complex128
	wantUnitConjTransNeg []complex128
}{
	{uplo: blas.Upper},
	{uplo: blas.Lower},
	{
		uplo: blas.Upper,
		a: []complex128{
			6 - 8i, -10 + 10i, -6 - 3i, -1 - 8i,
			znan, 7 + 8i, -7 + 9i, 3 + 6i,
			znan, znan, 6 - 4i, -2 - 5i,
			znan, znan, znan, 4 - 8i,
		},
		x: []complex128{
			10 - 5i,
			-2 + 2i,
			8 - 1i,
			-7 + 9i,
		},

		want: []complex128{
			48 - 121i,
			-152 + 62i,
			103 - 21i,
			44 + 92i,
		},
		wantNeg: []complex128{
			0 - 100i,
			-49 - 20i,
			120 + 70i,
			-72 + 119i,
		},
		wantTrans: []complex128{
			20 - 110i,
			-80 + 148i,
			-35 - 70i,
			-45 - 27i,
		},
		wantTransNeg: []complex128{
			123 - 2i,
			18 + 66i,
			44 - 103i,
			30 + 110i,
		},
		wantConjTrans: []complex128{
			100 + 50i,
			-148 - 20i,
			39 + 90i,
			-75 + 125i,
		},
		wantConjTransNeg: []complex128{
			27 - 70i,
			-70 - 136i,
			208 - 91i,
			-114 - 2i,
		},

		wantUnit: []complex128{
			38 - 16i,
			-124 + 66i,
			67 + 16i,
			-7 + 9i,
		},
		wantUnitNeg: []complex128{
			10 - 5i,
			-47 - 38i,
			64 + 12i,
			-109 + 18i,
		},
		wantUnitTrans: []complex128{
			10 - 5i,
			-52 + 152i,
			-71 - 33i,
			-96 - 110i,
		},
		wantUnitTransNeg: []complex128{
			133 + 93i,
			20 + 48i,
			-12 - 161i,
			-7 + 9i,
		},
		wantUnitConjTrans: []complex128{
			10 - 5i,
			-152 - 48i,
			-5 + 63i,
			18 + 154i,
		},
		wantUnitConjTransNeg: []complex128{
			-43 - 135i,
			-52 - 138i,
			168 - 21i,
			-7 + 9i,
		},
	},
	{
		uplo: blas.Lower,
		a: []complex128{
			10 - 8i, znan, znan, znan,
			1 - 6i, -4 + 8i, znan, znan,
			2 - 6i, 4 - 8i, 5 + 3i, znan,
			-7 - 4i, 1 + 3i, -2 - 4i, 9 + 8i,
		},
		x: []complex128{
			10 + 5i,
			-7 + 1i,
			3 - 1i,
			9 + 10i,
		},

		want: []complex128{
			140 - 30i,
			60 - 115i,
			48 + 14i,
			-69 + 57i,
		},
		wantNeg: []complex128{
			51 + 53i,
			44 - 78i,
			65 - 16i,
			170 + 28i,
		},
		wantTrans: []complex128{
			116 - 113i,
			3 - 51i,
			40 - 52i,
			1 + 162i,
		},
		wantTransNeg: []complex128{
			50 + 125i,
			-38 - 66i,
			-29 + 123i,
			109 - 22i,
		},
		wantConjTrans: []complex128{
			-44 + 71i,
			95 + 55i,
			-46 + 2i,
			161 + 18i,
		},
		wantConjTransNeg: []complex128{
			130 - 35i,
			-72 + 56i,
			-31 - 97i,
			-91 + 154i,
		},

		wantUnit: []complex128{
			10 + 5i,
			33 - 54i,
			33 + 9i,
			-61 - 95i,
		},
		wantUnitNeg: []complex128{
			11 - 67i,
			75 - 61i,
			72 - 45i,
			9 + 10i,
		},
		wantUnitTrans: []complex128{
			-14 - 78i,
			-24 + 10i,
			25 - 57i,
			9 + 10i,
		},
		wantUnitTransNeg: []complex128{
			10 + 5i,
			-7 - 49i,
			-22 + 94i,
			-52 - 40i,
		},
		wantUnitConjTrans: []complex128{
			-94 - 54i,
			52 + 4i,
			-55 + 15i,
			9 + 10i,
		},
		wantUnitConjTransNeg: []complex128{
			10 + 5i,
			-47 + 31i,
			-8 - 78i,
			-92 - 8i,
		},
	},
}

type Ztrmver interface {
	Ztrmv(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n int, a []complex128, lda int, x []complex128, incX int)
}

func ZtrmvTest(t *testing.T, impl Ztrmver) {
	for tc, test := range ztrmvTestCases {
		n := len(test.x)
		uplo := test.uplo
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
					for _, lda := range []int{max(1, n), n + 11} {
						a := makeZGeneral(test.a, n, n, lda)
						if diag == blas.Unit {
							for i := 0; i < n; i++ {
								a[i*lda+i] = znan
							}
						}
						aCopy := make([]complex128, len(a))
						copy(aCopy, a)

						x := makeZVector(test.x, incX)

						impl.Ztrmv(uplo, trans, diag, n, a, lda, x, incX)

						if !zsame(a, aCopy) {
							t.Errorf("Case %v (uplo=%v,trans=%v,diag=%v,lda=%v,incX=%v): unexpected modification of A", tc, uplo, trans, diag, lda, incX)
						}

						var want []complex128
						if diag == blas.NonUnit {
							switch {
							case trans == blas.NoTrans && incX > 0:
								want = makeZVector(test.want, incX)
							case trans == blas.NoTrans && incX < 0:
								want = makeZVector(test.wantNeg, incX)
							case trans == blas.Trans && incX > 0:
								want = makeZVector(test.wantTrans, incX)
							case trans == blas.Trans && incX < 0:
								want = makeZVector(test.wantTransNeg, incX)
							case trans == blas.ConjTrans && incX > 0:
								want = makeZVector(test.wantConjTrans, incX)
							case trans == blas.ConjTrans && incX < 0:
								want = makeZVector(test.wantConjTransNeg, incX)
							}
						} else {
							switch {
							case trans == blas.NoTrans && incX > 0:
								want = makeZVector(test.wantUnit, incX)
							case trans == blas.NoTrans && incX < 0:
								want = makeZVector(test.wantUnitNeg, incX)
							case trans == blas.Trans && incX > 0:
								want = makeZVector(test.wantUnitTrans, incX)
							case trans == blas.Trans && incX < 0:
								want = makeZVector(test.wantUnitTransNeg, incX)
							case trans == blas.ConjTrans && incX > 0:
								want = makeZVector(test.wantUnitConjTrans, incX)
							case trans == blas.ConjTrans && incX < 0:
								want = makeZVector(test.wantUnitConjTransNeg, incX)
							}
						}
						if !zsame(x, want) {
							t.Errorf("Case %v (uplo=%v,trans=%v,diag=%v,lda=%v,incX=%v): unexpected result\nwant %v\ngot  %v", tc, uplo, trans, diag, lda, incX, want, x)
						}
					}
				}
			}
		}
	}
}
