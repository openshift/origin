// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/blas"
)

type Zhbmver interface {
	Zhbmv(uplo blas.Uplo, n, k int, alpha complex128, ab []complex128, ldab int, x []complex128, incX int, beta complex128, y []complex128, incY int)

	Zhemver
}

func ZhbmvTest(t *testing.T, impl Zhbmver) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, n := range []int{0, 1, 2, 3, 5} {
			for k := 0; k < n; k++ {
				for _, ldab := range []int{k + 1, k + 1 + 10} {
					// Generate all possible combinations of given increments.
					// Use slices to reduce indentation.
					for _, inc := range allPairs([]int{-11, 1, 7}, []int{-3, 1, 5}) {
						incX := inc[0]
						incY := inc[1]
						for _, ab := range []struct {
							alpha complex128
							beta  complex128
						}{
							// All potentially relevant values of
							// alpha and beta.
							{0, 0},
							{0, 1},
							{0, complex(rnd.NormFloat64(), rnd.NormFloat64())},
							{complex(rnd.NormFloat64(), rnd.NormFloat64()), 0},
							{complex(rnd.NormFloat64(), rnd.NormFloat64()), 1},
							{complex(rnd.NormFloat64(), rnd.NormFloat64()), complex(rnd.NormFloat64(), rnd.NormFloat64())},
						} {
							testZhbmv(t, impl, rnd, uplo, n, k, ab.alpha, ab.beta, ldab, incX, incY)
						}
					}
				}
			}
		}
	}
}

// testZhbmv tests Zhbmv by comparing its output to that of Zhemv.
func testZhbmv(t *testing.T, impl Zhbmver, rnd *rand.Rand, uplo blas.Uplo, n, k int, alpha, beta complex128, ldab, incX, incY int) {
	const tol = 1e-13

	// Allocate a dense-storage Hermitian band matrix filled with NaNs that will be
	// used as the reference matrix for Zhemv.
	lda := max(1, n)
	a := makeZGeneral(nil, n, n, lda)
	// Fill the matrix with zeros.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			a[i*lda+j] = 0
		}
	}
	// Fill the triangle band with random data, invalidating the imaginary
	// part of diagonal elements because it should not be referenced by
	// Zhbmv and Zhemv.
	if uplo == blas.Upper {
		for i := 0; i < n; i++ {
			a[i*lda+i] = complex(rnd.NormFloat64(), math.NaN())
			for j := i + 1; j < min(n, i+k+1); j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
		}
	} else {
		for i := 0; i < n; i++ {
			for j := max(0, i-k); j < i; j++ {
				re := rnd.NormFloat64()
				im := rnd.NormFloat64()
				a[i*lda+j] = complex(re, im)
			}
			a[i*lda+i] = complex(rnd.NormFloat64(), math.NaN())
		}
	}
	// Create the actual Hermitian band matrix.
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
	xCopy := make([]complex128, len(x))
	copy(xCopy, x)

	// Generate a random complex vector y.
	ytest := make([]complex128, n)
	for i := range ytest {
		re := rnd.NormFloat64()
		im := rnd.NormFloat64()
		ytest[i] = complex(re, im)
	}
	y := makeZVector(ytest, incY)

	want := make([]complex128, len(y))
	copy(want, y)

	// Compute the reference result of alpha*op(A)*x + beta*y, storing it
	// into want.
	impl.Zhemv(uplo, n, alpha, a, lda, x, incX, beta, want, incY)
	// Compute alpha*op(A)*x + beta*y, storing the result in-place into y.
	impl.Zhbmv(uplo, n, k, alpha, ab, ldab, x, incX, beta, y, incY)

	prefix := fmt.Sprintf("uplo=%v,n=%v,k=%v,incX=%v,incY=%v,ldab=%v", uplo, n, k, incX, incY, ldab)
	if !zsame(x, xCopy) {
		t.Errorf("%v: unexpected modification of x", prefix)
	}
	if !zsame(ab, abCopy) {
		t.Errorf("%v: unexpected modification of ab", prefix)
	}
	if !zSameAtNonstrided(y, want, incY) {
		t.Errorf("%v: unexpected modification of y", prefix)
	}
	if !zEqualApproxAtStrided(y, want, incY, tol) {
		t.Errorf("%v: unexpected result\nwant %v\ngot  %v", prefix, want, y)
	}
}
