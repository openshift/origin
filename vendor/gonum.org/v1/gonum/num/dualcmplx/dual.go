// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualcmplx

import (
	"fmt"
	"math"
	"math/cmplx"
	"strings"
)

// Number is a float64 precision anti-commutative dual complex number.
type Number struct {
	Real, Dual complex128
}

var zero Number

// Format implements fmt.Formatter.
func (d Number) Format(fs fmt.State, c rune) {
	prec, pOk := fs.Precision()
	if !pOk {
		prec = -1
	}
	width, wOk := fs.Width()
	if !wOk {
		width = -1
	}
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "%T{Real:%#v, Dual:%#v}", d, d.Real, d.Dual)
			return
		}
		if fs.Flag('+') {
			fmt.Fprintf(fs, "{Real:%+v, Dual:%+v}", d.Real, d.Dual)
			return
		}
		c = 'g'
		prec = -1
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		fre := fmtString(fs, c, prec, width, false)
		fim := fmtString(fs, c, prec, width, true)
		fmt.Fprintf(fs, fmt.Sprintf("(%s+%[2]sϵ)", fre, fim), d.Real, d.Dual)
	default:
		fmt.Fprintf(fs, "%%!%c(%T=%[2]v)", c, d)
		return
	}
}

// This is horrible, but it's what we have.
func fmtString(fs fmt.State, c rune, prec, width int, wantPlus bool) string {
	var b strings.Builder
	b.WriteByte('%')
	for _, f := range "0+- " {
		if fs.Flag(int(f)) || (f == '+' && wantPlus) {
			b.WriteByte(byte(f))
		}
	}
	if width >= 0 {
		fmt.Fprint(&b, width)
	}
	if prec >= 0 {
		b.WriteByte('.')
		if prec > 0 {
			fmt.Fprint(&b, prec)
		}
	}
	b.WriteRune(c)
	return b.String()
}

// Add returns the sum of x and y.
func Add(x, y Number) Number {
	return Number{
		Real: x.Real + y.Real,
		Dual: x.Dual + y.Dual,
	}
}

// Sub returns the difference of x and y, x-y.
func Sub(x, y Number) Number {
	return Number{
		Real: x.Real - y.Real,
		Dual: x.Dual - y.Dual,
	}
}

// Mul returns the dual product of x and y, x×y.
func Mul(x, y Number) Number {
	return Number{
		Real: x.Real * y.Real,
		Dual: x.Real*y.Dual + x.Dual*cmplx.Conj(y.Real),
	}
}

// Inv returns the dual inverse of d.
func Inv(d Number) Number {
	return Number{
		Real: 1 / d.Real,
		Dual: -d.Dual / (d.Real * cmplx.Conj(d.Real)),
	}
}

// Conj returns the conjugate of d₁+d₂ϵ, d̅₁+d₂ϵ.
func Conj(d Number) Number {
	return Number{
		Real: cmplx.Conj(d.Real),
		Dual: d.Dual,
	}
}

// Scale returns d scaled by f.
func Scale(f float64, d Number) Number {
	return Number{Real: complex(f, 0) * d.Real, Dual: complex(f, 0) * d.Dual}
}

// Abs returns the absolute value of d.
func Abs(d Number) float64 {
	return cmplx.Abs(d.Real)
}

// PowReal returns d**p, the base-d exponential of p.
//
// Special cases are (in order):
//	PowReal(NaN+xϵ, ±0) = 1+NaNϵ for any x
//	Pow(0+xϵ, y) = 0+Infϵ for all y < 1.
//	Pow(0+xϵ, y) = 0 for all y > 1.
//	PowReal(x, ±0) = 1 for any x
//	PowReal(1+xϵ, y) = 1+xyϵ for any y
//	Pow(Inf, y) = +Inf+NaNϵ for y > 0
//	Pow(Inf, y) = +0+NaNϵ for y < 0
//	PowReal(x, 1) = x for any x
//	PowReal(NaN+xϵ, y) = NaN+NaNϵ
//	PowReal(x, NaN) = NaN+NaNϵ
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
		case cmplx.IsNaN(d.Real):
			return Number{Real: 1, Dual: cmplx.NaN()}
		case d.Real == 0, cmplx.IsInf(d.Real):
			return Number{Real: 1}
		}
	case p == 1:
		if cmplx.IsInf(d.Real) {
			d.Dual = cmplx.NaN()
		}
		return d
	case math.IsInf(p, 1):
		if d.Real == -1 {
			return Number{Real: 1, Dual: cmplx.NaN()}
		}
		if Abs(d) > 1 {
			if d.Dual == 0 {
				return Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}
			}
			return Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}
		}
		return Number{Real: 0, Dual: cmplx.NaN()}
	case math.IsInf(p, -1):
		if d.Real == -1 {
			return Number{Real: 1, Dual: cmplx.NaN()}
		}
		if Abs(d) > 1 {
			return Number{Real: 0, Dual: cmplx.NaN()}
		}
		if d.Dual == 0 {
			return Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}
		}
		return Number{Real: cmplx.Inf(), Dual: cmplx.Inf()}
	case math.IsNaN(p):
		return Number{Real: cmplx.NaN(), Dual: cmplx.NaN()}
	case d.Real == 0:
		if p < 1 {
			return Number{Real: d.Real, Dual: cmplx.Inf()}
		}
		return Number{Real: d.Real}
	case cmplx.IsInf(d.Real):
		if p < 0 {
			return Number{Real: 0, Dual: cmplx.NaN()}
		}
		return Number{Real: cmplx.Inf(), Dual: cmplx.NaN()}
	}
	return Pow(d, Number{Real: complex(p, 0)})
}

// Pow returns d**p, the base-d exponential of p.
func Pow(d, p Number) Number {
	return Exp(Mul(p, Log(d)))
}

// Sqrt returns the square root of d.
//
// Special cases are:
//	Sqrt(+Inf) = +Inf
//	Sqrt(±0) = (±0+Infϵ)
//	Sqrt(x < 0) = NaN
//	Sqrt(NaN) = NaN
func Sqrt(d Number) Number {
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
	fn := cmplx.Exp(d.Real)
	if imag(d.Real) == 0 {
		return Number{Real: fn, Dual: fn * d.Dual}
	}
	conj := cmplx.Conj(d.Real)
	return Number{
		Real: fn,
		Dual: ((fn - cmplx.Exp(conj)) / (d.Real - conj)) * d.Dual,
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
	fn := cmplx.Log(d.Real)
	switch {
	case d.Real == 0:
		return Number{
			Real: fn,
			Dual: complex(math.Copysign(math.Inf(1), real(d.Real)), math.NaN()),
		}
	case imag(d.Real) == 0:
		return Number{
			Real: fn,
			Dual: d.Dual / d.Real,
		}
	case cmplx.IsInf(d.Real):
		return Number{
			Real: fn,
			Dual: 0,
		}
	}
	conj := cmplx.Conj(d.Real)
	return Number{
		Real: fn,
		Dual: ((fn - cmplx.Log(conj)) / (d.Real - conj)) * d.Dual,
	}
}
