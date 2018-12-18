// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"testing"

	"golang.org/x/exp/rand"
)

type Zcopyer interface {
	Zcopy(n int, x []complex128, incX int, y []complex128, incY int)
}

func ZcopyTest(t *testing.T, impl Zcopyer) {
	rnd := rand.New(rand.NewSource(1))
	for n := 0; n <= 20; n++ {
		for _, inc := range allPairs([]int{-7, -3, 1, 13}, []int{-11, -5, 1, 17}) {
			incX := inc[0]
			incY := inc[1]
			aincX := abs(incX)
			aincY := abs(incY)

			var x []complex128
			if n > 0 {
				x = make([]complex128, (n-1)*aincX+1)
			}
			for i := range x {
				x[i] = znan
			}
			for i := 0; i < n; i++ {
				x[i*aincX] = complex(rnd.NormFloat64(), rnd.NormFloat64())
			}
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			var y []complex128
			if n > 0 {
				y = make([]complex128, (n-1)*aincY+1)
			}
			for i := range y {
				y[i] = znan
			}

			want := make([]complex128, len(y))
			for i := range want {
				want[i] = znan
			}
			if incX*incY > 0 {
				for i := 0; i < n; i++ {
					want[i*aincY] = x[i*aincX]
				}
			} else {
				for i := 0; i < n; i++ {
					want[i*aincY] = x[(n-1-i)*aincX]
				}
			}

			impl.Zcopy(n, x, incX, y, incY)

			prefix := fmt.Sprintf("Case n=%v,incX=%v,incY=%v:", n, incX, incY)

			if !zsame(x, xCopy) {
				t.Errorf("%v: unexpected modification of x", prefix)
			}
			if !zsame(y, want) {
				t.Errorf("%v: unexpected y:\nwant %v\ngot %v", prefix, want, y)
			}
		}
	}
}
