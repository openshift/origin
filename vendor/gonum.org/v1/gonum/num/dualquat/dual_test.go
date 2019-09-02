// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualquat

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/quat"
)

var formatTests = []struct {
	d      Number
	format string
	want   string
}{
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%#v", want: "dualquat.Number{Real:quat.Number{Real:1.1, Imag:2.1, Jmag:3.1, Kmag:4.1}, Dual:quat.Number{Real:1.2, Imag:2.2, Jmag:3.2, Kmag:4.2}}"},                 // Bootstrap test.
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%#v", want: "dualquat.Number{Real:quat.Number{Real:-1.1, Imag:-2.1, Jmag:-3.1, Kmag:-4.1}, Dual:quat.Number{Real:-1.2, Imag:-2.2, Jmag:-3.2, Kmag:-4.2}}"}, // Bootstrap test.
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%+v", want: "{Real:{Real:1.1, Imag:2.1, Jmag:3.1, Kmag:4.1}, Dual:{Real:1.2, Imag:2.2, Jmag:3.2, Kmag:4.2}}"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%+v", want: "{Real:{Real:-1.1, Imag:-2.1, Jmag:-3.1, Kmag:-4.1}, Dual:{Real:-1.2, Imag:-2.2, Jmag:-3.2, Kmag:-4.2}}"},
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%v", want: "((1.1+2.1i+3.1j+4.1k)+(+1.2+2.2i+3.2j+4.2k)ϵ)"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%v", want: "((-1.1-2.1i-3.1j-4.1k)+(-1.2-2.2i-3.2j-4.2k)ϵ)"},
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%g", want: "((1.1+2.1i+3.1j+4.1k)+(+1.2+2.2i+3.2j+4.2k)ϵ)"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%g", want: "((-1.1-2.1i-3.1j-4.1k)+(-1.2-2.2i-3.2j-4.2k)ϵ)"},
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%e", want: "((1.100000e+00+2.100000e+00i+3.100000e+00j+4.100000e+00k)+(+1.200000e+00+2.200000e+00i+3.200000e+00j+4.200000e+00k)ϵ)"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%e", want: "((-1.100000e+00-2.100000e+00i-3.100000e+00j-4.100000e+00k)+(-1.200000e+00-2.200000e+00i-3.200000e+00j-4.200000e+00k)ϵ)"},
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%E", want: "((1.100000E+00+2.100000E+00i+3.100000E+00j+4.100000E+00k)+(+1.200000E+00+2.200000E+00i+3.200000E+00j+4.200000E+00k)ϵ)"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%E", want: "((-1.100000E+00-2.100000E+00i-3.100000E+00j-4.100000E+00k)+(-1.200000E+00-2.200000E+00i-3.200000E+00j-4.200000E+00k)ϵ)"},
	{d: Number{quat.Number{1.1, 2.1, 3.1, 4.1}, quat.Number{1.2, 2.2, 3.2, 4.2}}, format: "%f", want: "((1.100000+2.100000i+3.100000j+4.100000k)+(+1.200000+2.200000i+3.200000j+4.200000k)ϵ)"},
	{d: Number{quat.Number{-1.1, -2.1, -3.1, -4.1}, quat.Number{-1.2, -2.2, -3.2, -4.2}}, format: "%f", want: "((-1.100000-2.100000i-3.100000j-4.100000k)+(-1.200000-2.200000i-3.200000j-4.200000k)ϵ)"},
}

func TestFormat(t *testing.T) {
	for _, test := range formatTests {
		got := fmt.Sprintf(test.format, test.d)
		if got != test.want {
			t.Errorf("unexpected result for fmt.Sprintf(%q, %#v): got:%q, want:%q", test.format, test.d, got, test.want)
		}
	}
}

// First derivatives:

func dPowReal(x quat.Number, y float64) quat.Number {
	return quat.Mul(quat.Number{Real: y}, quat.Pow(x, quat.Number{Real: y - 1}))
}
func dExp(x quat.Number) quat.Number { return quat.Exp(x) }
func dLog(x quat.Number) quat.Number {
	switch {
	case x == zeroQuat:
		return quat.Inf()
	case quat.IsInf(x):
		return zeroQuat
	}
	return quat.Inv(x)
}
func dPow(x, y quat.Number) quat.Number { return quat.Mul(y, quat.Pow(x, subQuatReal(y, 1))) }
func dSqrt(x quat.Number) quat.Number   { return quat.Scale(0.5, quat.Inv(quat.Sqrt(x))) }
func dInv(x quat.Number) quat.Number    { return quat.Scale(-1, quat.Inv(quat.Mul(x, x))) }

var (
	negZero     = math.Copysign(0, -1)
	oneReal     = quat.Number{Real: 1}
	negZeroQuat = quat.Scale(-1, zeroQuat)
	one         = quat.Number{1, 1, 1, 1}
	negOne      = quat.Scale(-1, one)
	half        = quat.Scale(0.5, one)
	negHalf     = quat.Scale(-1, half)
	two         = quat.Scale(2, one)
	negTwo      = quat.Scale(-1, two)
	three       = quat.Scale(3, one)
	negThree    = quat.Scale(-1, three)
	four        = quat.Scale(4, one)
	six         = quat.Scale(6, one)
)

var dualTests = []struct {
	name   string
	x      []quat.Number
	fnDual func(x Number) Number
	fn     func(x quat.Number) quat.Number
	dFn    func(x quat.Number) quat.Number
}{
	{
		name:   "exp",
		x:      []quat.Number{quat.NaN(), quat.Inf(), negThree, negTwo, negOne, negHalf, negZeroQuat, zeroQuat, half, one, two, three},
		fnDual: Exp,
		fn:     quat.Exp,
		dFn:    dExp,
	},
	{
		name:   "log",
		x:      []quat.Number{quat.NaN(), quat.Inf(), negThree, negTwo, negOne, negHalf, negZeroQuat, zeroQuat, half, one, two, three},
		fnDual: Log,
		fn:     quat.Log,
		dFn:    dLog,
	},
	{
		name:   "inv",
		x:      []quat.Number{quat.NaN(), quat.Inf(), negThree, negTwo, negOne, negHalf, negZeroQuat, zeroQuat, half, one, two, three},
		fnDual: Inv,
		fn:     quat.Inv,
		dFn:    dInv,
	},
	{
		name:   "sqrt",
		x:      []quat.Number{quat.NaN(), quat.Inf(), negThree, negTwo, negOne, negHalf, negZeroQuat, zeroQuat, half, one, two, three},
		fnDual: Sqrt,
		fn:     quat.Sqrt,
		dFn:    dSqrt,
	},
}

func TestNumber(t *testing.T) {
	const tol = 1e-15
	for _, test := range dualTests {
		for _, x := range test.x {
			fxDual := test.fnDual(Number{Real: x, Dual: oneReal})
			fx := test.fn(x)
			dFx := test.dFn(x)
			if !same(fxDual.Real, fx, tol) {
				t.Errorf("unexpected %s(%v): got:%v want:%v", test.name, x, fxDual.Real, fx)
			}
			if !same(fxDual.Dual, dFx, tol) {
				t.Errorf("unexpected %s'(%v): got:%v want:%v", test.name, x, fxDual.Dual, dFx)
			}
		}
	}
}

var invTests = []Number{
	{Real: quat.Number{Real: 1}},
	{Real: quat.Number{Real: 1}, Dual: quat.Number{Real: 1}},
	{Real: quat.Number{Imag: 1}, Dual: quat.Number{Real: 1}},
	{Real: quat.Number{Real: 1}, Dual: quat.Number{Real: 1, Imag: 1}},
	{Real: quat.Number{Real: 1, Imag: 1}, Dual: quat.Number{Real: 1, Imag: 1}},
	{Real: quat.Number{Real: 1, Imag: 10}, Dual: quat.Number{Real: 1, Imag: 5}},
	{Real: quat.Number{Real: 10, Imag: 1}, Dual: quat.Number{Real: 5, Imag: 1}},
	{Real: quat.Number{Real: 1}, Dual: quat.Number{Real: 1, Imag: 1, Kmag: 1}},
	{Real: quat.Number{Real: 12, Imag: 1}, Dual: quat.Number{Real: 1, Imag: 1}},
	{Real: quat.Number{Real: 12, Imag: 1, Jmag: 3}},
}

func TestInv(t *testing.T) {
	const tol = 1e-15
	for _, x := range invTests {
		got := Mul(x, Inv(x))
		want := Number{Real: quat.Number{Real: 1}}
		if !sameDual(got, want, tol) {
			t.Errorf("unexpected Mul(%[1]v, Inv(%[1]v)): got:%v want:%v", x, got, want)
		}
	}
}

var powRealTests = []struct {
	d    Number
	p    float64
	want Number
}{
	// PowReal(NaN+xϵ, ±0) = 1+NaNϵ for any x
	{d: Number{Real: quat.NaN(), Dual: zeroQuat}, p: 0, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: zeroQuat}, p: negZero, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: one}, p: 0, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: two}, p: negZero, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: three}, p: 0, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: one}, p: negZero, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: two}, p: 0, want: Number{Real: oneReal, Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: three}, p: negZero, want: Number{Real: oneReal, Dual: quat.NaN()}},

	// PowReal(x, ±0) = 1 for any x
	{d: Number{Real: zeroQuat, Dual: zeroQuat}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: negZeroQuat, Dual: zeroQuat}, p: negZero, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: quat.Inf(), Dual: zeroQuat}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: quat.Inf(), Dual: zeroQuat}, p: negZero, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: zeroQuat, Dual: one}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: negZeroQuat, Dual: one}, p: negZero, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: quat.Inf(), Dual: one}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: quat.Inf(), Dual: one}, p: negZero, want: Number{Real: oneReal, Dual: zeroQuat}},

	// PowReal(1+xϵ, y) = (1+xyϵ) for any y
	{d: Number{Real: oneReal, Dual: zeroQuat}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: zeroQuat}, p: 1, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: zeroQuat}, p: 2, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: zeroQuat}, p: 3, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: one}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: one}, p: 1, want: Number{Real: oneReal, Dual: one}},
	{d: Number{Real: oneReal, Dual: one}, p: 2, want: Number{Real: oneReal, Dual: two}},
	{d: Number{Real: oneReal, Dual: one}, p: 3, want: Number{Real: oneReal, Dual: three}},
	{d: Number{Real: oneReal, Dual: two}, p: 0, want: Number{Real: oneReal, Dual: zeroQuat}},
	{d: Number{Real: oneReal, Dual: two}, p: 1, want: Number{Real: oneReal, Dual: two}},
	{d: Number{Real: oneReal, Dual: two}, p: 2, want: Number{Real: oneReal, Dual: four}},
	{d: Number{Real: oneReal, Dual: two}, p: 3, want: Number{Real: oneReal, Dual: six}},

	// PowReal(x, 1) = x for any x
	{d: Number{Real: zeroQuat, Dual: zeroQuat}, p: 1, want: Number{Real: zeroQuat, Dual: zeroQuat}},
	{d: Number{Real: negZeroQuat, Dual: zeroQuat}, p: 1, want: Number{Real: negZeroQuat, Dual: zeroQuat}},
	{d: Number{Real: zeroQuat, Dual: one}, p: 1, want: Number{Real: zeroQuat, Dual: one}},
	{d: Number{Real: negZeroQuat, Dual: one}, p: 1, want: Number{Real: negZeroQuat, Dual: one}},
	{d: Number{Real: quat.NaN(), Dual: zeroQuat}, p: 1, want: Number{Real: quat.NaN(), Dual: zeroQuat}},
	{d: Number{Real: quat.NaN(), Dual: one}, p: 1, want: Number{Real: quat.NaN(), Dual: one}},
	{d: Number{Real: quat.NaN(), Dual: two}, p: 1, want: Number{Real: quat.NaN(), Dual: two}},

	// PowReal(NaN+xϵ, y) = NaN+NaNϵ
	{d: Number{Real: quat.NaN(), Dual: zeroQuat}, p: 2, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: zeroQuat}, p: 3, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: one}, p: 2, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: one}, p: 3, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: two}, p: 2, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: quat.NaN(), Dual: two}, p: 3, want: Number{Real: quat.NaN(), Dual: quat.NaN()}},

	// PowReal(x, NaN) = NaN+NaNϵ
	{d: Number{Real: zeroQuat, Dual: zeroQuat}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: two, Dual: zeroQuat}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: zeroQuat}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: zeroQuat, Dual: one}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: two, Dual: one}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: one}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: zeroQuat, Dual: two}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: two, Dual: two}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: two}, p: math.NaN(), want: Number{Real: quat.NaN(), Dual: quat.NaN()}},

	// Handled by quat.Pow tests:
	//
	// Pow(±0, y) = ±Inf for y an odd integer < 0
	// Pow(±0, -Inf) = +Inf
	// Pow(±0, +Inf) = +0
	// Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
	// Pow(±0, y) = ±0 for y an odd integer > 0
	// Pow(±0, y) = +0 for finite y > 0 and not an odd integer
	// Pow(-1, ±Inf) = 1

	// PowReal(x+0ϵ, +Inf) = +Inf+NaNϵ for |x| > 1
	{d: Number{Real: two, Dual: zeroQuat}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: zeroQuat}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.NaN()}},

	// PowReal(x+yϵ, +Inf) = +Inf for |x| > 1
	{d: Number{Real: two, Dual: one}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: three, Dual: one}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: two, Dual: two}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: three, Dual: two}, p: math.Inf(1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},

	// PowReal(x, -Inf) = +0+NaNϵ for |x| > 1
	{d: Number{Real: two, Dual: zeroQuat}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: zeroQuat}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: two, Dual: one}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: one}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: two, Dual: two}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: three, Dual: two}, p: math.Inf(-1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},

	// PowReal(x+yϵ, +Inf) = +0+NaNϵ for |x| < 1
	{d: Number{Real: quat.Scale(0.1, one), Dual: zeroQuat}, p: math.Inf(1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: quat.Scale(0.1, one), Dual: quat.Scale(0.1, one)}, p: math.Inf(1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: quat.Scale(0.2, one)}, p: math.Inf(1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},
	{d: Number{Real: quat.Scale(0.5, one), Dual: quat.Scale(0.5, one)}, p: math.Inf(1), want: Number{Real: zeroQuat, Dual: quat.NaN()}},

	// PowReal(x+0ϵ, -Inf) = +Inf+NaNϵ for |x| < 1
	{d: Number{Real: quat.Scale(0.1, one), Dual: zeroQuat}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.NaN()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: zeroQuat}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.NaN()}},

	// PowReal(x, -Inf) = +Inf-Infϵ for |x| < 1
	{d: Number{Real: quat.Scale(0.1, one), Dual: quat.Scale(0.1, one)}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: quat.Scale(0.1, one)}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.1, one), Dual: quat.Scale(0.2, one)}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: quat.Scale(0.2, one)}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.1, one), Dual: one}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: one}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.1, one), Dual: two}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},
	{d: Number{Real: quat.Scale(0.2, one), Dual: two}, p: math.Inf(-1), want: Number{Real: quat.Inf(), Dual: quat.Inf()}},

	// Handled by quat.Pow tests:
	//
	// Pow(+Inf, y) = +Inf for y > 0
	// Pow(+Inf, y) = +0 for y < 0
	// Pow(-Inf, y) = Pow(-0, -y)
}

func TestPowReal(t *testing.T) {
	const tol = 1e-15
	for _, test := range powRealTests {
		got := PowReal(test.d, test.p)
		if !sameDual(got, test.want, tol) {
			t.Errorf("unexpected PowReal(%v, %v): got:%v want:%v", test.d, test.p, got, test.want)
		}
	}
}

func sameDual(a, b Number, tol float64) bool {
	return same(a.Real, b.Real, tol) && same(a.Dual, b.Dual, tol)
}

func same(a, b quat.Number, tol float64) bool {
	return (quat.IsNaN(a) && quat.IsNaN(b)) || (quat.IsInf(a) && quat.IsInf(b)) || equalApprox(a, b, tol)
}

func equalApprox(a, b quat.Number, tol float64) bool {
	return floats.EqualWithinAbsOrRel(a.Real, b.Real, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Imag, b.Imag, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Jmag, b.Jmag, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Kmag, b.Kmag, tol, tol)
}
