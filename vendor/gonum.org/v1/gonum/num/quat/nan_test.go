// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"math"
	"testing"
)

var nan = math.NaN()

var nanTests = []struct {
	q    Number
	want bool
}{
	{q: NaN(), want: true},
	{q: Number{Real: nan, Imag: nan, Jmag: nan, Kmag: nan}, want: true},
	{q: Number{Real: nan, Imag: 0, Jmag: 0, Kmag: 0}, want: true},
	{q: Number{Real: 0, Imag: nan, Jmag: 0, Kmag: 0}, want: true},
	{q: Number{Real: 0, Imag: 0, Jmag: nan, Kmag: 0}, want: true},
	{q: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: nan}, want: true},
	{q: Number{Real: inf, Imag: nan, Jmag: nan, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: inf, Jmag: nan, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: nan, Jmag: inf, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: nan, Jmag: nan, Kmag: inf}, want: false},
	{q: Number{Real: -inf, Imag: nan, Jmag: nan, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: -inf, Jmag: nan, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: nan, Jmag: -inf, Kmag: nan}, want: false},
	{q: Number{Real: nan, Imag: nan, Jmag: nan, Kmag: -inf}, want: false},
	{q: Number{}, want: false},
}

func TestIsNaN(t *testing.T) {
	for _, test := range nanTests {
		got := IsNaN(test.q)
		if got != test.want {
			t.Errorf("unexpected result for IsNaN(%v): got:%t want:%t", test.q, got, test.want)
		}
	}
}
