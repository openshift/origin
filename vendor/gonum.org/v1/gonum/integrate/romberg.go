// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import (
	"math"
	"math/bits"
)

// Romberg returns an approximate value of the integral
//  \int_a^b f(x)dx
// computed using the Romberg's method. The function f is given
// as a slice of equally-spaced samples, that is,
//  f[i] = f(a + i*dx)
// and dx is the spacing between the samples.
//
// The length of f must be 2^k + 1, where k is a positive integer,
// and dx must be positive.
//
// See https://en.wikipedia.org/wiki/Romberg%27s_method for a description of
// the algorithm.
func Romberg(f []float64, dx float64) float64 {
	if len(f) < 3 {
		panic("integral: invalid slice length: must be at least 3")
	}

	if dx <= 0 {
		panic("integral: invalid spacing: must be larger than 0")
	}

	n := len(f) - 1
	k := bits.Len(uint(n - 1))

	if len(f) != 1<<uint(k)+1 {
		panic("integral: invalid slice length: must be 2^k + 1")
	}

	work := make([]float64, 2*(k+1))
	prev := work[:k+1]
	curr := work[k+1:]

	h := dx * float64(n)
	prev[0] = (f[0] + f[n]) * 0.5 * h

	step := n
	for i := 1; i <= k; i++ {
		h /= 2
		step /= 2
		var estimate float64
		for j := 0; j < n/2; j += step {
			estimate += f[2*j+step]
		}

		curr[0] = estimate*h + 0.5*prev[0]
		for j := 1; j <= i; j++ {
			factor := math.Pow(4, float64(j))
			curr[j] = (factor*curr[j-1] - prev[j-1]) / (factor - 1)
		}

		prev, curr = curr, prev
	}
	return prev[k]
}
