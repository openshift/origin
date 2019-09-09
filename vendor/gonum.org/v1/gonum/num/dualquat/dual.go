// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualquat

import (
	"fmt"
	"strings"

	"gonum.org/v1/gonum/num/dual"
	"gonum.org/v1/gonum/num/quat"
)

// Number is a float64 precision dual quaternion. A dual quaternion
// is a hypercomplex number composed of two quaternions, q₀+q₂ϵ,
// where ϵ²=0, but ϵ≠0. Here, q₀ is termed the real and q₂ the dual.
type Number struct {
	Real, Dual quat.Number
}

var (
	zero     Number
	zeroQuat quat.Number
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
		Real: quat.Add(x.Real, y.Real),
		Dual: quat.Add(x.Dual, y.Dual),
	}
}

// Sub returns the difference of x and y, x-y.
func Sub(x, y Number) Number {
	return Number{
		Real: quat.Sub(x.Real, y.Real),
		Dual: quat.Sub(x.Dual, y.Dual),
	}
}

// Mul returns the dual product of x and y.
func Mul(x, y Number) Number {
	return Number{
		Real: quat.Mul(x.Real, y.Real),
		Dual: quat.Add(quat.Mul(x.Real, y.Dual), quat.Mul(x.Dual, y.Real)),
	}
}

// Inv returns the dual inverse of d.
func Inv(d Number) Number {
	return Number{
		Real: quat.Inv(d.Real),
		Dual: quat.Scale(-1, quat.Mul(d.Dual, quat.Inv(quat.Mul(d.Real, d.Real)))),
	}
}

// Conj returns the dual quaternion conjugate of d₁+d₂ϵ, d̅₁-d̅₂ϵ.
func Conj(d Number) Number {
	return Number{
		Real: quat.Conj(d.Real),
		Dual: quat.Scale(-1, quat.Conj(d.Dual)),
	}
}

// ConjDual returns the dual conjugate of d₁+d₂ϵ, d₁-d₂ϵ.
func ConjDual(d Number) Number {
	return Number{
		Real: d.Real,
		Dual: quat.Scale(-1, d.Dual),
	}
}

// ConjQuat returns the quaternion conjugate of d₁+d₂ϵ, d̅₁+d̅₂ϵ.
func ConjQuat(d Number) Number {
	return Number{
		Real: quat.Conj(d.Real),
		Dual: quat.Conj(d.Dual),
	}
}

// Scale returns d scaled by f.
func Scale(f float64, d Number) Number {
	return Number{Real: quat.Scale(f, d.Real), Dual: quat.Scale(f, d.Dual)}
}

// Abs returns the absolute value of d.
func Abs(d Number) dual.Number {
	return dual.Number{
		Real: quat.Abs(d.Real),
		Emag: quat.Abs(d.Dual),
	}
}

func addRealQuat(r float64, q quat.Number) quat.Number {
	q.Real += r
	return q
}

func addQuatReal(q quat.Number, r float64) quat.Number {
	q.Real += r
	return q
}

func subRealQuat(r float64, q quat.Number) quat.Number {
	q.Real = r - q.Real
	return q
}

func subQuatReal(q quat.Number, r float64) quat.Number {
	q.Real -= r
	return q
}
