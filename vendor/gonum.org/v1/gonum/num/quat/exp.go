// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import "math"

// Exp returns e**q, the base-e exponential of q.
func Exp(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Exp(w))
	}
	v := Abs(uv)
	e := math.Exp(w)
	s, c := math.Sincos(v)
	return join(e*c, Scale(e*s/v, uv))
}

// Log returns the natural logarithm of q.
func Log(q Number) Number {
	w, uv := split(q)
	if uv == zero {
		return lift(math.Log(w))
	}
	v := Abs(uv)
	return join(math.Log(Abs(q)), Scale(math.Atan2(v, w)/v, uv))
}

// Pow return q**r, the base-q exponential of r.
// For generalized compatibility with math.Pow:
//      Pow(0, ±0) returns 1+0i+0j+0k
//      Pow(0, c) for real(c)<0 returns Inf+0i+0j+0k if imag(c), jmag(c), kmag(c) are zero,
//          otherwise Inf+Inf i+Inf j+Inf k.
func Pow(q, r Number) Number {
	if q == zero {
		w, uv := split(r)
		switch {
		case w == 0:
			return Number{Real: 1}
		case w < 0:
			if uv == zero {
				return Number{Real: math.Inf(1)}
			}
			return Inf()
		case w > 0:
			return zero
		}
	}
	return Exp(Mul(Log(q), r))
}

// Sqrt returns the square root of q.
func Sqrt(q Number) Number {
	if q == zero {
		return zero
	}
	return Pow(q, Number{Real: 0.5})
}
