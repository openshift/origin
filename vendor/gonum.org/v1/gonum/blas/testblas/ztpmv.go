// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"gonum.org/v1/gonum/blas"
)

type Ztpmver interface {
	Ztpmv(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n int, ap []complex128, x []complex128, incX int)
}

func ZtpmvTest(t *testing.T, impl Ztpmver) {
	for tc, test := range ztrmvTestCases {
		n := len(test.x)
		uplo := test.uplo
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, incX := range []int{-11, -2, -1, 1, 2, 7} {
					ap := zPack(uplo, n, test.a, n)
					apCopy := make([]complex128, len(ap))
					copy(apCopy, ap)

					x := makeZVector(test.x, incX)

					impl.Ztpmv(uplo, trans, diag, n, ap, x, incX)

					if !zsame(ap, apCopy) {
						t.Errorf("Case %v (uplo=%v,trans=%v,diag=%v,incX=%v): unexpected modification of A", tc, uplo, trans, diag, incX)
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
						t.Errorf("Case %v (uplo=%v,trans=%v,diag=%v,incX=%v): unexpected result\nwant %v\ngot  %v", tc, uplo, trans, diag, incX, want, x)
					}
				}
			}
		}
	}
}
