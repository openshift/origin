// Copyright ©2017 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"math/rand"
	"testing"

	"github.com/gonum/blas/blas64"
)

func TestDlagsy(t *testing.T) {
	const tol = 1e-14
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, lda := range []int{0, 2*n + 1} {
			if lda == 0 {
				lda = max(1, n)
			}
			d := make([]float64, n)
			for i := range d {
				d[i] = 1
			}
			a := blas64.General{
				Rows:   n,
				Cols:   n,
				Stride: lda,
				Data:   nanSlice(n * lda),
			}
			work := make([]float64, a.Rows+a.Cols)

			Dlagsy(a.Rows, 0, d, a.Data, a.Stride, rnd, work)

			isIdentity := true
		identityLoop:
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					aij := a.Data[i*a.Stride+j]
					if math.IsNaN(aij) {
						isIdentity = false
					}
					if i == j && math.Abs(aij-1) > tol {
						isIdentity = false
					}
					if i != j && math.Abs(aij) > tol {
						isIdentity = false
					}
					if !isIdentity {
						break identityLoop
					}
				}
			}
			if !isIdentity {
				t.Errorf("Case n=%v,lda=%v: unexpected result", n, lda)
			}
		}
	}
}

func TestDlagge(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10, 50} {
		for _, lda := range []int{0, 2*n + 1} {
			if lda == 0 {
				lda = max(1, n)
			}
			d := make([]float64, n)
			for i := range d {
				d[i] = 1
			}
			a := blas64.General{
				Rows:   n,
				Cols:   n,
				Stride: lda,
				Data:   nanSlice(n * lda),
			}
			work := make([]float64, a.Rows+a.Cols)

			Dlagge(a.Rows, a.Cols, 0, 0, d, a.Data, a.Stride, rnd, work)

			if !isOrthonormal(a) {
				t.Errorf("Case n=%v,lda=%v: unexpected result", n, lda)
			}
		}
	}

}
