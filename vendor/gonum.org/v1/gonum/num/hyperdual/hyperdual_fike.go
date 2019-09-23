// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Derived from code by Jeffrey A. Fike at http://adl.stanford.edu/hyperdual/

// The MIT License (MIT)
//
// Copyright (c) 2006 Jeffrey A. Fike
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package hyperdual

import "math"

// PowReal returns x**p, the base-x exponential of p.
//
// Special cases are (in order):
//	PowReal(NaN+xϵ₁+yϵ₂, ±0) = 1+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for any x and y
//	PowReal(x, ±0) = 1 for any x
//	PowReal(1+xϵ₁+yϵ₂, z) = 1+xzϵ₁+yzϵ₂+2xyzϵ₁ϵ₂ for any z
//	PowReal(NaN+xϵ₁+yϵ₂, 1) = NaN+xϵ₁+yϵ₂+NaNϵ₁ϵ₂ for any x
//	PowReal(x, 1) = x for any x
//	PowReal(NaN+xϵ₁+xϵ₂, y) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂
//	PowReal(x, NaN) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂
//	PowReal(±0, y) = ±Inf for y an odd integer < 0
//	PowReal(±0, -Inf) = +Inf
//	PowReal(±0, +Inf) = +0
//	PowReal(±0, y) = +Inf for finite y < 0 and not an odd integer
//	PowReal(±0, y) = ±0 for y an odd integer > 0
//	PowReal(±0, y) = +0 for finite y > 0 and not an odd integer
//	PowReal(-1, ±Inf) = 1
//	PowReal(x+0ϵ₁+0ϵ₂, +Inf) = +Inf+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| > 1
//	PowReal(x+xϵ₁+yϵ₂, +Inf) = +Inf+Infϵ₁+Infϵ₂+NaNϵ₁ϵ₂ for |x| > 1
//	PowReal(x, -Inf) = +0+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| > 1
//	PowReal(x+yϵ₁+zϵ₂, +Inf) = +0+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| < 1
//	PowReal(x+0ϵ₁+0ϵ₂, -Inf) = +Inf+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for |x| < 1
//	PowReal(x, -Inf) = +Inf-Infϵ₁-Infϵ₂+NaNϵ₁ϵ₂ for |x| < 1
//	PowReal(+Inf, y) = +Inf for y > 0
//	PowReal(+Inf, y) = +0 for y < 0
//	PowReal(-Inf, y) = Pow(-0, -y)
//	PowReal(x, y) = NaN+NaNϵ₁+NaNϵ₂+NaNϵ₁ϵ₂ for finite x < 0 and finite non-integer y
func PowReal(d Number, p float64) Number {
	const tol = 1e-15

	r := d.Real
	if math.Abs(r) < tol {
		if r >= 0 {
			r = tol
		}
		if r < 0 {
			r = -tol
		}
	}
	deriv := p * math.Pow(r, p-1)
	return Number{
		Real:    math.Pow(d.Real, p),
		E1mag:   d.E1mag * deriv,
		E2mag:   d.E2mag * deriv,
		E1E2mag: d.E1E2mag*deriv + p*(p-1)*d.E1mag*d.E2mag*math.Pow(r, (p-2)),
	}
}

// Pow returns x**p, the base-x exponential of p.
func Pow(d, p Number) Number {
	return Exp(Mul(p, Log(d)))
}

// Sqrt returns the square root of d.
//
// Special cases are:
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = (±0+Infϵ₁+Infϵ₂-Infϵ₁ϵ₂)
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN
func Sqrt(d Number) Number {
	if d.Real <= 0 {
		if d.Real == 0 {
			return Number{
				Real:    d.Real,
				E1mag:   math.Inf(1),
				E2mag:   math.Inf(1),
				E1E2mag: math.Inf(-1),
			}
		}
		return Number{
			Real:    math.NaN(),
			E1mag:   math.NaN(),
			E2mag:   math.NaN(),
			E1E2mag: math.NaN(),
		}
	}
	return PowReal(d, 0.5)
}

// Exp returns e**q, the base-e exponential of d.
//
// Special cases are:
//	Exp(+Inf) = +Inf
//	Exp(NaN) = NaN
// Very large values overflow to 0 or +Inf.
// Very small values underflow to 1.
func Exp(d Number) Number {
	exp := math.Exp(d.Real) // exp is also the derivative.
	return Number{
		Real:    exp,
		E1mag:   exp * d.E1mag,
		E2mag:   exp * d.E2mag,
		E1E2mag: exp * (d.E1E2mag + d.E1mag*d.E2mag),
	}
}

// Log returns the natural logarithm of d.
//
// Special cases are:
//	Log(+Inf) = (+Inf+0ϵ₁+0ϵ₂-0ϵ₁ϵ₂)
//	Log(0) = (-Inf±Infϵ₁±Infϵ₂-Infϵ₁ϵ₂)
//	Log(x < 0) = NaN
//	Log(NaN) = NaN
func Log(d Number) Number {
	switch d.Real {
	case 0:
		return Number{
			Real:    math.Log(d.Real),
			E1mag:   math.Copysign(math.Inf(1), d.Real),
			E2mag:   math.Copysign(math.Inf(1), d.Real),
			E1E2mag: math.Inf(-1),
		}
	case math.Inf(1):
		return Number{
			Real:    math.Log(d.Real),
			E1mag:   0,
			E2mag:   0,
			E1E2mag: negZero,
		}
	}
	if d.Real < 0 {
		return Number{
			Real:    math.NaN(),
			E1mag:   math.NaN(),
			E2mag:   math.NaN(),
			E1E2mag: math.NaN(),
		}
	}
	deriv1 := d.E1mag / d.Real
	deriv2 := d.E2mag / d.Real
	return Number{
		Real:    math.Log(d.Real),
		E1mag:   deriv1,
		E2mag:   deriv2,
		E1E2mag: d.E1E2mag/d.Real - (deriv1 * deriv2),
	}
}

// Sin returns the sine of d.
//
// Special cases are:
//	Sin(±0) = (±0+Nϵ₁+Nϵ₂∓0ϵ₁ϵ₂)
//	Sin(±Inf) = NaN
//	Sin(NaN) = NaN
func Sin(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: -d.Real,
		}
	}
	fn := math.Sin(d.Real)
	deriv := math.Cos(d.Real)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag - fn*d.E1mag*d.E2mag,
	}
}

// Cos returns the cosine of d.
//
// Special cases are:
//	Cos(±Inf) = NaN
//	Cos(NaN) = NaN
func Cos(d Number) Number {
	fn := math.Cos(d.Real)
	deriv := -math.Sin(d.Real)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag - fn*d.E1mag*d.E2mag,
	}
}

// Tan returns the tangent of d.
//
// Special cases are:
//	Tan(±0) = (±0+Nϵ₁+Nϵ₂±0ϵ₁ϵ₂)
//	Tan(±Inf) = NaN
//	Tan(NaN) = NaN
func Tan(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: d.Real,
		}
	}
	fn := math.Tan(d.Real)
	deriv := 1 + fn*fn
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(2*fn*deriv),
	}
}

// Asin returns the inverse sine of d.
//
// Special cases are:
//	Asin(±0) = (±0+Nϵ₁+Nϵ₂±0ϵ₁ϵ₂)
//	Asin(±1) = (±Inf+Infϵ₁+Infϵ₂±Infϵ₁ϵ₂)
//	Asin(x) = NaN if x < -1 or x > 1
func Asin(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: d.Real,
		}
	} else if m := math.Abs(d.Real); m >= 1 {
		if m == 1 {
			return Number{
				Real:    math.Asin(d.Real),
				E1mag:   math.Inf(1),
				E2mag:   math.Inf(1),
				E1E2mag: math.Copysign(math.Inf(1), d.Real),
			}
		}
		return Number{
			Real:    math.NaN(),
			E1mag:   math.NaN(),
			E2mag:   math.NaN(),
			E1E2mag: math.NaN(),
		}
	}
	fn := math.Asin(d.Real)
	deriv1 := 1 - d.Real*d.Real
	deriv := 1 / math.Sqrt(deriv1)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(d.Real*math.Pow(deriv1, -1.5)),
	}
}

// Acos returns the inverse cosine of d.
//
// Special cases are:
//	Acos(-1) = (Pi-Infϵ₁-Infϵ₂+Infϵ₁ϵ₂)
//	Acos(1) = (0-Infϵ₁-Infϵ₂-Infϵ₁ϵ₂)
//	Acos(x) = NaN if x < -1 or x > 1
func Acos(d Number) Number {
	if m := math.Abs(d.Real); m >= 1 {
		if m == 1 {
			return Number{
				Real:    math.Acos(d.Real),
				E1mag:   math.Inf(-1),
				E2mag:   math.Inf(-1),
				E1E2mag: math.Copysign(math.Inf(1), -d.Real),
			}
		}
		return Number{
			Real:    math.NaN(),
			E1mag:   math.NaN(),
			E2mag:   math.NaN(),
			E1E2mag: math.NaN(),
		}
	}
	fn := math.Acos(d.Real)
	deriv1 := 1 - d.Real*d.Real
	deriv := -1 / math.Sqrt(deriv1)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(-d.Real*math.Pow(deriv1, -1.5)),
	}
}

// Atan returns the inverse tangent of d.
//
// Special cases are:
//	Atan(±0) = (±0+Nϵ₁+Nϵ₂∓0ϵ₁ϵ₂)
//	Atan(±Inf) = (±Pi/2+0ϵ₁+0ϵ₂∓0ϵ₁ϵ₂)
func Atan(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: -d.Real,
		}
	}
	fn := math.Atan(d.Real)
	deriv1 := 1 + d.Real*d.Real
	deriv := 1 / deriv1
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(-2*d.Real/(deriv1*deriv1)),
	}
}
