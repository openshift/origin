// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dual

import (
	"fmt"
	"math"
	"strings"
)

// Number is a float64 precision dual number.
type Number struct {
	Real, Emag float64
}

var zero = Number{}

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
			fmt.Fprintf(fs, "%T{Real:%#v, Emag:%#v}", d, d.Real, d.Emag)
			return
		}
		if fs.Flag('+') {
			fmt.Fprintf(fs, "{Real:%+v, Emag:%+v}", d.Real, d.Emag)
			return
		}
		c = 'g'
		prec = -1
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		fre := fmtString(fs, c, prec, width, false)
		fim := fmtString(fs, c, prec, width, true)
		fmt.Fprintf(fs, fmt.Sprintf("(%s%[2]sϵ)", fre, fim), d.Real, d.Emag)
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
		Emag: x.Emag + y.Emag,
	}
}

// Sub returns the difference of x and y, x-y.
func Sub(x, y Number) Number {
	return Number{
		Real: x.Real - y.Real,
		Emag: x.Emag - y.Emag,
	}
}

// Mul returns the dual product of x and y.
func Mul(x, y Number) Number {
	return Number{
		Real: x.Real * y.Real,
		Emag: x.Real*y.Emag + x.Emag*y.Real,
	}
}

// Inv returns the dual inverse of d.
//
// Special cases are:
//	Inv(±Inf) = ±0-0ϵ
//	Inv(±0) = ±Inf-Infϵ
func Inv(d Number) Number {
	d2 := d.Real * d.Real
	return Number{
		Real: 1 / d.Real,
		Emag: -d.Emag / d2,
	}
}

// Scale returns d scaled by f.
func Scale(f float64, d Number) Number {
	return Number{Real: f * d.Real, Emag: f * d.Emag}
}

// Abs returns the absolute value of d.
func Abs(d Number) Number {
	if !math.Signbit(d.Real) {
		return d
	}
	return Scale(-1, d)
}
