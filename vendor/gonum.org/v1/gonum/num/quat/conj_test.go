// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"testing"

	"gonum.org/v1/gonum/floats"
)

var invTests = []struct {
	q       Number
	wantNaN bool
}{
	{q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}},
	{q: Number{Real: 3, Imag: -1, Jmag: 5, Kmag: -40}},
	{q: Number{Real: 1e6, Imag: -1e5, Jmag: 4, Kmag: -10}},
	{q: Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1}},
	{q: Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1}},
	{q: Number{Real: 1, Imag: 1, Jmag: 0, Kmag: 1}},
	{q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 0}},
	{q: Number{}, wantNaN: true},
}

func TestInv(t *testing.T) {
	const tol = 1e-14
	for _, test := range invTests {
		got := Mul(test.q, Inv(test.q))
		if test.wantNaN {
			if !IsNaN(got) {
				t.Errorf("unexpected result for Mul(%v, Inv(%[1]v)): got:%v want:%v", test.q, got, NaN())
			}
			continue
		}
		if !(floats.EqualWithinAbsOrRel(got.Real, 1, tol, tol) && floats.EqualWithinAbsOrRel(Abs(got), 1, tol, tol)) {
			t.Errorf("unexpected result for Mul(%v, Inv(%[1]v)): got:%v want:%v", test.q, got, Number{Real: 1})
		}
	}
}
