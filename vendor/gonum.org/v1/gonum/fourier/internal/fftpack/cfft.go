// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a translation of the FFTPACK cfft functions by
// Paul N Swarztrauber, placed in the public domain at
// http://www.netlib.org/fftpack/.

package fftpack

import "math"

// Cffti initializes the array work which is used in both Cfftf
// and Cfftb. the prime factorization of n together with a
// tabulation of the trigonometric functions are computed and
// stored in work.
//
//  input parameter
//
//  n      The length of the sequence to be transformed.
//
//  Output parameters:
//
//  work   A work array which must be dimensioned at least 4*n.
//         the same work array can be used for both Cfftf and Cfftb
//         as long as n remains unchanged. Different work arrays
//         are required for different values of n. The contents of
//         work must not be changed between calls of Cfftf or Cfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length 15.
func Cffti(n int, work []float64, ifac []int) {
	if len(work) < 4*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	cffti1(n, work[2*n:4*n], ifac[:15])
}

func cffti1(n int, wa []float64, ifac []int) {
	ntryh := [4]int{3, 4, 2, 5}

	nl := n
	nf := 0

outer:
	for j, ntry := 0, 0; ; j++ {
		if j < 4 {
			ntry = ntryh[j]
		} else {
			ntry += 2
		}
		for {
			if nl%ntry != 0 {
				continue outer
			}

			ifac[nf+2] = ntry
			nl /= ntry
			nf++

			if ntry == 2 && nf != 1 {
				for i := 1; i < nf; i++ {
					ib := nf - i + 1
					ifac[ib+1] = ifac[ib]
				}
				ifac[2] = 2
			}

			if nl == 1 {
				break outer
			}
		}
	}

	ifac[0] = n
	ifac[1] = nf

	argh := 2 * math.Pi / float64(n)
	i := 1
	l1 := 1
	for k1 := 0; k1 < nf; k1++ {
		ip := ifac[k1+2]
		ld := 0
		l2 := l1 * ip
		ido := n / l2
		idot := 2*ido + 2
		for j := 0; j < ip-1; j++ {
			i1 := i
			wa[i-1] = 1
			wa[i] = 0
			ld += l1
			var fi float64
			argld := float64(ld) * argh
			for ii := 3; ii < idot; ii += 2 {
				i += 2
				fi++
				arg := fi * argld
				wa[i-1] = math.Cos(arg)
				wa[i] = math.Sin(arg)
			}
			if ip > 5 {
				wa[i1-1] = wa[i-1]
				wa[i1] = wa[i]
			}
		}
		l1 = l2
	}
}

// Cfftf computes the forward complex Discrete Fourier transform
// (the Fourier analysis). Equivalently, Cfftf computes the
// Fourier coefficients of a complex periodic sequence. The
// transform is defined below at output parameter c.
//
//  Input parameters:
//
//  n      The length of the array c to be transformed. The method
//         is most efficient when n is a product of small primes.
//         n may change so long as different work arrays are provided.
//
//  c      A complex array of length n which contains the sequence
//         to be transformed.
//
//  work   A real work array which must be dimensioned at least 4*n.
//         in the program that calls Cfftf. The work array must be
//         initialized by calling subroutine Cffti(n,work,ifac) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//         the same work array can be used by Cfftf and Cfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length of at least 15.
//
//  Output parameters:
//
//   c     for j=0, ..., n-1
//           c[j]=the sum from k=0, ..., n-1 of
//             c[k]*exp(-i*j*k*2*pi/n)
//
//         where i=sqrt(-1)
//
//  This transform is unnormalized since a call of Cfftf
//  followed by a call of Cfftb will multiply the input
//  sequence by n.
//
//  The n elements of c are represented in n pairs of real
//  values in r where c[j] = r[j*2]+r[j*2+1]i.
//
//  work   Contains results which must not be destroyed between
//         calls of Cfftf or Cfftb.
//  ifac   Contains results which must not be destroyed between
//         calls of Cfftf or Cfftb.
func Cfftf(n int, r, work []float64, ifac []int) {
	if len(r) < 2*n {
		panic("fourier: short sequence")
	}
	if len(work) < 4*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	cfft1(n, r[:2*n], work[:2*n], work[2*n:4*n], ifac[:15], -1)
}

// Cfftb computes the backward complex Discrete Fourier Transform
// (the Fourier synthesis). Equivalently, Cfftf computes the computes
// a complex periodic sequence from its Fourier coefficients. The
// transform is defined below at output parameter c.
//
//  Input parameters:
//
//  n      The length of the array c to be transformed. The method
//         is most efficient when n is a product of small primes.
//         n may change so long as different work arrays are provided.
//
//  c      A complex array of length n which contains the sequence
//         to be transformed.
//
//  work   A real work array which must be dimensioned at least 4*n.
//         in the program that calls Cfftb. The work array must be
//         initialized by calling subroutine Cffti(n,work,ifac) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//         The same work array can be used by Cfftf and Cfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length of at least 15.
//
//  Output parameters:
//
//  c      for j=0, ..., n-1
//           c[j]=the sum from k=0, ..., n-1 of
//             c[k]*exp(i*j*k*2*pi/n)
//
//         where i=sqrt(-1)
//
//  This transform is unnormalized since a call of Cfftf
//  followed by a call of Cfftb will multiply the input
//  sequence by n.
//
//  The n elements of c are represented in n pairs of real
//  values in r where c[j] = r[j*2]+r[j*2+1]i.
//
//  work   Contains results which must not be destroyed between
//         calls of Cfftf or Cfftb.
//  ifac   Contains results which must not be destroyed between
//         calls of Cfftf or Cfftb.
func Cfftb(n int, r, work []float64, ifac []int) {
	if len(r) < 2*n {
		panic("fourier: short sequence")
	}
	if len(work) < 4*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	cfft1(n, r[:2*n], work[:2*n], work[2*n:4*n], ifac[:15], 1)
}

// cfft1 implements cfftf1 and cfftb1 depending on sign.
func cfft1(n int, c, ch, wa []float64, ifac []int, sign float64) {
	nf := ifac[1]
	na := false
	l1 := 1
	iw := 0

	for k1 := 1; k1 <= nf; k1++ {
		ip := ifac[k1+1]
		l2 := ip * l1
		ido := n / l2
		idot := 2 * ido
		idl1 := idot * l1

		switch ip {
		case 4:
			ix2 := iw + idot
			ix3 := ix2 + idot
			if na {
				pass4(idot, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:], sign)
			} else {
				pass4(idot, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:], sign)
			}
			na = !na
		case 2:
			if na {
				pass2(idot, l1, ch, c, wa[iw:], sign)
			} else {
				pass2(idot, l1, c, ch, wa[iw:], sign)
			}
			na = !na
		case 3:
			ix2 := iw + idot
			if na {
				pass3(idot, l1, ch, c, wa[iw:], wa[ix2:], sign)
			} else {
				pass3(idot, l1, c, ch, wa[iw:], wa[ix2:], sign)
			}
			na = !na
		case 5:
			ix2 := iw + idot
			ix3 := ix2 + idot
			ix4 := ix3 + idot
			if na {
				pass5(idot, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:], sign)
			} else {
				pass5(idot, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:], sign)
			}
			na = !na
		default:
			var nac bool
			if na {
				nac = pass(idot, ip, l1, idl1, ch, ch, ch, c, c, wa[iw:], sign)
			} else {
				nac = pass(idot, ip, l1, idl1, c, c, c, ch, ch, wa[iw:], sign)
			}
			if nac {
				na = !na
			}
		}

		l1 = l2
		iw += (ip - 1) * idot
	}

	if na {
		for i := 0; i < 2*n; i++ {
			c[i] = ch[i]
		}
	}
}

// pass2 implements passf2 and passb2 depending on sign.
func pass2(ido, l1 int, cc, ch, wa1 []float64, sign float64) {
	cc3 := newThreeArray(ido, 2, l1, cc)
	ch3 := newThreeArray(ido, l1, 2, ch)

	if ido <= 2 {
		for k := 0; k < l1; k++ {
			ch3.set(0, k, 0, cc3.at(0, 0, k)+cc3.at(0, 1, k))
			ch3.set(0, k, 1, cc3.at(0, 0, k)-cc3.at(0, 1, k))
			ch3.set(1, k, 0, cc3.at(1, 0, k)+cc3.at(1, 1, k))
			ch3.set(1, k, 1, cc3.at(1, 0, k)-cc3.at(1, 1, k))
		}
		return
	}
	for k := 0; k < l1; k++ {
		for i := 1; i < ido; i += 2 {
			ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+cc3.at(i-1, 1, k))
			tr2 := cc3.at(i-1, 0, k) - cc3.at(i-1, 1, k)
			ch3.set(i, k, 0, cc3.at(i, 0, k)+cc3.at(i, 1, k))
			ti2 := cc3.at(i, 0, k) - cc3.at(i, 1, k)
			ch3.set(i, k, 1, wa1[i-1]*ti2+sign*wa1[i]*tr2)
			ch3.set(i-1, k, 1, wa1[i-1]*tr2-sign*wa1[i]*ti2)
		}
	}
}

// pass3 implements passf3 and passb3 depending on sign.
func pass3(ido, l1 int, cc, ch, wa1, wa2 []float64, sign float64) {
	const (
		taur = -0.5
		taui = 0.866025403784439 // sqrt(3)/2
	)

	cc3 := newThreeArray(ido, 3, l1, cc)
	ch3 := newThreeArray(ido, l1, 3, ch)

	if ido == 2 {
		for k := 0; k < l1; k++ {
			tr2 := cc3.at(0, 1, k) + cc3.at(0, 2, k)
			cr2 := cc3.at(0, 0, k) + taur*tr2
			ch3.set(0, k, 0, cc3.at(0, 0, k)+tr2)
			ti2 := cc3.at(1, 1, k) + cc3.at(1, 2, k)
			ci2 := cc3.at(1, 0, k) + taur*ti2
			ch3.set(1, k, 0, cc3.at(1, 0, k)+ti2)
			cr3 := sign * taui * (cc3.at(0, 1, k) - cc3.at(0, 2, k))
			ci3 := sign * taui * (cc3.at(1, 1, k) - cc3.at(1, 2, k))
			ch3.set(0, k, 1, cr2-ci3)
			ch3.set(0, k, 2, cr2+ci3)
			ch3.set(1, k, 1, ci2+cr3)
			ch3.set(1, k, 2, ci2-cr3)
		}
		return
	}
	for k := 0; k < l1; k++ {
		for i := 1; i < ido; i += 2 {
			tr2 := cc3.at(i-1, 1, k) + cc3.at(i-1, 2, k)
			cr2 := cc3.at(i-1, 0, k) + taur*tr2
			ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+tr2)
			ti2 := cc3.at(i, 1, k) + cc3.at(i, 2, k)
			ci2 := cc3.at(i, 0, k) + taur*ti2
			ch3.set(i, k, 0, cc3.at(i, 0, k)+ti2)
			cr3 := sign * taui * (cc3.at(i-1, 1, k) - cc3.at(i-1, 2, k))
			ci3 := sign * taui * (cc3.at(i, 1, k) - cc3.at(i, 2, k))
			dr2 := cr2 - ci3
			dr3 := cr2 + ci3
			di2 := ci2 + cr3
			di3 := ci2 - cr3
			ch3.set(i, k, 1, wa1[i-1]*di2+sign*wa1[i]*dr2)
			ch3.set(i-1, k, 1, wa1[i-1]*dr2-sign*wa1[i]*di2)
			ch3.set(i, k, 2, wa2[i-1]*di3+sign*wa2[i]*dr3)
			ch3.set(i-1, k, 2, wa2[i-1]*dr3-sign*wa2[i]*di3)
		}
	}
}

// pass4 implements passf4 and passb4 depending on sign.
func pass4(ido, l1 int, cc, ch, wa1, wa2, wa3 []float64, sign float64) {
	cc3 := newThreeArray(ido, 4, l1, cc)
	ch3 := newThreeArray(ido, l1, 4, ch)

	if ido == 2 {
		for k := 0; k < l1; k++ {
			ti1 := cc3.at(1, 0, k) - cc3.at(1, 2, k)
			ti2 := cc3.at(1, 0, k) + cc3.at(1, 2, k)
			tr4 := sign * (cc3.at(1, 3, k) - cc3.at(1, 1, k))
			ti3 := cc3.at(1, 1, k) + cc3.at(1, 3, k)
			tr1 := cc3.at(0, 0, k) - cc3.at(0, 2, k)
			tr2 := cc3.at(0, 0, k) + cc3.at(0, 2, k)
			ti4 := sign * (cc3.at(0, 1, k) - cc3.at(0, 3, k))
			tr3 := cc3.at(0, 1, k) + cc3.at(0, 3, k)
			ch3.set(0, k, 0, tr2+tr3)
			ch3.set(0, k, 2, tr2-tr3)
			ch3.set(1, k, 0, ti2+ti3)
			ch3.set(1, k, 2, ti2-ti3)
			ch3.set(0, k, 1, tr1+tr4)
			ch3.set(0, k, 3, tr1-tr4)
			ch3.set(1, k, 1, ti1+ti4)
			ch3.set(1, k, 3, ti1-ti4)
		}
		return
	}
	for k := 0; k < l1; k++ {
		for i := 1; i < ido; i += 2 {
			ti1 := cc3.at(i, 0, k) - cc3.at(i, 2, k)
			ti2 := cc3.at(i, 0, k) + cc3.at(i, 2, k)
			ti3 := cc3.at(i, 1, k) + cc3.at(i, 3, k)
			tr4 := sign * (cc3.at(i, 3, k) - cc3.at(i, 1, k))
			tr1 := cc3.at(i-1, 0, k) - cc3.at(i-1, 2, k)
			tr2 := cc3.at(i-1, 0, k) + cc3.at(i-1, 2, k)
			ti4 := sign * (cc3.at(i-1, 1, k) - cc3.at(i-1, 3, k))
			tr3 := cc3.at(i-1, 1, k) + cc3.at(i-1, 3, k)
			ch3.set(i-1, k, 0, tr2+tr3)
			cr3 := tr2 - tr3
			ch3.set(i, k, 0, ti2+ti3)
			ci3 := ti2 - ti3
			cr2 := tr1 + tr4
			cr4 := tr1 - tr4
			ci2 := ti1 + ti4
			ci4 := ti1 - ti4
			ch3.set(i-1, k, 1, wa1[i-1]*cr2-sign*wa1[i]*ci2)
			ch3.set(i, k, 1, wa1[i-1]*ci2+sign*wa1[i]*cr2)
			ch3.set(i-1, k, 2, wa2[i-1]*cr3-sign*wa2[i]*ci3)
			ch3.set(i, k, 2, wa2[i-1]*ci3+sign*wa2[i]*cr3)
			ch3.set(i-1, k, 3, wa3[i-1]*cr4-sign*wa3[i]*ci4)
			ch3.set(i, k, 3, wa3[i-1]*ci4+sign*wa3[i]*cr4)
		}
	}
}

// pass5 implements passf5 and passb5 depending on sign.
func pass5(ido, l1 int, cc, ch, wa1, wa2, wa3, wa4 []float64, sign float64) {
	const (
		tr11 = 0.309016994374947
		ti11 = 0.951056516295154
		tr12 = -0.809016994374947
		ti12 = 0.587785252292473
	)

	cc3 := newThreeArray(ido, 5, l1, cc)
	ch3 := newThreeArray(ido, l1, 5, ch)

	if ido == 2 {
		for k := 0; k < l1; k++ {
			ti5 := cc3.at(1, 1, k) - cc3.at(1, 4, k)
			ti2 := cc3.at(1, 1, k) + cc3.at(1, 4, k)
			ti4 := cc3.at(1, 2, k) - cc3.at(1, 3, k)
			ti3 := cc3.at(1, 2, k) + cc3.at(1, 3, k)
			tr5 := cc3.at(0, 1, k) - cc3.at(0, 4, k)
			tr2 := cc3.at(0, 1, k) + cc3.at(0, 4, k)
			tr4 := cc3.at(0, 2, k) - cc3.at(0, 3, k)
			tr3 := cc3.at(0, 2, k) + cc3.at(0, 3, k)
			ch3.set(0, k, 0, cc3.at(0, 0, k)+tr2+tr3)
			ch3.set(1, k, 0, cc3.at(1, 0, k)+ti2+ti3)
			cr2 := cc3.at(0, 0, k) + tr11*tr2 + tr12*tr3
			ci2 := cc3.at(1, 0, k) + tr11*ti2 + tr12*ti3
			cr3 := cc3.at(0, 0, k) + tr12*tr2 + tr11*tr3
			ci3 := cc3.at(1, 0, k) + tr12*ti2 + tr11*ti3
			cr5 := sign * (ti11*tr5 + ti12*tr4)
			ci5 := sign * (ti11*ti5 + ti12*ti4)
			cr4 := sign * (ti12*tr5 - ti11*tr4)
			ci4 := sign * (ti12*ti5 - ti11*ti4)
			ch3.set(0, k, 1, cr2-ci5)
			ch3.set(0, k, 4, cr2+ci5)
			ch3.set(1, k, 1, ci2+cr5)
			ch3.set(1, k, 2, ci3+cr4)
			ch3.set(0, k, 2, cr3-ci4)
			ch3.set(0, k, 3, cr3+ci4)
			ch3.set(1, k, 3, ci3-cr4)
			ch3.set(1, k, 4, ci2-cr5)
		}
		return
	}
	for k := 0; k < l1; k++ {
		for i := 1; i < ido; i += 2 {
			ti5 := cc3.at(i, 1, k) - cc3.at(i, 4, k)
			ti2 := cc3.at(i, 1, k) + cc3.at(i, 4, k)
			ti4 := cc3.at(i, 2, k) - cc3.at(i, 3, k)
			ti3 := cc3.at(i, 2, k) + cc3.at(i, 3, k)
			tr5 := cc3.at(i-1, 1, k) - cc3.at(i-1, 4, k)
			tr2 := cc3.at(i-1, 1, k) + cc3.at(i-1, 4, k)
			tr4 := cc3.at(i-1, 2, k) - cc3.at(i-1, 3, k)
			tr3 := cc3.at(i-1, 2, k) + cc3.at(i-1, 3, k)
			ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+tr2+tr3)
			ch3.set(i, k, 0, cc3.at(i, 0, k)+ti2+ti3)
			cr2 := cc3.at(i-1, 0, k) + tr11*tr2 + tr12*tr3
			ci2 := cc3.at(i, 0, k) + tr11*ti2 + tr12*ti3
			cr3 := cc3.at(i-1, 0, k) + tr12*tr2 + tr11*tr3
			ci3 := cc3.at(i, 0, k) + tr12*ti2 + tr11*ti3
			cr5 := sign * (ti11*tr5 + ti12*tr4)
			ci5 := sign * (ti11*ti5 + ti12*ti4)
			cr4 := sign * (ti12*tr5 - ti11*tr4)
			ci4 := sign * (ti12*ti5 - ti11*ti4)
			dr3 := cr3 - ci4
			dr4 := cr3 + ci4
			di3 := ci3 + cr4
			di4 := ci3 - cr4
			dr5 := cr2 + ci5
			dr2 := cr2 - ci5
			di5 := ci2 - cr5
			di2 := ci2 + cr5
			ch3.set(i-1, k, 1, wa1[i-1]*dr2-sign*wa1[i]*di2)
			ch3.set(i, k, 1, wa1[i-1]*di2+sign*wa1[i]*dr2)
			ch3.set(i-1, k, 2, wa2[i-1]*dr3-sign*wa2[i]*di3)
			ch3.set(i, k, 2, wa2[i-1]*di3+sign*wa2[i]*dr3)
			ch3.set(i-1, k, 3, wa3[i-1]*dr4-sign*wa3[i]*di4)
			ch3.set(i, k, 3, wa3[i-1]*di4+sign*wa3[i]*dr4)
			ch3.set(i-1, k, 4, wa4[i-1]*dr5-sign*wa4[i]*di5)
			ch3.set(i, k, 4, wa4[i-1]*di5+sign*wa4[i]*dr5)
		}
	}
}

// pass implements passf and passb depending on sign.
func pass(ido, ip, l1, idl1 int, cc, c1, c2, ch, ch2, wa []float64, sign float64) (nac bool) {
	cc3 := newThreeArray(ido, ip, l1, cc)
	c13 := newThreeArray(ido, l1, ip, c1)
	ch3 := newThreeArray(ido, l1, ip, ch)
	c2m := newTwoArray(idl1, ip, c2)
	ch2m := newTwoArray(idl1, ip, ch2)

	idot := ido / 2
	ipph := (ip + 1) / 2
	idp := ip * ido

	if ido < l1 {
		for j := 1; j < ipph; j++ {
			jc := ip - j
			for i := 0; i < ido; i++ {
				for k := 0; k < l1; k++ {
					ch3.set(i, k, j, cc3.at(i, j, k)+cc3.at(i, jc, k))
					ch3.set(i, k, jc, cc3.at(i, j, k)-cc3.at(i, jc, k))
				}
			}
		}
		for i := 0; i < ido; i++ {
			for k := 0; k < l1; k++ {
				ch3.set(i, k, 0, cc3.at(i, 0, k))
			}
		}
	} else {
		for j := 1; j < ipph; j++ {
			jc := ip - j
			for k := 0; k < l1; k++ {
				for i := 0; i < ido; i++ {
					ch3.set(i, k, j, cc3.at(i, j, k)+cc3.at(i, jc, k))
					ch3.set(i, k, jc, cc3.at(i, j, k)-cc3.at(i, jc, k))
				}
			}
		}
		for k := 0; k < l1; k++ {
			for i := 0; i < ido; i++ {
				ch3.set(i, k, 0, cc3.at(i, 0, k))
			}
		}
	}

	idl := 1 - ido
	inc := 0
	for l := 1; l < ipph; l++ {
		lc := ip - l
		idl += ido
		for ik := 0; ik < idl1; ik++ {
			c2m.set(ik, l, ch2m.at(ik, 0)+wa[idl-1]*ch2m.at(ik, 1))
			c2m.set(ik, lc, sign*wa[idl]*ch2m.at(ik, ip-1))
		}
		idlj := idl
		inc += ido
		for j := 2; j < ipph; j++ {
			jc := ip - j
			idlj += inc
			if idlj > idp {
				idlj -= idp
			}
			war := wa[idlj-1]
			wai := wa[idlj]
			for ik := 0; ik < idl1; ik++ {
				c2m.add(ik, l, war*ch2m.at(ik, j))
				c2m.add(ik, lc, sign*wai*ch2m.at(ik, jc))
			}
		}
	}

	for j := 1; j < ipph; j++ {
		for ik := 0; ik < idl1; ik++ {
			ch2m.add(ik, 0, ch2m.at(ik, j))
		}
	}

	for j := 1; j < ipph; j++ {
		jc := ip - j
		for ik := 1; ik < idl1; ik += 2 {
			ch2m.set(ik-1, j, c2m.at(ik-1, j)-c2m.at(ik, jc))
			ch2m.set(ik-1, jc, c2m.at(ik-1, j)+c2m.at(ik, jc))
			ch2m.set(ik, j, c2m.at(ik, j)+c2m.at(ik-1, jc))
			ch2m.set(ik, jc, c2m.at(ik, j)-c2m.at(ik-1, jc))
		}
	}

	if ido == 2 {
		return true
	}

	for ik := 0; ik < idl1; ik++ {
		c2m.set(ik, 0, ch2m.at(ik, 0))
	}

	for j := 1; j < ip; j++ {
		for k := 0; k < l1; k++ {
			c13.set(0, k, j, ch3.at(0, k, j))
			c13.set(1, k, j, ch3.at(1, k, j))
		}
	}

	if idot > l1 {
		idj := 1 - ido
		for j := 1; j < ip; j++ {
			idj += ido
			for k := 0; k < l1; k++ {
				idij := idj
				for i := 3; i < ido; i += 2 {
					idij += 2
					c13.set(i-1, k, j, wa[idij-1]*ch3.at(i-1, k, j)-sign*wa[idij]*ch3.at(i, k, j))
					c13.set(i, k, j, wa[idij-1]*ch3.at(i, k, j)+sign*wa[idij]*ch3.at(i-1, k, j))
				}
			}
		}

		return false
	}

	idij := -1
	for j := 1; j < ip; j++ {
		idij += 2
		for i := 3; i < ido; i += 2 {
			idij += 2
			for k := 0; k < l1; k++ {
				c13.set(i-1, k, j, wa[idij-1]*ch3.at(i-1, k, j)-sign*wa[idij]*ch3.at(i, k, j))
				c13.set(i, k, j, wa[idij-1]*ch3.at(i, k, j)+sign*wa[idij]*ch3.at(i-1, k, j))
			}
		}
	}
	return false
}
