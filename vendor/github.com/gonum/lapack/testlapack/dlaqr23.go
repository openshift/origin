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
)

type Dlaqr23er interface {
	Dlaqr23(wantt, wantz bool, n, ktop, kbot, nw int, h []float64, ldh int, iloz, ihiz int, z []float64, ldz int, sr, si []float64, v []float64, ldv int, nh int, t []float64, ldt int, nv int, wv []float64, ldwv int, work []float64, lwork int, recur int) (ns, nd int)
}

type dlaqr23Test struct {
	wantt, wantz bool
	ktop, kbot   int
	nw           int
	h            blas64.General
	iloz, ihiz   int

	evWant []complex128 // Optional slice with known eigenvalues.
}

func Dlaqr23Test(t *testing.T, impl Dlaqr23er) {
	rnd := rand.New(rand.NewSource(1))

	for _, wantt := range []bool{true, false} {
		for _, wantz := range []bool{true, false} {
			for _, n := range []int{1, 2, 3, 4, 5, 6, 10, 18, 31, 100} {
				for _, extra := range []int{0, 11} {
					for cas := 0; cas < 30; cas++ {
						var nw int
						if nw <= 75 {
							nw = rnd.Intn(n) + 1
						} else {
							nw = 76 + rnd.Intn(n-75)
						}
						ktop := rnd.Intn(n - nw + 1)
						kbot := ktop + nw - 1
						kbot += rnd.Intn(n - kbot)
						h := randomHessenberg(n, n+extra, rnd)
						if ktop-1 >= 0 {
							h.Data[ktop*h.Stride+ktop-1] = 0
						}
						if kbot+1 < n {
							h.Data[(kbot+1)*h.Stride+kbot] = 0
						}
						iloz := rnd.Intn(ktop + 1)
						ihiz := kbot + rnd.Intn(n-kbot)
						test := dlaqr23Test{
							wantt: wantt,
							wantz: wantz,
							ktop:  ktop,
							kbot:  kbot,
							nw:    nw,
							h:     h,
							iloz:  iloz,
							ihiz:  ihiz,
						}
						testDlaqr23(t, impl, test, false, 1, rnd)
						testDlaqr23(t, impl, test, true, 1, rnd)
						testDlaqr23(t, impl, test, false, 0, rnd)
						testDlaqr23(t, impl, test, true, 0, rnd)
					}
				}
			}
		}
	}

	// Tests with n=0.
	for _, wantt := range []bool{true, false} {
		for _, wantz := range []bool{true, false} {
			for _, extra := range []int{0, 1, 11} {
				test := dlaqr23Test{
					wantt: wantt,
					wantz: wantz,
					h:     randomHessenberg(0, extra, rnd),
					ktop:  0,
					kbot:  -1,
					iloz:  0,
					ihiz:  -1,
					nw:    0,
				}
				testDlaqr23(t, impl, test, true, 1, rnd)
				testDlaqr23(t, impl, test, false, 1, rnd)
				testDlaqr23(t, impl, test, true, 0, rnd)
				testDlaqr23(t, impl, test, false, 0, rnd)
			}
		}
	}

	// Tests with explicit eigenvalues computed by Octave.
	for _, test := range []dlaqr23Test{
		{
			h: blas64.General{
				Rows:   1,
				Cols:   1,
				Stride: 1,
				Data:   []float64{7.09965484086874e-1},
			},
			ktop:   0,
			kbot:   0,
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
			ktop:   0,
			kbot:   1,
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
			ktop:   0,
			kbot:   1,
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
			ktop:   1,
			kbot:   2,
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
			ktop: 0,
			kbot: 1,
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
			ktop: 0,
			kbot: 4,
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
		test.nw = test.kbot - test.ktop + 1
		testDlaqr23(t, impl, test, true, 1, rnd)
		testDlaqr23(t, impl, test, false, 1, rnd)
		testDlaqr23(t, impl, test, true, 0, rnd)
		testDlaqr23(t, impl, test, false, 0, rnd)
	}
}

func testDlaqr23(t *testing.T, impl Dlaqr23er, test dlaqr23Test, opt bool, recur int, rnd *rand.Rand) {
	const tol = 1e-14

	h := cloneGeneral(test.h)
	n := h.Cols
	extra := h.Stride - h.Cols
	wantt := test.wantt
	wantz := test.wantz
	ktop := test.ktop
	kbot := test.kbot
	nw := test.nw
	iloz := test.iloz
	ihiz := test.ihiz

	var z, zCopy blas64.General
	if wantz {
		z = eye(n, n+extra)
		zCopy = cloneGeneral(z)
	}

	sr := nanSlice(kbot + 1)
	si := nanSlice(kbot + 1)

	v := randomGeneral(nw, nw, nw+extra, rnd)
	var nh int
	if nw > 0 {
		nh = nw + rnd.Intn(nw) // nh must be at least nw.
	}
	tmat := randomGeneral(nw, nh, nh+extra, rnd)
	var nv int
	if nw > 0 {
		nv = rnd.Intn(nw) + 1
	}
	wv := randomGeneral(nv, nw, nw+extra, rnd)

	var work []float64
	if opt {
		work = nanSlice(1)
		impl.Dlaqr23(wantt, wantz, n, ktop, kbot, nw, nil, h.Stride, iloz, ihiz, nil, z.Stride,
			nil, nil, nil, v.Stride, tmat.Cols, nil, tmat.Stride, wv.Rows, nil, wv.Stride, work, -1, recur)
		work = nanSlice(int(work[0]))
	} else {
		work = nanSlice(max(1, 2*nw))
	}

	ns, nd := impl.Dlaqr23(wantt, wantz, n, ktop, kbot, nw, h.Data, h.Stride, iloz, ihiz, z.Data, z.Stride,
		sr, si, v.Data, v.Stride, tmat.Cols, tmat.Data, tmat.Stride, wv.Rows, wv.Data, wv.Stride, work, len(work), recur)

	prefix := fmt.Sprintf("Case wantt=%v, wantz=%v, n=%v, ktop=%v, kbot=%v, nw=%v, iloz=%v, ihiz=%v, extra=%v",
		wantt, wantz, n, ktop, kbot, nw, iloz, ihiz, extra)

	if !generalOutsideAllNaN(h) {
		t.Errorf("%v: out-of-range write to H\n%v", prefix, h.Data)
	}
	if !generalOutsideAllNaN(z) {
		t.Errorf("%v: out-of-range write to Z\n%v", prefix, z.Data)
	}
	if !generalOutsideAllNaN(v) {
		t.Errorf("%v: out-of-range write to V\n%v", prefix, v.Data)
	}
	if !generalOutsideAllNaN(tmat) {
		t.Errorf("%v: out-of-range write to T\n%v", prefix, tmat.Data)
	}
	if !generalOutsideAllNaN(wv) {
		t.Errorf("%v: out-of-range write to WV\n%v", prefix, wv.Data)
	}
	if !isAllNaN(sr[:kbot-nd-ns+1]) || !isAllNaN(sr[kbot+1:]) {
		t.Errorf("%v: out-of-range write to sr")
	}
	if !isAllNaN(si[:kbot-nd-ns+1]) || !isAllNaN(si[kbot+1:]) {
		t.Errorf("%v: out-of-range write to si")
	}

	if !isUpperHessenberg(h) {
		t.Errorf("%v: H is not upper Hessenberg", prefix)
	}

	if test.evWant != nil {
		for i := kbot - nd + 1; i <= kbot; i++ {
			ev := complex(sr[i], si[i])
			found, _ := containsComplex(test.evWant, ev, tol)
			if !found {
				t.Errorf("%v: unexpected eigenvalue %v", prefix, ev)
			}
		}
	}

	if !wantz {
		return
	}

	var zmod bool
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if z.Data[i*z.Stride+j] == zCopy.Data[i*zCopy.Stride+j] {
				continue
			}
			if i < iloz || i > ihiz || j < kbot-nw+1 || j > kbot {
				zmod = true
			}
		}
	}
	if zmod {
		t.Errorf("%v: unexpected modification of Z", prefix)
	}
	if !isOrthonormal(z) {
		t.Errorf("%v: Z is not orthogonal", prefix)
	}
	if wantt {
		hu := eye(n, n)
		blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, test.h, z, 0, hu)
		uhu := eye(n, n)
		blas64.Gemm(blas.Trans, blas.NoTrans, 1, z, hu, 0, uhu)
		if !equalApproxGeneral(uhu, h, 10*tol) {
			t.Errorf("%v: Z^T*(initial H)*Z and (final H) are not equal", prefix)
		}
	}
}
