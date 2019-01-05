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

type Zgbmver interface {
	Zgbmv(trans blas.Transpose, m, n, kL, kU int, alpha complex128, ab []complex128, ldab int, x []complex128, incX int, beta complex128, y []complex128, incY int)

	Zgemver
}

func ZgbmvTest(t *testing.T, impl Zgbmver) {
	rnd := rand.New(rand.NewSource(1))
	for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
		// Generate all possible size combinations.
		for _, mn := range allPairs([]int{1, 2, 3, 5}, []int{1, 2, 3, 5}) {
			m := mn[0]
			n := mn[1]
			// Generate all possible numbers of lower and upper
			// diagonals. Use slices to reduce indentation.
			kLs := make([]int, max(1, m))
			for i := range kLs {
				kLs[i] = i
			}
			kUs := make([]int, max(1, n))
			for i := range kUs {
				kUs[i] = i
			}
			for _, ks := range allPairs(kLs, kUs) {
				kL := ks[0]
				kU := ks[1]
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
					for _, ldab := range []int{kL + kU + 1, kL + kU + 20} {
						for _, inc := range allPairs([]int{-3, -2, -1, 1, 2, 3}, []int{-3, -2, -1, 1, 2, 3}) {
							incX := inc[0]
							incY := inc[1]
							testZgbmv(t, impl, rnd, trans, m, n, kL, kU, ab.alpha, ab.beta, ldab, incX, incY)
						}
					}
				}
			}
		}
	}
}

// testZgbmv tests Zgbmv by comparing its output to that of Zgemv.
func testZgbmv(t *testing.T, impl Zgbmver, rnd *rand.Rand, trans blas.Transpose, m, n, kL, kU int, alpha, beta complex128, ldab, incX, incY int) {
	const tol = 1e-13

	// Allocate a dense-storage band matrix filled with NaNs that will be
	// used as the reference matrix for Zgemv.
	lda := max(1, n)
	a := makeZGeneral(nil, m, n, lda)
	// Fill the matrix with zeros.
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			a[i*lda+j] = 0
		}
	}
	// Fill the band with random data.
	for i := 0; i < m; i++ {
		for j := max(0, i-kL); j < min(n, i+kU+1); j++ {
			re := rnd.NormFloat64()
			im := rnd.NormFloat64()
			a[i*lda+j] = complex(re, im)
		}
	}
	// Create the actual band matrix.
	ab := zPackBand(kL, kU, ldab, m, n, a, lda)
	abCopy := make([]complex128, len(ab))
	copy(abCopy, ab)

	// Compute correct lengths of vectors x and y.
	var lenX, lenY int
	switch trans {
	case blas.NoTrans:
		lenX = n
		lenY = m
	case blas.Trans, blas.ConjTrans:
		lenX = m
		lenY = n
	}

	// Generate a random complex vector x.
	xtest := make([]complex128, lenX)
	for i := range xtest {
		re := rnd.NormFloat64()
		im := rnd.NormFloat64()
		xtest[i] = complex(re, im)
	}
	x := makeZVector(xtest, incX)
	xCopy := make([]complex128, len(x))
	copy(xCopy, x)

	// Generate a random complex vector y.
	ytest := make([]complex128, lenY)
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
	impl.Zgemv(trans, m, n, alpha, a, lda, x, incX, beta, want, incY)
	// Compute alpha*op(A)*x + beta*y, storing the result in-place into y.
	impl.Zgbmv(trans, m, n, kL, kU, alpha, ab, ldab, x, incX, beta, y, incY)

	name := fmt.Sprintf("trans=%v,m=%v,n=%v,kL=%v,kU=%v,lda=%v,incX=%v,incY=%v", trans, m, n, kL, kU, lda, incX, incY)
	if !zsame(ab, abCopy) {
		t.Errorf("%v: unexpected modification of ab", name)
	}
	if !zsame(x, xCopy) {
		t.Errorf("%v: unexpected modification of x", name)
	}
	if !zSameAtNonstrided(y, want, incY) {
		t.Errorf("%v: unexpected modification of y", name)
	}
	if !zEqualApproxAtStrided(y, want, incY, tol) {
		t.Errorf("%v: unexpected result\ngot  %v\nwant %v\n", name, y, want)
	}
}
