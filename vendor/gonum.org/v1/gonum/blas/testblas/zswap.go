// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"
)

type Zswaper interface {
	Zswap(n int, x []complex128, incX int, y []complex128, incY int)
}

func ZswapTest(t *testing.T, impl Zswaper) {
	rnd := rand.New(rand.NewSource(1))
	for n := 0; n < 20; n++ {
		for _, inc := range allPairs([]int{-5, -1, 1, 2, 5, 10}, []int{-3, -1, 1, 3, 7, 12}) {
			incX := inc[0]
			incY := inc[1]
			aincX := abs(incX)
			aincY := abs(incY)

			var x, y []complex128
			if n > 0 {
				x = make([]complex128, (n-1)*aincX+1)
				y = make([]complex128, (n-1)*aincY+1)
			}
			for i := range x {
				x[i] = znan
			}
			for i := range y {
				y[i] = znan
			}
			for i := 0; i < n; i++ {
				x[i*aincX] = complex(rnd.NormFloat64(), rnd.NormFloat64())
				y[i*aincY] = complex(rnd.NormFloat64(), rnd.NormFloat64())
			}

			xWant := make([]complex128, len(x))
			for i := range xWant {
				xWant[i] = znan
			}
			yWant := make([]complex128, len(y))
			for i := range yWant {
				yWant[i] = znan
			}
			if incX*incY > 0 {
				for i := 0; i < n; i++ {
					xWant[i*aincX] = y[i*aincY]
					yWant[i*aincY] = x[i*aincX]
				}
			} else {
				for i := 0; i < n; i++ {
					xWant[(n-i-1)*aincX] = y[i*aincY]
					yWant[(n-i-1)*aincY] = x[i*aincX]
				}
			}

			impl.Zswap(n, x, incX, y, incY)

			prefix := fmt.Sprintf("Case n=%v,incX=%v,incY=%v:", n, incX, incY)
			if !zsame(x, xWant) {
				t.Errorf("%v: unexpected x:\nwant %v\ngot %v", prefix, xWant, x)
			}
			if !zsame(y, yWant) {
				t.Errorf("%v: unexpected y:\nwant %v\ngot %v", prefix, yWant, y)
			}
		}
	}
}
