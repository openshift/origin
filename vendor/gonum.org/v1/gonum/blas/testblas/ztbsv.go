// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/blas"
)

type Ztbsver interface {
	Ztbsv(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n, k int, ab []complex128, ldab int, x []complex128, incX int)

	Ztbmver
}

func ZtbsvTest(t *testing.T, impl Ztbsver) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
			for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
				for _, n := range []int{1, 2, 3, 4, 10} {
					for k := 0; k < n; k++ {
						for _, ldab := range []int{k + 1, k + 1 + 10} {
							for _, incX := range []int{-4, 1, 5} {
								ztbsvTest(t, impl, rnd, uplo, trans, diag, n, k, ldab, incX)
							}
						}
					}
				}
			}
		}
	}
}

// ztbsvTest tests Ztbsv by checking whether Ztbmv followed by Ztbsv
// round-trip.
func ztbsvTest(t *testing.T, impl Ztbsver, rnd *rand.Rand, uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n, k, ldab, incX int) {
	const tol = 1e-10

	// Allocate a dense-storage triangular band matrix filled with NaNs that
	// will be used as a for creating the actual triangular band matrix.
	lda := max(1, n)
	a := makeZGeneral(nil, n, n, lda)
	// Fill the referenced triangle of A with random data within the band
	// and with zeros outside.
	if uplo == blas.Upper {
		for i := 0; i < n; i++ {
			for j := i; j < min(n, i+k+1); j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
			for j := i + k + 1; j < n; j++ {
				a[i*lda+j] = 0
			}
		}
	} else {
		for i := 0; i < n; i++ {
			for j := 0; j < i-k; j++ {
				a[i*lda+j] = 0
			}
			for j := max(0, i-k); j <= i; j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
		}
	}
	if diag == blas.Unit {
		// The diagonal should not be referenced by Ztbmv and Ztbsv, so
		// invalidate it with NaNs.
		for i := 0; i < n; i++ {
			a[i*lda+i] = znan
		}
	}
	// Create the triangular band matrix.
	ab := zPackTriBand(k, ldab, uplo, n, a, lda)
	abCopy := make([]complex128, len(ab))
	copy(abCopy, ab)

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
	impl.Ztbmv(uplo, trans, diag, n, k, ab, ldab, x, incX)
	// Solve A*x = b, that is, x = A^{-1}*b = A^{-1}*A*x.
	impl.Ztbsv(uplo, trans, diag, n, k, ab, ldab, x, incX)
	// If Ztbsv is correct, A^{-1}*A = I and x contains again its original value.

	name := fmt.Sprintf("uplo=%v,trans=%v,diag=%v,n=%v,k=%v,ldab=%v,incX=%v", uplo, trans, diag, n, k, ldab, incX)
	if !zsame(ab, abCopy) {
		t.Errorf("%v: unexpected modification of A", name)
	}
	if !zSameAtNonstrided(x, want, incX) {
		t.Errorf("%v: unexpected modification of x\nwant %v\ngot  %v", name, want, x)
	}
	if !zEqualApproxAtStrided(x, want, incX, tol) {
		t.Errorf("%v: unexpected result\nwant %v\ngot  %v", name, want, x)
	}
}
