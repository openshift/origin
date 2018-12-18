// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/blas"
)

type Ztpsver interface {
	Ztpsv(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n int, ap []complex128, x []complex128, incX int)

	Ztpmver
}

func ZtpsvTest(t *testing.T, impl Ztpsver) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, n := range []int{0, 1, 2, 3, 4, 10} {
					for _, incX := range []int{-11, -3, -2, -1, 1, 2, 3, 7} {
						ztpsvTest(t, impl, uplo, trans, diag, n, incX, rnd)
					}
				}
			}
		}
	}
}

// ztpsvTest tests Ztpsv by checking whether Ztpmv followed by Ztpsv
// round-trip.
func ztpsvTest(t *testing.T, impl Ztpsver, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n, incX int, rnd *rand.Rand) {
	const tol = 1e-10

	// Allocate a dense-storage triangular matrix filled with NaNs that
	// will be used as a for creating the actual triangular matrix in packed
	// storage.
	lda := n
	a := makeZGeneral(nil, n, n, max(1, lda))
	// Fill the referenced triangle of A with random data.
	if uplo == blas.Upper {
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
		}
	} else {
		for i := 0; i < n; i++ {
			for j := 0; j <= i; j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
		}
	}
	if diag == blas.Unit {
		// The diagonal should not be referenced by Ztpmv and Ztpsv, so
		// invalidate it with NaNs.
		for i := 0; i < n; i++ {
			a[i*lda+i] = znan
		}
	}
	// Create the triangular matrix in packed storage.
	ap := zPack(uplo, n, a, n)
	apCopy := make([]complex128, len(ap))
	copy(apCopy, ap)

	// Generate a random complex vector x.
	xtest := make([]complex128, n)
	for i := range xtest {
		re := rnd.NormFloat64()
		im := rnd.NormFloat64()
		xtest[i] = complex(re, im)
	}
	x := makeZVector(xtest, incX)

	// Store a copy of x as the correct result that we want.
	want := make([]complex128, len(x))
	copy(want, x)

	// Compute A*x, denoting the result by b and storing it in x.
	impl.Ztpmv(uplo, trans, diag, n, ap, x, incX)
	// Solve A*x = b, that is, x = A^{-1}*b = A^{-1}*A*x.
	impl.Ztpsv(uplo, trans, diag, n, ap, x, incX)
	// If Ztpsv is correct, A^{-1}*A = I and x contains again its original value.

	name := fmt.Sprintf("uplo=%v,trans=%v,diag=%v,n=%v,incX=%v", uplo, trans, diag, n, incX)
	if !zsame(ap, apCopy) {
		t.Errorf("%v: unexpected modification of ap", name)
	}
	if !zSameAtNonstrided(x, want, incX) {
		t.Errorf("%v: unexpected modification of x\nwant %v\ngot  %v", name, want, x)
	}
	if !zEqualApproxAtStrided(x, want, incX, tol) {
		t.Errorf("%v: unexpected result\nwant %v\ngot  %v", name, want, x)
	}
}
