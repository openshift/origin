// Copyright ©2019 The Gonum Authors. All rights reserved.
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

type Dlansber interface {
	Dlansb(norm lapack.MatrixNorm, uplo blas.Uplo, n, kd int, ab []float64, ldab int, work []float64) float64
}

func DlansbTest(t *testing.T, impl Dlansber) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10} {
		for _, kd := range []int{0, (n + 1) / 4, (3*n - 1) / 4, (5*n + 1) / 4} {
			for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
				for _, ldab := range []int{kd + 1, kd + 1 + 7} {
					dlansbTest(t, impl, rnd, uplo, n, kd, ldab)
				}
			}
		}
	}
}

func dlansbTest(t *testing.T, impl Dlansber, rnd *rand.Rand, uplo blas.Uplo, n, kd int, ldab int) {
	const tol = 1e-15

	// Generate a random symmetric band matrix and compute all its norms.
	ab := make([]float64, max(0, (n-1)*ldab+kd+1))
	rowsum := make([]float64, n)
	colsum := make([]float64, n)
	var frobWant, maxabsWant float64
	if uplo == blas.Upper {
		for i := 0; i < n; i++ {
			for jb := 0; jb < min(n-i, kd+1); jb++ {
				aij := 2*rnd.Float64() - 1
				ab[i*ldab+jb] = aij

				j := jb + i
				colsum[j] += math.Abs(aij)
				rowsum[i] += math.Abs(aij)
				maxabsWant = math.Max(maxabsWant, math.Abs(aij))
				frobWant += aij * aij
				if i != j {
					// Take into account the symmetric elements.
					colsum[i] += math.Abs(aij)
					rowsum[j] += math.Abs(aij)
					frobWant += aij * aij
				}
			}
		}
	} else {
		for i := 0; i < n; i++ {
			for jb := max(0, kd-i); jb < kd+1; jb++ {
				aij := 2*rnd.Float64() - 1
				ab[i*ldab+jb] = aij

				j := jb - kd + i
				colsum[j] += math.Abs(aij)
				rowsum[i] += math.Abs(aij)
				maxabsWant = math.Max(maxabsWant, math.Abs(aij))
				frobWant += aij * aij
				if i != j {
					// Take into account the symmetric elements.
					colsum[i] += math.Abs(aij)
					rowsum[j] += math.Abs(aij)
					frobWant += aij * aij
				}
			}
		}
	}
	frobWant = math.Sqrt(frobWant)
	var maxcolsumWant, maxrowsumWant float64
	if n > 0 {
		maxcolsumWant = floats.Max(colsum)
		maxrowsumWant = floats.Max(rowsum)
	}

	abCopy := make([]float64, len(ab))
	copy(abCopy, ab)

	work := make([]float64, n)
	var maxcolsumGot, maxrowsumGot float64
	for _, norm := range []lapack.MatrixNorm{lapack.MaxAbs, lapack.MaxColumnSum, lapack.MaxRowSum, lapack.Frobenius} {
		name := fmt.Sprintf("norm=%v,uplo=%v,n=%v,kd=%v,ldab=%v", string(norm), string(uplo), n, kd, ldab)

		normGot := impl.Dlansb(norm, uplo, n, kd, ab, ldab, work)

		if !floats.Equal(ab, abCopy) {
			t.Fatalf("%v: unexpected modification of ab", name)
		}

		if norm == lapack.MaxAbs {
			// MaxAbs norm involves no computation, so we expect
			// exact equality here.
			if normGot != maxabsWant {
				t.Errorf("%v: unexpected result; got %v, want %v", name, normGot, maxabsWant)
			}
			continue
		}

		var normWant float64
		switch norm {
		case lapack.MaxColumnSum:
			normWant = maxcolsumWant
			maxcolsumGot = normGot
		case lapack.MaxRowSum:
			normWant = maxrowsumWant
			maxrowsumGot = normGot
		case lapack.Frobenius:
			normWant = frobWant
		}
		if math.Abs(normGot-normWant) >= tol {
			t.Errorf("%v: unexpected result; got %v, want %v", name, normGot, normWant)
		}
	}
	// MaxColSum and MaxRowSum norms should be exactly equal because the
	// matrix is symmetric.
	if maxcolsumGot != maxrowsumGot {
		name := fmt.Sprintf("uplo=%v,n=%v,kd=%v,ldab=%v", string(uplo), n, kd, ldab)
		t.Errorf("%v: unexpected mismatch between MaxColSum and MaxRowSum norms of A; MaxColSum %v, MaxRowSum %v", name, maxcolsumGot, maxrowsumGot)
	}
}
