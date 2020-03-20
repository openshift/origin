// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

// Dlatm1 computes the entries of dst as specified by mode, cond and rsign.
//
// mode describes how dst will be computed:
//  |mode| == 1: dst[0] = 1 and dst[1:n] = 1/cond
//  |mode| == 2: dst[:n-1] = 1/cond and dst[n-1] = 1
//  |mode| == 3: dst[i] = cond^{-i/(n-1)}, i=0,...,n-1
//  |mode| == 4: dst[i] = 1 - i*(1-1/cond)/(n-1)
//  |mode| == 5: dst[i] = random number in the range (1/cond, 1) such that
//                    their logarithms are uniformly distributed
//  |mode| == 6: dst[i] = random number from the distribution given by dist
// If mode is negative, the order of the elements of dst will be reversed.
// For other values of mode Dlatm1 will panic.
//
// If rsign is true and mode is not ±6, each entry of dst will be multiplied by 1
// or -1 with probability 0.5
//
// dist specifies the type of distribution to be used when mode == ±6:
//  dist == 1: Uniform[0,1)
//  dist == 2: Uniform[-1,1)
//  dist == 3: Normal(0,1)
// For other values of dist Dlatm1 will panic.
//
// rnd is used as a source of random numbers.
func Dlatm1(dst []float64, mode int, cond float64, rsign bool, dist int, rnd *rand.Rand) {
	amode := mode
	if amode < 0 {
		amode = -amode
	}
	if amode < 1 || 6 < amode {
		panic("testlapack: invalid mode")
	}
	if cond < 1 {
		panic("testlapack: cond < 1")
	}
	if amode == 6 && (dist < 1 || 3 < dist) {
		panic("testlapack: invalid dist")
	}

	n := len(dst)
	if n == 0 {
		return
	}

	switch amode {
	case 1:
		dst[0] = 1
		for i := 1; i < n; i++ {
			dst[i] = 1 / cond
		}
	case 2:
		for i := 0; i < n-1; i++ {
			dst[i] = 1
		}
		dst[n-1] = 1 / cond
	case 3:
		dst[0] = 1
		if n > 1 {
			alpha := math.Pow(cond, -1/float64(n-1))
			for i := 1; i < n; i++ {
				dst[i] = math.Pow(alpha, float64(i))
			}
		}
	case 4:
		dst[0] = 1
		if n > 1 {
			condInv := 1 / cond
			alpha := (1 - condInv) / float64(n-1)
			for i := 1; i < n; i++ {
				dst[i] = float64(n-i-1)*alpha + condInv
			}
		}
	case 5:
		alpha := math.Log(1 / cond)
		for i := range dst {
			dst[i] = math.Exp(alpha * rnd.Float64())
		}
	case 6:
		switch dist {
		case 1:
			for i := range dst {
				dst[i] = rnd.Float64()
			}
		case 2:
			for i := range dst {
				dst[i] = 2*rnd.Float64() - 1
			}
		case 3:
			for i := range dst {
				dst[i] = rnd.NormFloat64()
			}
		}
	}

	if rsign && amode != 6 {
		for i, v := range dst {
			if rnd.Float64() < 0.5 {
				dst[i] = -v
			}
		}
	}

	if mode < 0 {
		for i := 0; i < n/2; i++ {
			dst[i], dst[n-i-1] = dst[n-i-1], dst[i]
		}
	}
}

// Dlagsy generates an n×n symmetric matrix A, by pre- and post- multiplying a
// real diagonal matrix D with a random orthogonal matrix:
//  A = U * D * Uᵀ.
//
// work must have length at least 2*n, otherwise Dlagsy will panic.
//
// The parameter k is unused but it must satisfy
//  0 <= k <= n-1.
func Dlagsy(n, k int, d []float64, a []float64, lda int, rnd *rand.Rand, work []float64) {
	checkMatrix(n, n, a, lda)
	if k < 0 || max(0, n-1) < k {
		panic("testlapack: invalid value of k")
	}
	if len(d) != n {
		panic("testlapack: bad length of d")
	}
	if len(work) < 2*n {
		panic("testlapack: insufficient work length")
	}

	// Initialize lower triangle of A to diagonal matrix.
	for i := 1; i < n; i++ {
		for j := 0; j < i; j++ {
			a[i*lda+j] = 0
		}
	}
	for i := 0; i < n; i++ {
		a[i*lda+i] = d[i]
	}

	bi := blas64.Implementation()

	// Generate lower triangle of symmetric matrix.
	for i := n - 2; i >= 0; i-- {
		for j := 0; j < n-i; j++ {
			work[j] = rnd.NormFloat64()
		}
		wn := bi.Dnrm2(n-i, work[:n-i], 1)
		wa := math.Copysign(wn, work[0])
		var tau float64
		if wn != 0 {
			wb := work[0] + wa
			bi.Dscal(n-i-1, 1/wb, work[1:n-i], 1)
			work[0] = 1
			tau = wb / wa
		}

		// Apply random reflection to A[i:n,i:n] from the left and the
		// right.
		//
		// Compute y := tau * A * u.
		bi.Dsymv(blas.Lower, n-i, tau, a[i*lda+i:], lda, work[:n-i], 1, 0, work[n:2*n-i], 1)

		// Compute v := y - 1/2 * tau * ( y, u ) * u.
		alpha := -0.5 * tau * bi.Ddot(n-i, work[n:2*n-i], 1, work[:n-i], 1)
		bi.Daxpy(n-i, alpha, work[:n-i], 1, work[n:2*n-i], 1)

		// Apply the transformation as a rank-2 update to A[i:n,i:n].
		bi.Dsyr2(blas.Lower, n-i, -1, work[:n-i], 1, work[n:2*n-i], 1, a[i*lda+i:], lda)
	}

	// Store full symmetric matrix.
	for i := 1; i < n; i++ {
		for j := 0; j < i; j++ {
			a[j*lda+i] = a[i*lda+j]
		}
	}
}

// Dlagge generates a real general m×n matrix A, by pre- and post-multiplying
// a real diagonal matrix D with random orthogonal matrices:
//  A = U*D*V.
//
// d must have length min(m,n), and work must have length m+n, otherwise Dlagge
// will panic.
//
// The parameters ku and kl are unused but they must satisfy
//  0 <= kl <= m-1,
//  0 <= ku <= n-1.
func Dlagge(m, n, kl, ku int, d []float64, a []float64, lda int, rnd *rand.Rand, work []float64) {
	checkMatrix(m, n, a, lda)
	if kl < 0 || max(0, m-1) < kl {
		panic("testlapack: invalid value of kl")
	}
	if ku < 0 || max(0, n-1) < ku {
		panic("testlapack: invalid value of ku")
	}
	if len(d) != min(m, n) {
		panic("testlapack: bad length of d")
	}
	if len(work) < m+n {
		panic("testlapack: insufficient work length")
	}

	// Initialize A to diagonal matrix.
	for i := 0; i < m; i++ {
		for j := 0; j < n; j++ {
			a[i*lda+j] = 0
		}
	}
	for i := 0; i < min(m, n); i++ {
		a[i*lda+i] = d[i]
	}

	// Quick exit if the user wants a diagonal matrix.
	// if kl == 0 && ku == 0 {
	// 	return
	// }

	bi := blas64.Implementation()

	// Pre- and post-multiply A by random orthogonal matrices.
	for i := min(m, n) - 1; i >= 0; i-- {
		if i < m-1 {
			for j := 0; j < m-i; j++ {
				work[j] = rnd.NormFloat64()
			}
			wn := bi.Dnrm2(m-i, work[:m-i], 1)
			wa := math.Copysign(wn, work[0])
			var tau float64
			if wn != 0 {
				wb := work[0] + wa
				bi.Dscal(m-i-1, 1/wb, work[1:m-i], 1)
				work[0] = 1
				tau = wb / wa
			}

			// Multiply A[i:m,i:n] by random reflection from the left.
			bi.Dgemv(blas.Trans, m-i, n-i,
				1, a[i*lda+i:], lda, work[:m-i], 1,
				0, work[m:m+n-i], 1)
			bi.Dger(m-i, n-i,
				-tau, work[:m-i], 1, work[m:m+n-i], 1,
				a[i*lda+i:], lda)
		}
		if i < n-1 {
			for j := 0; j < n-i; j++ {
				work[j] = rnd.NormFloat64()
			}
			wn := bi.Dnrm2(n-i, work[:n-i], 1)
			wa := math.Copysign(wn, work[0])
			var tau float64
			if wn != 0 {
				wb := work[0] + wa
				bi.Dscal(n-i-1, 1/wb, work[1:n-i], 1)
				work[0] = 1
				tau = wb / wa
			}

			// Multiply A[i:m,i:n] by random reflection from the right.
			bi.Dgemv(blas.NoTrans, m-i, n-i,
				1, a[i*lda+i:], lda, work[:n-i], 1,
				0, work[n:n+m-i], 1)
			bi.Dger(m-i, n-i,
				-tau, work[n:n+m-i], 1, work[:n-i], 1,
				a[i*lda+i:], lda)
		}
	}

	// TODO(vladimir-ch): Reduce number of subdiagonals to kl and number of
	// superdiagonals to ku.
}

// dlarnv fills dst with random numbers from a uniform or normal distribution
// specified by dist:
//  dist=1: uniform(0,1),
//  dist=2: uniform(-1,1),
//  dist=3: normal(0,1).
// For other values of dist dlarnv will panic.
func dlarnv(dst []float64, dist int, rnd *rand.Rand) {
	switch dist {
	default:
		panic("testlapack: invalid dist")
	case 1:
		for i := range dst {
			dst[i] = rnd.Float64()
		}
	case 2:
		for i := range dst {
			dst[i] = 2*rnd.Float64() - 1
		}
	case 3:
		for i := range dst {
			dst[i] = rnd.NormFloat64()
		}
	}
}

// dlattr generates an n×n triangular test matrix A with its properties uniquely
// determined by imat and uplo, and returns whether A has unit diagonal. If diag
// is blas.Unit, the diagonal elements are set so that A[k,k]=k.
//
// trans specifies whether the matrix A or its transpose will be used.
//
// If imat is greater than 10, dlattr also generates the right hand side of the
// linear system A*x=b, or Aᵀ*x=b. Valid values of imat are 7, and all between 11
// and 19, inclusive.
//
// b mush have length n, and work must have length 3*n, and dlattr will panic
// otherwise.
func dlattr(imat int, uplo blas.Uplo, trans blas.Transpose, n int, a []float64, lda int, b, work []float64, rnd *rand.Rand) (diag blas.Diag) {
	checkMatrix(n, n, a, lda)
	if len(b) != n {
		panic("testlapack: bad length of b")
	}
	if len(work) < 3*n {
		panic("testlapack: insufficient length of work")
	}
	if uplo != blas.Upper && uplo != blas.Lower {
		panic("testlapack: bad uplo")
	}
	if trans != blas.Trans && trans != blas.NoTrans {
		panic("testlapack: bad trans")
	}

	if n == 0 {
		return blas.NonUnit
	}

	ulp := dlamchE * dlamchB
	smlnum := dlamchS
	bignum := (1 - ulp) / smlnum

	bi := blas64.Implementation()

	switch imat {
	default:
		// TODO(vladimir-ch): Implement the remaining cases.
		panic("testlapack: invalid or unimplemented imat")
	case 7:
		// Identity matrix. The diagonal is set to NaN.
		diag = blas.Unit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				a[i*lda+i] = math.NaN()
				for j := i + 1; j < n; j++ {
					a[i*lda+j] = 0
				}
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				for j := 0; j < i; j++ {
					a[i*lda+j] = 0
				}
				a[i*lda+i] = math.NaN()
			}
		}
	case 11:
		// Generate a triangular matrix with elements between -1 and 1,
		// give the diagonal norm 2 to make it well-conditioned, and
		// make the right hand side large so that it requires scaling.
		diag = blas.NonUnit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n-1; i++ {
				dlarnv(a[i*lda+i:i*lda+n], 2, rnd)
			}
		case blas.Lower:
			for i := 1; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i+1], 2, rnd)
			}
		}
		for i := 0; i < n; i++ {
			a[i*lda+i] = math.Copysign(2, a[i*lda+i])
		}
		// Set the right hand side so that the largest value is bignum.
		dlarnv(b, 2, rnd)
		imax := bi.Idamax(n, b, 1)
		bscal := bignum / math.Max(1, b[imax])
		bi.Dscal(n, bscal, b, 1)
	case 12:
		// Make the first diagonal element in the solve small to cause
		// immediate overflow when dividing by T[j,j]. The off-diagonal
		// elements are small (cnorm[j] < 1).
		diag = blas.NonUnit
		tscal := 1 / math.Max(1, float64(n-1))
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda+i:i*lda+n], 2, rnd)
				bi.Dscal(n-i-1, tscal, a[i*lda+i+1:], 1)
				a[i*lda+i] = math.Copysign(1, a[i*lda+i])
			}
			a[(n-1)*lda+n-1] *= smlnum
		case blas.Lower:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i+1], 2, rnd)
				bi.Dscal(i, tscal, a[i*lda:], 1)
				a[i*lda+i] = math.Copysign(1, a[i*lda+i])
			}
			a[0] *= smlnum
		}
		dlarnv(b, 2, rnd)
	case 13:
		// Make the first diagonal element in the solve small to cause
		// immediate overflow when dividing by T[j,j]. The off-diagonal
		// elements are O(1) (cnorm[j] > 1).
		diag = blas.NonUnit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda+i:i*lda+n], 2, rnd)
				a[i*lda+i] = math.Copysign(1, a[i*lda+i])
			}
			a[(n-1)*lda+n-1] *= smlnum
		case blas.Lower:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i+1], 2, rnd)
				a[i*lda+i] = math.Copysign(1, a[i*lda+i])
			}
			a[0] *= smlnum
		}
		dlarnv(b, 2, rnd)
	case 14:
		// T is diagonal with small numbers on the diagonal to
		// make the growth factor underflow, but a small right hand side
		// chosen so that the solution does not overflow.
		diag = blas.NonUnit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				for j := i + 1; j < n; j++ {
					a[i*lda+j] = 0
				}
				if (n-1-i)&0x2 == 0 {
					a[i*lda+i] = smlnum
				} else {
					a[i*lda+i] = 1
				}
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				for j := 0; j < i; j++ {
					a[i*lda+j] = 0
				}
				if i&0x2 == 0 {
					a[i*lda+i] = smlnum
				} else {
					a[i*lda+i] = 1
				}
			}
		}
		// Set the right hand side alternately zero and small.
		switch uplo {
		case blas.Upper:
			b[0] = 0
			for i := n - 1; i > 0; i -= 2 {
				b[i] = 0
				b[i-1] = smlnum
			}
		case blas.Lower:
			for i := 0; i < n-1; i += 2 {
				b[i] = 0
				b[i+1] = smlnum
			}
			b[n-1] = 0
		}
	case 15:
		// Make the diagonal elements small to cause gradual overflow
		// when dividing by T[j,j]. To control the amount of scaling
		// needed, the matrix is bidiagonal.
		diag = blas.NonUnit
		texp := 1 / math.Max(1, float64(n-1))
		tscal := math.Pow(smlnum, texp)
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				a[i*lda+i] = tscal
				if i < n-1 {
					a[i*lda+i+1] = -1
				}
				for j := i + 2; j < n; j++ {
					a[i*lda+j] = 0
				}
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				for j := 0; j < i-1; j++ {
					a[i*lda+j] = 0
				}
				if i > 0 {
					a[i*lda+i-1] = -1
				}
				a[i*lda+i] = tscal
			}
		}
		dlarnv(b, 2, rnd)
	case 16:
		// One zero diagonal element.
		diag = blas.NonUnit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda+i:i*lda+n], 2, rnd)
				a[i*lda+i] = math.Copysign(2, a[i*lda+i])
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i+1], 2, rnd)
				a[i*lda+i] = math.Copysign(2, a[i*lda+i])
			}
		}
		iy := n / 2
		a[iy*lda+iy] = 0
		dlarnv(b, 2, rnd)
		bi.Dscal(n, 2, b, 1)
	case 17:
		// Make the offdiagonal elements large to cause overflow when
		// adding a column of T. In the non-transposed case, the matrix
		// is constructed to cause overflow when adding a column in
		// every other step.
		diag = blas.NonUnit
		tscal := (1 - ulp) / (dlamchS / ulp)
		texp := 1.0
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				for j := i; j < n; j++ {
					a[i*lda+j] = 0
				}
			}
			for j := n - 1; j >= 1; j -= 2 {
				a[j] = -tscal / float64(n+1)
				a[j*lda+j] = 1
				b[j] = texp * (1 - ulp)
				a[j-1] = -tscal / float64(n+1) / float64(n+2)
				a[(j-1)*lda+j-1] = 1
				b[j-1] = texp * float64(n*n+n-1)
				texp *= 2
			}
			b[0] = float64(n+1) / float64(n+2) * tscal
		case blas.Lower:
			for i := 0; i < n; i++ {
				for j := 0; j <= i; j++ {
					a[i*lda+j] = 0
				}
			}
			for j := 0; j < n-1; j += 2 {
				a[(n-1)*lda+j] = -tscal / float64(n+1)
				a[j*lda+j] = 1
				b[j] = texp * (1 - ulp)
				a[(n-1)*lda+j+1] = -tscal / float64(n+1) / float64(n+2)
				a[(j+1)*lda+j+1] = 1
				b[j+1] = texp * float64(n*n+n-1)
				texp *= 2
			}
			b[n-1] = float64(n+1) / float64(n+2) * tscal
		}
	case 18:
		// Generate a unit triangular matrix with elements between -1
		// and 1, and make the right hand side large so that it requires
		// scaling. The diagonal is set to NaN.
		diag = blas.Unit
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				a[i*lda+i] = math.NaN()
				dlarnv(a[i*lda+i+1:i*lda+n], 2, rnd)
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i], 2, rnd)
				a[i*lda+i] = math.NaN()
			}
		}
		// Set the right hand side so that the largest value is bignum.
		dlarnv(b, 2, rnd)
		iy := bi.Idamax(n, b, 1)
		bnorm := math.Abs(b[iy])
		bscal := bignum / math.Max(1, bnorm)
		bi.Dscal(n, bscal, b, 1)
	case 19:
		// Generate a triangular matrix with elements between
		// bignum/(n-1) and bignum so that at least one of the column
		// norms will exceed bignum.
		// Dlatrs cannot handle this case for (typically) n>5.
		diag = blas.NonUnit
		tleft := bignum / math.Max(1, float64(n-1))
		tscal := bignum * (float64(n-1) / math.Max(1, float64(n)))
		switch uplo {
		case blas.Upper:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda+i:i*lda+n], 2, rnd)
				for j := i; j < n; j++ {
					aij := a[i*lda+j]
					a[i*lda+j] = math.Copysign(tleft, aij) + tscal*aij
				}
			}
		case blas.Lower:
			for i := 0; i < n; i++ {
				dlarnv(a[i*lda:i*lda+i+1], 2, rnd)
				for j := 0; j <= i; j++ {
					aij := a[i*lda+j]
					a[i*lda+j] = math.Copysign(tleft, aij) + tscal*aij
				}
			}
		}
		dlarnv(b, 2, rnd)
		bi.Dscal(n, 2, b, 1)
	}

	// Flip the matrix if the transpose will be used.
	if trans == blas.Trans {
		switch uplo {
		case blas.Upper:
			for j := 0; j < n/2; j++ {
				bi.Dswap(n-2*j-1, a[j*lda+j:], 1, a[(j+1)*lda+n-j-1:], -lda)
			}
		case blas.Lower:
			for j := 0; j < n/2; j++ {
				bi.Dswap(n-2*j-1, a[j*lda+j:], lda, a[(n-j-1)*lda+j+1:], -1)
			}
		}
	}

	return diag
}

func checkMatrix(m, n int, a []float64, lda int) {
	if m < 0 {
		panic("testlapack: m < 0")
	}
	if n < 0 {
		panic("testlapack: n < 0")
	}
	if lda < max(1, n) {
		panic("testlapack: lda < max(1, n)")
	}
	if len(a) < (m-1)*lda+n {
		panic("testlapack: insufficient matrix slice length")
	}
}

// randomOrthogonal returns an n×n random orthogonal matrix.
func randomOrthogonal(n int, rnd *rand.Rand) blas64.General {
	q := eye(n, n)
	x := make([]float64, n)
	v := make([]float64, n)
	for j := 0; j < n-1; j++ {
		// x represents the j-th column of a random matrix.
		for i := 0; i < j; i++ {
			x[i] = 0
		}
		for i := j; i < n; i++ {
			x[i] = rnd.NormFloat64()
		}
		// Compute v that represents the elementary reflector that
		// annihilates the subdiagonal elements of x.
		reflector(v, x, j)
		// Compute Q * H_j and store the result into Q.
		applyReflector(q, q, v)
	}
	return q
}

// reflector generates a Householder reflector v that zeros out subdiagonal
// entries in the j-th column of a matrix.
func reflector(v, col []float64, j int) {
	n := len(col)
	if len(v) != n {
		panic("slice length mismatch")
	}
	if j < 0 || n <= j {
		panic("invalid column index")
	}

	for i := range v {
		v[i] = 0
	}
	if j == n-1 {
		return
	}
	s := floats.Norm(col[j:], 2)
	if s == 0 {
		return
	}
	v[j] = col[j] + math.Copysign(s, col[j])
	copy(v[j+1:], col[j+1:])
	s = floats.Norm(v[j:], 2)
	floats.Scale(1/s, v[j:])
}

// applyReflector computes Q*H where H is a Householder matrix represented by
// the Householder reflector v.
func applyReflector(qh blas64.General, q blas64.General, v []float64) {
	n := len(v)
	if qh.Rows != n || qh.Cols != n {
		panic("bad size of qh")
	}
	if q.Rows != n || q.Cols != n {
		panic("bad size of q")
	}
	qv := make([]float64, n)
	blas64.Gemv(blas.NoTrans, 1, q, blas64.Vector{Data: v, Inc: 1}, 0, blas64.Vector{Data: qv, Inc: 1})
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			qh.Data[i*qh.Stride+j] = q.Data[i*q.Stride+j]
		}
	}
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			qh.Data[i*qh.Stride+j] -= 2 * qv[i] * v[j]
		}
	}
	var norm2 float64
	for _, vi := range v {
		norm2 += vi * vi
	}
	norm2inv := 1 / norm2
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			qh.Data[i*qh.Stride+j] *= norm2inv
		}
	}
}

func dlattb(kind int, uplo blas.Uplo, trans blas.Transpose, n, kd int, ab []float64, ldab int, rnd *rand.Rand) (diag blas.Diag, b []float64) {
	switch {
	case kind < 1 || 18 < kind:
		panic("bad matrix kind")
	case (6 <= kind && kind <= 9) || kind == 17:
		diag = blas.Unit
	default:
		diag = blas.NonUnit
	}

	if n == 0 {
		return
	}

	const (
		unfl   = dlamchS
		ulp    = dlamchE * dlamchB
		smlnum = unfl
		bignum = (1 - ulp) / smlnum

		eps   = dlamchP
		small = 0.25 * (dlamchS / eps)
		large = 1 / small
		badc2 = 0.1 / eps
	)
	badc1 := math.Sqrt(badc2)

	var cndnum float64
	switch {
	case kind == 2 || kind == 8:
		cndnum = badc1
	case kind == 3 || kind == 9:
		cndnum = badc2
	default:
		cndnum = 2
	}

	uniformM11 := func() float64 {
		return 2*rnd.Float64() - 1
	}

	// Allocate the right-hand side and fill it with random numbers.
	// The pathological matrix types below overwrite it with their
	// custom vector.
	b = make([]float64, n)
	for i := range b {
		b[i] = uniformM11()
	}

	bi := blas64.Implementation()
	switch kind {
	default:
		panic("test matrix type not implemented")

	case 1, 2, 3, 4, 5:
		// Non-unit triangular matrix
		// TODO(vladimir-ch)
		var kl, ku int
		switch uplo {
		case blas.Upper:
			ku = kd
			kl = 0
			// IOFF = 1 + MAX( 0, KD-N+1 )
			// PACKIT = 'Q'
			// 'Q' => store the upper triangle in band storage scheme
			//        (only if matrix symmetric or upper triangular)
		case blas.Lower:
			ku = 0
			kl = kd
			// IOFF = 1
			// PACKIT = 'B'
			// 'B' => store the lower triangle in band storage scheme
			//        (only if matrix symmetric or lower triangular)
		}
		anorm := 1.0
		switch kind {
		case 4:
			anorm = small
		case 5:
			anorm = large
		}
		_, _, _ = kl, ku, anorm
		// // DIST = 'S' // UNIFORM(-1, 1)
		// // MODE = 3   // MODE = 3 sets D(I)=CNDNUM**(-(I-1)/(N-1))
		// // TYPE = 'N' // If TYPE='N', the generated matrix is nonsymmetric
		// CALL DLATMS( N, N, DIST, ISEED, TYPE, B, MODE, CNDNUM, ANORM,
		// $            KL, KU, PACKIT, AB( IOFF, 1 ), LDAB, WORK, INFO )
		panic("test matrix type not implemented")

	case 6:
		// Matrix is the identity.
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				// Fill the diagonal with non-unit numbers.
				ab[i*ldab] = float64(i + 2)
				for j := 1; j < min(n-i, kd+1); j++ {
					ab[i*ldab+j] = 0
				}
			}
		} else {
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd; j++ {
					ab[i*ldab+j] = 0
				}
				// Fill the diagonal with non-unit numbers.
				ab[i*ldab+kd] = float64(i + 2)
			}
		}

	case 7, 8, 9:
		// Non-trivial unit triangular matrix
		//
		// A unit triangular matrix T with condition cndnum is formed.
		// In this version, T only has bandwidth 2, the rest of it is
		// zero.

		tnorm := math.Sqrt(cndnum)

		// Initialize AB to zero.
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				// Fill the diagonal with non-unit numbers.
				ab[i*ldab] = float64(i + 2)
				for j := 1; j < min(n-i, kd+1); j++ {
					ab[i*ldab+j] = 0
				}
			}
		} else {
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd; j++ {
					ab[i*ldab+j] = 0
				}
				// Fill the diagonal with non-unit numbers.
				ab[i*ldab+kd] = float64(i + 2)
			}
		}

		switch kd {
		case 0:
			// Unit diagonal matrix, nothing else to do.
		case 1:
			// Special case: T is tridiagonal. Set every other
			// off-diagonal so that the matrix has norm tnorm+1.
			if n > 1 {
				if uplo == blas.Upper {
					ab[1] = math.Copysign(tnorm, uniformM11())
					for i := 2; i < n-1; i += 2 {
						ab[i*ldab+1] = tnorm * uniformM11()
					}
				} else {
					ab[ldab] = math.Copysign(tnorm, uniformM11())
					for i := 3; i < n; i += 2 {
						ab[i*ldab] = tnorm * uniformM11()
					}
				}
			}
		default:
			// Form a unit triangular matrix T with condition cndnum. T is given
			// by
			//      | 1   +   *                      |
			//      |     1   +                      |
			//  T = |         1   +   *              |
			//      |             1   +              |
			//      |                 1   +   *      |
			//      |                     1   +      |
			//      |                          . . . |
			// Each element marked with a '*' is formed by taking the product of
			// the adjacent elements marked with '+'. The '*'s can be chosen
			// freely, and the '+'s are chosen so that the inverse of T will
			// have elements of the same magnitude as T.
			work1 := make([]float64, n)
			work2 := make([]float64, n)
			star1 := math.Copysign(tnorm, uniformM11())
			sfac := math.Sqrt(tnorm)
			plus1 := math.Copysign(sfac, uniformM11())
			for i := 0; i < n; i += 2 {
				work1[i] = plus1
				work2[i] = star1
				if i+1 == n {
					continue
				}
				plus2 := star1 / plus1
				work1[i+1] = plus2
				plus1 = star1 / plus2
				// Generate a new *-value with norm between sqrt(tnorm)
				// and tnorm.
				rexp := uniformM11()
				if rexp < 0 {
					star1 = -math.Pow(sfac, 1-rexp)
				} else {
					star1 = math.Pow(sfac, 1+rexp)
				}
			}
			// Copy the diagonal to AB.
			if uplo == blas.Upper {
				bi.Dcopy(n-1, work1, 1, ab[1:], ldab)
				if n > 2 {
					bi.Dcopy(n-2, work2, 1, ab[2:], ldab)
				}
			} else {
				bi.Dcopy(n-1, work1, 1, ab[ldab+kd-1:], ldab)
				if n > 2 {
					bi.Dcopy(n-2, work2, 1, ab[2*ldab+kd-2:], ldab)
				}
			}
		}

	// Pathological test cases 10-18: these triangular matrices are badly
	// scaled or badly conditioned, so when used in solving a triangular
	// system they may cause overflow in the solution vector.

	case 10:
		// Generate a triangular matrix with elements between -1 and 1.
		// Give the diagonal norm 2 to make it well-conditioned.
		// Make the right hand side large so that it requires scaling.
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				for j := 0; j < min(n-j, kd+1); j++ {
					ab[i*ldab+j] = uniformM11()
				}
				ab[i*ldab] = math.Copysign(2, ab[i*ldab])
			}
		} else {
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd+1; j++ {
					ab[i*ldab+j] = uniformM11()
				}
				ab[i*ldab+kd] = math.Copysign(2, ab[i*ldab+kd])
			}
		}
		// Set the right hand side so that the largest value is bignum.
		bnorm := math.Abs(b[bi.Idamax(n, b, 1)])
		bscal := bignum / math.Max(1, bnorm)
		bi.Dscal(n, bscal, b, 1)

	case 11:
		// Make the first diagonal element in the solve small to cause
		// immediate overflow when dividing by T[j,j].
		// The offdiagonal elements are small (cnorm[j] < 1).
		tscal := 1 / float64(kd+1)
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				jlen := min(n-i, kd+1)
				arow := ab[i*ldab : i*ldab+jlen]
				dlarnv(arow, 2, rnd)
				if jlen > 1 {
					bi.Dscal(jlen-1, tscal, arow[1:], 1)
				}
				ab[i*ldab] = math.Copysign(1, ab[i*ldab])
			}
			ab[(n-1)*ldab] *= smlnum
		} else {
			for i := 0; i < n; i++ {
				jlen := min(i+1, kd+1)
				arow := ab[i*ldab+kd+1-jlen : i*ldab+kd+1]
				dlarnv(arow, 2, rnd)
				if jlen > 1 {
					bi.Dscal(jlen-1, tscal, arow[:jlen-1], 1)
				}
				ab[i*ldab+kd] = math.Copysign(1, ab[i*ldab+kd])
			}
			ab[kd] *= smlnum
		}

	case 12:
		// Make the first diagonal element in the solve small to cause
		// immediate overflow when dividing by T[j,j].
		// The offdiagonal elements are O(1) (cnorm[j] > 1).
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				jlen := min(n-i, kd+1)
				arow := ab[i*ldab : i*ldab+jlen]
				dlarnv(arow, 2, rnd)
				ab[i*ldab] = math.Copysign(1, ab[i*ldab])
			}
			ab[(n-1)*ldab] *= smlnum
		} else {
			for i := 0; i < n; i++ {
				jlen := min(i+1, kd+1)
				arow := ab[i*ldab+kd+1-jlen : i*ldab+kd+1]
				dlarnv(arow, 2, rnd)
				ab[i*ldab+kd] = math.Copysign(1, ab[i*ldab+kd])
			}
			ab[kd] *= smlnum
		}

	case 13:
		// T is diagonal with small numbers on the diagonal to make the growth
		// factor underflow, but a small right hand side chosen so that the
		// solution does not overflow.
		if uplo == blas.Upper {
			icount := 1
			for i := n - 1; i >= 0; i-- {
				if icount <= 2 {
					ab[i*ldab] = smlnum
				} else {
					ab[i*ldab] = 1
				}
				for j := 1; j < min(n-i, kd+1); j++ {
					ab[i*ldab+j] = 0
				}
				icount++
				if icount > 4 {
					icount = 1
				}
			}
		} else {
			icount := 1
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd; j++ {
					ab[i*ldab+j] = 0
				}
				if icount <= 2 {
					ab[i*ldab+kd] = smlnum
				} else {
					ab[i*ldab+kd] = 1
				}
				icount++
				if icount > 4 {
					icount = 1
				}
			}
		}
		// Set the right hand side alternately zero and small.
		if uplo == blas.Upper {
			b[0] = 0
			for i := n - 1; i > 1; i -= 2 {
				b[i] = 0
				b[i-1] = smlnum
			}
		} else {
			b[n-1] = 0
			for i := 0; i < n-1; i += 2 {
				b[i] = 0
				b[i+1] = smlnum
			}
		}

	case 14:
		// Make the diagonal elements small to cause gradual overflow when
		// dividing by T[j,j]. To control the amount of scaling needed, the
		// matrix is bidiagonal.
		tscal := math.Pow(smlnum, 1/float64(kd+1))
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				ab[i*ldab] = tscal
				if i < n-1 && kd > 0 {
					ab[i*ldab+1] = -1
				}
				for j := 2; j < min(n-i, kd+1); j++ {
					ab[i*ldab+j] = 0
				}
			}
			b[n-1] = 1
		} else {
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd-1; j++ {
					ab[i*ldab+j] = 0
				}
				if i > 0 && kd > 0 {
					ab[i*ldab+kd-1] = -1
				}
				ab[i*ldab+kd] = tscal
			}
			b[0] = 1
		}

	case 15:
		// One zero diagonal element.
		iy := n / 2
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				jlen := min(n-i, kd+1)
				dlarnv(ab[i*ldab:i*ldab+jlen], 2, rnd)
				if i != iy {
					ab[i*ldab] = math.Copysign(2, ab[i*ldab])
				} else {
					ab[i*ldab] = 0
				}
			}
		} else {
			for i := 0; i < n; i++ {
				jlen := min(i+1, kd+1)
				dlarnv(ab[i*ldab+kd+1-jlen:i*ldab+kd+1], 2, rnd)
				if i != iy {
					ab[i*ldab+kd] = math.Copysign(2, ab[i*ldab+kd])
				} else {
					ab[i*ldab+kd] = 0
				}
			}
		}
		bi.Dscal(n, 2, b, 1)

		// case 16:
		// TODO(vladimir-ch)
		// Make the off-diagonal elements large to cause overflow when adding a
		// column of T. In the non-transposed case, the matrix is constructed to
		// cause overflow when adding a column in every other step.

		// Initialize the matrix to zero.
		// if uplo == blas.Upper {
		// 	for i := 0; i < n; i++ {
		// 		for j := 0; j < min(n-i, kd+1); j++ {
		// 			ab[i*ldab+j] = 0
		// 		}
		// 	}
		// } else {
		// 	for i := 0; i < n; i++ {
		// 		for j := max(0, kd-i); j < kd+1; j++ {
		// 			ab[i*ldab+j] = 0
		// 		}
		// 	}
		// }

		// const tscal = (1 - ulp) / (unfl / ulp)
		// texp := 1.0
		// if kd > 0 {
		// 	if uplo == blas.Upper {
		// 		for j := n - 1; j >= 0; j -= kd {
		// 		}
		// 	} else {
		// 		for j := 0; j < n; j += kd {
		// 		}
		// 	}
		// } else {
		// 	// Diagonal matrix.
		// 	for i := 0; i < n; i++ {
		// 		ab[i*ldab] = 1
		// 		b[i] = float64(i + 1)
		// 	}
		// }

	case 17:
		// Generate a unit triangular matrix with elements between -1 and 1, and
		// make the right hand side large so that it requires scaling.
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				ab[i*ldab] = float64(i + 2)
				jlen := min(n-i-1, kd)
				if jlen > 0 {
					dlarnv(ab[i*ldab+1:i*ldab+1+jlen], 2, rnd)
				}
			}
		} else {
			for i := 0; i < n; i++ {
				jlen := min(i, kd)
				if jlen > 0 {
					dlarnv(ab[i*ldab+kd-jlen:i*ldab+kd], 2, rnd)
				}
				ab[i*ldab+kd] = float64(i + 2)
			}
		}
		// Set the right hand side so that the largest value is bignum.
		bnorm := math.Abs(b[bi.Idamax(n, b, 1)])
		bscal := bignum / math.Max(1, bnorm)
		bi.Dscal(n, bscal, b, 1)

	case 18:
		// Generate a triangular matrix with elements between bignum/kd and
		// bignum so that at least one of the column norms will exceed bignum.
		tleft := bignum / math.Max(1, float64(kd))
		// The reference LAPACK has
		//  tscal := bignum * (float64(kd) / float64(kd+1))
		// but this causes overflow when computing cnorm in Dlatbs. Our choice
		// is more conservative but increases coverage in the same way as the
		// LAPACK version.
		tscal := bignum / math.Max(1, float64(kd))
		if uplo == blas.Upper {
			for i := 0; i < n; i++ {
				for j := 0; j < min(n-i, kd+1); j++ {
					r := uniformM11()
					ab[i*ldab+j] = math.Copysign(tleft, r) + tscal*r
				}
			}
		} else {
			for i := 0; i < n; i++ {
				for j := max(0, kd-i); j < kd+1; j++ {
					r := uniformM11()
					ab[i*ldab+j] = math.Copysign(tleft, r) + tscal*r
				}
			}
		}
		bi.Dscal(n, 2, b, 1)
	}

	// Flip the matrix if the transpose will be used.
	if trans != blas.NoTrans {
		if uplo == blas.Upper {
			for j := 0; j < n/2; j++ {
				jlen := min(n-2*j-1, kd+1)
				bi.Dswap(jlen, ab[j*ldab:], 1, ab[(n-j-jlen)*ldab+jlen-1:], min(-ldab+1, -1))
			}
		} else {
			for j := 0; j < n/2; j++ {
				jlen := min(n-2*j-1, kd+1)
				bi.Dswap(jlen, ab[j*ldab+kd:], max(ldab-1, 1), ab[(n-j-1)*ldab+kd+1-jlen:], -1)
			}
		}
	}

	return diag, b
}
