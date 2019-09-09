// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import "math"

// Sin returns the sine of q.
func Sin(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Sin(w))
	}
	v := Abs(uv)
	s, c := math.Sincos(w)
	sh, ch := sinhcosh(v)
	return join(s*ch, Scale(c*sh/v, uv))
}

// Sinh returns the hyperbolic sine of q.
func Sinh(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Sinh(w))
	}
	v := Abs(uv)
	s, c := math.Sincos(v)
	sh, ch := sinhcosh(w)
	return join(c*sh, scale(s*ch/v, uv))
}

// Cos returns the cosine of q.
func Cos(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Cos(w))
	}
	v := Abs(uv)
	s, c := math.Sincos(w)
	sh, ch := sinhcosh(v)
	return join(c*ch, Scale(-s*sh/v, uv))
}

// Cosh returns the hyperbolic cosine of q.
func Cosh(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Cosh(w))
	}
	v := Abs(uv)
	s, c := math.Sincos(v)
	sh, ch := sinhcosh(w)
	return join(c*ch, scale(s*sh/v, uv))
}

// Tan returns the tangent of q.
func Tan(q Number) Number {
	d := Cos(q)
	if d == zero {
		return Inf()
	}
	return Mul(Sin(q), Inv(d))
}

// Tanh returns the hyperbolic tangent of q.
func Tanh(q Number) Number {
	if math.IsInf(q.Real, 1) {
		r := Number{Real: 1}
		// Change signs dependent on imaginary parts.
		r.Imag *= math.Sin(2 * q.Imag)
		r.Jmag *= math.Sin(2 * q.Jmag)
		r.Kmag *= math.Sin(2 * q.Kmag)
		return r
	}
	d := Cosh(q)
	if d == zero {
		return Inf()
	}
	return Mul(Sinh(q), Inv(d))
}

// Asin returns the inverse sine of q.
func Asin(q Number) Number {
	_, uv := split(q)
	if uv == zero {
		return lift(math.Asin(q.Real))
	}
	u := unit(uv)
	return Mul(Scale(-1, u), Log(Add(Mul(u, q), Sqrt(Sub(Number{Real: 1}, Mul(q, q))))))
}

// Asinh returns the inverse hyperbolic sine of q.
func Asinh(q Number) Number {
	return Log(Add(q, Sqrt(Add(Number{Real: 1}, Mul(q, q)))))
}

// Acos returns the inverse cosine of q.
func Acos(q Number) Number {
	w, uv := split(Asin(q))
	return join(math.Pi/2-w, Scale(-1, uv))
}

// Acosh returns the inverse hyperbolic cosine of q.
func Acosh(q Number) Number {
	w := Acos(q)
	_, uv := split(w)
	if uv == zero {
		return w
	}
	w = Mul(w, unit(uv))
	if w.Real < 0 {
		w = Scale(-1, w)
	}
	return w
}

// Atan returns the inverse tangent of q.
func Atan(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Atan(w))
	}
	u := unit(uv)
	return Mul(Mul(lift(0.5), u), Log(Mul(Add(u, q), Inv(Sub(u, q)))))
}

// Atanh returns the inverse hyperbolic tangent of q.
func Atanh(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Atanh(w))
	}
	u := unit(uv)
	return Mul(Scale(-1, u), Atan(Mul(u, q)))
}

// calculate sinh and cosh
func sinhcosh(x float64) (sh, ch float64) {
	if math.Abs(x) <= 0.5 {
		return math.Sinh(x), math.Cosh(x)
	}
	e := math.Exp(x)
	ei := 0.5 / e
	e *= 0.5
	return e - ei, e + ei
}

// scale returns q scaled by f, except that inf×0 is 0.
func scale(f float64, q Number) Number {
	if f == 0 {
		return Number{}
	}
	if q.Real != 0 {
		q.Real *= f
	}
	if q.Imag != 0 {
		q.Imag *= f
	}
	if q.Jmag != 0 {
		q.Jmag *= f
	}
	if q.Kmag != 0 {
		q.Kmag *= f
	}
	return q
}
