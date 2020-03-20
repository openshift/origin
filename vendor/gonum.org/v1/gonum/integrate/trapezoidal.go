// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import "sort"

// Trapezoidal returns an approximate value of the integral
//  \int_a^b f(x) dx
// computed using the trapezoidal rule. The function f is given as a slice of
// samples evaluated at locations in x, that is,
//  f[i] = f(x[i]), x[0] = a, x[len(x)-1] = b
// The slice x must be sorted in strictly increasing order. x and f must be of
// equal length and the length must be at least 2.
//
// The trapezoidal rule approximates f by a piecewise linear function and
// estimates
//  \int_x[i]^x[i+1] f(x) dx
// as
//  (x[i+1] - x[i]) * (f[i] + f[i+1])/2
// More details on the trapezoidal rule can be found at:
// https://en.wikipedia.org/wiki/Trapezoidal_rule
func Trapezoidal(x, f []float64) float64 {
	n := len(x)
	switch {
	case len(f) != n:
		panic("integrate: slice length mismatch")
	case n < 2:
		panic("integrate: input data too small")
	case !sort.Float64sAreSorted(x):
		panic("integrate: input must be sorted")
	}

	integral := 0.0
	for i := 0; i < n-1; i++ {
		integral += 0.5 * (x[i+1] - x[i]) * (f[i+1] + f[i])
	}

	return integral
}
