// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import "math"

// IsInf returns true if any of real(q), imag(q), jmag(q), or kmag(q) is an infinity.
func IsInf(q Number) bool {
	return math.IsInf(q.Real, 0) || math.IsInf(q.Imag, 0) || math.IsInf(q.Jmag, 0) || math.IsInf(q.Kmag, 0)
}

// Inf returns a quaternion infinity, quaternion(+Inf, +Inf, +Inf, +Inf).
func Inf() Number {
	inf := math.Inf(1)
	return Number{Real: inf, Imag: inf, Jmag: inf, Kmag: inf}
}
