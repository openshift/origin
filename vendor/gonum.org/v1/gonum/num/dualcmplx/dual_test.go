// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualcmplx

import (
	"fmt"
	"math"
	"math/cmplx"
	"testing"

	"gonum.org/v1/gonum/floats"
)

var formatTests = []struct {
	d      Number
	format string
	want   string
}{
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%#v", want: "dualcmplx.Number{Real:(1.1+2.1i), Dual:(1.2+2.2i)}"},     // Bootstrap test.
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%#v", want: "dualcmplx.Number{Real:(-1.1-2.1i), Dual:(-1.2-2.2i)}"}, // Bootstrap test.
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%+v", want: "{Real:(1.1+2.1i), Dual:(1.2+2.2i)}"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%+v", want: "{Real:(-1.1-2.1i), Dual:(-1.2-2.2i)}"},
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%v", want: "((1.1+2.1i)+(+1.2+2.2i)ϵ)"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%v", want: "((-1.1-2.1i)+(-1.2-2.2i)ϵ)"},
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%g", want: "((1.1+2.1i)+(+1.2+2.2i)ϵ)"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%g", want: "((-1.1-2.1i)+(-1.2-2.2i)ϵ)"},
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%e", want: "((1.100000e+00+2.100000e+00i)+(+1.200000e+00+2.200000e+00i)ϵ)"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%e", want: "((-1.100000e+00-2.100000e+00i)+(-1.200000e+00-2.200000e+00i)ϵ)"},
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%E", want: "((1.100000E+00+2.100000E+00i)+(+1.200000E+00+2.200000E+00i)ϵ)"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%E", want: "((-1.100000E+00-2.100000E+00i)+(-1.200000E+00-2.200000E+00i)ϵ)"},
	{d: Number{1.1 + 2.1i, 1.2 + 2.2i}, format: "%f", want: "((1.100000+2.100000i)+(+1.200000+2.200000i)ϵ)"},
	{d: Number{-1.1 - 2.1i, -1.2 - 2.2i}, format: "%f", want: "((-1.100000-2.100000i)+(-1.200000-2.200000i)ϵ)"},
}

func TestFormat(t *testing.T) {
	for _, test := range formatTests {
		got := fmt.Sprintf(test.format, test.d)
		if got != test.want {
			t.Errorf("unexpected result for fmt.Sprintf(%q, %#v): got:%q, want:%q", test.format, test.d, got, test.want)
		}
	}
}

// FIXME(kortschak): See golang/go#29320.

func sqrt(x complex128) complex128 {
	switch {
	case math.IsInf(imag(x), 1):
		return cmplx.Inf()
	case math.IsNaN(imag(x)):
		return cmplx.NaN()
	case math.IsInf(real(x), -1):
		if imag(x) >= 0 && !math.IsInf(imag(x), 1) {
			return complex(0, math.NaN())
		}
		if math.IsNaN(imag(x)) {
			return complex(math.NaN(), math.Inf(1))
		}
	case math.IsInf(real(x), 1):
		if imag(x) >= 0 && !math.IsInf(imag(x), 1) {
			return complex(math.Inf(1), 0)
		}
		if math.IsNaN(imag(x)) {
			return complex(math.Inf(1), math.NaN())
		}
	case math.IsInf(real(x), -1):
		return complex(0, math.Inf(1))
	case math.IsNaN(real(x)):
		if math.IsNaN(imag(x)) || math.IsInf(imag(x), 0) {
			return cmplx.NaN()
		}
	}
	return cmplx.Sqrt(x)
}

// First derivatives:

func dExp(x complex128) complex128 {
	if imag(x) == 0 {
		return cmplx.Exp(x)
	}
	return (cmplx.Exp(x) - cmplx.Exp(cmplx.Conj(x))) / (x - cmplx.Conj(x))
}
func dLog(x complex128) complex128 {
	if cmplx.IsInf(x) {
		return 0
	}
	if x == 0 {
		if math.Copysign(1, real(x)) < 0 {
			return complex(math.Inf(-1), math.NaN())
		}
		return complex(math.Inf(1), math.NaN())
	}
	return (cmplx.Log(x) - cmplx.Log(cmplx.Conj(x))) / (x - cmplx.Conj(x))
}
func dInv(x complex128) complex128 { return -1 / (x * cmplx.Conj(x)) }

var (
	negZero      = math.Copysign(0, -1)
	zeroCmplx    = 0 + 0i
	negZeroCmplx = -1 * zeroCmplx
	one          = 1 + 1i
	negOne       = -1 - 1i
	half         = one / 2
	negHalf      = negOne / 2
	two          = 2 + 2i
	negTwo       = -2 - 2i
	three        = 3 + 3i
	negThree     = -3 + 3i
)

var dualTests = []struct {
	name   string
	x      []complex128
	fnDual func(x Number) Number
	fn     func(x complex128) complex128
	dFn    func(x complex128) complex128
}{
	{
		name:   "exp",
		x:      []complex128{cmplx.NaN(), cmplx.Inf(), negThree, negTwo, negOne, negHalf, negZeroCmplx, zeroCmplx, half, one, two, three},
		fnDual: Exp,
		fn:     cmplx.Exp,
		dFn:    dExp,
	},
	{
		name:   "log",
		x:      []complex128{cmplx.NaN(), cmplx.Inf(), negThree, negTwo, negOne, negHalf, negZeroCmplx, zeroCmplx, half, one, two, three},
		fnDual: Log,
		fn:     cmplx.Log,
		dFn:    dLog,
	},
	{
		name:   "inv",
		x:      []complex128{cmplx.NaN(), cmplx.Inf(), negThree, negTwo, negOne, negHalf, negZeroCmplx, zeroCmplx, half, one, two, three},
		fnDual: Inv,
		fn:     func(x complex128) complex128 { return 1 / x },
		dFn:    dInv,
	},
	{
		name:   "sqrt",
		x:      []complex128{cmplx.NaN(), cmplx.Inf(), negThree, negTwo, negOne, negHalf, negZeroCmplx, zeroCmplx, half, one, two, three},
		fnDual: Sqrt,
		fn:     sqrt,
		// TODO(kortschak): Find a concise dSqrt.
	},
}

func TestNumber(t *testing.T) {
	const tol = 1e-15
	for _, test := range dualTests {
		for _, x := range test.x {
			fxDual := test.fnDual(Number{Real: x, Dual: 1})
			fx := test.fn(x)
			if !same(fxDual.Real, fx, tol) {
				t.Errorf("unexpected %s(%v): got:%v want:%v", test.name, x, fxDual.Real, fx)
			}
			if test.dFn == nil {
				continue
			}
			dFx := test.dFn(x)
			if !same(fxDual.Dual, dFx, tol) {
				t.Errorf("unexpected %s'(%v): got:%v want:%v", test.name, x, fxDual.Dual, dFx)
			}
		}
	}
}

var invTests = []Number{
	{Real: 1, Dual: 0},
	{Real: 1, Dual: 1},
	{Real: 1i, Dual: 1},
	{Real: 1, Dual: 1 + 1i},
	{Real: 1 + 1i, Dual: 1 + 1i},
	{Real: 1 + 10i, Dual: 1 + 5i},
	{Real: 10 + 1i, Dual: 5 + 1i},
}

func TestInv(t *testing.T) {
	const tol = 1e-15
	for _, x := range invTests {
		got := Mul(x, Inv(x))
		want := Number{Real: 1}
		if !sameDual(got, want, tol) {
			t.Errorf("unexpected Mul(%[1]v, Inv(%[1]v)): got:%v want:%v", x, got, want)
		}
	}
}

var expLogTests = []Number{
	{Real: 1i, Dual: 1i},
	{Real: 1 + 1i, Dual: 1 + 1i},
	{Real: 1 + 1e-1i, Dual: 1 + 1i},
	{Real: 1 + 1e-2i, Dual: 1 + 1i},
	{Real: 1 + 1e-4i, Dual: 1 + 1i},
	{Real: 1 + 1e-6i, Dual: 1 + 1i},
	{Real: 1 + 1e-8i, Dual: 1 + 1i},
	{Real: 1 + 1e-10i, Dual: 1 + 1i},
	{Real: 1 + 1e-12i, Dual: 1 + 1i},
	{Real: 1 + 1e-14i, Dual: 1 + 1i},
	{Dual: 1 + 1i},
	{Dual: 1 + 2i},
	{Dual: 2 + 1i},
	{Dual: 2 + 2i},
	{Dual: 1 + 1i},
	{Dual: 1 + 5i},
	{Dual: 5 + 1i},
	{Real: 1 + 0i, Dual: 1 + 1i},
	{Real: 1 + 0i, Dual: 1 + 2i},
	{Real: 1 + 0i, Dual: 2 + 1i},
	{Real: 1 + 0i, Dual: 2 + 2i},
	{Real: 1 + 1i, Dual: 1 + 1i},
	{Real: 1 + 3i, Dual: 1 + 5i},
	{Real: 2 + 1i, Dual: 5 + 1i},
}

func TestExpLog(t *testing.T) {
	const tol = 1e-15
	for _, x := range expLogTests {
		got := Log(Exp(x))
		want := x
		if !sameDual(got, want, tol) {
			t.Errorf("unexpected Log(Exp(%v)): got:%v want:%v", x, got, want)
		}
	}
}

func TestLogExp(t *testing.T) {
	const tol = 1e-15
	for _, x := range expLogTests {
		if x.Real == 0 {
			continue
		}
		got := Exp(Log(x))
		want := x
		if !sameDual(got, want, tol) {
			t.Errorf("unexpected Log(Exp(%v)): got:%v want:%v", x, got, want)
		}
	}
}

var powRealSpecialTests = []struct {
	d    Number
	p    float64
	want Number
}{
	// PowReal(NaN+xϵ, ±0) = 1+NaNϵ for any x
	{d: Number{Real: cmplx.NaN(), Dual: 0}, p: 0, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 0}, p: negZero, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 1}, p: 0, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 2}, p: negZero, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 3}, p: 0, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 1}, p: negZero, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 2}, p: 0, want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 3}, p: negZero, want: Number{Real: 1, Dual: cmplx.NaN()}},

	// Pow(0+xϵ, y) = 0+Infϵ for all y < 1.
	{d: Number{Real: 0}, p: 0.1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Real: 0}, p: -1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1}, p: 0.1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1}, p: -1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1 + 1i}, p: 0.1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1 + 1i}, p: -1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1i}, p: 0.1, want: Number{Dual: cmplx.Inf()}},
	{d: Number{Dual: 1i}, p: -1, want: Number{Dual: cmplx.Inf()}},
	// Pow(0+xϵ, y) = 0 for all y > 1.
	{d: Number{Real: 0}, p: 1.1, want: Number{Real: 0}},
	{d: Number{Real: 0}, p: 2, want: Number{Real: 0}},
	{d: Number{Dual: 1}, p: 1.1, want: Number{Real: 0}},
	{d: Number{Dual: 1}, p: 2, want: Number{Real: 0}},
	{d: Number{Dual: 1 + 1i}, p: 1.1, want: Number{Real: 0}},
	{d: Number{Dual: 1 + 1i}, p: 2, want: Number{Real: 0}},
	{d: Number{Dual: 1i}, p: 1.1, want: Number{Real: 0}},
	{d: Number{Dual: 1i}, p: 2, want: Number{Real: 0}},

	// PowReal(x, ±0) = 1 for any x
	{d: Number{Real: 0, Dual: 0}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: negZeroCmplx, Dual: 0}, p: negZero, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: cmplx.Inf(), Dual: 0}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: cmplx.Inf(), Dual: 0}, p: negZero, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 0, Dual: 1}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: negZeroCmplx, Dual: 1}, p: negZero, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: cmplx.Inf(), Dual: 1}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: cmplx.Inf(), Dual: 1}, p: negZero, want: Number{Real: 1, Dual: 0}},

	// PowReal(1+xϵ, y) = (1+xyϵ) for any y
	{d: Number{Real: 1, Dual: 0}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 0}, p: 1, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 0}, p: 2, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 0}, p: 3, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 1}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 1}, p: 1, want: Number{Real: 1, Dual: 1}},
	{d: Number{Real: 1, Dual: 1}, p: 2, want: Number{Real: 1, Dual: 2}},
	{d: Number{Real: 1, Dual: 1}, p: 3, want: Number{Real: 1, Dual: 3}},
	{d: Number{Real: 1, Dual: 2}, p: 0, want: Number{Real: 1, Dual: 0}},
	{d: Number{Real: 1, Dual: 2}, p: 1, want: Number{Real: 1, Dual: 2}},
	{d: Number{Real: 1, Dual: 2}, p: 2, want: Number{Real: 1, Dual: 4}},
	{d: Number{Real: 1, Dual: 2}, p: 3, want: Number{Real: 1, Dual: 6}},

	// Pow(Inf, y) = +Inf+NaNϵ for y > 0
	{d: Number{Real: cmplx.Inf()}, p: 0.5, want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: 1, want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: 1.1, want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: 2, want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	// Pow(Inf, y) = +0+NaNϵ for y < 0
	{d: Number{Real: cmplx.Inf()}, p: -0.5, want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: -1, want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: -1.1, want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.Inf()}, p: -2, want: Number{Real: 0, Dual: cmplx.NaN()}},

	// PowReal(x, 1) = x for any x
	{d: Number{Real: 0, Dual: 0}, p: 1, want: Number{Real: 0, Dual: 0}},
	{d: Number{Real: negZeroCmplx, Dual: 0}, p: 1, want: Number{Real: negZeroCmplx, Dual: 0}},
	{d: Number{Real: 0, Dual: 1}, p: 1, want: Number{Real: 0, Dual: 1}},
	{d: Number{Real: negZeroCmplx, Dual: 1}, p: 1, want: Number{Real: negZeroCmplx, Dual: 1}},
	{d: Number{Real: cmplx.NaN(), Dual: 0}, p: 1, want: Number{Real: cmplx.NaN(), Dual: 0}},
	{d: Number{Real: cmplx.NaN(), Dual: 1}, p: 1, want: Number{Real: cmplx.NaN(), Dual: 1}},
	{d: Number{Real: cmplx.NaN(), Dual: 2}, p: 1, want: Number{Real: cmplx.NaN(), Dual: 2}},

	// PowReal(NaN+xϵ, y) = NaN+NaNϵ
	{d: Number{Real: cmplx.NaN(), Dual: 0}, p: 2, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 0}, p: 3, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 1}, p: 2, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 1}, p: 3, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 2}, p: 2, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: cmplx.NaN(), Dual: 2}, p: 3, want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},

	// PowReal(x, NaN) = NaN+NaNϵ
	{d: Number{Real: 0, Dual: 0}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 2, Dual: 0}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 0}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 0, Dual: 1}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 2, Dual: 1}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 1}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 0, Dual: 2}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 2, Dual: 2}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 2}, p: math.NaN(), want: Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}},

	// Pow(-1, ±Inf) = 1
	{d: Number{Real: -1}, p: math.Inf(-1), want: Number{Real: 1, Dual: cmplx.NaN()}},
	{d: Number{Real: -1}, p: math.Inf(1), want: Number{Real: 1, Dual: cmplx.NaN()}},

	// The following tests described for cmplx.Pow ar enot valid for this type and
	// are handled by the special cases Pow(0+xϵ, y) above.
	// Pow(±0, y) = ±Inf for y an odd integer < 0
	// Pow(±0, -Inf) = +Inf
	// Pow(±0, +Inf) = +0
	// Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
	// Pow(±0, y) = ±0 for y an odd integer > 0
	// Pow(±0, y) = +0 for finite y > 0 and not an odd integer

	// PowReal(x+0ϵ, +Inf) = +Inf+NaNϵ for |x| > 1
	{d: Number{Real: 2, Dual: 0}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 0}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},

	// PowReal(x+yϵ, +Inf) = +Inf for |x| > 1
	{d: Number{Real: 2, Dual: 1}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 3, Dual: 1}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 2, Dual: 2}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 3, Dual: 2}, p: math.Inf(1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},

	// PowReal(x, -Inf) = +0+NaNϵ for |x| > 1
	{d: Number{Real: 2, Dual: 0}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 0}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 2, Dual: 1}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 1}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 2, Dual: 2}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 3, Dual: 2}, p: math.Inf(-1), want: Number{Real: 0, Dual: cmplx.NaN()}},

	// PowReal(x+yϵ, +Inf) = +0+NaNϵ for |x| < 1
	{d: Number{Real: 0.1, Dual: 0}, p: math.Inf(1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 0.1, Dual: 0.1}, p: math.Inf(1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 0.2, Dual: 0.2}, p: math.Inf(1), want: Number{Real: 0, Dual: cmplx.NaN()}},
	{d: Number{Real: 0.5, Dual: 0.5}, p: math.Inf(1), want: Number{Real: 0, Dual: cmplx.NaN()}},

	// PowReal(x+0ϵ, -Inf) = +Inf+NaNϵ for |x| < 1
	{d: Number{Real: 0.1, Dual: 0}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},
	{d: Number{Real: 0.2, Dual: 0}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}},

	// PowReal(x, -Inf) = +Inf-Infϵ for |x| < 1
	{d: Number{Real: 0.1, Dual: 0.1}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.2, Dual: 0.1}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.1, Dual: 0.2}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.2, Dual: 0.2}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.1, Dual: 1}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.2, Dual: 1}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.1, Dual: 2}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
	{d: Number{Real: 0.2, Dual: 2}, p: math.Inf(-1), want: Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}},
}

func TestPowRealSpecial(t *testing.T) {
	const tol = 1e-15
	for _, test := range powRealSpecialTests {
		got := PowReal(test.d, test.p)
		if !sameDual(got, test.want, tol) {
			t.Errorf("unexpected PowReal(%v, %v): got:%v want:%v", test.d, test.p, got, test.want)
		}
	}
}

var powRealTests = []struct {
	d Number
	p float64
}{
	{d: Number{Real: 1e-1, Dual: 2 + 2i}, p: 0.2},
	{d: Number{Real: 1e-1, Dual: 2 + 2i}, p: 5},
	{d: Number{Real: 1e-2, Dual: 2 + 2i}, p: 0.2},
	{d: Number{Real: 1e-2, Dual: 2 + 2i}, p: 5},
	{d: Number{Real: 1e-3, Dual: 2 + 2i}, p: 0.2},
	{d: Number{Real: 1e-3, Dual: 2 + 2i}, p: 5},
	{d: Number{Real: 1e-4, Dual: 2 + 2i}, p: 0.2},
	{d: Number{Real: 1e-4, Dual: 2 + 2i}, p: 5},
	{d: Number{Real: 2, Dual: 0}, p: 0.5},
	{d: Number{Real: 2, Dual: 0}, p: 2},
	{d: Number{Real: 4, Dual: 0}, p: 0.5},
	{d: Number{Real: 4, Dual: 0}, p: 2},
	{d: Number{Real: 8, Dual: 0}, p: 1.0 / 3},
	{d: Number{Real: 8, Dual: 0}, p: 3},
}

func TestPowReal(t *testing.T) {
	const tol = 1e-14
	for _, test := range powRealTests {
		got := PowReal(PowReal(test.d, test.p), 1/test.p)
		if !sameDual(got, test.d, tol) {
			t.Errorf("unexpected PowReal(PowReal(%v, %v), 1/%[2]v): got:%v want:%[1]v", test.d, test.p, got)
		}
		if test.p != math.Floor(test.p) {
			continue
		}
		root := PowReal(test.d, 1/test.p)
		got = Number{Real: 1}
		for i := 0; i < int(test.p); i++ {
			got = Mul(got, root)
		}
		if !sameDual(got, test.d, tol) {
			t.Errorf("unexpected PowReal(%v, 1/%v)^%[2]v: got:%v want:%[1]v", test.d, test.p, got)
		}
	}
}

func sameDual(a, b Number, tol float64) bool {
	return same(a.Real, b.Real, tol) && same(a.Dual, b.Dual, tol)
}

func same(a, b complex128, tol float64) bool {
	return ((math.IsNaN(real(a)) && (math.IsNaN(real(b)))) || floats.EqualWithinAbsOrRel(real(a), real(b), tol, tol)) &&
		((math.IsNaN(imag(a)) && (math.IsNaN(imag(b)))) || floats.EqualWithinAbsOrRel(imag(a), imag(b), tol, tol))
}

func equalApprox(a, b complex128, tol float64) bool {
	return floats.EqualWithinAbsOrRel(real(a), real(b), tol, tol) &&
		floats.EqualWithinAbsOrRel(imag(a), imag(b), tol, tol)
}
