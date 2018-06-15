// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/lapack"
)

type Dgebaler interface {
	Dgebal(job lapack.Job, n int, a []float64, lda int, scale []float64) (int, int)
}

func DgebalTest(t *testing.T, impl Dgebaler) {
	rnd := rand.New(rand.NewSource(1))

	for _, job := range []lapack.Job{lapack.None, lapack.Permute, lapack.Scale, lapack.PermuteScale} {
		for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 10, 18, 31, 53, 100} {
			for _, extra := range []int{0, 11} {
				for cas := 0; cas < 100; cas++ {
					a := unbalancedSparseGeneral(n, n, n+extra, 2*n, rnd)
					testDgebal(t, impl, job, a)
				}
			}
		}
	}
}

func testDgebal(t *testing.T, impl Dgebaler, job lapack.Job, a blas64.General) {
	const tol = 1e-14

	n := a.Rows
	extra := a.Stride - n

	var scale []float64
	if n > 0 {
		scale = nanSlice(n)
	}

	want := cloneGeneral(a)

	ilo, ihi := impl.Dgebal(job, n, a.Data, a.Stride, scale)

	prefix := fmt.Sprintf("Case job=%v, n=%v, extra=%v", job, n, extra)

	if !generalOutsideAllNaN(a) {
		t.Errorf("%v: out-of-range write to A\n%v", prefix, a.Data)
	}

	if n == 0 {
		if ilo != 0 {
			t.Errorf("%v: unexpected ilo when n=0. Want 0, got %v", prefix, n, ilo)
		}
		if ihi != -1 {
			t.Errorf("%v: unexpected ihi when n=0. Want -1, got %v", prefix, n, ihi)
		}
		return
	}

	if job == lapack.None {
		if ilo != 0 {
			t.Errorf("%v: unexpected ilo when job=None. Want 0, got %v", prefix, ilo)
		}
		if ihi != n-1 {
			t.Errorf("%v: unexpected ihi when job=None. Want %v, got %v", prefix, n-1, ihi)
		}
		k := -1
		for i := range scale {
			if scale[i] != 1 {
				k = i
				break
			}
		}
		if k != -1 {
			t.Errorf("%v: unexpected scale[%v] when job=None. Want 1, got %v", prefix, k, scale[k])
		}
		if !equalApproxGeneral(a, want, 0) {
			t.Errorf("%v: unexpected modification of A when job=None", prefix)
		}
		return
	}

	if ilo < 0 || ihi < ilo || n <= ihi {
		t.Errorf("%v: invalid ordering of ilo=%v and ihi=%v", prefix, ilo, ihi)
	}

	if ilo >= 2 && !isUpperTriangular(blas64.General{ilo - 1, ilo - 1, a.Stride, a.Data}) {
		t.Errorf("%v: T1 is not upper triangular", prefix)
	}
	m := n - ihi - 1 // Order of T2.
	k := ihi + 1
	if m >= 2 && !isUpperTriangular(blas64.General{m, m, a.Stride, a.Data[k*a.Stride+k:]}) {
		t.Errorf("%v: T2 is not upper triangular", prefix)
	}

	if job == lapack.Permute || job == lapack.PermuteScale {
		// Check that all rows in [ilo:ihi+1] have at least one nonzero
		// off-diagonal element.
		zeroRow := -1
		for i := ilo; i <= ihi; i++ {
			onlyZeros := true
			for j := ilo; j <= ihi; j++ {
				if i != j && a.Data[i*a.Stride+j] != 0 {
					onlyZeros = false
					break
				}
			}
			if onlyZeros {
				zeroRow = i
				break
			}
		}
		if zeroRow != -1 && ilo != ihi {
			t.Errorf("%v: row %v has only zero off-diagonal elements, ilo=%v, ihi=%v", prefix, zeroRow, ilo, ihi)
		}
		// Check that all columns in [ilo:ihi+1] have at least one nonzero
		// off-diagonal element.
		zeroCol := -1
		for j := ilo; j <= ihi; j++ {
			onlyZeros := true
			for i := ilo; i <= ihi; i++ {
				if i != j && a.Data[i*a.Stride+j] != 0 {
					onlyZeros = false
					break
				}
			}
			if onlyZeros {
				zeroCol = j
				break
			}
		}
		if zeroCol != -1 && ilo != ihi {
			t.Errorf("%v: column %v has only zero off-diagonal elements, ilo=%v, ihi=%v", prefix, zeroCol, ilo, ihi)
		}

		// Create the permutation matrix P.
		p := eye(n, n)
		for j := n - 1; j > ihi; j-- {
			blas64.Swap(n,
				blas64.Vector{p.Stride, p.Data[j:]},
				blas64.Vector{p.Stride, p.Data[int(scale[j]):]})
		}
		for j := 0; j < ilo; j++ {
			blas64.Swap(n,
				blas64.Vector{p.Stride, p.Data[j:]},
				blas64.Vector{p.Stride, p.Data[int(scale[j]):]})
		}
		// Compute P^T*A*P and store into want.
		ap := zeros(n, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, want, p, 0, ap)
		blas64.Gemm(blas.Trans, blas.NoTrans, 1, p, ap, 0, want)
	}
	if job == lapack.Scale || job == lapack.PermuteScale {
		// Modify want by D and D^{-1}.
		d := eye(n, n)
		dinv := eye(n, n)
		for i := ilo; i <= ihi; i++ {
			d.Data[i*d.Stride+i] = scale[i]
			dinv.Data[i*dinv.Stride+i] = 1 / scale[i]
		}
		ad := zeros(n, n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, want, d, 0, ad)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, dinv, ad, 0, want)
	}
	if !equalApproxGeneral(want, a, tol) {
		t.Errorf("%v: unexpected value of A, ilo=%v, ihi=%v", prefix, ilo, ihi)
	}
}
