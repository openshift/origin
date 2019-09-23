// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
)

type Dlacn2er interface {
	Dlacn2(n int, v, x []float64, isgn []int, est float64, kase int, isave *[3]int) (float64, int)
}

func Dlacn2Test(t *testing.T, impl Dlacn2er) {
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{1, 2, 3, 4, 5, 7, 10, 15, 20, 100} {
		for cas := 0; cas < 10; cas++ {
			a := randomGeneral(n, n, n, rnd)

			// Compute the 1-norm of A explicitly.
			var norm1 float64
			for j := 0; j < n; j++ {
				var sum float64
				for i := 0; i < n; i++ {
					sum += math.Abs(a.Data[i*a.Stride+j])
				}
				if sum > norm1 {
					norm1 = sum
				}
			}

			// Compute the estimate of 1-norm using Dlanc2.
			x := make([]float64, n)
			work := make([]float64, n)
			v := make([]float64, n)
			isgn := make([]int, n)
			var (
				kase  int
				isave [3]int
				got   float64
			)
		loop:
			for {
				got, kase = impl.Dlacn2(n, v, x, isgn, got, kase, &isave)
				switch kase {
				default:
					panic("Dlacn2 returned invalid value of kase")
				case 0:
					break loop
				case 1:
					blas64.Gemv(blas.NoTrans, 1, a, blas64.Vector{Data: x, Inc: 1}, 0, blas64.Vector{Data: work, Inc: 1})
					copy(x, work)
				case 2:
					blas64.Gemv(blas.Trans, 1, a, blas64.Vector{Data: x, Inc: 1}, 0, blas64.Vector{Data: work, Inc: 1})
					copy(x, work)
				}
			}

			// Check that got is either accurate enough or a
			// lower estimate of the 1-norm of A.
			if math.Abs(got-norm1) > 1e-8 && got > norm1 {
				t.Errorf("Case n=%v: not lower estimate. 1-norm %v, estimate %v", n, norm1, got)
			}
		}
	}
}
