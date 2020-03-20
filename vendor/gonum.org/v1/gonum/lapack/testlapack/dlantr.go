// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dlantrer interface {
	Dlanger
	Dlantr(norm lapack.MatrixNorm, uplo blas.Uplo, diag blas.Diag, m, n int, a []float64, lda int, work []float64) float64
}

func DlantrTest(t *testing.T, impl Dlantrer) {
	rnd := rand.New(rand.NewSource(1))
	for _, m := range []int{0, 1, 2, 3, 4, 5, 10} {
		for _, n := range []int{0, 1, 2, 3, 4, 5, 10} {
			for _, uplo := range []blas.Uplo{blas.Lower, blas.Upper} {
				if uplo == blas.Upper && m > n {
					continue
				}
				if uplo == blas.Lower && n > m {
					continue
				}
				for _, diag := range []blas.Diag{blas.NonUnit, blas.Unit} {
					for _, lda := range []int{max(1, n), n + 3} {
						dlantrTest(t, impl, rnd, uplo, diag, m, n, lda)
					}
				}
			}
		}
	}
}

func dlantrTest(t *testing.T, impl Dlantrer, rnd *rand.Rand, uplo blas.Uplo, diag blas.Diag, m, n, lda int) {
	const tol = 1e-14

	// Generate a random triangular matrix. If the matrix has unit diagonal,
	// don't set the diagonal elements to 1.
	a := make([]float64, max(0, (m-1)*lda+n))
	for i := range a {
		a[i] = rnd.NormFloat64()
	}
	rowsum := make([]float64, m)
	colsum := make([]float64, n)
	var frobWant, maxabsWant float64
	if diag == blas.Unit {
		// Account for the unit diagonal.
		for i := 0; i < min(m, n); i++ {
			rowsum[i] = 1
			colsum[i] = 1
		}
		frobWant = float64(min(m, n))
		if min(m, n) > 0 {
			maxabsWant = 1
		}
	}
	if uplo == blas.Upper {
		for i := 0; i < min(m, n); i++ {
			start := i
			if diag == blas.Unit {
				start = i + 1
			}
			for j := start; j < n; j++ {
				aij := 2*rnd.Float64() - 1
				a[i*lda+j] = aij
				rowsum[i] += math.Abs(aij)
				colsum[j] += math.Abs(aij)
				maxabsWant = math.Max(maxabsWant, math.Abs(aij))
				frobWant += aij * aij
			}
		}
	} else {
		for i := 0; i < m; i++ {
			end := i
			if diag == blas.Unit {
				end = i - 1
			}
			for j := 0; j <= min(end, n-1); j++ {
				aij := 2*rnd.Float64() - 1
				a[i*lda+j] = aij
				rowsum[i] += math.Abs(aij)
				colsum[j] += math.Abs(aij)
				maxabsWant = math.Max(maxabsWant, math.Abs(aij))
				frobWant += aij * aij
			}
		}
	}
	frobWant = math.Sqrt(frobWant)
	var maxcolsumWant, maxrowsumWant float64
	if n > 0 {
		maxcolsumWant = floats.Max(colsum)
	}
	if m > 0 {
		maxrowsumWant = floats.Max(rowsum)
	}

	aCopy := make([]float64, len(a))
	copy(aCopy, a)

	for _, norm := range []lapack.MatrixNorm{lapack.MaxAbs, lapack.MaxColumnSum, lapack.MaxRowSum, lapack.Frobenius} {
		name := fmt.Sprintf("norm=%v,uplo=%v,diag=%v,m=%v,n=%v,lda=%v", string(norm), string(uplo), string(diag), m, n, lda)

		var work []float64
		if norm == lapack.MaxColumnSum {
			work = make([]float64, n)
		}
		normGot := impl.Dlantr(norm, uplo, diag, m, n, a, lda, work)

		if !floats.Equal(a, aCopy) {
			t.Fatalf("%v: unexpected modification of a", name)
		}

		if norm == lapack.MaxAbs {
			// MaxAbs norm involves no floating-point computation,
			// so we expect exact equality here.
			if normGot != maxabsWant {
				t.Errorf("%v: unexpected result; got %v, want %v", name, normGot, maxabsWant)
			}
			continue
		}

		var normWant float64
		switch norm {
		case lapack.MaxColumnSum:
			normWant = maxcolsumWant
		case lapack.MaxRowSum:
			normWant = maxrowsumWant
		case lapack.Frobenius:
			normWant = frobWant
		}
		if math.Abs(normGot-normWant) >= tol {
			t.Errorf("%v: unexpected result; got %v, want %v", name, normGot, normWant)
		}
	}
}
