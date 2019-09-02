// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import "math"

// Abs returns the absolute value (also called the modulus) of q.
func Abs(q Number) float64 {
	// Special cases.
	switch {
	case IsInf(q):
		return math.Inf(1)
	case IsNaN(q):
		return math.NaN()
	}

	r, i, j, k := q.Real, q.Imag, q.Jmag, q.Kmag
	if r < 0 {
		r = -r
	}
	if i < 0 {
		i = -i
	}
	if j < 0 {
		j = -j
	}
	if k < 0 {
		k = -k
	}
	if r < i {
		r, i = i, r
	}
	if r < j {
		r, j = j, r
	}
	if r < k {
		r, k = k, r
	}
	if r == 0 {
		return 0
	}
	i /= r
	j /= r
	k /= r
	return r * math.Sqrt(1+i*i+j*j+k*k)
}
