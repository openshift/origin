// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

// Conj returns the quaternion conjugate of q.
func Conj(q Number) Number {
	return Number{Real: q.Real, Imag: -q.Imag, Jmag: -q.Jmag, Kmag: -q.Kmag}
}

// Inv returns the quaternion inverse of q.
func Inv(q Number) Number {
	if IsInf(q) {
		return zero
	}
	a := Abs(q)
	return Scale(1/(a*a), Conj(q))
}
