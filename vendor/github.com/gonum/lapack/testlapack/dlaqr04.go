// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
)

type Dlaqr04er interface {
	Dlaqr04(wantt, wantz bool, n, ilo, ihi int, h []float64, ldh int, wr, wi []float64, iloz, ihiz int, z []float64, ldz int, work []float64, lwork int, recur int) int

	Dlahqrer
}

type dlaqr04Test struct {
	h            blas64.General
	ilo, ihi     int
	iloz, ihiz   int
	wantt, wantz bool

	evWant []complex128 // Optional slice holding known eigenvalues.
}

func Dlaqr04Test(t *testing.T, impl Dlaqr04er) {
	rnd := rand.New(rand.NewSource(1))

	// Tests for small matrices that choose the ilo,ihi and iloz,ihiz pairs
	// randomly.
	for _, wantt := range []bool{true, false} {
		for _, wantz := range []bool{true, false} {
			for _, n := range []int{1, 2, 3, 4, 5, 6, 10, 11, 12, 18, 29} {
				for _, extra := range []int{0, 11} {
					for recur := 0; recur <= 2; recur++ {
						for cas := 0; cas < n; cas++ {
							ilo := rnd.Intn(n)
							ihi := rnd.Intn(n)
							if ilo > ihi {
								ilo, ihi = ihi, ilo
							}
							iloz := rnd.Intn(ilo + 1)
							ihiz := ihi + rnd.Intn(n-ihi)
							h := randomHessenberg(n, n+extra, rnd)
							if ilo-1 >= 0 {
								h.Data[ilo*h.Stride+ilo-1] = 0
							}
							if ihi+1 < n {
								h.Data[(ihi+1)*h.Stride+ihi] = 0
							}
							test := dlaqr04Test{
								h:     h,
								ilo:   ilo,
								ihi:   ihi,
								iloz:  iloz,
								ihiz:  ihiz,
								wantt: wantt,
								wantz: wantz,
							}
							testDlaqr04(t, impl, test, false, recur)
							testDlaqr04(t, impl, test, true, recur)
						}
					}
				}
			}
		}
	}

	// Tests for matrices large enough to possibly use the recursion (but it
	// doesn't seem to be the case).
	for _, n := range []int{100, 500} {
		for cas := 0; cas < 5; cas++ {
			h := randomHessenberg(n, n, rnd)
			test := dlaqr04Test{
				h:     h,
				ilo:   0,
				ihi:   n - 1,
				iloz:  0,
				ihiz:  n - 1,
				wantt: true,
				wantz: true,
			}
			testDlaqr04(t, impl, test, true, 1)
		}
	}

	// Tests that make sure that some potentially problematic corner cases,
	// like zero-sized matrix, are covered.
	for _, wantt := range []bool{true, false} {
		for _, wantz := range []bool{true, false} {
			for _, extra := range []int{0, 1, 11} {
				for _, test := range []dlaqr04Test{
					{
						h:    randomHessenberg(0, extra, rnd),
						ilo:  0,
						ihi:  -1,
						iloz: 0,
						ihiz: -1,
					},
					{
						h:    randomHessenberg(1, 1+extra, rnd),
						ilo:  0,
						ihi:  0,
						iloz: 0,
						ihiz: 0,
					},
					{
						h:    randomHessenberg(2, 2+extra, rnd),
						ilo:  1,
						ihi:  1,
						iloz: 1,
						ihiz: 1,
					},
					{
						h:    randomHessenberg(2, 2+extra, rnd),
						ilo:  0,
						ihi:  1,
						iloz: 0,
						ihiz: 1,
					},
					{
						h:    randomHessenberg(10, 10+extra, rnd),
						ilo:  0,
						ihi:  0,
						iloz: 0,
						ihiz: 0,
					},
					{
						h:    randomHessenberg(10, 10+extra, rnd),
						ilo:  0,
						ihi:  9,
						iloz: 0,
						ihiz: 9,
					},
					{
						h:    randomHessenberg(10, 10+extra, rnd),
						ilo:  0,
						ihi:  1,
						iloz: 0,
						ihiz: 1,
					},
					{
						h:    randomHessenberg(10, 10+extra, rnd),
						ilo:  0,
						ihi:  1,
						iloz: 0,
						ihiz: 9,
					},
					{
						h:    randomHessenberg(10, 10+extra, rnd),
						ilo:  9,
						ihi:  9,
						iloz: 0,
						ihiz: 9,
					},
				} {
					if test.ilo-1 >= 0 {
						test.h.Data[test.ilo*test.h.Stride+test.ilo-1] = 0
					}
					if test.ihi+1 < test.h.Rows {
						test.h.Data[(test.ihi+1)*test.h.Stride+test.ihi] = 0
					}
					test.wantt = wantt
					test.wantz = wantz
					testDlaqr04(t, impl, test, false, 1)
					testDlaqr04(t, impl, test, true, 1)
				}
			}
		}
	}

	// Tests with known eigenvalues computed by Octave.
	for _, test := range []dlaqr04Test{
		{
			h: blas64.General{
				Rows:   1,
				Cols:   1,
				Stride: 1,
				Data:   []float64{7.09965484086874e-1},
			},
			ilo:    0,
			ihi:    0,
			iloz:   0,
			ihiz:   0,
			evWant: []complex128{7.09965484086874e-1},
		},
		{
			h: blas64.General{
				Rows:   2,
				Cols:   2,
				Stride: 2,
				Data: []float64{
					0, -1,
					1, 0,
				},
			},
			ilo:    0,
			ihi:    1,
			iloz:   0,
			ihiz:   1,
			evWant: []complex128{1i, -1i},
		},
		{
			h: blas64.General{
				Rows:   2,
				Cols:   2,
				Stride: 2,
				Data: []float64{
					6.25219991450918e-1, 8.17510791994361e-1,
					3.31218891622294e-1, 1.24103744878131e-1,
				},
			},
			ilo:    0,
			ihi:    1,
			iloz:   0,
			ihiz:   1,
			evWant: []complex128{9.52203547663447e-1, -2.02879811334398e-1},
		},
		{
			h: blas64.General{
				Rows:   4,
				Cols:   4,
				Stride: 4,
				Data: []float64{
					1, 0, 0, 0,
					0, 6.25219991450918e-1, 8.17510791994361e-1, 0,
					0, 3.31218891622294e-1, 1.24103744878131e-1, 0,
					0, 0, 0, 1,
				},
			},
			ilo:    1,
			ihi:    2,
			iloz:   0,
			ihiz:   3,
			evWant: []complex128{9.52203547663447e-1, -2.02879811334398e-1},
		},
		{
			h: blas64.General{
				Rows:   2,
				Cols:   2,
				Stride: 2,
				Data: []float64{
					-1.1219562276608, 6.85473513349362e-1,
					-8.19951061145131e-1, 1.93728523178888e-1,
				},
			},
			ilo:  0,
			ihi:  1,
			iloz: 0,
			ihiz: 1,
			evWant: []complex128{
				-4.64113852240958e-1 + 3.59580510817350e-1i,
				-4.64113852240958e-1 - 3.59580510817350e-1i,
			},
		},
		{
			h: blas64.General{
				Rows:   5,
				Cols:   5,
				Stride: 5,
				Data: []float64{
					9.57590178533658e-1, -5.10651295522708e-1, 9.24974510015869e-1, -1.30016306879522e-1, 2.92601986926954e-2,
					-1.08084756637964, 1.77529701001213, -1.36480197632509, 2.23196371219601e-1, 1.12912853063308e-1,
					0, -8.44075612174676e-1, 1.067867614486, -2.55782915176399e-1, -2.00598563137468e-1,
					0, 0, -5.67097237165410e-1, 2.07205057427341e-1, 6.54998340743380e-1,
					0, 0, 0, -1.89441413886041e-1, -4.18125416021786e-1,
				},
			},
			ilo:  0,
			ihi:  4,
			iloz: 0,
			ihiz: 4,
			evWant: []complex128{
				2.94393309555622,
				4.97029793606701e-1 + 3.63041654992384e-1i,
				4.97029793606701e-1 - 3.63041654992384e-1i,
				-1.74079119166145e-1 + 2.01570009462092e-1i,
				-1.74079119166145e-1 - 2.01570009462092e-1i,
			},
		},
	} {
		test.wantt = true
		test.wantz = true
		testDlaqr04(t, impl, test, false, 1)
		testDlaqr04(t, impl, test, true, 1)
	}
}

func testDlaqr04(t *testing.T, impl Dlaqr04er, test dlaqr04Test, optwork bool, recur int) {
	const tol = 1e-14

	h := cloneGeneral(test.h)
	n := h.Cols
	extra := h.Stride - h.Cols
	wantt := test.wantt
	wantz := test.wantz
	ilo := test.ilo
	ihi := test.ihi
	iloz := test.iloz
	ihiz := test.ihiz

	var z, zCopy blas64.General
	if wantz {
		z = eye(n, n+extra)
		zCopy = cloneGeneral(z)
	}

	wr := nanSlice(ihi + 1)
	wi := nanSlice(ihi + 1)

	var work []float64
	if optwork {
		work = nanSlice(1)
		impl.Dlaqr04(wantt, wantz, n, ilo, ihi, nil, 0, nil, nil, iloz, ihiz, nil, 0, work, -1, recur)
		work = nanSlice(int(work[0]))
	} else {
		work = nanSlice(max(1, n))
	}

	unconverged := impl.Dlaqr04(wantt, wantz, n, ilo, ihi, h.Data, h.Stride, wr, wi, iloz, ihiz, z.Data, z.Stride, work, len(work), recur)

	prefix := fmt.Sprintf("Case wantt=%v, wantz=%v, n=%v, ilo=%v, ihi=%v, iloz=%v, ihiz=%v, extra=%v, opt=%v",
		wantt, wantz, n, ilo, ihi, iloz, ihiz, extra, optwork)

	if !generalOutsideAllNaN(h) {
		t.Errorf("%v: out-of-range write to H\n%v", prefix, h.Data)
	}
	if !generalOutsideAllNaN(z) {
		t.Errorf("%v: out-of-range write to Z\n%v", prefix, z.Data)
	}

	start := ilo // Index of the first computed eigenvalue.
	if unconverged != 0 {
		start = unconverged
		if start == ihi+1 {
			t.Logf("%v: no eigenvalue has converged", prefix)
		}
	}

	// Check that wr and wi have not been modified within [:start].
	if !isAllNaN(wr[:start]) {
		t.Errorf("%v: unexpected modification of wr", prefix)
	}
	if !isAllNaN(wi[:start]) {
		t.Errorf("%v: unexpected modification of wi", prefix)
	}

	var hasReal bool
	for i := start; i <= ihi; {
		if wi[i] == 0 { // Real eigenvalue.
			hasReal = true
			// Check that the eigenvalue corresponds to a 1×1 block
			// on the diagonal of H.
			if wantt && wr[i] != h.Data[i*h.Stride+i] {
				t.Errorf("%v: wr[%v] != H[%v,%v]", prefix, i, i, i)
			}
			i++
			continue
		}

		// Complex eigenvalue.

		// In the conjugate pair the real parts must be equal.
		if wr[i] != wr[i+1] {
			t.Errorf("%v: real part of conjugate pair not equal, i=%v", prefix, i)
		}
		// The first imaginary part must be positive.
		if wi[i] < 0 {
			t.Errorf("%v: wi[%v] not positive", prefix, i)
		}
		// The second imaginary part must be negative with the same
		// magnitude.
		if wi[i] != -wi[i+1] {
			t.Errorf("%v: wi[%v] != wi[%v]", prefix, i, i+1)
		}
		if wantt {
			// Check that wi[i] has the correct value.
			if wr[i] != h.Data[i*h.Stride+i] {
				t.Errorf("%v: wr[%v] != H[%v,%v]", prefix, i, i, i)
			}
			if wr[i] != h.Data[(i+1)*h.Stride+i+1] {
				t.Errorf("%v: wr[%v] != H[%v,%v]", prefix, i, i+1, i+1)
			}
			im := math.Sqrt(math.Abs(h.Data[(i+1)*h.Stride+i])) * math.Sqrt(math.Abs(h.Data[i*h.Stride+i+1]))
			if math.Abs(im-wi[i]) > tol {
				t.Errorf("%v: unexpected value of wi[%v]: want %v, got %v", prefix, i, im, wi[i])
			}
		}
		i += 2
	}
	// If the number of found eigenvalues is odd, at least one must be real.
	if (ihi+1-start)%2 != 0 && !hasReal {
		t.Errorf("%v: expected at least one real eigenvalue", prefix)
	}

	// Compare found eigenvalues to the reference, if known.
	if test.evWant != nil {
		for i := start; i <= ihi; i++ {
			ev := complex(wr[i], wi[i])
			found, _ := containsComplex(test.evWant, ev, tol)
			if !found {
				t.Errorf("%v: unexpected eigenvalue %v", prefix, ev)
			}
		}
	}

	if !wantz {
		return
	}

	// Z should contain the orthogonal matrix U.
	if !isOrthonormal(z) {
		t.Errorf("%v: Z is not orthogonal", prefix)
	}
	// Z should have been modified only in the
	// [iloz:ihiz+1,ilo:ihi+1] block.
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if iloz <= i && i <= ihiz && ilo <= j && j <= ihi {
				continue
			}
			if z.Data[i*z.Stride+j] != zCopy.Data[i*zCopy.Stride+j] {
				t.Errorf("%v: Z modified outside of [iloz:ihiz+1,ilo:ihi+1] block", prefix)
			}
		}
	}
	if wantt {
		// Zero out h under the subdiagonal because Dlaqr04 uses it as
		// workspace.
		for i := 2; i < n; i++ {
			for j := 0; j < i-1; j++ {
				h.Data[i*h.Stride+j] = 0
			}
		}
		hz := eye(n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, test.h, z, 0, hz)
		zhz := eye(n, n)
		blas64.Gemm(blas.Trans, blas.NoTrans, 1, z, hz, 0, zhz)
		if !equalApproxGeneral(zhz, h, 10*tol) {
			t.Errorf("%v: Z^T*(initial H)*Z and (final H) are not equal", prefix)
		}
	}
}
