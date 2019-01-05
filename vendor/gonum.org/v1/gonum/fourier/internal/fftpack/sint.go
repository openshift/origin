// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a translation of the FFTPACK sint functions by
// Paul N Swarztrauber, placed in the public domain at
// http://www.netlib.org/fftpack/.

package fftpack

import "math"

// Sinti initializes the array work which is used in subroutine Sint.
// The prime factorization of n together with a tabulation of the
// trigonometric functions are computed and stored in work.
//
// Input parameter
//
// n       The length of the sequence to be transformed. The method
//         is most efficient when n+1 is a product of small primes.
//
// Output parameter
//
// work    A work array with at least ceil(2.5*n) locations.
//         Different work arrays are required for different values
//         of n. The contents of work must not be changed between
//         calls of Sint.
//
// ifac    An integer work array of length at least 15.
func Sinti(n int, work []float64, ifac []int) {
	if len(work) < 5*(n+1)/2 {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n <= 1 {
		return
	}
	dt := math.Pi / float64(n+1)
	for k := 0; k < n/2; k++ {
		work[k] = 2 * math.Sin(float64(k+1)*dt)
	}
	Rffti(n+1, work[n/2:], ifac)
}

// Sint computes the Discrete Fourier Sine Transform of an odd
// sequence x(i). The transform is defined below at output parameter x.
//
// Sint is the unnormalized inverse of itself since a call of Sint
// followed by another call of Sint will multiply the input sequence
// x by 2*(n+1).
//
// The array work which is used by subroutine Sint must be
// initialized by calling subroutine Sinti(n,work).
//
// Input parameters
//
// n       The length of the sequence to be transformed. The method
//         is most efficient when n+1 is the product of small primes.
//
// x       An array which contains the sequence to be transformed.
//
//
// work    A work array with dimension at least ceil(2.5*n)
//         in the program that calls Sint. The work array must be
//         initialized by calling subroutine Sinti(n,work) and a
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
//           x(i)= the sum from k=1 to k=n
//             2*x(k)*sin(k*i*pi/(n+1))
//
//         A call of Sint followed by another call of
//         Sint will multiply the sequence x by 2*(n+1).
//         Hence Sint is the unnormalized inverse
//         of itself.
//
// work    Contains initialization calculations which must not be
//         destroyed between calls of Sint.
// ifac    Contains initialization calculations which must not be
//         destroyed between calls of Sint.
func Sint(n int, x, work []float64, ifac []int) {
	if len(x) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 5*(n+1)/2 {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 0 {
		return
	}
	sint1(n, x, work, work[n/2:], work[n/2+n+1:], ifac)
}

func sint1(n int, war, was, xh, x []float64, ifac []int) {
	const sqrt3 = 1.73205080756888

	for i := 0; i < n; i++ {
		xh[i] = war[i]
		war[i] = x[i]
	}

	switch n {
	case 1:
		xh[0] *= 2
	case 2:
		xh[0], xh[1] = sqrt3*(xh[0]+xh[1]), sqrt3*(xh[0]-xh[1])
	default:
		x[0] = 0
		for k := 0; k < n/2; k++ {
			kc := n - k - 1
			t1 := xh[k] - xh[kc]
			t2 := was[k] * (xh[k] + xh[kc])
			x[k+1] = t1 + t2
			x[kc+1] = t2 - t1
		}
		if n%2 != 0 {
			x[n/2+1] = 4 * xh[n/2]
		}
		rfftf1(n+1, x, xh, war, ifac)
		xh[0] = 0.5 * x[0]
		for i := 2; i < n; i += 2 {
			xh[i-1] = -x[i]
			xh[i] = xh[i-2] + x[i-1]
		}
		if n%2 == 0 {
			xh[n-1] = -x[n]
		}
	}

	for i := 0; i < n; i++ {
		x[i] = war[i]
		war[i] = xh[i]
	}
}
