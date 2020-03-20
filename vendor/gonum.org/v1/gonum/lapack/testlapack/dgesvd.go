// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dgesvder interface {
	Dgesvd(jobU, jobVT lapack.SVDJob, m, n int, a []float64, lda int, s, u []float64, ldu int, vt []float64, ldvt int, work []float64, lwork int) (ok bool)
}

func DgesvdTest(t *testing.T, impl Dgesvder, tol float64) {
	for _, m := range []int{0, 1, 2, 3, 4, 5, 10, 150, 300} {
		for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 150} {
			for _, mtype := range []int{1, 2, 3, 4, 5} {
				dgesvdTest(t, impl, m, n, mtype, tol)
			}
		}
	}
}

// dgesvdTest tests a Dgesvd implementation on an m×n matrix A generated
// according to mtype as:
//  - the zero matrix if mtype == 1,
//  - the identity matrix if mtype == 2,
//  - a random matrix with a given condition number and singular values if mtype == 3, 4, or 5.
// It first computes the full SVD  A = U*Sigma*Vᵀ  and checks that
//  - U has orthonormal columns, and Vᵀ has orthonormal rows,
//  - U*Sigma*Vᵀ multiply back to A,
//  - the singular values are non-negative and sorted in decreasing order.
// Then all combinations of partial SVD results are computed and checked whether
// they match the full SVD result.
func dgesvdTest(t *testing.T, impl Dgesvder, m, n, mtype int, tol float64) {
	rnd := rand.New(rand.NewSource(1))

	// Use a fixed leading dimension to reduce testing time.
	lda := n + 3
	ldu := m + 5
	ldvt := n + 7

	minmn := min(m, n)

	// Allocate A and fill it with random values. The in-range elements will
	// be overwritten below according to mtype.
	a := make([]float64, m*lda)
	for i := range a {
		a[i] = rnd.NormFloat64()
	}

	var aNorm float64
	switch mtype {
	default:
		panic("unknown test matrix type")
	case 1:
		// Zero matrix.
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				a[i*lda+j] = 0
			}
		}
		aNorm = 0
	case 2:
		// Identity matrix.
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				if i == j {
					a[i*lda+i] = 1
				} else {
					a[i*lda+j] = 0
				}
			}
		}
		aNorm = 1
	case 3, 4, 5:
		// Scaled random matrix.
		// Generate singular values.
		s := make([]float64, minmn)
		Dlatm1(s,
			4,                      // s[i] = 1 - i*(1-1/cond)/(minmn-1)
			float64(max(1, minmn)), // where cond = max(1,minmn)
			false,                  // signs of s[i] are not randomly flipped
			1, rnd)                 // random numbers are drawn uniformly from [0,1)
		// Decide scale factor for the singular values based on the matrix type.
		ulp := dlamchP
		unfl := dlamchS
		ovfl := 1 / unfl
		aNorm = 1
		if mtype == 4 {
			aNorm = unfl / ulp
		}
		if mtype == 5 {
			aNorm = ovfl * ulp
		}
		// Scale singular values so that the maximum singular value is
		// equal to aNorm (we know that the singular values are
		// generated above to be spread linearly between 1/cond and 1).
		floats.Scale(aNorm, s)
		// Generate A by multiplying S by random orthogonal matrices
		// from left and right.
		Dlagge(m, n, max(0, m-1), max(0, n-1), s, a, lda, rnd, make([]float64, m+n))
	}
	aCopy := make([]float64, len(a))
	copy(aCopy, a)

	for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
		// Restore A because Dgesvd overwrites it.
		copy(a, aCopy)

		// Allocate slices that will be used below to store the results of full
		// SVD and fill them.
		uAll := make([]float64, m*ldu)
		for i := range uAll {
			uAll[i] = rnd.NormFloat64()
		}
		vtAll := make([]float64, n*ldvt)
		for i := range vtAll {
			vtAll[i] = rnd.NormFloat64()
		}
		sAll := make([]float64, min(m, n))
		for i := range sAll {
			sAll[i] = math.NaN()
		}

		prefix := fmt.Sprintf("m=%v,n=%v,work=%v,mtype=%v", m, n, wl, mtype)

		// Determine workspace size based on wl.
		minwork := max(1, max(5*min(m, n), 3*min(m, n)+max(m, n)))
		var lwork int
		switch wl {
		case minimumWork:
			lwork = minwork
		case mediumWork:
			work := make([]float64, 1)
			impl.Dgesvd(lapack.SVDAll, lapack.SVDAll, m, n, a, lda, sAll, uAll, ldu, vtAll, ldvt, work, -1)
			lwork = (int(work[0]) + minwork) / 2
		case optimumWork:
			work := make([]float64, 1)
			impl.Dgesvd(lapack.SVDAll, lapack.SVDAll, m, n, a, lda, sAll, uAll, ldu, vtAll, ldvt, work, -1)
			lwork = int(work[0])
		}
		work := make([]float64, max(1, lwork))
		for i := range work {
			work[i] = math.NaN()
		}

		// Compute the full SVD which will be used later for checking the partial results.
		ok := impl.Dgesvd(lapack.SVDAll, lapack.SVDAll, m, n, a, lda, sAll, uAll, ldu, vtAll, ldvt, work, len(work))
		if !ok {
			t.Fatalf("Case %v: unexpected failure in full SVD", prefix)
		}

		// Check that uAll, sAll, and vtAll multiply back to A by computing a residual
		//  |A - U*S*VT| / (n*aNorm)
		if resid := svdFullResidual(m, n, aNorm, aCopy, lda, uAll, ldu, sAll, vtAll, ldvt); resid > tol {
			t.Errorf("Case %v: original matrix not recovered for full SVD, |A - U*D*VT|=%v", prefix, resid)
		}
		if minmn > 0 {
			// Check that uAll is orthogonal.
			if !hasOrthonormalColumns(blas64.General{Rows: m, Cols: m, Data: uAll, Stride: ldu}) {
				t.Errorf("Case %v: UAll is not orthogonal", prefix)
			}
			// Check that vtAll is orthogonal.
			if !hasOrthonormalRows(blas64.General{Rows: n, Cols: n, Data: vtAll, Stride: ldvt}) {
				t.Errorf("Case %v: VTAll is not orthogonal", prefix)
			}
		}
		// Check that singular values are decreasing.
		if !sort.IsSorted(sort.Reverse(sort.Float64Slice(sAll))) {
			t.Errorf("Case %v: singular values from full SVD are not decreasing", prefix)
		}
		// Check that singular values are non-negative.
		if minmn > 0 && floats.Min(sAll) < 0 {
			t.Errorf("Case %v: some singular values from full SVD are negative", prefix)
		}

		// Do partial SVD and compare the results to sAll, uAll, and vtAll.
		for _, jobU := range []lapack.SVDJob{lapack.SVDAll, lapack.SVDStore, lapack.SVDOverwrite, lapack.SVDNone} {
			for _, jobVT := range []lapack.SVDJob{lapack.SVDAll, lapack.SVDStore, lapack.SVDOverwrite, lapack.SVDNone} {
				if jobU == lapack.SVDOverwrite || jobVT == lapack.SVDOverwrite {
					// Not implemented.
					continue
				}
				if jobU == lapack.SVDAll && jobVT == lapack.SVDAll {
					// Already checked above.
					continue
				}

				prefix := prefix + ",job=" + svdJobString(jobU) + "U-" + svdJobString(jobVT) + "VT"

				// Restore A to its original values.
				copy(a, aCopy)

				// Allocate slices for the results of partial SVD and fill them.
				u := make([]float64, m*ldu)
				for i := range u {
					u[i] = rnd.NormFloat64()
				}
				vt := make([]float64, n*ldvt)
				for i := range vt {
					vt[i] = rnd.NormFloat64()
				}
				s := make([]float64, min(m, n))
				for i := range s {
					s[i] = math.NaN()
				}

				for i := range work {
					work[i] = math.NaN()
				}

				ok := impl.Dgesvd(jobU, jobVT, m, n, a, lda, s, u, ldu, vt, ldvt, work, len(work))
				if !ok {
					t.Fatalf("Case %v: unexpected failure in partial Dgesvd", prefix)
				}

				if minmn == 0 {
					// No panic and the result is ok, there is
					// nothing else to check.
					continue
				}

				// Check that U has orthogonal columns and that it matches UAll.
				switch jobU {
				case lapack.SVDStore:
					if !hasOrthonormalColumns(blas64.General{Rows: m, Cols: minmn, Data: u, Stride: ldu}) {
						t.Errorf("Case %v: columns of U are not orthogonal", prefix)
					}
					if res := svdPartialUResidual(m, minmn, u, uAll, ldu); res > tol {
						t.Errorf("Case %v: columns of U do not match UAll", prefix)
					}
				case lapack.SVDAll:
					if !hasOrthonormalColumns(blas64.General{Rows: m, Cols: m, Data: u, Stride: ldu}) {
						t.Errorf("Case %v: columns of U are not orthogonal", prefix)
					}
					if res := svdPartialUResidual(m, m, u, uAll, ldu); res > tol {
						t.Errorf("Case %v: columns of U do not match UAll", prefix)
					}
				}
				// Check that VT has orthogonal rows and that it matches VTAll.
				switch jobVT {
				case lapack.SVDStore:
					if !hasOrthonormalRows(blas64.General{Rows: minmn, Cols: n, Data: vtAll, Stride: ldvt}) {
						t.Errorf("Case %v: rows of VT are not orthogonal", prefix)
					}
					if res := svdPartialVTResidual(minmn, n, vt, vtAll, ldvt); res > tol {
						t.Errorf("Case %v: rows of VT do not match VTAll", prefix)
					}
				case lapack.SVDAll:
					if !hasOrthonormalRows(blas64.General{Rows: n, Cols: n, Data: vtAll, Stride: ldvt}) {
						t.Errorf("Case %v: rows of VT are not orthogonal", prefix)
					}
					if res := svdPartialVTResidual(n, n, vt, vtAll, ldvt); res > tol {
						t.Errorf("Case %v: rows of VT do not match VTAll", prefix)
					}
				}
				// Check that singular values are decreasing.
				if !sort.IsSorted(sort.Reverse(sort.Float64Slice(s))) {
					t.Errorf("Case %v: singular values from full SVD are not decreasing", prefix)
				}
				// Check that singular values are non-negative.
				if floats.Min(s) < 0 {
					t.Errorf("Case %v: some singular values from full SVD are negative", prefix)
				}
				if !floats.EqualApprox(s, sAll, tol/10) {
					t.Errorf("Case %v: singular values differ between full and partial SVD\n%v\n%v", prefix, s, sAll)
				}
			}
		}
	}
}

// svdFullResidual returns
//  |A - U*D*VT| / (n * aNorm)
// where U, D, and VT are as computed by Dgesvd with jobU = jobVT = lapack.SVDAll.
func svdFullResidual(m, n int, aNorm float64, a []float64, lda int, u []float64, ldu int, d []float64, vt []float64, ldvt int) float64 {
	// The implementation follows TESTING/dbdt01.f from the reference.

	minmn := min(m, n)
	if minmn == 0 {
		return 0
	}

	// j-th column of A - U*D*VT.
	aMinusUDVT := make([]float64, m)
	// D times the j-th column of VT.
	dvt := make([]float64, minmn)
	// Compute the residual |A - U*D*VT| one column at a time.
	var resid float64
	for j := 0; j < n; j++ {
		// Copy j-th column of A to aj.
		blas64.Copy(blas64.Vector{N: m, Data: a[j:], Inc: lda}, blas64.Vector{N: m, Data: aMinusUDVT, Inc: 1})
		// Multiply D times j-th column of VT.
		for i := 0; i < minmn; i++ {
			dvt[i] = d[i] * vt[i*ldvt+j]
		}
		// Compute the j-th column of A - U*D*VT.
		blas64.Gemv(blas.NoTrans,
			-1, blas64.General{Rows: m, Cols: minmn, Data: u, Stride: ldu}, blas64.Vector{N: minmn, Data: dvt, Inc: 1},
			1, blas64.Vector{N: m, Data: aMinusUDVT, Inc: 1})
		resid = math.Max(resid, blas64.Asum(blas64.Vector{N: m, Data: aMinusUDVT, Inc: 1}))
	}
	if aNorm == 0 {
		if resid != 0 {
			// Original matrix A is zero but the residual is non-zero,
			// return infinity.
			return math.Inf(1)
		}
		// Original matrix A is zero, residual is zero, return 0.
		return 0
	}
	// Original matrix A is non-zero.
	if aNorm >= resid {
		resid = resid / aNorm / float64(n)
	} else {
		if aNorm < 1 {
			resid = math.Min(resid, float64(n)*aNorm) / aNorm / float64(n)
		} else {
			resid = math.Min(resid/aNorm, float64(n)) / float64(n)
		}
	}
	return resid
}

// svdPartialUResidual compares U and URef to see if their columns span the same
// spaces. It returns the maximum over columns of
//  |URef(i) - S*U(i)|
// where URef(i) and U(i) are the i-th columns of URef and U, respectively, and
// S is ±1 chosen to minimize the expression.
func svdPartialUResidual(m, n int, u, uRef []float64, ldu int) float64 {
	var res float64
	for j := 0; j < n; j++ {
		imax := blas64.Iamax(blas64.Vector{N: m, Data: uRef[j:], Inc: ldu})
		s := math.Copysign(1, uRef[imax*ldu+j]) * math.Copysign(1, u[imax*ldu+j])
		for i := 0; i < m; i++ {
			diff := math.Abs(uRef[i*ldu+j] - s*u[i*ldu+j])
			res = math.Max(res, diff)
		}
	}
	return res
}

// svdPartialVTResidual compares VT and VTRef to see if their rows span the same
// spaces. It returns the maximum over rows of
//  |VTRef(i) - S*VT(i)|
// where VTRef(i) and VT(i) are the i-th columns of VTRef and VT, respectively, and
// S is ±1 chosen to minimize the expression.
func svdPartialVTResidual(m, n int, vt, vtRef []float64, ldvt int) float64 {
	var res float64
	for i := 0; i < m; i++ {
		jmax := blas64.Iamax(blas64.Vector{N: n, Data: vtRef[i*ldvt:], Inc: 1})
		s := math.Copysign(1, vtRef[i*ldvt+jmax]) * math.Copysign(1, vt[i*ldvt+jmax])
		for j := 0; j < n; j++ {
			diff := math.Abs(vtRef[i*ldvt+j] - s*vt[i*ldvt+j])
			res = math.Max(res, diff)
		}
	}
	return res
}
