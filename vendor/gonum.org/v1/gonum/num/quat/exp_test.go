// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"math"
	"testing"
)

var expTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: 1}},
	// Expected velues below are from pyquaternion.
	{
		q:    Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: -0.43643792124786496, Imag: 1.549040352371697, Jmag: 1.549040352371697, Kmag: 1.549040352371697},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.42389891174348104, Imag: 0, Jmag: 1.8986002490721081, Kmag: 1.8986002490721081},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 1.4686939399158851, Imag: 0, Jmag: 0, Kmag: 2.2873552871788423},
	},
	{
		q:    Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: -0.16055653857469052, Imag: 0.569860099182514, Jmag: 0.569860099182514, Kmag: 0.569860099182514},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.15594369476537437, Imag: 0, Jmag: 0.6984559986366083, Kmag: 0.6984559986366083},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 0.5403023058681398, Imag: 0, Jmag: 0, Kmag: 0.8414709848078965},
	},
}

func TestExp(t *testing.T) {
	const tol = 1e-14
	for _, test := range expTests {
		got := Exp(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Exp(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var logTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: -inf}},
	// Expected velues below are from pyquaternion.
	{
		q:    Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.6931471805599453, Imag: 0.6045997880780728, Jmag: 0.6045997880780728, Kmag: 0.6045997880780728},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.5493061443340548, Imag: 0, Jmag: 0.6755108588560398, Kmag: 0.6755108588560398},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 0.3465735902799727, Imag: 0, Jmag: 0, Kmag: 0.7853981633974484},
	},
	{
		q:    Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.5493061443340548, Imag: 0.906899682117109, Jmag: 0.906899682117109, Kmag: 0.906899682117109},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.3465735902799727, Imag: 0, Jmag: 1.1107207345395915, Kmag: 1.1107207345395915},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1.5707963267948966},
	},
}

func TestLog(t *testing.T) {
	const tol = 1e-14
	for _, test := range logTests {
		got := Log(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Log(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var powTests = []struct {
	q, r Number
	want Number
}{
	{q: Number{}, r: Number{}, want: Number{Real: 1}},
	// Expected velues below are from pyquaternion.
	// pyquaternion does not support quaternion powers.
	// TODO(kortschak): Add non-real r cases.
	{
		q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: -2, Imag: 2, Jmag: 2, Kmag: 2},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: -1, Imag: 0, Jmag: 2, Kmag: 2},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 2},
	},
	{
		q: Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: -3, Imag: 0, Jmag: 0, Kmag: 0},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: -2, Imag: 0, Jmag: 0, Kmag: 0},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: 2},
		want: Number{Real: -1, Imag: 0, Jmag: 0, Kmag: 0},
	},

	{
		q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: -8.728144138959564, Imag: -0.7527136547040768, Jmag: -0.7527136547040768, Kmag: -0.7527136547040768},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: -5.561182514695044, Imag: 0, Jmag: 0.5556661490713818, Kmag: 0.5556661490713818},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: -2.320735561810013, Imag: 0, Jmag: 0, Kmag: 1.8544983901925216},
	},
	{
		q: Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: 1.2388947209955585, Imag: -3.162774128856231, Jmag: -3.162774128856231, Kmag: -3.162774128856231},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: 0.6552860151073727, Imag: 0, Jmag: -2.0488506614051922, Kmag: -2.0488506614051922},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: math.Pi},
		want: Number{Real: 0.22058404074969779, Imag: 0, Jmag: 0, Kmag: -0.9753679720836315},
	},

	{
		q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: -8, Imag: 0, Jmag: 0, Kmag: 0},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: -5, Imag: 0, Jmag: 1, Kmag: 1},
	},
	{
		q: Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: -2, Imag: 0, Jmag: 0, Kmag: 2},
	},
	{
		q: Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: 0, Imag: -3, Jmag: -3, Kmag: -3},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: 0, Imag: 0, Jmag: -2, Kmag: -2},
	},
	{
		q: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1}, r: Number{Real: 3},
		want: Number{Real: 0, Imag: 0, Jmag: 0, Kmag: -1},
	},
}

func TestPow(t *testing.T) {
	const tol = 1e-14
	for _, test := range powTests {
		got := Pow(test.q, test.r)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Pow(%v, %v): got:%v want:%v", test.q, test.r, got, test.want)
		}
	}
}

var sqrtTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	// Expected velues below are from pyquaternion.
	{
		q:    Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: 1.2247448713915892, Imag: 0.4082482904638631, Jmag: 0.4082482904638631, Kmag: 0.4082482904638631},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 1.1687708944803676, Imag: 0, Jmag: 0.42779983858367593, Kmag: 0.42779983858367593},
	},
	{
		q:    Number{Real: 1, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 1.0986841134678098, Imag: 0, Jmag: 0, Kmag: 0.45508986056222733},
	},
	{
		q:    Number{Real: 0, Imag: 1, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.9306048591020996, Imag: 0.5372849659117709, Jmag: 0.5372849659117709, Kmag: 0.5372849659117709},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 1, Kmag: 1},
		want: Number{Real: 0.8408964152537146, Imag: 0, Jmag: 0.5946035575013604, Kmag: 0.5946035575013604},
	},
	{
		q:    Number{Real: 0, Imag: 0, Jmag: 0, Kmag: 1},
		want: Number{Real: 0.7071067811865476, Imag: 0, Jmag: 0, Kmag: 0.7071067811865475},
	},
}

func TestSqrt(t *testing.T) {
	const tol = 1e-14
	for _, test := range sqrtTests {
		got := Sqrt(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Sqrt(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}
