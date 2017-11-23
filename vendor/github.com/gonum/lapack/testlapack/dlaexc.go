// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
)

type Dlaexcer interface {
	Dlaexc(wantq bool, n int, t []float64, ldt int, q []float64, ldq int, j1, n1, n2 int, work []float64) bool
}

func DlaexcTest(t *testing.T, impl Dlaexcer) {
	rnd := rand.New(rand.NewSource(1))

	for _, wantq := range []bool{true, false} {
		for _, n := range []int{1, 2, 3, 4, 5, 6, 10, 18, 31, 53} {
			for _, extra := range []int{0, 1, 11} {
				for cas := 0; cas < 100; cas++ {
					j1 := rnd.Intn(n)
					n1 := min(rnd.Intn(3), n-j1)
					n2 := min(rnd.Intn(3), n-j1-n1)
					testDlaexc(t, impl, wantq, n, j1, n1, n2, extra, rnd)
				}
			}
		}
	}
}

func testDlaexc(t *testing.T, impl Dlaexcer, wantq bool, n, j1, n1, n2, extra int, rnd *rand.Rand) {
	const tol = 1e-14

	tmat := randomGeneral(n, n, n+extra, rnd)
	// Zero out the lower triangle.
	for i := 1; i < n; i++ {
		for j := 0; j < i; j++ {
			tmat.Data[i*tmat.Stride+j] = 0
		}
	}
	// Make any 2x2 diagonal block to be in Schur canonical form.
	if n1 == 2 {
		// Diagonal elements equal.
		tmat.Data[(j1+1)*tmat.Stride+j1+1] = tmat.Data[j1*tmat.Stride+j1]
		// Off-diagonal elements of opposite sign.
		c := rnd.NormFloat64()
		if math.Signbit(c) == math.Signbit(tmat.Data[j1*tmat.Stride+j1+1]) {
			c *= -1
		}
		tmat.Data[(j1+1)*tmat.Stride+j1] = c
	}
	if n2 == 2 {
		// Diagonal elements equal.
		tmat.Data[(j1+n1+1)*tmat.Stride+j1+n1+1] = tmat.Data[(j1+n1)*tmat.Stride+j1+n1]
		// Off-diagonal elements of opposite sign.
		c := rnd.NormFloat64()
		if math.Signbit(c) == math.Signbit(tmat.Data[(j1+n1)*tmat.Stride+j1+n1+1]) {
			c *= -1
		}
		tmat.Data[(j1+n1+1)*tmat.Stride+j1+n1] = c
	}
	tmatCopy := cloneGeneral(tmat)
	var q, qCopy blas64.General
	if wantq {
		q = eye(n, n+extra)
		qCopy = cloneGeneral(q)
	}
	work := nanSlice(n)

	ok := impl.Dlaexc(wantq, n, tmat.Data, tmat.Stride, q.Data, q.Stride, j1, n1, n2, work)

	prefix := fmt.Sprintf("Case n=%v, j1=%v, n1=%v, n2=%v, wantq=%v, extra=%v", n, j1, n1, n2, wantq, extra)

	if !generalOutsideAllNaN(tmat) {
		t.Errorf("%v: out-of-range write to T", prefix)
	}
	if wantq && !generalOutsideAllNaN(q) {
		t.Errorf("%v: out-of-range write to Q", prefix)
	}

	if !ok {
		if n1 == 1 && n2 == 1 {
			t.Errorf("%v: unexpected failure", prefix)
		} else {
			t.Logf("%v: Dlaexc returned false")
		}
	}

	if !ok || n1 == 0 || n2 == 0 || j1+n1 >= n {
		// Check that T is not modified.
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if tmat.Data[i*tmat.Stride+j] != tmatCopy.Data[i*tmatCopy.Stride+j] {
					t.Errorf("%v: ok == false but T[%v,%v] modified", prefix, i, j)
				}
			}
		}
		if !wantq {
			return
		}
		// Check that Q is not modified.
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				if q.Data[i*q.Stride+j] != qCopy.Data[i*qCopy.Stride+j] {
					t.Errorf("%v: ok == false but Q[%v,%v] modified", prefix, i, j)
				}
			}
		}
		return
	}

	// Check that T is not modified outside of rows and columns [j1:j1+n1+n2].
	for i := 0; i < n; i++ {
		if j1 <= i && i < j1+n1+n2 {
			continue
		}
		for j := 0; j < n; j++ {
			if j1 <= j && j < j1+n1+n2 {
				continue
			}
			diff := tmat.Data[i*tmat.Stride+j] - tmatCopy.Data[i*tmatCopy.Stride+j]
			if diff != 0 {
				t.Errorf("%v: unexpected modification of T[%v,%v]", prefix, i, j)
			}
		}
	}

	if n1 == 1 {
		// 1×1 blocks are swapped exactly.
		got := tmat.Data[(j1+n2)*tmat.Stride+j1+n2]
		want := tmatCopy.Data[j1*tmatCopy.Stride+j1]
		if want != got {
			t.Errorf("%v: unexpected value of T[%v,%v]. Want %v, got %v", prefix, j1+n2, j1+n2, want, got)
		}
	} else {
		// Check that the swapped 2×2 block is in Schur canonical form.
		// The n1×n1 block is now located at T[j1+n2,j1+n2].
		a, b, c, d := extract2x2Block(tmat.Data[(j1+n2)*tmat.Stride+j1+n2:], tmat.Stride)
		if !isSchurCanonical(a, b, c, d) {
			t.Errorf("%v: 2×2 block at T[%v,%v] not in Schur canonical form", prefix, j1+n2, j1+n2)
		}
		ev1Got, ev2Got := schurBlockEigenvalues(a, b, c, d)

		// Check that the swapped 2×2 block has the same eigenvalues.
		// The n1×n1 block was originally located at T[j1,j1].
		a, b, c, d = extract2x2Block(tmatCopy.Data[j1*tmatCopy.Stride+j1:], tmatCopy.Stride)
		ev1Want, ev2Want := schurBlockEigenvalues(a, b, c, d)
		if cmplx.Abs(ev1Got-ev1Want) > tol {
			t.Errorf("%v: unexpected first eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, j1+n2, j1+n2, ev1Want, ev1Got)
		}
		if cmplx.Abs(ev2Got-ev2Want) > tol {
			t.Errorf("%v: unexpected second eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, j1+n2, j1+n2, ev2Want, ev2Got)
		}
	}
	if n2 == 1 {
		// 1×1 blocks are swapped exactly.
		got := tmat.Data[j1*tmat.Stride+j1]
		want := tmatCopy.Data[(j1+n1)*tmatCopy.Stride+j1+n1]
		if want != got {
			t.Errorf("%v: unexpected value of T[%v,%v]. Want %v, got %v", prefix, j1, j1, want, got)
		}
	} else {
		// Check that the swapped 2×2 block is in Schur canonical form.
		// The n2×n2 block is now located at T[j1,j1].
		a, b, c, d := extract2x2Block(tmat.Data[j1*tmat.Stride+j1:], tmat.Stride)
		if !isSchurCanonical(a, b, c, d) {
			t.Errorf("%v: 2×2 block at T[%v,%v] not in Schur canonical form", prefix, j1, j1)
		}
		ev1Got, ev2Got := schurBlockEigenvalues(a, b, c, d)

		// Check that the swapped 2×2 block has the same eigenvalues.
		// The n2×n2 block was originally located at T[j1+n1,j1+n1].
		a, b, c, d = extract2x2Block(tmatCopy.Data[(j1+n1)*tmatCopy.Stride+j1+n1:], tmatCopy.Stride)
		ev1Want, ev2Want := schurBlockEigenvalues(a, b, c, d)
		if cmplx.Abs(ev1Got-ev1Want) > tol {
			t.Errorf("%v: unexpected first eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, j1, j1, ev1Want, ev1Got)
		}
		if cmplx.Abs(ev2Got-ev2Want) > tol {
			t.Errorf("%v: unexpected second eigenvalue of 2×2 block at T[%v,%v]. Want %v, got %v",
				prefix, j1, j1, ev2Want, ev2Got)
		}
	}

	if !wantq {
		return
	}

	if !isOrthonormal(q) {
		t.Errorf("%v: Q is not orthogonal", prefix)
	}
	// Check that Q is unchanged outside of columns [j1:j1+n1+n2].
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if j1 <= j && j < j1+n1+n2 {
				continue
			}
			diff := q.Data[i*q.Stride+j] - qCopy.Data[i*qCopy.Stride+j]
			if diff != 0 {
				t.Errorf("%v: unexpected modification of Q[%v,%v]", prefix, i, j)
			}
		}
	}
	// Check that Q^T TOrig Q == T.
	tq := eye(n, n)
	blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, tmatCopy, q, 0, tq)
	qtq := eye(n, n)
	blas64.Gemm(blas.Trans, blas.NoTrans, 1, q, tq, 0, qtq)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			diff := qtq.Data[i*qtq.Stride+j] - tmat.Data[i*tmat.Stride+j]
			if math.Abs(diff) > tol {
				t.Errorf("%v: unexpected value of T[%v,%v]", prefix, i, j)
			}
		}
	}
}
