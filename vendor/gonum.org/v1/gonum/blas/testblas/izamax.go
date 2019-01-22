// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"golang.org/x/exp/rand"
)

type Izamaxer interface {
	Izamax(n int, x []complex128, incX int) int
}

func IzamaxTest(t *testing.T, impl Izamaxer) {
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
				re := 2*rnd.Float64() - 1
				im := 2*rnd.Float64() - 1
				x[i*aincX] = complex(re, im)
			}

			want := -1
			if incX > 0 && n > 0 {
				want = rnd.Intn(n)
				x[want*incX] = 10 + 10i
			}
			got := impl.Izamax(n, x, incX)

			if got != want {
				t.Errorf("Case n=%v,incX=%v: unexpected result. want %v, got %v", n, incX, want, got)
			}
		}
	}
}
