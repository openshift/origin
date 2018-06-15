// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/gonum/blas/blas64"
	"github.com/gonum/floats"
	"github.com/gonum/lapack"
)

type Dtrevc3er interface {
	Dtrevc3(side lapack.EVSide, howmny lapack.HowMany, selected []bool, n int, t []float64, ldt int, vl []float64, ldvl int, vr []float64, ldvr int, mm int, work []float64, lwork int) int
}

func Dtrevc3Test(t *testing.T, impl Dtrevc3er) {
	rnd := rand.New(rand.NewSource(1))
	for _, side := range []lapack.EVSide{lapack.RightEV, lapack.LeftEV, lapack.RightLeftEV} {
		for _, howmny := range []lapack.HowMany{lapack.AllEV, lapack.AllEVMulQ, lapack.SelectedEV} {
			for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 34, 100} {
				for _, extra := range []int{0, 11} {
					for _, optwork := range []bool{true, false} {
						for cas := 0; cas < 10; cas++ {
							tmat := randomSchurCanonical(n, n+extra, rnd)
							testDtrevc3(t, impl, side, howmny, tmat, optwork, rnd)
						}
					}
				}
			}
		}
	}
}

func testDtrevc3(t *testing.T, impl Dtrevc3er, side lapack.EVSide, howmny lapack.HowMany, tmat blas64.General, optwork bool, rnd *rand.Rand) {
	const tol = 1e-14

	n := tmat.Rows
	extra := tmat.Stride - tmat.Cols
	right := side != lapack.LeftEV
	left := side != lapack.RightEV

	var selected, selectedWant []bool
	var mWant int // How many columns will the eigenvectors occupy.
	if howmny == lapack.SelectedEV {
		selected = make([]bool, n)
		selectedWant = make([]bool, n)
		// Dtrevc3 will compute only selected eigenvectors. Pick them
		// randomly disregarding whether they are real or complex.
		for i := range selected {
			if rnd.Float64() < 0.5 {
				selected[i] = true
			}
		}
		// Dtrevc3 will modify (standardize) the slice selected based on
		// whether the corresponding eigenvalues are real or complex. Do
		// the same process here to fill selectedWant.
		for i := 0; i < n; {
			if i == n-1 || tmat.Data[(i+1)*tmat.Stride+i] == 0 {
				// Real eigenvalue.
				if selected[i] {
					selectedWant[i] = true
					mWant++ // Real eigenvectors occupy one column.
				}
				i++
			} else {
				// Complex eigenvalue.
				if selected[i] || selected[i+1] {
					// Dtrevc3 will modify selected so that
					// only the first element of the pair is
					// true.
					selectedWant[i] = true
					mWant += 2 // Complex eigenvectors occupy two columns.
				}
				i += 2
			}
		}
	} else {
		// All eigenvectors occupy n columns.
		mWant = n
	}

	var vr blas64.General
	if right {
		if howmny == lapack.AllEVMulQ {
			vr = eye(n, n+extra)
		} else {
			// VR will be overwritten.
			vr = nanGeneral(n, mWant, n+extra)
		}
	}

	var vl blas64.General
	if left {
		if howmny == lapack.AllEVMulQ {
			vl = eye(n, n+extra)
		} else {
			// VL will be overwritten.
			vl = nanGeneral(n, mWant, n+extra)
		}
	}

	work := make([]float64, max(1, 3*n))
	if optwork {
		impl.Dtrevc3(side, howmny, nil, n, nil, 1, nil, 1, nil, 1, mWant, work, -1)
		work = make([]float64, int(work[0]))
	}

	m := impl.Dtrevc3(side, howmny, selected, n, tmat.Data, tmat.Stride,
		vl.Data, vl.Stride, vr.Data, vr.Stride, mWant, work, len(work))

	prefix := fmt.Sprintf("Case side=%v, howmny=%v, n=%v, extra=%v, optwk=%v",
		side, howmny, n, extra, optwork)

	if !generalOutsideAllNaN(tmat) {
		t.Errorf("%v: out-of-range write to T", prefix)
	}
	if !generalOutsideAllNaN(vl) {
		t.Errorf("%v: out-of-range write to VL", prefix)
	}
	if !generalOutsideAllNaN(vr) {
		t.Errorf("%v: out-of-range write to VR", prefix)
	}

	if m != mWant {
		t.Errorf("%v: unexpected value of m. Want %v, got %v", prefix, mWant, m)
	}

	if howmny == lapack.SelectedEV {
		for i := range selected {
			if selected[i] != selectedWant[i] {
				t.Errorf("%v: unexpected selected[%v]", prefix, i)
			}
		}
	}

	// Check that the columns of VR and VL are actually eigenvectors and
	// that the magnitude of their largest element is 1.
	var k int
	for j := 0; j < n; {
		re := tmat.Data[j*tmat.Stride+j]
		if j == n-1 || tmat.Data[(j+1)*tmat.Stride+j] == 0 {
			if howmny == lapack.SelectedEV && !selected[j] {
				j++
				continue
			}
			if right {
				ev := columnOf(vr, k)
				norm := floats.Norm(ev, math.Inf(1))
				if math.Abs(norm-1) > tol {
					t.Errorf("%v: magnitude of largest element of VR[:,%v] not 1", prefix, k)
				}
				if !isRightEigenvectorOf(tmat, ev, nil, complex(re, 0), tol) {
					t.Errorf("%v: VR[:,%v] is not real right eigenvector", prefix, k)
				}
			}
			if left {
				ev := columnOf(vl, k)
				norm := floats.Norm(ev, math.Inf(1))
				if math.Abs(norm-1) > tol {
					t.Errorf("%v: magnitude of largest element of VL[:,%v] not 1", prefix, k)
				}
				if !isLeftEigenvectorOf(tmat, ev, nil, complex(re, 0), tol) {
					t.Errorf("%v: VL[:,%v] is not real left eigenvector", prefix, k)
				}
			}
			k++
			j++
			continue
		}
		if howmny == lapack.SelectedEV && !selected[j] {
			j += 2
			continue
		}
		im := math.Sqrt(math.Abs(tmat.Data[(j+1)*tmat.Stride+j])) *
			math.Sqrt(math.Abs(tmat.Data[j*tmat.Stride+j+1]))
		if right {
			evre := columnOf(vr, k)
			evim := columnOf(vr, k+1)
			var evmax float64
			for i, v := range evre {
				evmax = math.Max(evmax, math.Abs(v)+math.Abs(evim[i]))
			}
			if math.Abs(evmax-1) > tol {
				t.Errorf("%v: magnitude of largest element of VR[:,%v] not 1", prefix, k)
			}
			if !isRightEigenvectorOf(tmat, evre, evim, complex(re, im), tol) {
				t.Errorf("%v: VR[:,%v:%v] is not complex right eigenvector", prefix, k, k+1)
			}
			floats.Scale(-1, evim)
			if !isRightEigenvectorOf(tmat, evre, evim, complex(re, -im), tol) {
				t.Errorf("%v: VR[:,%v:%v] is not complex right eigenvector", prefix, k, k+1)
			}
		}
		if left {
			evre := columnOf(vl, k)
			evim := columnOf(vl, k+1)
			var evmax float64
			for i, v := range evre {
				evmax = math.Max(evmax, math.Abs(v)+math.Abs(evim[i]))
			}
			if math.Abs(evmax-1) > tol {
				t.Errorf("%v: magnitude of largest element of VL[:,%v] not 1", prefix, k)
			}
			if !isLeftEigenvectorOf(tmat, evre, evim, complex(re, im), tol) {
				t.Errorf("%v: VL[:,%v:%v] is not complex left eigenvector", prefix, k, k+1)
			}
			floats.Scale(-1, evim)
			if !isLeftEigenvectorOf(tmat, evre, evim, complex(re, -im), tol) {
				t.Errorf("%v: VL[:,%v:%v] is not complex left eigenvector", prefix, k, k+1)
			}
		}
		k += 2
		j += 2
	}
}
