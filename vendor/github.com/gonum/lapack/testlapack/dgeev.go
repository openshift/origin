// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"math/cmplx"
	"math/rand"
	"strconv"
	"testing"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/floats"
	"github.com/gonum/lapack"
)

type Dgeever interface {
	Dgeev(jobvl lapack.LeftEVJob, jobvr lapack.RightEVJob, n int, a []float64, lda int,
		wr, wi []float64, vl []float64, ldvl int, vr []float64, ldvr int, work []float64, lwork int) int
}

type dgeevTest struct {
	a      blas64.General
	evWant []complex128 // If nil, the eigenvalues are not known.
	valTol float64      // Tolerance for eigenvalue checks.
	vecTol float64      // Tolerance for eigenvector checks.
}

func DgeevTest(t *testing.T, impl Dgeever) {
	rnd := rand.New(rand.NewSource(1))

	for i, test := range []dgeevTest{
		{
			a:      A123{}.Matrix(),
			evWant: A123{}.Eigenvalues(),
		},

		dgeevTestForAntisymRandom(10, rnd),
		dgeevTestForAntisymRandom(11, rnd),
		dgeevTestForAntisymRandom(50, rnd),
		dgeevTestForAntisymRandom(51, rnd),
		dgeevTestForAntisymRandom(100, rnd),
		dgeevTestForAntisymRandom(101, rnd),

		{
			a:      Circulant(2).Matrix(),
			evWant: Circulant(2).Eigenvalues(),
		},
		{
			a:      Circulant(3).Matrix(),
			evWant: Circulant(3).Eigenvalues(),
		},
		{
			a:      Circulant(4).Matrix(),
			evWant: Circulant(4).Eigenvalues(),
		},
		{
			a:      Circulant(5).Matrix(),
			evWant: Circulant(5).Eigenvalues(),
		},
		{
			a:      Circulant(10).Matrix(),
			evWant: Circulant(10).Eigenvalues(),
		},
		{
			a:      Circulant(15).Matrix(),
			evWant: Circulant(15).Eigenvalues(),
			valTol: 1e-12,
		},
		{
			a:      Circulant(30).Matrix(),
			evWant: Circulant(30).Eigenvalues(),
			valTol: 1e-11,
			vecTol: 1e-12,
		},
		{
			a:      Circulant(50).Matrix(),
			evWant: Circulant(50).Eigenvalues(),
			valTol: 1e-11,
			vecTol: 1e-12,
		},
		{
			a:      Circulant(101).Matrix(),
			evWant: Circulant(101).Eigenvalues(),
			valTol: 1e-10,
			vecTol: 1e-11,
		},
		{
			a:      Circulant(150).Matrix(),
			evWant: Circulant(150).Eigenvalues(),
			valTol: 1e-9,
			vecTol: 1e-10,
		},

		{
			a:      Clement(2).Matrix(),
			evWant: Clement(2).Eigenvalues(),
		},
		{
			a:      Clement(3).Matrix(),
			evWant: Clement(3).Eigenvalues(),
		},
		{
			a:      Clement(4).Matrix(),
			evWant: Clement(4).Eigenvalues(),
		},
		{
			a:      Clement(5).Matrix(),
			evWant: Clement(5).Eigenvalues(),
		},
		{
			a:      Clement(10).Matrix(),
			evWant: Clement(10).Eigenvalues(),
		},
		{
			a:      Clement(15).Matrix(),
			evWant: Clement(15).Eigenvalues(),
		},
		{
			a:      Clement(30).Matrix(),
			evWant: Clement(30).Eigenvalues(),
			valTol: 1e-11,
		},
		{
			a:      Clement(50).Matrix(),
			evWant: Clement(50).Eigenvalues(),
			valTol: 1e-7,
			vecTol: 1e-11,
		},

		{
			a:      Creation(2).Matrix(),
			evWant: Creation(2).Eigenvalues(),
		},
		{
			a:      Creation(3).Matrix(),
			evWant: Creation(3).Eigenvalues(),
		},
		{
			a:      Creation(4).Matrix(),
			evWant: Creation(4).Eigenvalues(),
		},
		{
			a:      Creation(5).Matrix(),
			evWant: Creation(5).Eigenvalues(),
		},
		{
			a:      Creation(10).Matrix(),
			evWant: Creation(10).Eigenvalues(),
		},
		{
			a:      Creation(15).Matrix(),
			evWant: Creation(15).Eigenvalues(),
		},
		{
			a:      Creation(30).Matrix(),
			evWant: Creation(30).Eigenvalues(),
		},
		{
			a:      Creation(50).Matrix(),
			evWant: Creation(50).Eigenvalues(),
		},
		{
			a:      Creation(101).Matrix(),
			evWant: Creation(101).Eigenvalues(),
		},
		{
			a:      Creation(150).Matrix(),
			evWant: Creation(150).Eigenvalues(),
		},

		{
			a:      Diagonal(0).Matrix(),
			evWant: Diagonal(0).Eigenvalues(),
		},
		{
			a:      Diagonal(10).Matrix(),
			evWant: Diagonal(10).Eigenvalues(),
		},
		{
			a:      Diagonal(50).Matrix(),
			evWant: Diagonal(50).Eigenvalues(),
		},
		{
			a:      Diagonal(151).Matrix(),
			evWant: Diagonal(151).Eigenvalues(),
		},

		{
			a:      Downshift(2).Matrix(),
			evWant: Downshift(2).Eigenvalues(),
		},
		{
			a:      Downshift(3).Matrix(),
			evWant: Downshift(3).Eigenvalues(),
		},
		{
			a:      Downshift(4).Matrix(),
			evWant: Downshift(4).Eigenvalues(),
		},
		{
			a:      Downshift(5).Matrix(),
			evWant: Downshift(5).Eigenvalues(),
		},
		{
			a:      Downshift(10).Matrix(),
			evWant: Downshift(10).Eigenvalues(),
		},
		{
			a:      Downshift(15).Matrix(),
			evWant: Downshift(15).Eigenvalues(),
		},
		{
			a:      Downshift(30).Matrix(),
			evWant: Downshift(30).Eigenvalues(),
		},
		{
			a:      Downshift(50).Matrix(),
			evWant: Downshift(50).Eigenvalues(),
		},
		{
			a:      Downshift(101).Matrix(),
			evWant: Downshift(101).Eigenvalues(),
		},
		{
			a:      Downshift(150).Matrix(),
			evWant: Downshift(150).Eigenvalues(),
		},

		{
			a:      Fibonacci(2).Matrix(),
			evWant: Fibonacci(2).Eigenvalues(),
		},
		{
			a:      Fibonacci(3).Matrix(),
			evWant: Fibonacci(3).Eigenvalues(),
		},
		{
			a:      Fibonacci(4).Matrix(),
			evWant: Fibonacci(4).Eigenvalues(),
		},
		{
			a:      Fibonacci(5).Matrix(),
			evWant: Fibonacci(5).Eigenvalues(),
		},
		{
			a:      Fibonacci(10).Matrix(),
			evWant: Fibonacci(10).Eigenvalues(),
		},
		{
			a:      Fibonacci(15).Matrix(),
			evWant: Fibonacci(15).Eigenvalues(),
		},
		{
			a:      Fibonacci(30).Matrix(),
			evWant: Fibonacci(30).Eigenvalues(),
		},
		{
			a:      Fibonacci(50).Matrix(),
			evWant: Fibonacci(50).Eigenvalues(),
		},
		{
			a:      Fibonacci(101).Matrix(),
			evWant: Fibonacci(101).Eigenvalues(),
		},
		{
			a:      Fibonacci(150).Matrix(),
			evWant: Fibonacci(150).Eigenvalues(),
		},

		{
			a:      Gear(2).Matrix(),
			evWant: Gear(2).Eigenvalues(),
		},
		{
			a:      Gear(3).Matrix(),
			evWant: Gear(3).Eigenvalues(),
		},
		{
			a:      Gear(4).Matrix(),
			evWant: Gear(4).Eigenvalues(),
			valTol: 1e-7,
		},
		{
			a:      Gear(5).Matrix(),
			evWant: Gear(5).Eigenvalues(),
		},
		{
			a:      Gear(10).Matrix(),
			evWant: Gear(10).Eigenvalues(),
			valTol: 1e-8,
		},
		{
			a:      Gear(15).Matrix(),
			evWant: Gear(15).Eigenvalues(),
		},
		{
			a:      Gear(30).Matrix(),
			evWant: Gear(30).Eigenvalues(),
			valTol: 1e-8,
		},
		{
			a:      Gear(50).Matrix(),
			evWant: Gear(50).Eigenvalues(),
			valTol: 1e-8,
		},
		{
			a:      Gear(101).Matrix(),
			evWant: Gear(101).Eigenvalues(),
		},
		{
			a:      Gear(150).Matrix(),
			evWant: Gear(150).Eigenvalues(),
			valTol: 1e-8,
		},

		{
			a:      Grcar{N: 10, K: 3}.Matrix(),
			evWant: Grcar{N: 10, K: 3}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 10, K: 7}.Matrix(),
			evWant: Grcar{N: 10, K: 7}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 11, K: 7}.Matrix(),
			evWant: Grcar{N: 11, K: 7}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 50, K: 3}.Matrix(),
			evWant: Grcar{N: 50, K: 3}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 51, K: 3}.Matrix(),
			evWant: Grcar{N: 51, K: 3}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 50, K: 10}.Matrix(),
			evWant: Grcar{N: 50, K: 10}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 51, K: 10}.Matrix(),
			evWant: Grcar{N: 51, K: 10}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 50, K: 30}.Matrix(),
			evWant: Grcar{N: 50, K: 30}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 150, K: 2}.Matrix(),
			evWant: Grcar{N: 150, K: 2}.Eigenvalues(),
		},
		{
			a:      Grcar{N: 150, K: 148}.Matrix(),
			evWant: Grcar{N: 150, K: 148}.Eigenvalues(),
		},

		{
			a:      Hanowa{N: 6, Alpha: 17}.Matrix(),
			evWant: Hanowa{N: 6, Alpha: 17}.Eigenvalues(),
		},
		{
			a:      Hanowa{N: 50, Alpha: -1}.Matrix(),
			evWant: Hanowa{N: 50, Alpha: -1}.Eigenvalues(),
		},
		{
			a:      Hanowa{N: 100, Alpha: -1}.Matrix(),
			evWant: Hanowa{N: 100, Alpha: -1}.Eigenvalues(),
		},

		{
			a:      Lesp(2).Matrix(),
			evWant: Lesp(2).Eigenvalues(),
		},
		{
			a:      Lesp(3).Matrix(),
			evWant: Lesp(3).Eigenvalues(),
		},
		{
			a:      Lesp(4).Matrix(),
			evWant: Lesp(4).Eigenvalues(),
		},
		{
			a:      Lesp(5).Matrix(),
			evWant: Lesp(5).Eigenvalues(),
		},
		{
			a:      Lesp(10).Matrix(),
			evWant: Lesp(10).Eigenvalues(),
		},
		{
			a:      Lesp(15).Matrix(),
			evWant: Lesp(15).Eigenvalues(),
		},
		{
			a:      Lesp(30).Matrix(),
			evWant: Lesp(30).Eigenvalues(),
		},
		{
			a:      Lesp(50).Matrix(),
			evWant: Lesp(50).Eigenvalues(),
			valTol: 1e-12,
			vecTol: 1e-12,
		},
		{
			a:      Lesp(101).Matrix(),
			evWant: Lesp(101).Eigenvalues(),
			valTol: 1e-12,
			vecTol: 1e-12,
		},
		{
			a:      Lesp(150).Matrix(),
			evWant: Lesp(150).Eigenvalues(),
			valTol: 1e-12,
			vecTol: 1e-12,
		},

		{
			a:      Rutis{}.Matrix(),
			evWant: Rutis{}.Eigenvalues(),
		},

		{
			a:      Tris{N: 74, X: 1, Y: -2, Z: 1}.Matrix(),
			evWant: Tris{N: 74, X: 1, Y: -2, Z: 1}.Eigenvalues(),
		},
		{
			a:      Tris{N: 74, X: 1, Y: 2, Z: -3}.Matrix(),
			evWant: Tris{N: 74, X: 1, Y: 2, Z: -3}.Eigenvalues(),
		},
		{
			a:      Tris{N: 75, X: 1, Y: 2, Z: -3}.Matrix(),
			evWant: Tris{N: 75, X: 1, Y: 2, Z: -3}.Eigenvalues(),
		},

		{
			a:      Wilk4{}.Matrix(),
			evWant: Wilk4{}.Eigenvalues(),
		},
		{
			a:      Wilk12{}.Matrix(),
			evWant: Wilk12{}.Eigenvalues(),
			valTol: 1e-8,
		},
		{
			a:      Wilk20(0).Matrix(),
			evWant: Wilk20(0).Eigenvalues(),
		},
		{
			a:      Wilk20(1e-10).Matrix(),
			evWant: Wilk20(1e-10).Eigenvalues(),
			valTol: 1e-12,
			vecTol: 1e-12,
		},

		{
			a:      Zero(1).Matrix(),
			evWant: Zero(1).Eigenvalues(),
		},
		{
			a:      Zero(10).Matrix(),
			evWant: Zero(10).Eigenvalues(),
		},
		{
			a:      Zero(50).Matrix(),
			evWant: Zero(50).Eigenvalues(),
		},
		{
			a:      Zero(100).Matrix(),
			evWant: Zero(100).Eigenvalues(),
		},
	} {
		for _, jobvl := range []lapack.LeftEVJob{lapack.ComputeLeftEV, lapack.None} {
			for _, jobvr := range []lapack.RightEVJob{lapack.ComputeRightEV, lapack.None} {
				for _, extra := range []int{0, 11} {
					for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
						testDgeev(t, impl, strconv.Itoa(i), test, jobvl, jobvr, extra, wl)
					}
				}
			}
		}
	}

	for _, n := range []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 20, 50, 51, 100, 101} {
		for _, jobvl := range []lapack.LeftEVJob{lapack.ComputeLeftEV, lapack.None} {
			for _, jobvr := range []lapack.RightEVJob{lapack.ComputeRightEV, lapack.None} {
				for cas := 0; cas < 10; cas++ {
					// Create a block diagonal matrix with
					// random eigenvalues of random multiplicity.
					ev := make([]complex128, n)
					tmat := zeros(n, n, n)
					for i := 0; i < n; {
						re := rnd.NormFloat64()
						if i == n-1 || rnd.Float64() < 0.5 {
							// Real eigenvalue.
							nb := rnd.Intn(min(4, n-i)) + 1
							for k := 0; k < nb; k++ {
								tmat.Data[i*tmat.Stride+i] = re
								ev[i] = complex(re, 0)
								i++
							}
							continue
						}
						// Complex eigenvalue.
						im := rnd.NormFloat64()
						nb := rnd.Intn(min(4, (n-i)/2)) + 1
						for k := 0; k < nb; k++ {
							// 2×2 block for the complex eigenvalue.
							tmat.Data[i*tmat.Stride+i] = re
							tmat.Data[(i+1)*tmat.Stride+i+1] = re
							tmat.Data[(i+1)*tmat.Stride+i] = -im
							tmat.Data[i*tmat.Stride+i+1] = im
							ev[i] = complex(re, im)
							ev[i+1] = complex(re, -im)
							i += 2
						}
					}

					// Compute A = Q T Q^T where Q is an
					// orthogonal matrix.
					q := randomOrthogonal(n, rnd)
					tq := zeros(n, n, n)
					blas64.Gemm(blas.NoTrans, blas.Trans, 1, tmat, q, 0, tq)
					a := zeros(n, n, n)
					blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, q, tq, 0, a)

					test := dgeevTest{
						a:      a,
						evWant: ev,
						valTol: 1e-12,
						vecTol: 1e-8,
					}
					testDgeev(t, impl, "random", test, jobvl, jobvr, 0, optimumWork)
				}
			}
		}
	}
}

func testDgeev(t *testing.T, impl Dgeever, tc string, test dgeevTest, jobvl lapack.LeftEVJob, jobvr lapack.RightEVJob, extra int, wl worklen) {
	const defaultTol = 1e-13
	valTol := test.valTol
	if valTol == 0 {
		valTol = defaultTol
	}
	vecTol := test.vecTol
	if vecTol == 0 {
		vecTol = defaultTol
	}

	a := cloneGeneral(test.a)
	n := a.Rows

	var vl blas64.General
	if jobvl == lapack.ComputeLeftEV {
		vl = nanGeneral(n, n, n)
	}

	var vr blas64.General
	if jobvr == lapack.ComputeRightEV {
		vr = nanGeneral(n, n, n)
	}

	wr := make([]float64, n)
	wi := make([]float64, n)

	var lwork int
	switch wl {
	case minimumWork:
		if jobvl == lapack.ComputeLeftEV || jobvr == lapack.ComputeRightEV {
			lwork = max(1, 4*n)
		} else {
			lwork = max(1, 3*n)
		}
	case mediumWork:
		work := make([]float64, 1)
		impl.Dgeev(jobvl, jobvr, n, nil, 1, nil, nil, nil, 1, nil, 1, work, -1)
		if jobvl == lapack.ComputeLeftEV || jobvr == lapack.ComputeRightEV {
			lwork = (int(work[0]) + 4*n) / 2
		} else {
			lwork = (int(work[0]) + 3*n) / 2
		}
		lwork = max(1, lwork)
	case optimumWork:
		work := make([]float64, 1)
		impl.Dgeev(jobvl, jobvr, n, nil, 1, nil, nil, nil, 1, nil, 1, work, -1)
		lwork = int(work[0])
	}
	work := make([]float64, lwork)

	first := impl.Dgeev(jobvl, jobvr, n, a.Data, a.Stride, wr, wi,
		vl.Data, vl.Stride, vr.Data, vr.Stride, work, len(work))

	prefix := fmt.Sprintf("Case #%v: n=%v, jobvl=%v, jobvr=%v, extra=%v, work=%v",
		tc, n, jobvl, jobvr, extra, wl)

	if !generalOutsideAllNaN(vl) {
		t.Errorf("%v: out-of-range write to VL", prefix)
	}
	if !generalOutsideAllNaN(vr) {
		t.Errorf("%v: out-of-range write to VR", prefix)
	}

	if first > 0 {
		t.Log("%v: all eigenvalues haven't been computed, first=%v", prefix, first)
	}

	// Check that conjugate pair eigevalues are ordered correctly.
	for i := first; i < n; {
		if wi[i] == 0 {
			i++
			continue
		}
		if wr[i] != wr[i+1] {
			t.Errorf("%v: real parts of %vth conjugate pair not equal", prefix, i)
		}
		if wi[i] < 0 || wi[i+1] > 0 {
			t.Errorf("%v: unexpected ordering of %vth conjugate pair", prefix, i)
		}
		i += 2
	}

	// Check the computed eigenvalues against provided known eigenvalues.
	if test.evWant != nil {
		used := make([]bool, n)
		for i := first; i < n; i++ {
			evGot := complex(wr[i], wi[i])
			idx := -1
			for k, evWant := range test.evWant {
				if !used[k] && cmplx.Abs(evWant-evGot) < valTol {
					idx = k
					used[k] = true
					break
				}
			}
			if idx == -1 {
				t.Errorf("%v: unexpected eigenvalue %v", prefix, evGot)
			}
		}
	}

	if first > 0 || (jobvl == lapack.None && jobvr == lapack.None) {
		// No eigenvectors have been computed.
		return
	}

	// Check that the columns of VL and VR are eigenvectors that correspond
	// to the computed eigenvalues.
	for k := 0; k < n; {
		if wi[k] == 0 {
			if jobvl == lapack.ComputeLeftEV {
				ev := columnOf(vl, k)
				if !isLeftEigenvectorOf(test.a, ev, nil, complex(wr[k], 0), vecTol) {
					t.Errorf("%v: VL[:,%v] is not left real eigenvector",
						prefix, k)
				}

				norm := floats.Norm(ev, 2)
				if math.Abs(norm-1) >= defaultTol {
					t.Errorf("%v: norm of left real eigenvector %v not equal to 1: got %v",
						prefix, k, norm)
				}
			}
			if jobvr == lapack.ComputeRightEV {
				ev := columnOf(vr, k)
				if !isRightEigenvectorOf(test.a, ev, nil, complex(wr[k], 0), vecTol) {
					t.Errorf("%v: VR[:,%v] is not right real eigenvector",
						prefix, k)
				}

				norm := floats.Norm(ev, 2)
				if math.Abs(norm-1) >= defaultTol {
					t.Errorf("%v: norm of right real eigenvector %v not equal to 1: got %v",
						prefix, k, norm)
				}
			}
			k++
		} else {
			if jobvl == lapack.ComputeLeftEV {
				evre := columnOf(vl, k)
				evim := columnOf(vl, k+1)
				if !isLeftEigenvectorOf(test.a, evre, evim, complex(wr[k], wi[k]), vecTol) {
					t.Errorf("%v: VL[:,%v:%v] is not left complex eigenvector",
						prefix, k, k+1)
				}
				floats.Scale(-1, evim)
				if !isLeftEigenvectorOf(test.a, evre, evim, complex(wr[k+1], wi[k+1]), vecTol) {
					t.Errorf("%v: VL[:,%v:%v] is not left complex eigenvector",
						prefix, k, k+1)
				}

				norm := math.Hypot(floats.Norm(evre, 2), floats.Norm(evim, 2))
				if math.Abs(norm-1) > defaultTol {
					t.Errorf("%v: norm of left complex eigenvector %v not equal to 1: got %v",
						prefix, k, norm)
				}
			}
			if jobvr == lapack.ComputeRightEV {
				evre := columnOf(vr, k)
				evim := columnOf(vr, k+1)
				if !isRightEigenvectorOf(test.a, evre, evim, complex(wr[k], wi[k]), vecTol) {
					t.Errorf("%v: VR[:,%v:%v] is not right complex eigenvector",
						prefix, k, k+1)
				}
				floats.Scale(-1, evim)
				if !isRightEigenvectorOf(test.a, evre, evim, complex(wr[k+1], wi[k+1]), vecTol) {
					t.Errorf("%v: VR[:,%v:%v] is not right complex eigenvector",
						prefix, k, k+1)
				}

				norm := math.Hypot(floats.Norm(evre, 2), floats.Norm(evim, 2))
				if math.Abs(norm-1) > defaultTol {
					t.Errorf("%v: norm of right complex eigenvector %v not equal to 1: got %v",
						prefix, k, norm)
				}
			}
			// We don't test whether the largest component is real
			// because checking it is flaky due to rounding errors.

			k += 2
		}
	}
}

func dgeevTestForAntisymRandom(n int, rnd *rand.Rand) dgeevTest {
	a := NewAntisymRandom(n, rnd)
	return dgeevTest{
		a:      a.Matrix(),
		evWant: a.Eigenvalues(),
	}
}
