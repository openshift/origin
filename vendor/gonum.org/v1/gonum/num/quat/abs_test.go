// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"math"
	"testing"
)

var absTests = []struct {
	q    Number
	want float64
}{
	{q: Number{}, want: 0},
	{q: NaN(), want: nan},
	{q: Inf(), want: inf},
	{q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, want: 2},
	{q: Number{Real: -1, Imag: 1, Jmag: -1, Kmag: 1}, want: 2},
	{q: Number{Real: 1, Imag: 2, Jmag: 3, Kmag: 4}, want: math.Sqrt(1 + 4 + 9 + 16)},
	{q: Number{Real: -1, Imag: -2, Jmag: -3, Kmag: -4}, want: math.Sqrt(1 + 4 + 9 + 16)},
}

func TestAbs(t *testing.T) {
	for _, test := range absTests {
		got := Abs(test.q)
		if math.IsNaN(test.want) {
			if !math.IsNaN(got) {
				t.Errorf("unexpected result for Abs(%v): got:%v want:%v", test.q, got, test.want)
			}
			continue
		}
		if got != test.want {
			t.Errorf("unexpected result for Abs(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}
