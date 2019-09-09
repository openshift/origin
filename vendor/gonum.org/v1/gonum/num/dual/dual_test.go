// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dual

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

var formatTests = []struct {
	d      Number
	format string
	want   string
}{
	{d: Number{1.1, 2.1}, format: "%#v", want: "dual.Number{Real:1.1, Emag:2.1}"},     // Bootstrap test.
	{d: Number{-1.1, -2.1}, format: "%#v", want: "dual.Number{Real:-1.1, Emag:-2.1}"}, // Bootstrap test.
	{d: Number{1.1, 2.1}, format: "%+v", want: "{Real:1.1, Emag:2.1}"},
	{d: Number{-1.1, -2.1}, format: "%+v", want: "{Real:-1.1, Emag:-2.1}"},
	{d: Number{1, 2}, format: "%v", want: "(1+2ϵ)"},
	{d: Number{-1, -2}, format: "%v", want: "(-1-2ϵ)"},
	{d: Number{1, 2}, format: "%g", want: "(1+2ϵ)"},
	{d: Number{-1, -2}, format: "%g", want: "(-1-2ϵ)"},
	{d: Number{1, 2}, format: "%e", want: "(1.000000e+00+2.000000e+00ϵ)"},
	{d: Number{-1, -2}, format: "%e", want: "(-1.000000e+00-2.000000e+00ϵ)"},
	{d: Number{1, 2}, format: "%E", want: "(1.000000E+00+2.000000E+00ϵ)"},
	{d: Number{-1, -2}, format: "%E", want: "(-1.000000E+00-2.000000E+00ϵ)"},
	{d: Number{1, 2}, format: "%f", want: "(1.000000+2.000000ϵ)"},
	{d: Number{-1, -2}, format: "%f", want: "(-1.000000-2.000000ϵ)"},
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

func dSin(x float64) float64  { return math.Cos(x) }
func dCos(x float64) float64  { return -math.Sin(x) }
func dTan(x float64) float64  { return sec(x) * sec(x) }
func dAsin(x float64) float64 { return 1 / math.Sqrt(1-x*x) }
func dAcos(x float64) float64 { return -1 / math.Sqrt(1-x*x) }
func dAtan(x float64) float64 { return 1 / (1 + x*x) }

func dSinh(x float64) float64  { return math.Cosh(x) }
func dCosh(x float64) float64  { return math.Sinh(x) }
func dTanh(x float64) float64  { return sech(x) * sech(x) }
func dAsinh(x float64) float64 { return 1 / math.Sqrt(x*x+1) }
func dAcosh(x float64) float64 { return 1 / (math.Sqrt(x-1) * math.Sqrt(x+1)) }
func dAtanh(x float64) float64 {
	switch {
	case math.Abs(x) == 1:
		return math.NaN()
	case math.IsInf(x, 0):
		return negZero
	}
	return 1 / (1 - x*x)
}

func dExp(x float64) float64 { return math.Exp(x) }
func dLog(x float64) float64 {
	if x < 0 {
		return math.NaN()
	}
	return 1 / x
}
func dPow(x, y float64) float64 { return y * math.Pow(x, y-1) }
func dSqrt(x float64) float64 {
	// For whatever reason, math.Sqrt(-0) returns -0.
	// In this case, that is clearly a wrong approach.
	if x == 0 {
		return math.Inf(1)
	}
	return 0.5 / math.Sqrt(x)
}
func dInv(x float64) float64 { return -1 / (x * x) }

// Helpers:

func sec(x float64) float64  { return 1 / math.Cos(x) }
func sech(x float64) float64 { return 1 / math.Cosh(x) }

var negZero = math.Float64frombits(1 << 63)

var dualTests = []struct {
	name   string
	x      []float64
	fnDual func(x Number) Number
	fn     func(x float64) float64
	dFn    func(x float64) float64
}{
	{
		name:   "sin",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Sin,
		fn:     math.Sin,
		dFn:    dSin,
	},
	{
		name:   "cos",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Cos,
		fn:     math.Cos,
		dFn:    dCos,
	},
	{
		name:   "tan",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Tan,
		fn:     math.Tan,
		dFn:    dTan,
	},
	{
		name:   "sinh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Sinh,
		fn:     math.Sinh,
		dFn:    dSinh,
	},
	{
		name:   "cosh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Cosh,
		fn:     math.Cosh,
		dFn:    dCosh,
	},
	{
		name:   "tanh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Tanh,
		fn:     math.Tanh,
		dFn:    dTanh,
	},

	{
		name:   "asin",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Asin,
		fn:     math.Asin,
		dFn:    dAsin,
	},
	{
		name:   "acos",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Acos,
		fn:     math.Acos,
		dFn:    dAcos,
	},
	{
		name:   "atan",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Atan,
		fn:     math.Atan,
		dFn:    dAtan,
	},
	{
		name:   "asinh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Asinh,
		fn:     math.Asinh,
		dFn:    dAsinh,
	},
	{
		name:   "acosh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Acosh,
		fn:     math.Acosh,
		dFn:    dAcosh,
	},
	{
		name:   "atanh",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Atanh,
		fn:     math.Atanh,
		dFn:    dAtanh,
	},

	{
		name:   "exp",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Exp,
		fn:     math.Exp,
		dFn:    dExp,
	},
	{
		name:   "log",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Log,
		fn:     math.Log,
		dFn:    dLog,
	},
	{
		name:   "inv",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Inv,
		fn:     func(x float64) float64 { return 1 / x },
		dFn:    dInv,
	},
	{
		name:   "sqrt",
		x:      []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnDual: Sqrt,
		fn:     math.Sqrt,
		dFn:    dSqrt,
	},

	{
		name: "Fike example fn",
		x:    []float64{1, 2, 3, 4, 5},
		fnDual: func(x Number) Number {
			return Mul(
				Exp(x),
				Inv(Sqrt(
					Add(
						PowReal(Sin(x), 3),
						PowReal(Cos(x), 3)))))
		},
		fn: func(x float64) float64 {
			return math.Exp(x) / math.Sqrt(math.Pow(math.Sin(x), 3)+math.Pow(math.Cos(x), 3))
		},
		dFn: func(x float64) float64 {
			return math.Exp(x) * (3*math.Cos(x) + 5*math.Cos(3*x) + 9*math.Sin(x) + math.Sin(3*x)) /
				(8 * math.Pow(math.Pow(math.Sin(x), 3)+math.Pow(math.Cos(x), 3), 1.5))
		},
	},
}

func TestDual(t *testing.T) {
	const tol = 1e-15
	for _, test := range dualTests {
		for _, x := range test.x {
			fxDual := test.fnDual(Number{Real: x, Emag: 1})
			fx := test.fn(x)
			dFx := test.dFn(x)
			if !same(fxDual.Real, fx, tol) {
				t.Errorf("unexpected %s(%v): got:%v want:%v", test.name, x, fxDual.Real, fx)
			}
			if !same(fxDual.Emag, dFx, tol) {
				t.Errorf("unexpected %s'(%v): got:%v want:%v", test.name, x, fxDual.Emag, dFx)
			}
		}
	}
}

var powRealTests = []struct {
	d    Number
	p    float64
	want Number
}{
	// PowReal(NaN+xϵ, ±0) = 1+NaNϵ for any x
	{d: Number{Real: math.NaN(), Emag: 0}, p: 0, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 0}, p: negZero, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 1}, p: 0, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 2}, p: negZero, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 3}, p: 0, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 1}, p: negZero, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 2}, p: 0, want: Number{Real: 1, Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 3}, p: negZero, want: Number{Real: 1, Emag: math.NaN()}},

	// PowReal(x, ±0) = 1 for any x
	{d: Number{Real: 0, Emag: 0}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: negZero, Emag: 0}, p: negZero, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: math.Inf(1), Emag: 0}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: math.Inf(-1), Emag: 0}, p: negZero, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 0, Emag: 1}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: negZero, Emag: 1}, p: negZero, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: math.Inf(1), Emag: 1}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: math.Inf(-1), Emag: 1}, p: negZero, want: Number{Real: 1, Emag: 0}},

	// PowReal(1+xϵ, y) = (1+xyϵ) for any y
	{d: Number{Real: 1, Emag: 0}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 0}, p: 1, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 0}, p: 2, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 0}, p: 3, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 1}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 1}, p: 1, want: Number{Real: 1, Emag: 1}},
	{d: Number{Real: 1, Emag: 1}, p: 2, want: Number{Real: 1, Emag: 2}},
	{d: Number{Real: 1, Emag: 1}, p: 3, want: Number{Real: 1, Emag: 3}},
	{d: Number{Real: 1, Emag: 2}, p: 0, want: Number{Real: 1, Emag: 0}},
	{d: Number{Real: 1, Emag: 2}, p: 1, want: Number{Real: 1, Emag: 2}},
	{d: Number{Real: 1, Emag: 2}, p: 2, want: Number{Real: 1, Emag: 4}},
	{d: Number{Real: 1, Emag: 2}, p: 3, want: Number{Real: 1, Emag: 6}},

	// PowReal(x, 1) = x for any x
	{d: Number{Real: 0, Emag: 0}, p: 1, want: Number{Real: 0, Emag: 0}},
	{d: Number{Real: negZero, Emag: 0}, p: 1, want: Number{Real: negZero, Emag: 0}},
	{d: Number{Real: 0, Emag: 1}, p: 1, want: Number{Real: 0, Emag: 1}},
	{d: Number{Real: negZero, Emag: 1}, p: 1, want: Number{Real: negZero, Emag: 1}},
	{d: Number{Real: math.NaN(), Emag: 0}, p: 1, want: Number{Real: math.NaN(), Emag: 0}},
	{d: Number{Real: math.NaN(), Emag: 1}, p: 1, want: Number{Real: math.NaN(), Emag: 1}},
	{d: Number{Real: math.NaN(), Emag: 2}, p: 1, want: Number{Real: math.NaN(), Emag: 2}},

	// PowReal(NaN+xϵ, y) = NaN+NaNϵ
	{d: Number{Real: math.NaN(), Emag: 0}, p: 2, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 0}, p: 3, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 1}, p: 2, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 1}, p: 3, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 2}, p: 2, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: math.NaN(), Emag: 2}, p: 3, want: Number{Real: math.NaN(), Emag: math.NaN()}},

	// PowReal(x, NaN) = NaN+NaNϵ
	{d: Number{Real: 0, Emag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 2, Emag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 0, Emag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 2, Emag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 0, Emag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 2, Emag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), Emag: math.NaN()}},

	// Handled by math.Pow tests:
	//
	// Pow(±0, y) = ±Inf for y an odd integer < 0
	// Pow(±0, -Inf) = +Inf
	// Pow(±0, +Inf) = +0
	// Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
	// Pow(±0, y) = ±0 for y an odd integer > 0
	// Pow(±0, y) = +0 for finite y > 0 and not an odd integer
	// Pow(-1, ±Inf) = 1

	// PowReal(x+0ϵ, +Inf) = +Inf+NaNϵ for |x| > 1
	{d: Number{Real: 2, Emag: 0}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 0}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.NaN()}},

	// PowReal(x+yϵ, +Inf) = +Inf for |x| > 1
	{d: Number{Real: 2, Emag: 1}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.Inf(1)}},
	{d: Number{Real: 3, Emag: 1}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.Inf(1)}},
	{d: Number{Real: 2, Emag: 2}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.Inf(1)}},
	{d: Number{Real: 3, Emag: 2}, p: math.Inf(1), want: Number{Real: math.Inf(1), Emag: math.Inf(1)}},

	// PowReal(x, -Inf) = +0+NaNϵ for |x| > 1
	{d: Number{Real: 2, Emag: 0}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 0}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 2, Emag: 1}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 1}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 2, Emag: 2}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 3, Emag: 2}, p: math.Inf(-1), want: Number{Real: 0, Emag: math.NaN()}},

	// PowReal(x+yϵ, +Inf) = +0+NaNϵ for |x| < 1
	{d: Number{Real: 0.1, Emag: 0}, p: math.Inf(1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 0.1, Emag: 0.1}, p: math.Inf(1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 0.2, Emag: 0.2}, p: math.Inf(1), want: Number{Real: 0, Emag: math.NaN()}},
	{d: Number{Real: 0.5, Emag: 0.5}, p: math.Inf(1), want: Number{Real: 0, Emag: math.NaN()}},

	// PowReal(x+0ϵ, -Inf) = +Inf+NaNϵ for |x| < 1
	{d: Number{Real: 0.1, Emag: 0}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.NaN()}},
	{d: Number{Real: 0.2, Emag: 0}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.NaN()}},

	// PowReal(x, -Inf) = +Inf-Infϵ for |x| < 1
	{d: Number{Real: 0.1, Emag: 0.1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.2, Emag: 0.1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.1, Emag: 0.2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.2, Emag: 0.2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.1, Emag: 1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.2, Emag: 1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.1, Emag: 2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},
	{d: Number{Real: 0.2, Emag: 2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), Emag: math.Inf(-1)}},

	// Handled by math.Pow tests:
	//
	// Pow(+Inf, y) = +Inf for y > 0
	// Pow(+Inf, y) = +0 for y < 0
	// Pow(-Inf, y) = Pow(-0, -y)

	// PowReal(x, y) = NaN+NaNϵ for finite x < 0 and finite non-integer y
	{d: Number{Real: -1, Emag: -1}, p: 0.5, want: Number{Real: math.NaN(), Emag: math.NaN()}},
	{d: Number{Real: -1, Emag: 2}, p: 0.5, want: Number{Real: math.NaN(), Emag: math.NaN()}},
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
	return same(a.Real, b.Real, tol) && same(a.Emag, b.Emag, tol)
}

func same(a, b, tol float64) bool {
	return (math.IsNaN(a) && math.IsNaN(b)) || floats.EqualWithinAbsOrRel(a, b, tol, tol)
}
