// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dzasumer interface {
	Dzasum(n int, x []complex128, incX int) float64
}

func DzasumTest(t *testing.T, impl Dzasumer) {
	const tol = 1e-14
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 50, 100} {
		for _, incX := range []int{-5, 1, 2, 10} {
			aincX := abs(incX)
			var x []complex128
			if n > 0 {
				x = make([]complex128, (n-1)*aincX+1)
			}
			for i := range x {
				x[i] = znan
			}
			for i := 0; i < n; i++ {
				re := float64(2*i + 1)
				if rnd.Intn(2) == 0 {
					re *= -1
				}
				im := float64(2 * (i + 1))
				if rnd.Intn(2) == 0 {
					im *= -1
				}
				x[i*aincX] = complex(re, im)
			}

			want := float64(n * (2*n + 1))
			got := impl.Dzasum(n, x, incX)

			if incX < 0 {
				if got != 0 {
					t.Errorf("Case n=%v,incX=%v: non-zero result when incX < 0. got %v", n, incX, got)
				}
				continue
			}
			if !floats.EqualWithinAbsOrRel(got, want, tol, tol) {
				t.Errorf("Case n=%v,incX=%v: unexpected result. want %v, got %v", n, incX, want, got)
			}
		}
	}
}
