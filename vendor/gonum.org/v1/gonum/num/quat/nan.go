// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import "math"

// IsNaN returns true if any of real(q), imag(q), jmag(q), or kmag(q) is NaN
// and none are an infinity.
func IsNaN(q Number) bool {
	if math.IsInf(q.Real, 0) || math.IsInf(q.Imag, 0) || math.IsInf(q.Jmag, 0) || math.IsInf(q.Kmag, 0) {
		return false
	}
	return math.IsNaN(q.Real) || math.IsNaN(q.Imag) || math.IsNaN(q.Jmag) || math.IsNaN(q.Kmag)
}

// NaN returns a quaternion ``not-a-number'' value.
func NaN() Number {
	nan := math.NaN()
	return Number{Real: nan, Imag: nan, Jmag: nan, Kmag: nan}
}
