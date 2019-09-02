// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hyperdual

import "math"

// Sinh returns the hyperbolic sine of d.
//
// Special cases are:
//	Sinh(±0) = (±0+Nϵ₁+Nϵ₂±0ϵ₁ϵ₂)
//	Sinh(±Inf) = ±Inf
//	Sinh(NaN) = NaN
func Sinh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E1mag,
			E1E2mag: d.Real,
		}
	}
	if math.IsInf(d.Real, 0) {
		return Number{
			Real:    d.Real,
			E1mag:   math.Inf(1),
			E2mag:   math.Inf(1),
			E1E2mag: d.Real,
		}
	}
	fn := math.Sinh(d.Real)
	deriv := math.Cosh(d.Real)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + fn*d.E1mag*d.E2mag,
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
			Real:    math.Inf(1),
			E1mag:   d.Real,
			E2mag:   d.Real,
			E1E2mag: math.Inf(1),
		}
	}
	fn := math.Cosh(d.Real)
	deriv := math.Sinh(d.Real)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + fn*d.E1mag*d.E2mag,
	}
}

// Tanh returns the hyperbolic tangent of d.
//
// Special cases are:
//	Tanh(±0) = (±0+Nϵ₁+Nϵ₂∓0ϵ₁ϵ₂)
//	Tanh(±Inf) = (±1+0ϵ₁+0ϵ₂∓0ϵ₁ϵ₂)
//	Tanh(NaN) = NaN
func Tanh(d Number) Number {
	switch d.Real {
	case 0:
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: -d.Real,
		}
	case math.Inf(1):
		return Number{
			Real:    1,
			E1mag:   0,
			E2mag:   0,
			E1E2mag: negZero,
		}
	case math.Inf(-1):
		return Number{
			Real:    -1,
			E1mag:   0,
			E2mag:   0,
			E1E2mag: 0,
		}
	}
	fn := math.Tanh(d.Real)
	deriv := 1 - fn*fn
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag - d.E1mag*d.E2mag*(2*fn*deriv),
	}
}

// Asinh returns the inverse hyperbolic sine of d.
//
// Special cases are:
//	Asinh(±0) = (±0+Nϵ₁+Nϵ₂∓0ϵ₁ϵ₂)
//	Asinh(±Inf) = ±Inf
//	Asinh(NaN) = NaN
func Asinh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: -d.Real,
		}
	}
	fn := math.Asinh(d.Real)
	deriv1 := d.Real*d.Real + 1
	deriv := 1 / math.Sqrt(deriv1)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(-d.Real*(deriv/deriv1)),
	}
}

// Acosh returns the inverse hyperbolic cosine of d.
//
// Special cases are:
//	Acosh(+Inf) = +Inf
//	Acosh(1) = (0+Infϵ₁+Infϵ₂-Infϵ₁ϵ₂)
//	Acosh(x) = NaN if x < 1
//	Acosh(NaN) = NaN
func Acosh(d Number) Number {
	if d.Real <= 1 {
		if d.Real == 1 {
			return Number{
				Real:    0,
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
	fn := math.Acosh(d.Real)
	deriv1 := d.Real*d.Real - 1
	deriv := 1 / math.Sqrt(deriv1)
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(-d.Real*(deriv/deriv1)),
	}
}

// Atanh returns the inverse hyperbolic tangent of d.
//
// Special cases are:
//	Atanh(1) = +Inf
//	Atanh(±0) = (±0+Nϵ₁+Nϵ₂±0ϵ₁ϵ₂)
//	Atanh(-1) = -Inf
//	Atanh(x) = NaN if x < -1 or x > 1
//	Atanh(NaN) = NaN
func Atanh(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    d.Real,
			E1mag:   d.E1mag,
			E2mag:   d.E2mag,
			E1E2mag: d.Real,
		}
	}
	if math.Abs(d.Real) == 1 {
		return Number{
			Real:    math.Inf(int(d.Real)),
			E1mag:   math.NaN(),
			E2mag:   math.NaN(),
			E1E2mag: math.Inf(int(d.Real)),
		}
	}
	fn := math.Atanh(d.Real)
	deriv1 := 1 - d.Real*d.Real
	deriv := 1 / deriv1
	return Number{
		Real:    fn,
		E1mag:   deriv * d.E1mag,
		E2mag:   deriv * d.E2mag,
		E1E2mag: deriv*d.E1E2mag + d.E1mag*d.E2mag*(2*d.Real/(deriv1*deriv1)),
	}
}
