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

type Dorgqler interface {
	Dorgql(m, n, k int, a []float64, lda int, tau, work []float64, lwork int)

	Dlarfger
}

func DorgqlTest(t *testing.T, impl Dorgqler) {
	const tol = 1e-14

	type Dorg2ler interface {
		Dorg2l(m, n, k int, a []float64, lda int, tau, work []float64)
	}
	dorg2ler, hasDorg2l := impl.(Dorg2ler)

	rnd := rand.New(rand.NewSource(1))
	for _, m := range []int{0, 1, 2, 3, 4, 5, 7, 10, 15, 30, 50, 150} {
		for _, extra := range []int{0, 11} {
			for _, wl := range []worklen{minimumWork, mediumWork, optimumWork} {
				var k int
				if m >= 129 {
					// For large matrices make sure that k
					// is large enough to trigger blocked
					// path.
					k = 129 + rnd.Intn(m-129+1)
				} else {
					k = rnd.Intn(m + 1)
				}
				n := k + rnd.Intn(m-k+1)
				if m == 0 || n == 0 {
					m = 0
					n = 0
					k = 0
				}

				// Generate k elementary reflectors in the last
				// k columns of A.
				a := nanGeneral(m, n, n+extra)
				tau := make([]float64, k)
				for l := 0; l < k; l++ {
					jj := m - k + l
					v := randomSlice(jj, rnd)
					_, tau[l] = impl.Dlarfg(len(v)+1, rnd.NormFloat64(), v, 1)
					j := n - k + l
					for i := 0; i < jj; i++ {
						a.Data[i*a.Stride+j] = v[i]
					}
				}
				aCopy := cloneGeneral(a)

				// Compute the full matrix Q by forming the
				// Householder reflectors explicitly.
				q := eye(m, m)
				qCopy := eye(m, m)
				for l := 0; l < k; l++ {
					h := eye(m, m)
					jj := m - k + l
					j := n - k + l
					v := blas64.Vector{1, make([]float64, m)}
					for i := 0; i < jj; i++ {
						v.Data[i] = a.Data[i*a.Stride+j]
					}
					v.Data[jj] = 1
					blas64.Ger(-tau[l], v, v, h)
					copy(qCopy.Data, q.Data)
					blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, h, qCopy, 0, q)
				}
				// View the last n columns of Q as 'want'.
				want := blas64.General{
					Rows:   m,
					Cols:   n,
					Stride: q.Stride,
					Data:   q.Data[m-n:],
				}

				var lwork int
				switch wl {
				case minimumWork:
					lwork = max(1, n)
				case mediumWork:
					work := make([]float64, 1)
					impl.Dorgql(m, n, k, nil, a.Stride, nil, work, -1)
					lwork = (int(work[0]) + n) / 2
					lwork = max(1, lwork)
				case optimumWork:
					work := make([]float64, 1)
					impl.Dorgql(m, n, k, nil, a.Stride, nil, work, -1)
					lwork = int(work[0])
				}
				work := make([]float64, lwork)

				// Compute the last n columns of Q by a call to
				// Dorgql.
				impl.Dorgql(m, n, k, a.Data, a.Stride, tau, work, len(work))

				prefix := fmt.Sprintf("Case m=%v,n=%v,k=%v,wl=%v", m, n, k, wl)
				if !generalOutsideAllNaN(a) {
					t.Errorf("%v: out-of-range write to A", prefix)
				}
				if !equalApproxGeneral(want, a, tol) {
					t.Errorf("%v: unexpected Q", prefix)
				}

				// Compute the last n columns of Q by a call to
				// Dorg2l and check that we get the same result.
				if !hasDorg2l {
					continue
				}
				dorg2ler.Dorg2l(m, n, k, aCopy.Data, aCopy.Stride, tau, work)
				if !equalApproxGeneral(aCopy, a, tol) {
					t.Errorf("%v: mismatch between Dorgql and Dorg2l", prefix)
				}
			}
		}
	}
}
