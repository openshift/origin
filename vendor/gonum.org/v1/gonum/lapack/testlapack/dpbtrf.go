// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dpbtrfer interface {
	Dpbtrf(uplo blas.Uplo, n, kd int, ab []float64, ldab int) (ok bool)
}

// DpbtrfTest tests a band Cholesky factorization on random symmetric positive definite
// band matrices by checking that the Cholesky factors multiply back to the original matrix.
func DpbtrfTest(t *testing.T, impl Dpbtrfer) {
	// TODO(vladimir-ch): include expected-failure test case.

	// With the current implementation of Ilaenv the blocked code path is taken if kd > 64.
	// Unfortunately, with the block size nb=32 this also means that in Dpbtrf
	// it never happens that i2 <= 0 and the state coverage (unlike code coverage) is not complete.
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 64, 65, 66, 91, 96, 97, 101, 128, 130} {
		for _, kd := range []int{0, (n + 1) / 4, (3*n - 1) / 4, (5*n + 1) / 4} {
			for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
				for _, ldab := range []int{kd + 1, kd + 1 + 7} {
					dpbtrfTest(t, impl, uplo, n, kd, ldab, rnd)
				}
			}
		}
	}
}

func dpbtrfTest(t *testing.T, impl Dpbtrfer, uplo blas.Uplo, n, kd int, ldab int, rnd *rand.Rand) {
	const tol = 1e-12

	name := fmt.Sprintf("uplo=%v,n=%v,kd=%v,ldab=%v", string(uplo), n, kd, ldab)

	// Generate a random symmetric positive definite band matrix.
	ab := randSymBand(uplo, n, kd, ldab, rnd)

	// Compute the Cholesky decomposition of A.
	abFac := make([]float64, len(ab))
	copy(abFac, ab)
	ok := impl.Dpbtrf(uplo, n, kd, abFac, ldab)
	if !ok {
		t.Fatalf("%v: bad test matrix, Dpbtrf failed", name)
	}

	// Reconstruct an symmetric band matrix from the Uᵀ*U or L*Lᵀ factorization, overwriting abFac.
	dsbmm(uplo, n, kd, abFac, ldab)

	// Compute and check the max-norm distance between the reconstructed and original matrix A.
	dist := distSymBand(uplo, n, kd, abFac, ldab, ab, ldab)
	if dist > tol*float64(n) {
		t.Errorf("%v: unexpected result, diff=%v", name, dist)
	}
}

// dsbmm computes a symmetric band matrix A
//  A = Uᵀ*U  if uplo == blas.Upper,
//  A = L*Lᵀ  if uplo == blas.Lower,
// where U and L is an upper, respectively lower, triangular band matrix
// stored on entry in ab. The result is stored in-place into ab.
func dsbmm(uplo blas.Uplo, n, kd int, ab []float64, ldab int) {
	bi := blas64.Implementation()
	switch uplo {
	case blas.Upper:
		// Compute the product Uᵀ * U.
		for k := n - 1; k >= 0; k-- {
			klen := min(kd, n-k-1) // Number of stored off-diagonal elements in the row
			// Add a multiple of row k of the factor U to each of rows k+1 through n.
			if klen > 0 {
				bi.Dsyr(blas.Upper, klen, 1, ab[k*ldab+1:], 1, ab[(k+1)*ldab:], ldab-1)
			}
			// Scale row k by the diagonal element.
			bi.Dscal(klen+1, ab[k*ldab], ab[k*ldab:], 1)
		}
	case blas.Lower:
		// Compute the product L * Lᵀ.
		for k := n - 1; k >= 0; k-- {
			kc := max(0, kd-k) // Index of the first valid element in the row
			klen := kd - kc    // Number of stored off-diagonal elements in the row
			// Compute the diagonal [k,k] element.
			ab[k*ldab+kd] = bi.Ddot(klen+1, ab[k*ldab+kc:], 1, ab[k*ldab+kc:], 1)
			// Compute the rest of column k.
			if klen > 0 {
				bi.Dtrmv(blas.Lower, blas.NoTrans, blas.NonUnit, klen,
					ab[(k-klen)*ldab+kd:], ldab-1, ab[k*ldab+kc:], 1)
			}
		}
	}
}
