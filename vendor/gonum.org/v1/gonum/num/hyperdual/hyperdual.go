// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hyperdual

import (
	"fmt"
	"math"
	"strings"
)

// Number is a float64 precision hyperdual number.
type Number struct {
	Real, E1mag, E2mag, E1E2mag float64
}

var (
	zero    = Number{}
	negZero = math.Float64frombits(1 << 63)
)

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
			fmt.Fprintf(fs, "%T{Real:%#v, E1mag:%#v, E2mag:%#v, E1E2mag:%#v}", d, d.Real, d.E1mag, d.E2mag, d.E1E2mag)
			return
		}
		if fs.Flag('+') {
			fmt.Fprintf(fs, "{Real:%+v, E1mag:%+v, E2mag:%+v, E1E2mag:%+v}", d.Real, d.E1mag, d.E2mag, d.E1E2mag)
			return
		}
		c = 'g'
		prec = -1
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		fre := fmtString(fs, c, prec, width, false)
		fim := fmtString(fs, c, prec, width, true)
		fmt.Fprintf(fs, fmt.Sprintf("(%s%[2]sϵ₁%[2]sϵ₂%[2]sϵ₁ϵ₂)", fre, fim), d.Real, d.E1mag, d.E2mag, d.E1E2mag)
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
		Real:    x.Real + y.Real,
		E1mag:   x.E1mag + y.E1mag,
		E2mag:   x.E2mag + y.E2mag,
		E1E2mag: x.E1E2mag + y.E1E2mag,
	}
}

// Sub returns the difference of x and y, x-y.
func Sub(x, y Number) Number {
	return Number{
		Real:    x.Real - y.Real,
		E1mag:   x.E1mag - y.E1mag,
		E2mag:   x.E2mag - y.E2mag,
		E1E2mag: x.E1E2mag - y.E1E2mag,
	}
}

// Mul returns the hyperdual product of x and y.
func Mul(x, y Number) Number {
	return Number{
		Real:    x.Real * y.Real,
		E1mag:   x.Real*y.E1mag + x.E1mag*y.Real,
		E2mag:   x.Real*y.E2mag + x.E2mag*y.Real,
		E1E2mag: x.Real*y.E1E2mag + x.E1mag*y.E2mag + x.E2mag*y.E1mag + x.E1E2mag*y.Real,
	}
}

// Inv returns the hyperdual inverse of d.
//
// Special cases are:
//	Inv(±Inf) = ±0-0ϵ₁-0ϵ₂±0ϵ₁ϵ₂
//	Inv(±0) = ±Inf-Infϵ₁-Infϵ₂±Infϵ₁ϵ₂
func Inv(d Number) Number {
	if d.Real == 0 {
		return Number{
			Real:    1 / d.Real,
			E1mag:   math.Inf(-1),
			E2mag:   math.Inf(-1),
			E1E2mag: 1 / d.Real, // Return a signed inf from a signed zero.
		}
	}
	d2 := d.Real * d.Real
	return Number{
		Real:    1 / d.Real,
		E1mag:   -d.E1mag / d2,
		E2mag:   -d.E2mag / d2,
		E1E2mag: -d.E1E2mag/d2 + 2*d.E1mag*d.E2mag/(d2*d.Real),
	}
}

// Scale returns d scaled by f.
func Scale(f float64, d Number) Number {
	return Number{Real: f * d.Real, E1mag: f * d.E1mag, E2mag: f * d.E2mag, E1E2mag: f * d.E1E2mag}
}

// Abs returns the absolute value of d.
func Abs(d Number) Number {
	if math.Float64bits(d.Real)&(1<<63) == 0 {
		return d
	}
	return Scale(-1, d)
}
