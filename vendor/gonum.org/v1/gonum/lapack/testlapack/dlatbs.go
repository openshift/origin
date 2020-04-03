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
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

type Dlatbser interface {
	Dlatbs(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, normin bool, n, kd int, ab []float64, ldab int, x []float64, cnorm []float64) float64
}

// DlatbsTest tests Dlatbs by generating a random triangular band system and
// checking that a residual for the computed solution is small.
func DlatbsTest(t *testing.T, impl Dlatbser) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, kd := range []int{0, (n + 1) / 4, (3*n - 1) / 4, (5*n + 1) / 4} {
			for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
				for _, trans := range []blas.Transpose{blas.NoTrans, blas.Trans, blas.ConjTrans} {
					for _, ldab := range []int{kd + 1, kd + 1 + 7} {
						for _, kind := range []int{6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 17, 18} {
							dlatbsTest(t, impl, rnd, kind, uplo, trans, n, kd, ldab)
						}
					}
				}
			}
		}
	}
}

func dlatbsTest(t *testing.T, impl Dlatbser, rnd *rand.Rand, kind int, uplo blas.Uplo, trans blas.Transpose, n, kd, ldab int) {
	const eps = 1e-15

	// Allocate a triangular band matrix.
	var ab []float64
	if n > 0 {
		ab = make([]float64, (n-1)*ldab+kd+1)
	}
	for i := range ab {
		ab[i] = rnd.NormFloat64()
	}

	// Generate a triangular test matrix and the right-hand side.
	diag, b := dlattb(kind, uplo, trans, n, kd, ab, ldab, rnd)

	// Make a copy of AB to make sure that it is not modified in Dlatbs.
	abCopy := make([]float64, len(ab))
	copy(abCopy, ab)

	// Allocate cnorm and fill it with impossible result to make sure that it
	// _is_ updated in the first Dlatbs call below.
	cnorm := make([]float64, n)
	for i := range cnorm {
		cnorm[i] = -1
	}

	// Solve the system op(A)*x = b.
	x := make([]float64, n)
	copy(x, b)
	scale := impl.Dlatbs(uplo, trans, diag, false, n, kd, ab, ldab, x, cnorm)

	name := fmt.Sprintf("kind=%v,uplo=%v,trans=%v,diag=%v,n=%v,kd=%v,ldab=%v",
		kind, string(uplo), string(trans), string(diag), n, kd, ldab)

	if !floats.Equal(ab, abCopy) {
		t.Errorf("%v: unexpected modification of ab", name)
	}
	if floats.Count(func(v float64) bool { return v == -1 }, cnorm) > 0 {
		t.Errorf("%v: expected modification of cnorm", name)
	}

	resid := dlatbsResidual(uplo, trans, diag, n, kd, ab, ldab, scale, cnorm, b, x)
	if resid >= eps {
		t.Errorf("%v: unexpected result when normin=false. residual=%v", name, resid)
	}

	// Make a copy of cnorm to check that it is _not_ modified.
	cnormCopy := make([]float64, len(cnorm))
	copy(cnormCopy, cnorm)
	// Restore x.
	copy(x, b)
	// Solve the system op(A)*x = b again with normin = true.
	scale = impl.Dlatbs(uplo, trans, diag, true, n, kd, ab, ldab, x, cnorm)

	// Cannot test for exact equality because Dlatbs may scale cnorm by s and
	// then by 1/s before return.
	if !floats.EqualApprox(cnorm, cnormCopy, 1e-15) {
		t.Errorf("%v: unexpected modification of cnorm", name)
	}

	resid = dlatbsResidual(uplo, trans, diag, n, kd, ab, ldab, scale, cnorm, b, x)
	if resid >= eps {
		t.Errorf("%v: unexpected result when normin=true. residual=%v", name, resid)
	}
}

// dlatbsResidual returns the residual for the solution to a scaled triangular
// system of equations  A*x = s*b  or  Aᵀ*x = s*b  when A is an n×n triangular
// band matrix with kd super- or sub-diagonals. The residual is computed as
//  norm( op(A)*x - scale*b ) / ( norm(op(A)) * norm(x) ).
//
// This function corresponds to DTBT03 in Reference LAPACK.
func dlatbsResidual(uplo blas.Uplo, trans blas.Transpose, diag blas.Diag, n, kd int, ab []float64, ldab int, scale float64, cnorm, b, x []float64) float64 {
	if n == 0 {
		return 0
	}

	// Compute the norm of the triangular matrix A using the columns norms
	// already computed by Dlatbs.
	var tnorm float64
	if diag == blas.NonUnit {
		if uplo == blas.Upper {
			for j := 0; j < n; j++ {
				tnorm = math.Max(tnorm, math.Abs(ab[j*ldab])+cnorm[j])
			}
		} else {
			for j := 0; j < n; j++ {
				tnorm = math.Max(tnorm, math.Abs(ab[j*ldab+kd])+cnorm[j])
			}
		}
	} else {
		for j := 0; j < n; j++ {
			tnorm = math.Max(tnorm, 1+cnorm[j])
		}
	}

	bi := blas64.Implementation()
	eps := dlamchE
	smlnum := dlamchS

	ix := bi.Idamax(n, x, 1)
	xNorm := math.Max(1, math.Abs(x[ix]))
	xScal := (1 / xNorm) / float64(kd+1)

	resid := make([]float64, len(x))
	copy(resid, x)
	bi.Dscal(n, xScal, resid, 1)
	bi.Dtbmv(uplo, trans, diag, n, kd, ab, ldab, resid, 1)
	bi.Daxpy(n, -scale*xScal, b, 1, resid, 1)

	ix = bi.Idamax(n, resid, 1)
	residNorm := math.Abs(resid[ix])
	if residNorm*smlnum <= xNorm {
		if xNorm > 0 {
			residNorm /= xNorm
		}
	} else if residNorm > 0 {
		residNorm = 1 / eps
	}
	if residNorm*smlnum <= tnorm {
		if tnorm > 0 {
			residNorm /= tnorm
		}
	} else if residNorm > 0 {
		residNorm = 1 / eps
	}

	return residNorm
}
