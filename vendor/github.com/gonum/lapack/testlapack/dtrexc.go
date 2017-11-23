// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math/cmplx"
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/lapack"
)

type Dtrexcer interface {
	Dtrexc(compq lapack.EVComp, n int, t []float64, ldt int, q []float64, ldq int, ifst, ilst int, work []float64) (ifstOut, ilstOut int, ok bool)
}

func DtrexcTest(t *testing.T, impl Dtrexcer) {
	rnd := rand.New(rand.NewSource(1))

	for _, compq := range []lapack.EVComp{lapack.None, lapack.UpdateSchur} {
		for _, n := range []int{1, 2, 3, 4, 5, 6, 10, 18, 31, 53} {
			for _, extra := range []int{0, 1, 11} {
				for cas := 0; cas < 100; cas++ {
					tmat := randomSchurCanonical(n, n+extra, rnd)
					ifst := rnd.Intn(n)
					ilst := rnd.Intn(n)
					testDtrexc(t, impl, compq, tmat, ifst, ilst, extra, rnd)
				}
			}
		}
	}

	for _, compq := range []lapack.EVComp{lapack.None, lapack.UpdateSchur} {
		for _, extra := range []int{0, 1, 11} {
			tmat := randomSchurCanonical(0, extra, rnd)
			testDtrexc(t, impl, compq, tmat, 0, 0, extra, rnd)
		}
	}
}

func testDtrexc(t *testing.T, impl Dtrexcer, compq lapack.EVComp, tmat blas64.General, ifst, ilst, extra int, rnd *rand.Rand) {
	const tol = 1e-13

	n := tmat.Rows
	fstSize, fstFirst := schurBlockSize(tmat, ifst)
	lstSize, lstFirst := schurBlockSize(tmat, ilst)

	tmatCopy := cloneGeneral(tmat)

	var wantq bool
	var q, qCopy blas64.General
	if compq == lapack.UpdateSchur {
		wantq = true
		q = eye(n, n+extra)
		qCopy = cloneGeneral(q)
	}

	work := nanSlice(n)

	ifstGot, ilstGot, ok := impl.Dtrexc(compq, n, tmat.Data, tmat.Stride, q.Data, q.Stride, ifst, ilst, work)

	prefix := fmt.Sprintf("Case compq=%v, n=%v, ifst=%v, nbf=%v, ilst=%v, nbl=%v, extra=%v",
		compq, n, ifst, fstSize, ilst, lstSize, extra)

	if !generalOutsideAllNaN(tmat) {
		t.Errorf("%v: out-of-range write to T", prefix)
	}
	if wantq && !generalOutsideAllNaN(q) {
		t.Errorf("%v: out-of-range write to Q", prefix)
	}

	if !ok {
		t.Logf("%v: Dtrexc returned ok=false", prefix)
	}

	// Check that the index of the first block was correctly updated (if
	// necessary).
	ifstWant := ifst
	if !fstFirst {
		ifstWant = ifst - 1
	}
	if ifstWant != ifstGot {
		t.Errorf("%v: unexpected ifst index. Want %v, got %v ", prefix, ifstWant, ifstGot)
	}

	// Check that the index of the last block is as expected when ok=true.
	// When ok=false, we don't know at which block the algorithm failed, so
	// we don't check.
	ilstWant := ilst
	if !lstFirst {
		ilstWant--
	}
	if ok {
		if ifstWant < ilstWant {
			// If the blocks are swapped backwards, these
			// adjustments are not necessary, the first row of the
			// last block will end up at ifst.
			switch {
			case fstSize == 2 && lstSize == 1:
				ilstWant--
			case fstSize == 1 && lstSize == 2:
				ilstWant++
			}
		}
		if ilstWant != ilstGot {
			t.Errorf("%v: unexpected ilst index. Want %v, got %v", prefix, ilstWant, ilstGot)
		}
	}

	if n <= 1 || ifstGot == ilstGot {
		// Too small matrix or no swapping.
		// Check that T was not modified.
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if tmat.Data[i*tmat.Stride+j] != tmatCopy.Data[i*tmatCopy.Stride+j] {
					t.Errorf("%v: unexpected modification at T[%v,%v]", prefix, i, j)
				}
			}
		}
		if !wantq {
			return
		}
		// Check that Q was not modified.
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if q.Data[i*q.Stride+j] != qCopy.Data[i*qCopy.Stride+j] {
					t.Errorf("%v: unexpected modification at Q[%v,%v]", prefix, i, j)
				}
			}
		}
		return
	}

	if !isSchurCanonicalGeneral(tmat) {
		t.Errorf("%v: T is not in Schur canonical form", prefix)
	}

	// Check that T was not modified except above the second subdiagonal in
	// rows and columns [modMin,modMax].
	modMin := min(ifstGot, ilstGot)
	modMax := max(ifstGot, ilstGot) + fstSize
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if modMin <= i && i < modMax && j+1 >= i {
				continue
			}
			if modMin <= j && j < modMax && j+1 >= i {
				continue
			}
			diff := tmat.Data[i*tmat.Stride+j] - tmatCopy.Data[i*tmatCopy.Stride+j]
			if diff != 0 {
				t.Errorf("%v: unexpected modification at T[%v,%v]", prefix, i, j)
			}
		}
	}

	// Check that the block at ifstGot was delivered to ilstGot correctly.
	if fstSize == 1 {
		// 1×1 blocks are swapped exactly.
		got := tmat.Data[ilstGot*tmat.Stride+ilstGot]
		want := tmatCopy.Data[ifstGot*tmatCopy.Stride+ifstGot]
		if want != got {
			t.Errorf("%v: unexpected 1×1 block at T[%v,%v]. Want %v, got %v",
				prefix, want, got, ilstGot, ilstGot)
		}
	} else {
		// Check that the swapped 2×2 block is in Schur canonical form.
		a, b, c, d := extract2x2Block(tmat.Data[ilstGot*tmat.Stride+ilstGot:], tmat.Stride)
		if !isSchurCanonical(a, b, c, d) {
			t.Errorf("%v: 2×2 block at T[%v,%v] not in Schur canonical form", prefix, ilstGot, ilstGot)
		}
		ev1Got, ev2Got := schurBlockEigenvalues(a, b, c, d)

		// Check that the swapped 2×2 block has the same eigenvalues.
		// The block was originally located at T[ifstGot,ifstGot].
		a, b, c, d = extract2x2Block(tmatCopy.Data[ifstGot*tmatCopy.Stride+ifstGot:], tmatCopy.Stride)
		ev1Want, ev2Want := schurBlockEigenvalues(a, b, c, d)
		if cmplx.Abs(ev1Got-ev1Want) > tol {
			t.Errorf("%v: unexpected first eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, ilstGot, ilstGot, ev1Want, ev1Got)
		}
		if cmplx.Abs(ev2Got-ev2Want) > tol {
			t.Errorf("%v: unexpected second eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, ilstGot, ilstGot, ev2Want, ev2Got)
		}
	}

	if !wantq {
		return
	}

	if !isOrthonormal(q) {
		t.Errorf("%v: Q is not orthogonal", prefix)
	}
	// Check that Q is unchanged outside of columns [modMin,modMax].
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if modMin <= j && j < modMax {
				continue
			}
			if q.Data[i*q.Stride+j]-qCopy.Data[i*qCopy.Stride+j] != 0 {
				t.Errorf("%v: unexpected modification of Q[%v,%v]", prefix, i, j)
			}
		}
	}
	// Check that Q^T TOrig Q == T.
	tq := eye(n, n)
	blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, tmatCopy, q, 0, tq)
	qtq := eye(n, n)
	blas64.Gemm(blas.Trans, blas.NoTrans, 1, q, tq, 0, qtq)
	if !equalApproxGeneral(qtq, tmat, tol) {
		t.Errorf("%v: Q^T (initial T) Q and (final T) are not equal", prefix)
	}
}
