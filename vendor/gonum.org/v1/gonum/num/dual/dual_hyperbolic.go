// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dual

import "math"

// Sinh returns the hyperbolic sine of d.
//
// Special cases are:
//	Sinh(±0) = (±0+Nϵ)
//	Sinh(±Inf) = ±Inf
//	Sinh(NaN) = NaN
func Sinh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real: d.Real,
			Emag: d.Emag,
		}
	}
	if math.IsInf(d.Real, 0) {
		return Number{
			Real: d.Real,
			Emag: math.Inf(1),
		}
	}
	fn := math.Sinh(d.Real)
	deriv := math.Cosh(d.Real)
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}

// Cosh returns the hyperbolic cosine of d.
//
// Special cases are:
//	Cosh(±0) = 1
//	Cosh(±Inf) = +Inf
//	Cosh(NaN) = NaN
func Cosh(d Number) Number {
	if math.IsInf(d.Real, 0) {
		return Number{
			Real: math.Inf(1),
			Emag: d.Real,
		}
	}
	fn := math.Cosh(d.Real)
	deriv := math.Sinh(d.Real)
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}

// Tanh returns the hyperbolic tangent of d.
//
// Special cases are:
//	Tanh(±0) = (±0+Nϵ)
//	Tanh(±Inf) = (±1+0ϵ)
//	Tanh(NaN) = NaN
func Tanh(d Number) Number {
	switch d.Real {
	case 0:
		return Number{
			Real: d.Real,
			Emag: d.Emag,
		}
	case math.Inf(1):
		return Number{
			Real: 1,
			Emag: 0,
		}
	case math.Inf(-1):
		return Number{
			Real: -1,
			Emag: 0,
		}
	}
	fn := math.Tanh(d.Real)
	deriv := 1 - fn*fn
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}

// Asinh returns the inverse hyperbolic sine of d.
//
// Special cases are:
//	Asinh(±0) = (±0+Nϵ)
//	Asinh(±Inf) = ±Inf
//	Asinh(NaN) = NaN
func Asinh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real: d.Real,
			Emag: d.Emag,
		}
	}
	fn := math.Asinh(d.Real)
	deriv := 1 / math.Sqrt(d.Real*d.Real+1)
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}

// Acosh returns the inverse hyperbolic cosine of d.
//
// Special cases are:
//	Acosh(+Inf) = +Inf
//	Acosh(1) = (0+Infϵ)
//	Acosh(x) = NaN if x < 1
//	Acosh(NaN) = NaN
func Acosh(d Number) Number {
	if d.Real <= 1 {
		if d.Real == 1 {
			return Number{
				Real: 0,
				Emag: math.Inf(1),
			}
		}
		return Number{
			Real: math.NaN(),
			Emag: math.NaN(),
		}
	}
	fn := math.Acosh(d.Real)
	deriv := 1 / math.Sqrt(d.Real*d.Real-1)
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}

// Atanh returns the inverse hyperbolic tangent of d.
//
// Special cases are:
//	Atanh(1) = +Inf
//	Atanh(±0) = (±0+Nϵ)
//	Atanh(-1) = -Inf
//	Atanh(x) = NaN if x < -1 or x > 1
//	Atanh(NaN) = NaN
func Atanh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real: d.Real,
			Emag: d.Emag,
		}
	}
	if math.Abs(d.Real) == 1 {
		return Number{
			Real: math.Inf(int(d.Real)),
			Emag: math.NaN(),
		}
	}
	fn := math.Atanh(d.Real)
	deriv := 1 / (1 - d.Real*d.Real)
	return Number{
		Real: fn,
		Emag: deriv * d.Emag,
	}
}
