// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c64

import (
	"fmt"
	"testing"
)

var benchSink complex64

func BenchmarkDotUnitary(t *testing.B) {
	for _, tst := range []struct {
		name string
		f    func(x, y []complex64) complex64
	}{
		{"DotcUnitary", DotcUnitary},
		{"DotuUnitary", DotuUnitary},
	} {
		for _, v := range []int64{1, 2, 3, 4, 5, 10, 100, 1e3, 5e3, 1e4, 5e4} {
			t.Run(fmt.Sprintf("%s-%d", tst.name, v), func(b *testing.B) {
				x, y := x[:v], y[:v]
				b.SetBytes(128 * v)
				for i := 0; i < b.N; i++ {
					benchSink = tst.f(x, y)
				}
			})
		}
	}
}

func BenchmarkDotInc(t *testing.B) {
	for _, tst := range []struct {
		name string
		f    func(x, y []complex64, n, incX, incY, ix, iy uintptr) complex64
	}{
		{"DotcInc", DotcInc},
		{"DotuInc", DotuInc},
	} {
		for _, ln := range []int{1, 2, 3, 4, 5, 10, 100, 1e3, 5e3, 1e4, 5e4} {
			for _, inc := range []int{1, 2, 4, 10, -1, -2, -4, -10} {
				t.Run(fmt.Sprintf("%s-%d-inc%d", tst.name, ln, inc), func(b *testing.B) {
					b.SetBytes(int64(128 * ln))
					var idx int
					if inc < 0 {
						idx = (-ln + 1) * inc
					}
					for i := 0; i < b.N; i++ {
						benchSink = tst.f(x, y, uintptr(ln), uintptr(inc), uintptr(inc), uintptr(idx), uintptr(idx))
					}
				})
			}
		}
	}
}
