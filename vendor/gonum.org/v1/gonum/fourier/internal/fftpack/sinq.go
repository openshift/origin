// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a translation of the FFTPACK sinq functions by
// Paul N Swarztrauber, placed in the public domain at
// http://www.netlib.org/fftpack/.

package fftpack

import "math"

// Sinqi initializes the array work which is used in both Sinqf and
// Sinqb. The prime factorization of n together with a tabulation
// of the trigonometric functions are computed and stored in work.
//
// Input parameter
//
// n       The length of the sequence to be transformed. The method
//         is most efficient when n+1 is a product of small primes.
//
// Output parameter
//
// work    A work array which must be dimensioned at least 3*n.
//         The same work array can be used for both Sinqf and Sinqb
//         as long as n remains unchanged. Different work arrays
//         are required for different values of n. The contents of
//         work must not be changed between calls of Sinqf or Sinqb.
//
// ifac    An integer work array of length at least 15.
func Sinqi(n int, work []float64, ifac []int) {
	if len(work) < 3*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	dt := 0.5 * math.Pi / float64(n)
	for k := range work[:n] {
		work[k] = math.Cos(float64(k+1) * dt)
	}
	Rffti(n, work[n:], ifac)
}

// Sinqf computes the Fast Fourier Transform of quarter wave data.
// That is, Sinqf computes the coefficients in a sine series
// representation with only odd wave numbers. The transform is
// defined below at output parameter x.
//
// Sinqb is the unnormalized inverse of Sinqf since a call of Sinqf
// followed by a call of Sinqb will multiply the input sequence x
// by 4*n.
//
// The array work which is used by subroutine Sinqf must be
// initialized by calling subroutine Sinqi(n,work).
//
// Input parameters
//
// n       The length of the array x to be transformed. The method
//         is most efficient when n is a product of small primes.
//
// x       An array which contains the sequence to be transformed.
//
// work    A work array which must be dimensioned at least 3*n.
//         in the program that calls Sinqf. The work array must be
//         initialized by calling subroutine Sinqi(n,work) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//
// ifac    An integer work array of length at least 15.
//
// Output parameters
//
// x       for i=0, ..., n-1
//           x[i] = (-1)^(i)*x[n-1]
//             + the sum from k=0 to k=n-2 of
//               2*x[k]*sin((2*i+1)*k*pi/(2*n))
//
//         A call of Sinqf followed by a call of
//         Sinqb will multiply the sequence x by 4*n.
//         Therefore Sinqb is the unnormalized inverse
//         of Sinqf.
//
// work    Contains initialization calculations which must not
//         be destroyed between calls of Sinqf or Sinqb.
func Sinqf(n int, x, work []float64, ifac []int) {
	if len(x) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 3*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	for k := 0; k < n/2; k++ {
		kc := n - k - 1
		x[k], x[kc] = x[kc], x[k]
	}
	Cosqf(n, x, work, ifac)
	for k := 1; k < n; k += 2 {
		x[k] = -x[k]
	}
}

// Sinqb computes the Fast Fourier Transform of quarter wave data.
// That is, Sinqb computes a sequence from its representation in
// terms of a sine series with odd wave numbers. The transform is
// defined below at output parameter x.
//
// Sinqf is the unnormalized inverse of Sinqb since a call of Sinqb
// followed by a call of Sinqf will multiply the input sequence x
// by 4*n.
//
// The array work which is used by subroutine Sinqb must be
// initialized by calling subroutine Sinqi(n,work).
//
// Input parameters
//
// n       The length of the array x to be transformed. The method
//         is most efficient when n is a product of small primes.
//
// x       An array which contains the sequence to be transformed.
//
// work    A work array which must be dimensioned at least 3*n.
//         in the program that calls Sinqb. The work array must be
//         initialized by calling subroutine Sinqi(n,work) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//
// ifac    An integer work array of length at least 15.
//
// Output parameters
//
// x       for i=0, ..., n-1
//           x[i]= the sum from k=0 to k=n-1 of
//             4*x[k]*sin((2*k+1)*i*pi/(2*n))
//
//         A call of Sinqb followed by a call of
//         Sinqf will multiply the sequence x by 4*n.
//         Therefore Sinqf is the unnormalized inverse
//         of Sinqb.
//
// work    Contains initialization calculations which must not
//         be destroyed between calls of Sinqb or Sinqf.
func Sinqb(n int, x, work []float64, ifac []int) {
	if len(x) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 3*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	switch n {
	case 1:
		x[0] *= 4
		fallthrough
	case 0:
		return
	default:
		for k := 1; k < n; k += 2 {
			x[k] = -x[k]
		}
		Cosqb(n, x, work, ifac)
		for k := 0; k < n/2; k++ {
			kc := n - k - 1
			x[k], x[kc] = x[kc], x[k]
		}
	}
}
