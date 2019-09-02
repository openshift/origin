// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hyperdual

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

var formatTests = []struct {
	h      Number
	format string
	want   string
}{
	{h: Number{1.1, 2.1, 3.1, 4.1}, format: "%#v", want: "hyperdual.Number{Real:1.1, E1mag:2.1, E2mag:3.1, E1E2mag:4.1}"},         // Bootstrap test.
	{h: Number{-1.1, -2.1, -3.1, -4.1}, format: "%#v", want: "hyperdual.Number{Real:-1.1, E1mag:-2.1, E2mag:-3.1, E1E2mag:-4.1}"}, // Bootstrap test.
	{h: Number{1.1, 2.1, 3.1, 4.1}, format: "%+v", want: "{Real:1.1, E1mag:2.1, E2mag:3.1, E1E2mag:4.1}"},
	{h: Number{-1.1, -2.1, -3.1, -4.1}, format: "%+v", want: "{Real:-1.1, E1mag:-2.1, E2mag:-3.1, E1E2mag:-4.1}"},
	{h: Number{1, 2, 3, 4}, format: "%v", want: "(1+2ϵ₁+3ϵ₂+4ϵ₁ϵ₂)"},
	{h: Number{-1, -2, -3, -4}, format: "%v", want: "(-1-2ϵ₁-3ϵ₂-4ϵ₁ϵ₂)"},
	{h: Number{1, 2, 3, 4}, format: "%g", want: "(1+2ϵ₁+3ϵ₂+4ϵ₁ϵ₂)"},
	{h: Number{-1, -2, -3, -4}, format: "%g", want: "(-1-2ϵ₁-3ϵ₂-4ϵ₁ϵ₂)"},
	{h: Number{1, 2, 3, 4}, format: "%e", want: "(1.000000e+00+2.000000e+00ϵ₁+3.000000e+00ϵ₂+4.000000e+00ϵ₁ϵ₂)"},
	{h: Number{-1, -2, -3, -4}, format: "%e", want: "(-1.000000e+00-2.000000e+00ϵ₁-3.000000e+00ϵ₂-4.000000e+00ϵ₁ϵ₂)"},
	{h: Number{1, 2, 3, 4}, format: "%E", want: "(1.000000E+00+2.000000E+00ϵ₁+3.000000E+00ϵ₂+4.000000E+00ϵ₁ϵ₂)"},
	{h: Number{-1, -2, -3, -4}, format: "%E", want: "(-1.000000E+00-2.000000E+00ϵ₁-3.000000E+00ϵ₂-4.000000E+00ϵ₁ϵ₂)"},
	{h: Number{1, 2, 3, 4}, format: "%f", want: "(1.000000+2.000000ϵ₁+3.000000ϵ₂+4.000000ϵ₁ϵ₂)"},
	{h: Number{-1, -2, -3, -4}, format: "%f", want: "(-1.000000-2.000000ϵ₁-3.000000ϵ₂-4.000000ϵ₁ϵ₂)"},
}

func TestFormat(t *testing.T) {
	for _, test := range formatTests {
		got := fmt.Sprintf(test.format, test.h)
		if got != test.want {
			t.Errorf("unexpected result for fmt.Sprintf(%q, %#v): got:%q, want:%q", test.format, test.h, got, test.want)
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

// Second derivatives:

func d2Sin(x float64) float64  { return -math.Sin(x) }
func d2Cos(x float64) float64  { return -math.Cos(x) }
func d2Tan(x float64) float64  { return 2 * math.Tan(x) * sec(x) * sec(x) }
func d2Asin(x float64) float64 { return x / math.Pow(1-x*x, 1.5) }
func d2Acos(x float64) float64 { return -x / math.Pow(1-x*x, 1.5) }
func d2Atan(x float64) float64 { return -2 * x / ((x*x + 1) * (x*x + 1)) }

func d2Sinh(x float64) float64  { return math.Sinh(x) }
func d2Cosh(x float64) float64  { return math.Cosh(x) }
func d2Tanh(x float64) float64  { return -2 * math.Tanh(x) * sech(x) * sech(x) }
func d2Asinh(x float64) float64 { return -x / math.Pow((x*x+1), 1.5) }
func d2Acosh(x float64) float64 { return -x / (math.Pow(x-1, 1.5) * math.Pow(x+1, 1.5)) }
func d2Atanh(x float64) float64 { return 2 * x / ((1 - x*x) * (1 - x*x)) }

func d2Exp(x float64) float64 { return math.Exp(x) }
func d2Log(x float64) float64 {
	if x < 0 {
		return math.NaN()
	}
	return -1 / (x * x)
}
func d2Pow(x, y float64) float64 { return y * (y - 1) * math.Pow(x, y-2) }
func d2Sqrt(x float64) float64 {
	// Again math.Sqyu, and math.Pow are odd.
	switch x {
	case math.Inf(1):
		return 0
	case math.Inf(-1):
		return math.NaN()
	}
	return -0.25 * math.Pow(x, -1.5)
}
func d2Inv(x float64) float64 { return 2 / (x * x * x) }

// Helpers:

func sec(x float64) float64  { return 1 / math.Cos(x) }
func sech(x float64) float64 { return 1 / math.Cosh(x) }

var hyperdualTests = []struct {
	name        string
	x           []float64
	fnHyperdual func(x Number) Number
	fn          func(x float64) float64
	dFn         func(x float64) float64
	d2Fn        func(x float64) float64
}{
	{
		name:        "sin",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Sin,
		fn:          math.Sin,
		dFn:         dSin,
		d2Fn:        d2Sin,
	},
	{
		name:        "cos",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Cos,
		fn:          math.Cos,
		dFn:         dCos,
		d2Fn:        d2Cos,
	},
	{
		name:        "tan",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Tan,
		fn:          math.Tan,
		dFn:         dTan,
		d2Fn:        d2Tan,
	},
	{
		name:        "sinh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Sinh,
		fn:          math.Sinh,
		dFn:         dSinh,
		d2Fn:        d2Sinh,
	},
	{
		name:        "cosh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Cosh,
		fn:          math.Cosh,
		dFn:         dCosh,
		d2Fn:        d2Cosh,
	},
	{
		name:        "tanh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Tanh,
		fn:          math.Tanh,
		dFn:         dTanh,
		d2Fn:        d2Tanh,
	},

	{
		name:        "asin",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Asin,
		fn:          math.Asin,
		dFn:         dAsin,
		d2Fn:        d2Asin,
	},
	{
		name:        "acos",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Acos,
		fn:          math.Acos,
		dFn:         dAcos,
		d2Fn:        d2Acos,
	},
	{
		name:        "atan",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Atan,
		fn:          math.Atan,
		dFn:         dAtan,
		d2Fn:        d2Atan,
	},
	{
		name:        "asinh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Asinh,
		fn:          math.Asinh,
		dFn:         dAsinh,
		d2Fn:        d2Asinh,
	},
	{
		name:        "acosh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Acosh,
		fn:          math.Acosh,
		dFn:         dAcosh,
		d2Fn:        d2Acosh,
	},
	{
		name:        "atanh",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Atanh,
		fn:          math.Atanh,
		dFn:         dAtanh,
		d2Fn:        d2Atanh,
	},

	{
		name:        "exp",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Exp,
		fn:          math.Exp,
		dFn:         dExp,
		d2Fn:        d2Exp,
	},
	{
		name:        "log",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Log,
		fn:          math.Log,
		dFn:         dLog,
		d2Fn:        d2Log,
	},
	{
		name:        "inv",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Inv,
		fn:          func(x float64) float64 { return 1 / x },
		dFn:         dInv,
		d2Fn:        d2Inv,
	},
	{
		name:        "sqrt",
		x:           []float64{math.NaN(), math.Inf(-1), -3, -2, -1, -0.5, negZero, 0, 0.5, 1, 2, 3, math.Inf(1)},
		fnHyperdual: Sqrt,
		fn:          math.Sqrt,
		dFn:         dSqrt,
		d2Fn:        d2Sqrt,
	},

	{
		name: "Fike example fn",
		x:    []float64{1, 2, 3, 4, 5},
		fnHyperdual: func(x Number) Number {
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
		d2Fn: func(x float64) float64 {
			return math.Exp(x) * (130 - 12*math.Cos(2*x) + 30*math.Cos(4*x) + 12*math.Cos(6*x) - 111*math.Sin(2*x) + 48*math.Sin(4*x) + 5*math.Sin(6*x)) /
				(64 * math.Pow(math.Pow(math.Sin(x), 3)+math.Pow(math.Cos(x), 3), 2.5))
		},
	},
}

func TestHyperdual(t *testing.T) {
	const tol = 1e-14
	for _, test := range hyperdualTests {
		for _, x := range test.x {
			fxHyperdual := test.fnHyperdual(Number{Real: x, E1mag: 1, E2mag: 1})
			fx := test.fn(x)
			dFx := test.dFn(x)
			d2Fx := test.d2Fn(x)
			if !same(fxHyperdual.Real, fx, tol) {
				t.Errorf("unexpected %s(%v): got:%v want:%v", test.name, x, fxHyperdual.Real, fx)
			}
			if !same(fxHyperdual.E1mag, dFx, tol) {
				t.Errorf("unexpected %s'(%v) (ϵ₁): got:%v want:%v", test.name, x, fxHyperdual.E1mag, dFx)
			}
			if !same(fxHyperdual.E1mag, fxHyperdual.E2mag, tol) {
				t.Errorf("mismatched ϵ₁ and ϵ₂ for %s(%v): ϵ₁:%v ϵ₂:%v", test.name, x, fxHyperdual.E1mag, fxHyperdual.E2mag)
			}
			if !same(fxHyperdual.E1E2mag, d2Fx, tol) {
				t.Errorf("unexpected %s''(%v): got:%v want:%v", test.name, x, fxHyperdual.E1E2mag, d2Fx)
			}
		}
	}
}

var powRealTests = []struct {
	d    Number
	p    float64
	want Number
}{
	// PowReal(NaN+xϵ₁+yϵ₂, ±0) = 1+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for any x and y
	{d: Number{Real: math.NaN(), E1mag: 0, E2mag: 0}, p: 0, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 0, E2mag: 0}, p: negZero, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 1}, p: 0, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 2}, p: negZero, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 3, E2mag: 3}, p: 0, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 1}, p: negZero, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 2}, p: 0, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 3, E2mag: 3}, p: negZero, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 3}, p: 0, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 3}, p: negZero, want: Number{Real: 1, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x, ±0) = 1 for any x
	{d: Number{Real: 0, E1mag: 0, E2mag: 0}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	{d: Number{Real: math.Inf(1), E1mag: 0, E2mag: 0}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	{d: Number{Real: math.Inf(-1), E1mag: 0, E2mag: 0}, p: negZero, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	{d: Number{Real: 0, E1mag: 1, E2mag: 1}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	{d: Number{Real: math.Inf(1), E1mag: 1, E2mag: 1}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	{d: Number{Real: math.Inf(-1), E1mag: 1, E2mag: 1}, p: negZero, want: Number{Real: 1, E1mag: 0, E2mag: 0}},
	// These two satisfy the claim above, but the sign of zero is negative. Do we care?
	{d: Number{Real: negZero, E1mag: 0, E2mag: 0}, p: negZero, want: Number{Real: 1, E1mag: negZero, E2mag: negZero}},
	{d: Number{Real: negZero, E1mag: 1, E2mag: 1}, p: negZero, want: Number{Real: 1, E1mag: negZero, E2mag: negZero}},

	// PowReal(1+xϵ₁+yϵ₂, z) = 1+xzϵ₁+yzϵ₂+2xyzϵ₁ϵ₂ for any z
	{d: Number{Real: 1, E1mag: 0, E2mag: 0}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 0, E2mag: 0}, p: 1, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 0, E2mag: 0}, p: 2, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 0, E2mag: 0}, p: 3, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 1}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 1}, p: 1, want: Number{Real: 1, E1mag: 1, E2mag: 1, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 1}, p: 2, want: Number{Real: 1, E1mag: 2, E2mag: 2, E1E2mag: 2}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 1}, p: 3, want: Number{Real: 1, E1mag: 3, E2mag: 3, E1E2mag: 6}},
	{d: Number{Real: 1, E1mag: 2, E2mag: 2}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 2, E2mag: 2}, p: 1, want: Number{Real: 1, E1mag: 2, E2mag: 2, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 2, E2mag: 2}, p: 2, want: Number{Real: 1, E1mag: 4, E2mag: 4, E1E2mag: 8}},
	{d: Number{Real: 1, E1mag: 2, E2mag: 2}, p: 3, want: Number{Real: 1, E1mag: 6, E2mag: 6, E1E2mag: 24}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 2}, p: 0, want: Number{Real: 1, E1mag: 0, E2mag: 0, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 2}, p: 1, want: Number{Real: 1, E1mag: 1, E2mag: 2, E1E2mag: 0}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 2}, p: 2, want: Number{Real: 1, E1mag: 2, E2mag: 4, E1E2mag: 4}},
	{d: Number{Real: 1, E1mag: 1, E2mag: 2}, p: 3, want: Number{Real: 1, E1mag: 3, E2mag: 6, E1E2mag: 12}},

	// PowReal(NaN+xϵ₁+yϵ₂, 1) = NaN+xϵ₁+yϵ₂+NaNϵ₁ϵ₂ for any x
	{d: Number{Real: math.NaN(), E1mag: 0, E2mag: 0}, p: 1, want: Number{Real: math.NaN(), E1mag: 0, E2mag: 0, E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 1}, p: 1, want: Number{Real: math.NaN(), E1mag: 1, E2mag: 1, E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 2}, p: 1, want: Number{Real: math.NaN(), E1mag: 2, E2mag: 2, E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 2}, p: 1, want: Number{Real: math.NaN(), E1mag: 1, E2mag: 2, E1E2mag: math.NaN()}},

	// PowReal(x, 1) = x for any x
	{d: Number{Real: 0, E1mag: 0, E2mag: 0}, p: 1, want: Number{Real: 0, E1mag: 0, E2mag: 0}},
	{d: Number{Real: negZero, E1mag: 0, E2mag: 0}, p: 1, want: Number{Real: negZero, E1mag: 0, E2mag: 0}},
	{d: Number{Real: 0, E1mag: 1, E2mag: 1}, p: 1, want: Number{Real: 0, E1mag: 1, E2mag: 1}},
	{d: Number{Real: negZero, E1mag: 1, E2mag: 1}, p: 1, want: Number{Real: negZero, E1mag: 1, E2mag: 1}},
	{d: Number{Real: 0, E1mag: 1, E2mag: 2}, p: 1, want: Number{Real: 0, E1mag: 1, E2mag: 2}},
	{d: Number{Real: negZero, E1mag: 1, E2mag: 2}, p: 1, want: Number{Real: negZero, E1mag: 1, E2mag: 2}},

	// PowReal(NaN+xϵ₁+xϵ₂, y) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂
	{d: Number{Real: math.NaN(), E1mag: 0, E2mag: 0}, p: 2, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 0, E2mag: 0}, p: 3, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 1}, p: 2, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 1}, p: 3, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 2}, p: 2, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 2, E2mag: 2}, p: 3, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 2}, p: 2, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: math.NaN(), E1mag: 1, E2mag: 2}, p: 3, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x, NaN) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂
	{d: Number{Real: 0, E1mag: 0, E2mag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 0, E2mag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 0, E2mag: 0}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0, E1mag: 1, E2mag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 1, E2mag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 1, E2mag: 1}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0, E1mag: 2, E2mag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 2, E2mag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 2, E2mag: 2}, p: math.NaN(), want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// Handled by math.Pow tests:
	//
	// Pow(±0, y) = ±Inf for y an odd integer < 0
	// Pow(±0, -Inf) = +Inf
	// Pow(±0, +Inf) = +0
	// Pow(±0, y) = +Inf for finite y < 0 and not an odd integer
	// Pow(±0, y) = ±0 for y an odd integer > 0
	// Pow(±0, y) = +0 for finite y > 0 and not an odd integer
	// Pow(-1, ±Inf) = 1

	// PowReal(x+0ϵ₁+0ϵ₂, +Inf) = +Inf+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| > 1
	{d: Number{Real: 2, E1mag: 0, E2mag: 0}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 0, E2mag: 0}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x+xϵ₁+yϵ₂, +Inf) = +Inf+Infϵ₁+Infϵ₂+NaNϵ₁ϵ₂ for |x| > 1
	{d: Number{Real: 2, E1mag: 1, E2mag: 1}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.Inf(1), E2mag: math.Inf(1), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 1, E2mag: 1}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.Inf(1), E2mag: math.Inf(1), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 2, E2mag: 2}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.Inf(1), E2mag: math.Inf(1), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 2, E2mag: 2}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.Inf(1), E2mag: math.Inf(1), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 2, E2mag: 3}, p: math.Inf(1), want: Number{Real: math.Inf(1), E1mag: math.Inf(1), E2mag: math.Inf(1), E1E2mag: math.NaN()}},

	// PowReal(x, -Inf) = +0+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| > 1
	{d: Number{Real: 2, E1mag: 0, E2mag: 0}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 0, E2mag: 0}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 1, E2mag: 1}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 1, E2mag: 1}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 2, E1mag: 2, E2mag: 2}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 3, E1mag: 2, E2mag: 2}, p: math.Inf(-1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x+yϵ₁+zϵ₂, +Inf) = +0+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| < 1
	{d: Number{Real: 0.1, E1mag: 0, E2mag: 0}, p: math.Inf(1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.1, E1mag: 0.1, E2mag: 0.1}, p: math.Inf(1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 0.2, E2mag: 0.2}, p: math.Inf(1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.5, E1mag: 0.3, E2mag: 0.5}, p: math.Inf(1), want: Number{Real: 0, E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x+0ϵ₁+0ϵ₂, -Inf) = +Inf+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| < 1
	{d: Number{Real: 0.1, E1mag: 0, E2mag: 0}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 0, E2mag: 0}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},

	// PowReal(x, -Inf) = +Inf-Infϵ₁-Infϵ₂+NaNϵ₁ϵ₂ for |x| < 1
	{d: Number{Real: 0.1, E1mag: 0.1, E2mag: 0.1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 0.1, E2mag: 0.1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.1, E1mag: 0.2, E2mag: 0.2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 0.3, E2mag: 0.2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.1, E1mag: 1, E2mag: 1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 1, E2mag: 1}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.1, E1mag: 2, E2mag: 2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},
	{d: Number{Real: 0.2, E1mag: 2, E2mag: 2}, p: math.Inf(-1), want: Number{Real: math.Inf(1), E1mag: math.Inf(-1), E2mag: math.Inf(-1), E1E2mag: math.NaN()}},

	// Handled by math.Pow tests:
	//
	// Pow(+Inf, y) = +Inf for y > 0
	// Pow(+Inf, y) = +0 for y < 0
	// Pow(-Inf, y) = Pow(-0, -y)

	// PowReal(x, y) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for finite x < 0 and finite non-integer y
	{d: Number{Real: -1, E1mag: -1, E2mag: -1}, p: 0.5, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: -1, E1mag: 2, E2mag: 2}, p: 0.5, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
	{d: Number{Real: -1, E1mag: -1, E2mag: 2}, p: 0.5, want: Number{Real: math.NaN(), E1mag: math.NaN(), E2mag: math.NaN(), E1E2mag: math.NaN()}},
}

func TestPowReal(t *testing.T) {
	const tol = 1e-15
	for _, test := range powRealTests {
		got := PowReal(test.d, test.p)
		if !sameHyperdual(got, test.want, tol) {
			t.Errorf("unexpected PowReal(%v, %v): got:%v want:%v", test.d, test.p, got, test.want)
		}
	}
}

func sameHyperdual(a, b Number, tol float64) bool {
	return same(a.Real, b.Real, tol) && same(a.E1mag, b.E1mag, tol) &&
		same(a.E2mag, b.E2mag, tol) && same(a.E1E2mag, b.E1E2mag, tol)
}

func same(a, b, tol float64) bool {
	return (math.IsNaN(a) && math.IsNaN(b)) ||
		(floats.EqualWithinAbsOrRel(a, b, tol, tol) && math.Float64bits(a)&(1<<63) == math.Float64bits(b)&(1<<63))
}
