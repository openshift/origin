// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import "sort"

// Trapezoidal estimates the integral of a function f
//  \int_a^b f(x) dx
// from a set of evaluations of the function using the trapezoidal rule.
// The trapezoidal rule makes piecewise linear approximations to the function,
// and estimates
//  \int_x[i]^x[i+1] f(x) dx
// as
//  (x[i+1] - x[i]) * (f[i] + f[i+1])/2
// where f[i] is the value of the function at x[i].
// More details on the trapezoidal rule can be found at:
// https://en.wikipedia.org/wiki/Trapezoidal_rule
//
// The (x,f) input data points must be sorted along x.
// One can use github.com/gonum/stat.SortWeighted to do that.
// The x and f slices must be of equal length and have length > 1.
func Trapezoidal(x, f []float64) float64 {
	switch {
	case len(x) != len(f):
		panic("integrate: slice length mismatch")
	case len(x) < 2:
		panic("integrate: input data too small")
	case !sort.Float64sAreSorted(x):
		panic("integrate: input must be sorted")
	}

	integral := 0.0
	for i := 0; i < len(x)-1; i++ {
		integral += 0.5 * (x[i+1] - x[i]) * (f[i+1] + f[i])
	}

	return integral
}
