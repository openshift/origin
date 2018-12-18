// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a translation of the FFTPACK cost functions by
// Paul N Swarztrauber, placed in the public domain at
// http://www.netlib.org/fftpack/.

package fftpack

import "math"

// Costi initializes the array work which is used in subroutine
// Cost. The prime factorization of n together with a tabulation
// of the trigonometric functions are computed and stored in work.
//
// Input parameter
//
// n       The length of the sequence to be transformed. The method
//         is most efficient when n-1 is a product of small primes.
//
// Output parameters
//
// work    A work array which must be dimensioned at least 3*n.
//         Different work arrays are required for different values
//         of n. The contents of work must not be changed between
//         calls of Cost.
//
// ifac    An integer work array of length at least 15.
func Costi(n int, work []float64, ifac []int) {
	if len(work) < 3*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n < 4 {
		return
	}
	dt := math.Pi / float64(n-1)
	for k := 1; k < n/2; k++ {
		fk := float64(k)
		work[k] = 2 * math.Sin(fk*dt)
		work[n-k-1] = 2 * math.Cos(fk*dt)
	}
	Rffti(n-1, work[n:], ifac)
}

// Cost computes the Discrete Fourier Cosine Transform of an even
// sequence x(i). The transform is defined below at output parameter x.
//
// Cost is the unnormalized inverse of itself since a call of Cost
// followed by another call of Cost will multiply the input sequence
// x by 2*(n-1). The transform is defined below at output parameter x
//
// The array work which is used by subroutine Cost must be
// initialized by calling subroutine Costi(n,work).
//
// Input parameters
//
// n       The length of the sequence x. n must be greater than 1.
//         The method is most efficient when n-1 is a product of
//         small primes.
//
// x       An array which contains the sequence to be transformed.
//
// work    A work array which must be dimensioned at least 3*n
//         in the program that calls Cost. The work array must be
//         initialized by calling subroutine Costi(n,work) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//
// ifac    An integer work array of length at least 15.
//
// Output parameters
//
// x       for i=1,...,n
//           x(i) = x(1)+(-1)**(i-1)*x(n)
//             + the sum from k=2 to k=n-1
//               2*x(k)*cos((k-1)*(i-1)*pi/(n-1))
//
//         A call of Cost followed by another call of
//         Cost will multiply the sequence x by 2*(n-1).
//         Hence Cost is the unnormalized inverse
//         of itself.
//
// work    Contains initialization calculations which must not be
//         destroyed between calls of Cost.
//
// ifac    An integer work array of length at least 15.
func Cost(n int, x, work []float64, ifac []int) {
	if len(x) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 3*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n < 2 {
		return
	}
	switch n {
	case 2:
		x[0], x[1] = x[0]+x[1], x[0]-x[1]
	case 3:
		x1p3 := x[0] + x[2]
		tx2 := 2 * x[1]
		x[1] = x[0] - x[2]
		x[0] = x1p3 + tx2
		x[2] = x1p3 - tx2
	default:
		c1 := x[0] - x[n-1]
		x[0] += x[n-1]
		for k := 1; k < n/2; k++ {
			kc := n - k - 1
			t1 := x[k] + x[kc]
			t2 := x[k] - x[kc]
			c1 += work[kc] * t2
			t2 *= work[k]
			x[k] = t1 - t2
			x[kc] = t1 + t2
		}
		if n%2 != 0 {
			x[n/2] *= 2
		}
		Rfftf(n-1, x, work[n:], ifac)
		xim2 := x[1]
		x[1] = c1
		for i := 3; i < n; i += 2 {
			xi := x[i]
			x[i] = x[i-2] - x[i-1]
			x[i-1] = xim2
			xim2 = xi
		}
		if n%2 != 0 {
			x[n-1] = xim2
		}
	}
}
