// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dlatrser interface {
	Dlatrs(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, normin bool, n int, a []float64, lda int, x []float64, cnorm []float64) (scale float64)
}

func DlatrsTest(t *testing.T, impl Dlatrser) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, trans := range []blas.Transpose{blas.Trans, blas.NoTrans} {
			for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 7, 10, 20, 50, 100} {
				for _, lda := range []int{n, 2*n + 1} {
					lda = max(1, lda)
					imats := []int{7, 11, 12, 13, 14, 15, 16, 17, 18}
					if n < 6 {
						imats = append(imats, 19)
					}
					for _, imat := range imats {
						testDlatrs(t, impl, imat, uplo, trans, n, lda, rnd)
					}
				}
			}
		}
	}
}

func testDlatrs(t *testing.T, impl Dlatrser, imat int, uplo blas.Uplo, trans blas.Transpose, n, lda int, rnd *rand.Rand) {
	const tol = 1e-14

	a := nanSlice(n * lda)
	b := nanSlice(n)
	work := make([]float64, 3*n)

	// Generate triangular test matrix and right hand side.
	diag := dlattr(imat, uplo, trans, n, a, lda, b, work, rnd)
	if imat <= 10 {
		// b has not been generated.
		dlarnv(b, 3, rnd)
	}

	cnorm := nanSlice(n)
	x := make([]float64, n)

	// Call Dlatrs with normin=false.
	copy(x, b)
	scale := impl.Dlatrs(uplo, trans, diag, false, n, a, lda, x, cnorm)
	prefix := fmt.Sprintf("Case imat=%v (n=%v,lda=%v,trans=%v,uplo=%v,diag=%v", imat, n, lda, trans, uplo, diag)
	for i, v := range cnorm {
		if math.IsNaN(v) {
			t.Errorf("%v: cnorm[%v] not computed (scale=%v,normin=false)", prefix, i, scale)
		}
	}
	resid, hasNaN := dlatrsResidual(uplo, trans, diag, n, a, lda, scale, cnorm, x, b, work[:n])
	if hasNaN {
		t.Errorf("%v: unexpected NaN (scale=%v,normin=false)", prefix, scale)
	} else if resid > tol {
		t.Errorf("%v: residual %v too large (scale=%v,normin=false)", prefix, resid, scale)
	}

	// Call Dlatrs with normin=true because cnorm has been filled.
	copy(x, b)
	scale = impl.Dlatrs(uplo, trans, diag, true, n, a, lda, x, cnorm)
	resid, hasNaN = dlatrsResidual(uplo, trans, diag, n, a, lda, scale, cnorm, x, b, work[:n])
	if hasNaN {
		t.Errorf("%v: unexpected NaN (scale=%v,normin=true)", prefix, scale)
	} else if resid > tol {
		t.Errorf("%v: residual %v too large (scale=%v,normin=true)", prefix, resid, scale)
	}
}

// dlatrsResidual returns norm(trans(A)*x-scale*b) / (norm(trans(A))*norm(x)*eps)
// and whether NaN has been encountered in the process.
func dlatrsResidual(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n int, a []float64, lda int, scale float64, cnorm []float64, x, b, work []float64) (resid float64, hasNaN bool) {
	if n == 0 {
		return 0, false
	}

	// Compute the norm of the triangular matrix A using the column norms
	// already computed by Dlatrs.
	var tnorm float64
	if diag == blas.NonUnit {
		for j := 0; j < n; j++ {
			tnorm = math.Max(tnorm, math.Abs(a[j*lda+j])+cnorm[j])
		}
	} else {
		for j := 0; j < n; j++ {
			tnorm = math.Max(tnorm, 1+cnorm[j])
		}
	}

	eps := dlamchE
	smlnum := dlamchS
	bi := blas64.Implementation()

	// Compute norm(trans(A)*x-scale*b) / (norm(trans(A))*norm(x)*eps)
	copy(work, x)
	ix := bi.Idamax(n, work, 1)
	xnorm := math.Max(1, math.Abs(work[ix]))
	xscal := 1 / xnorm / float64(n)
	bi.Dscal(n, xscal, work, 1)
	bi.Dtrmv(uplo, trans, diag, n, a, lda, work, 1)
	bi.Daxpy(n, -scale*xscal, b, 1, work, 1)
	for _, v := range work {
		if math.IsNaN(v) {
			return 1 / eps, true
		}
	}
	ix = bi.Idamax(n, work, 1)
	resid = math.Abs(work[ix])
	ix = bi.Idamax(n, x, 1)
	xnorm = math.Abs(x[ix])
	if resid*smlnum <= xnorm {
		if xnorm > 0 {
			resid /= xnorm
		}
	} else if resid > 0 {
		resid = 1 / eps
	}
	if resid*smlnum <= tnorm {
		if tnorm > 0 {
			resid /= tnorm
		}
	} else if resid > 0 {
		resid = 1 / eps
	}
	return resid, false
}
