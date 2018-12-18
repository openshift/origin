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

type Ztrsver interface {
	Ztrsv(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n int, a []complex128, lda int, x []complex128, incX int)

	Ztrmver
}

func ZtrsvTest(t *testing.T, impl Ztrsver) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, n := range []int{0, 1, 2, 3, 4, 10} {
					for _, lda := range []int{max(1, n), n + 11} {
						for _, incX := range []int{-11, -3, -2, -1, 1, 2, 3, 7} {
							ztrsvTest(t, impl, uplo, trans, diag, n, lda, incX, rnd)
						}
					}
				}
			}
		}
	}
}

// ztrsvTest tests Ztrsv by checking whether Ztrmv followed by Ztrsv
// round-trip.
func ztrsvTest(t *testing.T, impl Ztrsver, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n, lda, incX int, rnd *rand.Rand) {
	const tol = 1e-10

	// Allocate a dense-storage triangular matrix A filled with NaNs.
	a := makeZGeneral(nil, n, n, lda)
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
		// The diagonal should not be referenced by Ztrmv and Ztrsv, so
		// invalidate it with NaNs.
		for i := 0; i < n; i++ {
			a[i*lda+i] = znan
		}
	}
	aCopy := make([]complex128, len(a))
	copy(aCopy, a)

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
	impl.Ztrmv(uplo, trans, diag, n, a, lda, x, incX)
	// Solve A*x = b, that is, x = A^{-1}*b = A^{-1}*A*x.
	impl.Ztrsv(uplo, trans, diag, n, a, lda, x, incX)
	// If Ztrsv is correct, A^{-1}*A = I and x contains again its original value.

	name := fmt.Sprintf("uplo=%v,trans=%v,diag=%v,n=%v,lda=%v,incX=%v", uplo, trans, diag, n, lda, incX)
	if !zsame(a, aCopy) {
		t.Errorf("%v: unexpected modification of A", name)
	}
	if !zSameAtNonstrided(x, want, incX) {
		t.Errorf("%v: unexpected modification of x\nwant %v\ngot  %v", name, want, x)
	}
	if !zEqualApproxAtStrided(x, want, incX, tol) {
		t.Errorf("%v: unexpected result\nwant %v\ngot  %v", name, want, x)
	}
}
