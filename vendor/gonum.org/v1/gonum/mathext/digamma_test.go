// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mathext

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

var result float64

func TestDigamma(t *testing.T) {
	const tol = 1e-10

	for i, test := range []struct {
		x, want float64
	}{
		// Results computed using WolframAlpha.
		{0.0, math.Inf(-1)},
		{math.Copysign(0.0, -1.0), math.Inf(1)},
		{math.Inf(1), math.Inf(1)},
		{math.Inf(-1), math.NaN()},
		{math.NaN(), math.NaN()},
		{-1.0, math.NaN()},
		{-100.5, 4.615124601338064117341315601525112558522917517910505881343},
		{.5, -1.96351002602142347944097633299875556719315960466043},
		{10, 2.251752589066721107647456163885851537211808918028330369448},
		{math.Pow10(20), 46.05170185988091368035482909368728415202202143924212618733},
		{-1.111111111e9, math.NaN()},
		{1.46, -0.001580561987083417676105544023567034348339520110000},
	} {

		got := Digamma(test.x)
		if !(math.IsNaN(got) && math.IsNaN(test.want)) && !floats.EqualWithinAbsOrRel(got, test.want, tol, tol) {
			t.Errorf("test %d Digamma(%g) failed: got %g want %g", i, test.x, got, test.want)
		}
	}
}

func BenchmarkDigamma(b *testing.B) {
	var r float64
	for i := 0; i < b.N; i++ {
		r = Digamma(-1.111111111e9)
	}
	result = r
}
