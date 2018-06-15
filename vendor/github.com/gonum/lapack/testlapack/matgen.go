// Copyright ©2017 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"math/rand"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
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
//  A = U * D * U^T.
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
// linear system A*x=b, or A^T*x=b. Valid values of imat are 7, and all between 11
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
