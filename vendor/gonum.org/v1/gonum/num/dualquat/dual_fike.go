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

package dualquat

import (
	"math"

	"gonum.org/v1/gonum/num/quat"
)

// PowReal returns d**p, the base-d exponential of p.
//
// Special cases are (in order):
//	PowReal(NaN+xϵ, ±0) = 1+NaNϵ for any x
//	PowReal(x, ±0) = 1 for any x
//	PowReal(1+xϵ, y) = 1+xyϵ for any y
//	PowReal(x, 1) = x for any x
//	PowReal(NaN+xϵ, y) = NaN+NaNϵ
//	PowReal(x, NaN) = NaN+NaNϵ
//	PowReal(±0, y) = ±Inf for y an odd integer < 0
//	PowReal(±0, -Inf) = +Inf
//	PowReal(±0, +Inf) = +0
//	PowReal(±0, y) = +Inf for finite y < 0 and not an odd integer
//	PowReal(±0, y) = ±0 for y an odd integer > 0
//	PowReal(±0, y) = +0 for finite y > 0 and not an odd integer
//	PowReal(-1, ±Inf) = 1
//	PowReal(x+0ϵ, +Inf) = +Inf+NaNϵ for |x| > 1
//	PowReal(x+yϵ, +Inf) = +Inf for |x| > 1
//	PowReal(x, -Inf) = +0+NaNϵ for |x| > 1
//	PowReal(x, +Inf) = +0+NaNϵ for |x| < 1
//	PowReal(x+0ϵ, -Inf) = +Inf+NaNϵ for |x| < 1
//	PowReal(x, -Inf) = +Inf-Infϵ for |x| < 1
//	PowReal(+Inf, y) = +Inf for y > 0
//	PowReal(+Inf, y) = +0 for y < 0
//	PowReal(-Inf, y) = Pow(-0, -y)
func PowReal(d Number, p float64) Number {
	switch {
	case p == 0:
		switch {
		case quat.IsNaN(d.Real):
			return Number{Real: quat.Number{Real: 1}, Dual: quat.NaN()}
		case d.Real == zeroQuat, quat.IsInf(d.Real):
			return Number{Real: quat.Number{Real: 1}}
		}
	case p == 1:
		return d
	case math.IsInf(p, 1):
		if Abs(d).Real > 1 {
			if d.Dual == zeroQuat {
				return Number{Real: quat.Inf(), Dual: quat.NaN()}
			}
			return Number{Real: quat.Inf(), Dual: quat.Inf()}
		}
		return Number{Real: zeroQuat, Dual: quat.NaN()}
	case math.IsInf(p, -1):
		if Abs(d).Real > 1 {
			return Number{Real: zeroQuat, Dual: quat.NaN()}
		}
		if d.Dual == zeroQuat {
			return Number{Real: quat.Inf(), Dual: quat.NaN()}
		}
		return Number{Real: quat.Inf(), Dual: quat.Inf()}
	}
	deriv := quat.Mul(quat.Number{Real: p}, quat.Pow(d.Real, quat.Number{Real: p - 1}))
	return Number{
		Real: quat.Pow(d.Real, quat.Number{Real: p}),
		Dual: quat.Mul(d.Dual, deriv),
	}
}

// Pow return d**p, the base-d exponential of p.
func Pow(d, p Number) Number {
	return Exp(Mul(p, Log(d)))
}

// Sqrt returns the square root of d
//
// Special cases are:
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = (±0+Infϵ)
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN
func Sqrt(d Number) Number {
	return PowReal(d, 0.5)
}

// Exp returns e**d, the base-e exponential of d.
//
// Special cases are:
//	Exp(+Inf) = +Inf
//	Exp(NaN) = NaN
// Very large values overflow to 0 or +Inf.
// Very small values underflow to 1.
func Exp(d Number) Number {
	fnDeriv := quat.Exp(d.Real)
	return Number{
		Real: fnDeriv,
		Dual: quat.Mul(fnDeriv, d.Dual),
	}
}

// Log returns the natural logarithm of d.
//
// Special cases are:
//	Log(+Inf) = (+Inf+0ϵ)
//	Log(0) = (-Inf±Infϵ)
//	Log(x < 0) = NaN
//	Log(NaN) = NaN
func Log(d Number) Number {
	switch {
	case d.Real == zeroQuat:
		return Number{
			Real: quat.Log(d.Real),
			Dual: quat.Inf(),
		}
	case quat.IsInf(d.Real):
		return Number{
			Real: quat.Log(d.Real),
			Dual: zeroQuat,
		}
	}
	return Number{
		Real: quat.Log(d.Real),
		Dual: quat.Mul(d.Dual, quat.Inv(d.Real)),
	}
}
