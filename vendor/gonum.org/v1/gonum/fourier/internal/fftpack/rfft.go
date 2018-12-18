// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This is a translation of the FFTPACK rfft functions by
// Paul N Swarztrauber, placed in the public domain at
// http://www.netlib.org/fftpack/.

package fftpack

import "math"

// Rffti initializes the array work which is used in both Rfftf
// and Rfftb. The prime factorization of n together with a
// tabulation of the trigonometric functions are computed and
// stored in work.
//
//  Input parameter:
//
//  n      The length of the sequence to be transformed.
//
//  Output parameters:
//
//  work   A work array which must be dimensioned at least 2*n.
//         The same work array can be used for both Rfftf and Rfftb
//         as long as n remains unchanged. different work arrays
//         are required for different values of n. The contents of
//         work must not be changed between calls of Rfftf or Rfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length of at least 15.
func Rffti(n int, work []float64, ifac []int) {
	if len(work) < 2*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	rffti1(n, work[n:2*n], ifac[:15])
}

func rffti1(n int, wa []float64, ifac []int) {
	ntryh := [4]int{4, 2, 3, 5}

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
	if nf == 1 {
		return
	}
	argh := 2 * math.Pi / float64(n)

	is := 0
	l1 := 1
	for k1 := 0; k1 < nf-1; k1++ {
		ip := ifac[k1+2]
		ld := 0
		l2 := l1 * ip
		ido := n / l2
		for j := 0; j < ip-1; j++ {
			ld += l1
			i := is
			fi := 0.
			argld := float64(ld) * argh
			for ii := 2; ii < ido; ii += 2 {
				fi++
				arg := fi * argld
				wa[i] = math.Cos(arg)
				wa[i+1] = math.Sin(arg)
				i += 2
			}
			is += ido
		}
		l1 = l2
	}
}

// Rfftf computes the Fourier coefficients of a real perodic sequence
// (Fourier analysis). The transform is defined below at output
// parameter r.
//
//  Input parameters:
//
//  n      The length of the array r to be transformed. The method
//         is most efficient when n is a product of small primes.
//         n may change so long as different work arrays are provided.
//
//  r      A real array of length n which contains the sequence
//         to be transformed.
//
//  work   a work array which must be dimensioned at least 2*n.
//         in the program that calls Rfftf. the work array must be
//         initialized by calling subroutine rffti(n,work,ifac) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged. Thus subsequent
//         transforms can be obtained faster than the first.
//         The same work array can be used by Rfftf and Rfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length of at least 15.
//
//  Output parameters:
//
//  r      r[0] = the sum from i=0 to i=n-1 of r[i]
//
//         if n is even set l=n/2, if n is odd set l = (n+1)/2
//           then for k = 1, ..., l-1
//             r[2*k-1] = the sum from i = 0 to i = n-1 of
//               r[i]*cos(k*i*2*pi/n)
//             r[2*k] = the sum from i = 0 to i = n-1 of
//               -r[i]*sin(k*i*2*pi/n)
//
//         if n is even
//           r[n-1] = the sum from i = 0 to i = n-1 of
//             (-1)^i*r[i]
//
//  This transform is unnormalized since a call of Rfftf
//  followed by a call of Rfftb will multiply the input
//  sequence by n.
//
//  work   contains results which must not be destroyed between
//         calls of Rfftf or Rfftb.
//  ifac   contains results which must not be destroyed between
//         calls of Rfftf or Rfftb.
func Rfftf(n int, r, work []float64, ifac []int) {
	if len(r) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 2*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	rfftf1(n, r[:n], work[:n], work[n:2*n], ifac[:15])
}

func rfftf1(n int, c, ch, wa []float64, ifac []int) {
	nf := ifac[1]
	na := true
	l2 := n
	iw := n - 1

	for k1 := 1; k1 <= nf; k1++ {
		kh := nf - k1
		ip := ifac[kh+2]
		l1 := l2 / ip
		ido := n / l2
		idl1 := ido * l1
		iw -= (ip - 1) * ido
		na = !na

		switch ip {
		case 4:
			ix2 := iw + ido
			ix3 := ix2 + ido
			if na {
				radf4(ido, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:])
			} else {
				radf4(ido, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:])
			}
		case 2:
			if na {
				radf2(ido, l1, ch, c, wa[iw:])
			} else {
				radf2(ido, l1, c, ch, wa[iw:])
			}
		case 3:
			ix2 := iw + ido
			if na {
				radf3(ido, l1, ch, c, wa[iw:], wa[ix2:])
			} else {
				radf3(ido, l1, c, ch, wa[iw:], wa[ix2:])
			}
		case 5:
			ix2 := iw + ido
			ix3 := ix2 + ido
			ix4 := ix3 + ido
			if na {
				radf5(ido, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:])
			} else {
				radf5(ido, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:])
			}
		default:
			if ido == 1 {
				na = !na
			}
			if na {
				radfg(ido, ip, l1, idl1, ch, ch, ch, c, c, wa[iw:])
				na = false
			} else {
				radfg(ido, ip, l1, idl1, c, c, c, ch, ch, wa[iw:])
				na = true
			}
		}

		l2 = l1
	}

	if na {
		return
	}
	for i := 0; i < n; i++ {
		c[i] = ch[i]
	}
}

func radf2(ido, l1 int, cc, ch, wa1 []float64) {
	cc3 := newThreeArray(ido, l1, 2, cc)
	ch3 := newThreeArray(ido, 2, l1, ch)

	for k := 0; k < l1; k++ {
		ch3.set(0, 0, k, cc3.at(0, k, 0)+cc3.at(0, k, 1))
		ch3.set(ido-1, 1, k, cc3.at(0, k, 0)-cc3.at(0, k, 1))
	}
	if ido < 2 {
		return
	}
	if ido > 2 {
		idp2 := ido + 1
		for k := 0; k < l1; k++ {
			for i := 2; i < ido; i += 2 {
				ic := idp2 - (i + 1)
				tr2 := wa1[i-2]*cc3.at(i-1, k, 1) + wa1[i-1]*cc3.at(i, k, 1)
				ti2 := wa1[i-2]*cc3.at(i, k, 1) - wa1[i-1]*cc3.at(i-1, k, 1)
				ch3.set(i, 0, k, cc3.at(i, k, 0)+ti2)
				ch3.set(ic, 1, k, ti2-cc3.at(i, k, 0))
				ch3.set(i-1, 0, k, cc3.at(i-1, k, 0)+tr2)
				ch3.set(ic-1, 1, k, cc3.at(i-1, k, 0)-tr2)
			}
		}
		if ido%2 == 1 {
			return
		}
	}
	for k := 0; k < l1; k++ {
		ch3.set(0, 1, k, -cc3.at(ido-1, k, 1))
		ch3.set(ido-1, 0, k, cc3.at(ido-1, k, 0))
	}
}

func radf3(ido, l1 int, cc, ch, wa1, wa2 []float64) {
	const (
		taur = -0.5
		taui = 0.866025403784439 // sqrt(3)/2
	)

	cc3 := newThreeArray(ido, l1, 3, cc)
	ch3 := newThreeArray(ido, 3, l1, ch)

	for k := 0; k < l1; k++ {
		cr2 := cc3.at(0, k, 1) + cc3.at(0, k, 2)
		ch3.set(0, 0, k, cc3.at(0, k, 0)+cr2)
		ch3.set(0, 2, k, taui*(cc3.at(0, k, 2)-cc3.at(0, k, 1)))
		ch3.set(ido-1, 1, k, cc3.at(0, k, 0)+taur*cr2)
	}
	if ido < 2 {
		return
	}
	idp2 := ido + 1
	for k := 0; k < l1; k++ {
		for i := 2; i < ido; i += 2 {
			ic := idp2 - (i + 1)
			dr2 := wa1[i-2]*cc3.at(i-1, k, 1) + wa1[i-1]*cc3.at(i, k, 1)
			di2 := wa1[i-2]*cc3.at(i, k, 1) - wa1[i-1]*cc3.at(i-1, k, 1)
			dr3 := wa2[i-2]*cc3.at(i-1, k, 2) + wa2[i-1]*cc3.at(i, k, 2)
			di3 := wa2[i-2]*cc3.at(i, k, 2) - wa2[i-1]*cc3.at(i-1, k, 2)
			cr2 := dr2 + dr3
			ci2 := di2 + di3
			ch3.set(i-1, 0, k, cc3.at(i-1, k, 0)+cr2)
			ch3.set(i, 0, k, cc3.at(i, k, 0)+ci2)
			tr2 := cc3.at(i-1, k, 0) + taur*cr2
			ti2 := cc3.at(i, k, 0) + taur*ci2
			tr3 := taui * (di2 - di3)
			ti3 := taui * (dr3 - dr2)
			ch3.set(i-1, 2, k, tr2+tr3)
			ch3.set(ic-1, 1, k, tr2-tr3)
			ch3.set(i, 2, k, ti2+ti3)
			ch3.set(ic, 1, k, ti3-ti2)
		}
	}
}

func radf4(ido, l1 int, cc, ch, wa1, wa2, wa3 []float64) {
	const hsqt2 = math.Sqrt2 / 2

	cc3 := newThreeArray(ido, l1, 4, cc)
	ch3 := newThreeArray(ido, 4, l1, ch)

	for k := 0; k < l1; k++ {
		tr1 := cc3.at(0, k, 1) + cc3.at(0, k, 3)
		tr2 := cc3.at(0, k, 0) + cc3.at(0, k, 2)
		ch3.set(0, 0, k, tr1+tr2)
		ch3.set(ido-1, 3, k, tr2-tr1)
		ch3.set(ido-1, 1, k, cc3.at(0, k, 0)-cc3.at(0, k, 2))
		ch3.set(0, 2, k, cc3.at(0, k, 3)-cc3.at(0, k, 1))
	}
	if ido < 2 {
		return
	}
	if ido > 2 {
		idp2 := ido + 1
		for k := 0; k < l1; k++ {
			for i := 2; i < ido; i += 2 {
				ic := idp2 - (i + 1)
				cr2 := wa1[i-2]*cc3.at(i-1, k, 1) + wa1[i-1]*cc3.at(i, k, 1)
				ci2 := wa1[i-2]*cc3.at(i, k, 1) - wa1[i-1]*cc3.at(i-1, k, 1)
				cr3 := wa2[i-2]*cc3.at(i-1, k, 2) + wa2[i-1]*cc3.at(i, k, 2)
				ci3 := wa2[i-2]*cc3.at(i, k, 2) - wa2[i-1]*cc3.at(i-1, k, 2)
				cr4 := wa3[i-2]*cc3.at(i-1, k, 3) + wa3[i-1]*cc3.at(i, k, 3)
				ci4 := wa3[i-2]*cc3.at(i, k, 3) - wa3[i-1]*cc3.at(i-1, k, 3)
				tr1 := cr2 + cr4
				tr4 := cr4 - cr2
				ti1 := ci2 + ci4
				ti4 := ci2 - ci4
				ti2 := cc3.at(i, k, 0) + ci3
				ti3 := cc3.at(i, k, 0) - ci3
				tr2 := cc3.at(i-1, k, 0) + cr3
				tr3 := cc3.at(i-1, k, 0) - cr3
				ch3.set(i-1, 0, k, tr1+tr2)
				ch3.set(ic-1, 3, k, tr2-tr1)
				ch3.set(i, 0, k, ti1+ti2)
				ch3.set(ic, 3, k, ti1-ti2)
				ch3.set(i-1, 2, k, ti4+tr3)
				ch3.set(ic-1, 1, k, tr3-ti4)
				ch3.set(i, 2, k, tr4+ti3)
				ch3.set(ic, 1, k, tr4-ti3)
			}
		}

		if ido%2 == 1 {
			return
		}
	}
	for k := 0; k < l1; k++ {
		ti1 := -hsqt2 * (cc3.at(ido-1, k, 1) + cc3.at(ido-1, k, 3))
		tr1 := hsqt2 * (cc3.at(ido-1, k, 1) - cc3.at(ido-1, k, 3))
		ch3.set(ido-1, 0, k, tr1+cc3.at(ido-1, k, 0))
		ch3.set(ido-1, 2, k, cc3.at(ido-1, k, 0)-tr1)
		ch3.set(0, 1, k, ti1-cc3.at(ido-1, k, 2))
		ch3.set(0, 3, k, ti1+cc3.at(ido-1, k, 2))
	}
}

func radf5(ido, l1 int, cc, ch, wa1, wa2, wa3, wa4 []float64) {
	const (
		tr11 = 0.309016994374947
		ti11 = 0.951056516295154
		tr12 = -0.809016994374947
		ti12 = 0.587785252292473
	)

	cc3 := newThreeArray(ido, l1, 5, cc)
	ch3 := newThreeArray(ido, 5, l1, ch)

	for k := 0; k < l1; k++ {
		cr2 := cc3.at(0, k, 4) + cc3.at(0, k, 1)
		ci5 := cc3.at(0, k, 4) - cc3.at(0, k, 1)
		cr3 := cc3.at(0, k, 3) + cc3.at(0, k, 2)
		ci4 := cc3.at(0, k, 3) - cc3.at(0, k, 2)
		ch3.set(0, 0, k, cc3.at(0, k, 0)+cr2+cr3)
		ch3.set(ido-1, 1, k, cc3.at(0, k, 0)+tr11*cr2+tr12*cr3)
		ch3.set(0, 2, k, ti11*ci5+ti12*ci4)
		ch3.set(ido-1, 3, k, cc3.at(0, k, 0)+tr12*cr2+tr11*cr3)
		ch3.set(0, 4, k, ti12*ci5-ti11*ci4)
	}

	if ido < 2 {
		return
	}
	idp2 := ido + 1
	for k := 0; k < l1; k++ {
		for i := 2; i < ido; i += 2 {
			ic := idp2 - (i + 1)
			dr2 := wa1[i-2]*cc3.at(i-1, k, 1) + wa1[i-1]*cc3.at(i, k, 1)
			di2 := wa1[i-2]*cc3.at(i, k, 1) - wa1[i-1]*cc3.at(i-1, k, 1)
			dr3 := wa2[i-2]*cc3.at(i-1, k, 2) + wa2[i-1]*cc3.at(i, k, 2)
			di3 := wa2[i-2]*cc3.at(i, k, 2) - wa2[i-1]*cc3.at(i-1, k, 2)
			dr4 := wa3[i-2]*cc3.at(i-1, k, 3) + wa3[i-1]*cc3.at(i, k, 3)
			di4 := wa3[i-2]*cc3.at(i, k, 3) - wa3[i-1]*cc3.at(i-1, k, 3)
			dr5 := wa4[i-2]*cc3.at(i-1, k, 4) + wa4[i-1]*cc3.at(i, k, 4)
			di5 := wa4[i-2]*cc3.at(i, k, 4) - wa4[i-1]*cc3.at(i-1, k, 4)
			cr2 := dr2 + dr5
			ci5 := dr5 - dr2
			cr5 := di2 - di5
			ci2 := di2 + di5
			cr3 := dr3 + dr4
			ci4 := dr4 - dr3
			cr4 := di3 - di4
			ci3 := di3 + di4
			ch3.set(i-1, 0, k, cc3.at(i-1, k, 0)+cr2+cr3)
			ch3.set(i, 0, k, cc3.at(i, k, 0)+ci2+ci3)
			tr2 := cc3.at(i-1, k, 0) + tr11*cr2 + tr12*cr3
			ti2 := cc3.at(i, k, 0) + tr11*ci2 + tr12*ci3
			tr3 := cc3.at(i-1, k, 0) + tr12*cr2 + tr11*cr3
			ti3 := cc3.at(i, k, 0) + tr12*ci2 + tr11*ci3
			tr5 := ti11*cr5 + ti12*cr4
			ti5 := ti11*ci5 + ti12*ci4
			tr4 := ti12*cr5 - ti11*cr4
			ti4 := ti12*ci5 - ti11*ci4
			ch3.set(i-1, 2, k, tr2+tr5)
			ch3.set(ic-1, 1, k, tr2-tr5)
			ch3.set(i, 2, k, ti2+ti5)
			ch3.set(ic, 1, k, ti5-ti2)
			ch3.set(i-1, 4, k, tr3+tr4)
			ch3.set(ic-1, 3, k, tr3-tr4)
			ch3.set(i, 4, k, ti3+ti4)
			ch3.set(ic, 3, k, ti4-ti3)
		}
	}
}

func radfg(ido, ip, l1, idl1 int, cc, c1, c2, ch, ch2, wa []float64) {
	cc3 := newThreeArray(ido, ip, l1, cc)
	c13 := newThreeArray(ido, l1, ip, c1)
	ch3 := newThreeArray(ido, l1, ip, ch)
	c2m := newTwoArray(idl1, ip, c2)
	ch2m := newTwoArray(idl1, ip, ch2)

	arg := 2 * math.Pi / float64(ip)
	dcp := math.Cos(arg)
	dsp := math.Sin(arg)
	ipph := (ip + 1) / 2
	nbd := (ido - 1) / 2

	if ido == 1 {
		for ik := 0; ik < idl1; ik++ {
			c2m.set(ik, 0, ch2m.at(ik, 0))
		}
	} else {
		for ik := 0; ik < idl1; ik++ {
			ch2m.set(ik, 0, c2m.at(ik, 0))
		}
		for j := 1; j < ip; j++ {
			for k := 0; k < l1; k++ {
				ch3.set(0, k, j, c13.at(0, k, j))
			}
		}

		is := -ido - 1
		if nbd > l1 {
			for j := 1; j < ip; j++ {
				is += ido
				for k := 0; k < l1; k++ {
					idij := is
					for i := 2; i < ido; i += 2 {
						idij += 2
						ch3.set(i-1, k, j, wa[idij-1]*c13.at(i-1, k, j)+wa[idij]*c13.at(i, k, j))
						ch3.set(i, k, j, wa[idij-1]*c13.at(i, k, j)-wa[idij]*c13.at(i-1, k, j))
					}
				}
			}
		} else {
			for j := 1; j < ip; j++ {
				is += ido
				idij := is
				for i := 2; i < ido; i += 2 {
					idij += 2
					for k := 0; k < l1; k++ {
						ch3.set(i-1, k, j, wa[idij-1]*c13.at(i-1, k, j)+wa[idij]*c13.at(i, k, j))
						ch3.set(i, k, j, wa[idij-1]*c13.at(i, k, j)-wa[idij]*c13.at(i-1, k, j))
					}
				}
			}
		}
		if nbd < l1 {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				for i := 2; i < ido; i += 2 {
					for k := 0; k < l1; k++ {
						c13.set(i-1, k, j, ch3.at(i-1, k, j)+ch3.at(i-1, k, jc))
						c13.set(i-1, k, jc, ch3.at(i, k, j)-ch3.at(i, k, jc))
						c13.set(i, k, j, ch3.at(i, k, j)+ch3.at(i, k, jc))
						c13.set(i, k, jc, ch3.at(i-1, k, jc)-ch3.at(i-1, k, j))
					}
				}
			}
		} else {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				for k := 0; k < l1; k++ {
					for i := 2; i < ido; i += 2 {
						c13.set(i-1, k, j, ch3.at(i-1, k, j)+ch3.at(i-1, k, jc))
						c13.set(i-1, k, jc, ch3.at(i, k, j)-ch3.at(i, k, jc))
						c13.set(i, k, j, ch3.at(i, k, j)+ch3.at(i, k, jc))
						c13.set(i, k, jc, ch3.at(i-1, k, jc)-ch3.at(i-1, k, j))
					}
				}
			}
		}
	}

	for j := 1; j < ipph; j++ {
		jc := ip - j
		for k := 0; k < l1; k++ {
			c13.set(0, k, j, ch3.at(0, k, j)+ch3.at(0, k, jc))
			c13.set(0, k, jc, ch3.at(0, k, jc)-ch3.at(0, k, j))
		}
	}
	ar1 := 1.0
	ai1 := 0.0
	for l := 1; l < ipph; l++ {
		lc := ip - l
		ar1h := dcp*ar1 - dsp*ai1
		ai1 = dcp*ai1 + dsp*ar1
		ar1 = ar1h
		for ik := 0; ik < idl1; ik++ {
			ch2m.set(ik, l, c2m.at(ik, 0)+ar1*c2m.at(ik, 1))
			ch2m.set(ik, lc, ai1*c2m.at(ik, ip-1))
		}
		dc2 := ar1
		ds2 := ai1
		ar2 := ar1
		ai2 := ai1
		for j := 2; j < ipph; j++ {
			jc := ip - j
			ar2h := dc2*ar2 - ds2*ai2
			ai2 = dc2*ai2 + ds2*ar2
			ar2 = ar2h
			for ik := 0; ik < idl1; ik++ {
				ch2m.add(ik, l, ar2*c2m.at(ik, j))
				ch2m.add(ik, lc, ai2*c2m.at(ik, jc))
			}
		}
	}
	for j := 1; j < ipph; j++ {
		for ik := 0; ik < idl1; ik++ {
			ch2m.add(ik, 0, c2m.at(ik, j))
		}
	}

	if ido < l1 {
		for i := 0; i < ido; i++ {
			for k := 0; k < l1; k++ {
				cc3.set(i, 0, k, ch3.at(i, k, 0))
			}
		}
	} else {
		for k := 0; k < l1; k++ {
			for i := 0; i < ido; i++ {
				cc3.set(i, 0, k, ch3.at(i, k, 0))
			}
		}
	}
	for j := 1; j < ipph; j++ {
		jc := ip - j
		j2 := 2 * j
		for k := 0; k < l1; k++ {
			cc3.set(ido-1, j2-1, k, ch3.at(0, k, j))
			cc3.set(0, j2, k, ch3.at(0, k, jc))
		}
	}

	if ido == 1 {
		return
	}
	if nbd < l1 {
		for j := 1; j < ipph; j++ {
			jc := ip - j
			j2 := 2 * j
			for i := 2; i < ido; i += 2 {
				ic := ido - i
				for k := 0; k < l1; k++ {
					cc3.set(i-1, j2, k, ch3.at(i-1, k, j)+ch3.at(i-1, k, jc))
					cc3.set(ic-1, j2-1, k, ch3.at(i-1, k, j)-ch3.at(i-1, k, jc))
					cc3.set(i, j2, k, ch3.at(i, k, j)+ch3.at(i, k, jc))
					cc3.set(ic, j2-1, k, ch3.at(i, k, jc)-ch3.at(i, k, j))
				}
			}
		}
		return
	}
	for j := 1; j < ipph; j++ {
		jc := ip - j
		j2 := 2 * j
		for k := 0; k < l1; k++ {
			for i := 2; i < ido; i += 2 {
				ic := ido - i
				cc3.set(i-1, j2, k, ch3.at(i-1, k, j)+ch3.at(i-1, k, jc))
				cc3.set(ic-1, j2-1, k, ch3.at(i-1, k, j)-ch3.at(i-1, k, jc))
				cc3.set(i, j2, k, ch3.at(i, k, j)+ch3.at(i, k, jc))
				cc3.set(ic, j2-1, k, ch3.at(i, k, jc)-ch3.at(i, k, j))
			}
		}
	}
}

// Rfftb computes the real perodic sequence from its Fourier
// coefficients (Fourier synthesis). The transform is defined
// below at output parameter r.
//
//  Input parameters
//
//  n      The length of the array r to be transformed. The method
//         is most efficient when n is a product of small primes.
//         n may change so long as different work arrays are provided.
//
//  r      A real array of length n which contains the sequence
//         to be transformed.
//
//  work   A work array which must be dimensioned at least 2*n.
//         in the program that calls Rfftb. The work array must be
//         initialized by calling subroutine rffti(n,work,ifac) and a
//         different work array must be used for each different
//         value of n. This initialization does not have to be
//         repeated so long as n remains unchanged thus subsequent
//         transforms can be obtained faster than the first.
//         The same work array can be used by Rfftf and Rfftb.
//
//  ifac   A work array containing the factors of n. ifac must have
//         length of at least 15.
//
//  output parameters
//
//  r      for n even and for i = 0, ..., n
//           r[i] = r[0]+(-1)^i*r[n-1]
//             plus the sum from k=1 to k=n/2-1 of
//               2*r(2*k-1)*cos(k*i*2*pi/n)
//               -2*r(2*k)*sin(k*i*2*pi/n)
//
//         for n odd and for i = 0, ..., n-1
//           r[i] = r[0] plus the sum from k=1 to k=(n-1)/2 of
//             2*r(2*k-1)*cos(k*i*2*pi/n)
//             -2*r(2*k)*sin(k*i*2*pi/n)
//
//  This transform is unnormalized since a call of Rfftf
//  followed by a call of Rfftb will multiply the input
//  sequence by n.
//
//  work   Contains results which must not be destroyed between
//         calls of Rfftf or Rfftb.
//  ifac   Contains results which must not be destroyed between
//         calls of Rfftf or Rfftb.
func Rfftb(n int, r, work []float64, ifac []int) {
	if len(r) < n {
		panic("fourier: short sequence")
	}
	if len(work) < 2*n {
		panic("fourier: short work")
	}
	if len(ifac) < 15 {
		panic("fourier: short ifac")
	}
	if n == 1 {
		return
	}
	rfftb1(n, r[:n], work[:n], work[n:2*n], ifac[:15])
}

func rfftb1(n int, c, ch, wa []float64, ifac []int) {
	nf := ifac[1]
	na := false
	l1 := 1
	iw := 0

	for k1 := 1; k1 <= nf; k1++ {
		ip := ifac[k1+1]
		l2 := ip * l1
		ido := n / l2
		idl1 := ido * l1

		switch ip {
		case 4:
			ix2 := iw + ido
			ix3 := ix2 + ido
			if na {
				radb4(ido, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:])
			} else {
				radb4(ido, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:])
			}
			na = !na
		case 2:
			if na {
				radb2(ido, l1, ch, c, wa[iw:])
			} else {
				radb2(ido, l1, c, ch, wa[iw:])
			}
			na = !na
		case 3:
			ix2 := iw + ido
			if na {
				radb3(ido, l1, ch, c, wa[iw:], wa[ix2:])
			} else {
				radb3(ido, l1, c, ch, wa[iw:], wa[ix2:])
			}
			na = !na
		case 5:
			ix2 := iw + ido
			ix3 := ix2 + ido
			ix4 := ix3 + ido
			if na {
				radb5(ido, l1, ch, c, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:])
			} else {
				radb5(ido, l1, c, ch, wa[iw:], wa[ix2:], wa[ix3:], wa[ix4:])
			}
			na = !na
		default:
			if na {
				radbg(ido, ip, l1, idl1, ch, ch, ch, c, c, wa[iw:])
			} else {
				radbg(ido, ip, l1, idl1, c, c, c, ch, ch, wa[iw:])
			}
			if ido == 1 {
				na = !na
			}
		}

		l1 = l2
		iw += (ip - 1) * ido
	}

	if na {
		for i := 0; i < n; i++ {
			c[i] = ch[i]
		}
	}
}

func radb2(ido, l1 int, cc, ch, wa1 []float64) {
	cc3 := newThreeArray(ido, 2, l1, cc)
	ch3 := newThreeArray(ido, l1, 2, ch)

	for k := 0; k < l1; k++ {
		ch3.set(0, k, 0, cc3.at(0, 0, k)+cc3.at(ido-1, 1, k))
		ch3.set(0, k, 1, cc3.at(0, 0, k)-cc3.at(ido-1, 1, k))
	}

	if ido < 2 {
		return
	}
	if ido > 2 {
		idp2 := ido + 1
		for k := 0; k < l1; k++ {
			for i := 2; i < ido; i += 2 {
				ic := idp2 - (i + 1)
				ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+cc3.at(ic-1, 1, k))
				tr2 := cc3.at(i-1, 0, k) - cc3.at(ic-1, 1, k)
				ch3.set(i, k, 0, cc3.at(i, 0, k)-cc3.at(ic, 1, k))
				ti2 := cc3.at(i, 0, k) + cc3.at(ic, 1, k)
				ch3.set(i-1, k, 1, wa1[i-2]*tr2-wa1[i-1]*ti2)
				ch3.set(i, k, 1, wa1[i-2]*ti2+wa1[i-1]*tr2)
			}
		}

		if ido%2 == 1 {
			return
		}
	}
	for k := 0; k < l1; k++ {
		ch3.set(ido-1, k, 0, 2*cc3.at(ido-1, 0, k))
		ch3.set(ido-1, k, 1, -2*cc3.at(0, 1, k))
	}
}

func radb3(ido, l1 int, cc, ch, wa1, wa2 []float64) {
	const (
		taur = -0.5
		taui = 0.866025403784439 // sqrt(3)/2
	)

	cc3 := newThreeArray(ido, 3, l1, cc)
	ch3 := newThreeArray(ido, l1, 3, ch)

	for k := 0; k < l1; k++ {
		tr2 := cc3.at(ido-1, 1, k) + cc3.at(ido-1, 1, k)
		cr2 := cc3.at(0, 0, k) + taur*tr2
		ch3.set(0, k, 0, cc3.at(0, 0, k)+tr2)
		ci3 := taui * (cc3.at(0, 2, k) + cc3.at(0, 2, k))
		ch3.set(0, k, 1, cr2-ci3)
		ch3.set(0, k, 2, cr2+ci3)
	}

	if ido == 1 {
		return
	}

	idp2 := ido + 1
	for k := 0; k < l1; k++ {
		for i := 2; i < ido; i += 2 {
			ic := idp2 - (i + 1)
			tr2 := cc3.at(i-1, 2, k) + cc3.at(ic-1, 1, k)
			cr2 := cc3.at(i-1, 0, k) + taur*tr2
			ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+tr2)
			ti2 := cc3.at(i, 2, k) - cc3.at(ic, 1, k)
			ci2 := cc3.at(i, 0, k) + taur*ti2
			ch3.set(i, k, 0, cc3.at(i, 0, k)+ti2)
			cr3 := taui * (cc3.at(i-1, 2, k) - cc3.at(ic-1, 1, k))
			ci3 := taui * (cc3.at(i, 2, k) + cc3.at(ic, 1, k))
			dr2 := cr2 - ci3
			dr3 := cr2 + ci3
			di2 := ci2 + cr3
			di3 := ci2 - cr3
			ch3.set(i-1, k, 1, wa1[i-2]*dr2-wa1[i-1]*di2)
			ch3.set(i, k, 1, wa1[i-2]*di2+wa1[i-1]*dr2)
			ch3.set(i-1, k, 2, wa2[i-2]*dr3-wa2[i-1]*di3)
			ch3.set(i, k, 2, wa2[i-2]*di3+wa2[i-1]*dr3)
		}
	}
}

func radb4(ido, l1 int, cc, ch, wa1, wa2, wa3 []float64) {
	cc3 := newThreeArray(ido, 4, l1, cc)
	ch3 := newThreeArray(ido, l1, 4, ch)

	for k := 0; k < l1; k++ {
		tr1 := cc3.at(0, 0, k) - cc3.at(ido-1, 3, k)
		tr2 := cc3.at(0, 0, k) + cc3.at(ido-1, 3, k)
		tr3 := cc3.at(ido-1, 1, k) + cc3.at(ido-1, 1, k)
		tr4 := cc3.at(0, 2, k) + cc3.at(0, 2, k)
		ch3.set(0, k, 0, tr2+tr3)
		ch3.set(0, k, 1, tr1-tr4)
		ch3.set(0, k, 2, tr2-tr3)
		ch3.set(0, k, 3, tr1+tr4)
	}

	if ido < 2 {
		return
	}
	if ido > 2 {
		idp2 := ido + 1
		for k := 0; k < l1; k++ {
			for i := 2; i < ido; i += 2 {
				ic := idp2 - (i + 1)
				ti1 := cc3.at(i, 0, k) + cc3.at(ic, 3, k)
				ti2 := cc3.at(i, 0, k) - cc3.at(ic, 3, k)
				ti3 := cc3.at(i, 2, k) - cc3.at(ic, 1, k)
				tr4 := cc3.at(i, 2, k) + cc3.at(ic, 1, k)
				tr1 := cc3.at(i-1, 0, k) - cc3.at(ic-1, 3, k)
				tr2 := cc3.at(i-1, 0, k) + cc3.at(ic-1, 3, k)
				ti4 := cc3.at(i-1, 2, k) - cc3.at(ic-1, 1, k)
				tr3 := cc3.at(i-1, 2, k) + cc3.at(ic-1, 1, k)
				ch3.set(i-1, k, 0, tr2+tr3)
				cr3 := tr2 - tr3
				ch3.set(i, k, 0, ti2+ti3)
				ci3 := ti2 - ti3
				cr2 := tr1 - tr4
				cr4 := tr1 + tr4
				ci2 := ti1 + ti4
				ci4 := ti1 - ti4
				ch3.set(i-1, k, 1, wa1[i-2]*cr2-wa1[i-1]*ci2)
				ch3.set(i, k, 1, wa1[i-2]*ci2+wa1[i-1]*cr2)
				ch3.set(i-1, k, 2, wa2[i-2]*cr3-wa2[i-1]*ci3)
				ch3.set(i, k, 2, wa2[i-2]*ci3+wa2[i-1]*cr3)
				ch3.set(i-1, k, 3, wa3[i-2]*cr4-wa3[i-1]*ci4)
				ch3.set(i, k, 3, wa3[i-2]*ci4+wa3[i-1]*cr4)
			}
		}

		if ido%2 == 1 {
			return
		}
	}
	for k := 0; k < l1; k++ {
		ti1 := cc3.at(0, 1, k) + cc3.at(0, 3, k)
		ti2 := cc3.at(0, 3, k) - cc3.at(0, 1, k)
		tr1 := cc3.at(ido-1, 0, k) - cc3.at(ido-1, 2, k)
		tr2 := cc3.at(ido-1, 0, k) + cc3.at(ido-1, 2, k)
		ch3.set(ido-1, k, 0, tr2+tr2)
		ch3.set(ido-1, k, 1, math.Sqrt2*(tr1-ti1))
		ch3.set(ido-1, k, 2, ti2+ti2)
		ch3.set(ido-1, k, 3, -math.Sqrt2*(tr1+ti1))
	}
}

func radb5(ido, l1 int, cc, ch, wa1, wa2, wa3, wa4 []float64) {
	const (
		tr11 = 0.309016994374947
		ti11 = 0.951056516295154
		tr12 = -0.809016994374947
		ti12 = 0.587785252292473
	)

	cc3 := newThreeArray(ido, 5, l1, cc)
	ch3 := newThreeArray(ido, l1, 5, ch)

	for k := 0; k < l1; k++ {
		ti5 := cc3.at(0, 2, k) + cc3.at(0, 2, k)
		ti4 := cc3.at(0, 4, k) + cc3.at(0, 4, k)
		tr2 := cc3.at(ido-1, 1, k) + cc3.at(ido-1, 1, k)
		tr3 := cc3.at(ido-1, 3, k) + cc3.at(ido-1, 3, k)
		ch3.set(0, k, 0, cc3.at(0, 0, k)+tr2+tr3)
		cr2 := cc3.at(0, 0, k) + tr11*tr2 + tr12*tr3
		cr3 := cc3.at(0, 0, k) + tr12*tr2 + tr11*tr3
		ci5 := ti11*ti5 + ti12*ti4
		ci4 := ti12*ti5 - ti11*ti4
		ch3.set(0, k, 1, cr2-ci5)
		ch3.set(0, k, 2, cr3-ci4)
		ch3.set(0, k, 3, cr3+ci4)
		ch3.set(0, k, 4, cr2+ci5)
	}

	if ido == 1 {
		return
	}

	idp2 := ido + 1
	for k := 0; k < l1; k++ {
		for i := 2; i < ido; i += 2 {
			ic := idp2 - (i + 1)
			ti5 := cc3.at(i, 2, k) + cc3.at(ic, 1, k)
			ti2 := cc3.at(i, 2, k) - cc3.at(ic, 1, k)
			ti4 := cc3.at(i, 4, k) + cc3.at(ic, 3, k)
			ti3 := cc3.at(i, 4, k) - cc3.at(ic, 3, k)
			tr5 := cc3.at(i-1, 2, k) - cc3.at(ic-1, 1, k)
			tr2 := cc3.at(i-1, 2, k) + cc3.at(ic-1, 1, k)
			tr4 := cc3.at(i-1, 4, k) - cc3.at(ic-1, 3, k)
			tr3 := cc3.at(i-1, 4, k) + cc3.at(ic-1, 3, k)
			ch3.set(i-1, k, 0, cc3.at(i-1, 0, k)+tr2+tr3)
			ch3.set(i, k, 0, cc3.at(i, 0, k)+ti2+ti3)
			cr2 := cc3.at(i-1, 0, k) + tr11*tr2 + tr12*tr3
			ci2 := cc3.at(i, 0, k) + tr11*ti2 + tr12*ti3
			cr3 := cc3.at(i-1, 0, k) + tr12*tr2 + tr11*tr3
			ci3 := cc3.at(i, 0, k) + tr12*ti2 + tr11*ti3
			cr5 := ti11*tr5 + ti12*tr4
			ci5 := ti11*ti5 + ti12*ti4
			cr4 := ti12*tr5 - ti11*tr4
			ci4 := ti12*ti5 - ti11*ti4
			dr3 := cr3 - ci4
			dr4 := cr3 + ci4
			di3 := ci3 + cr4
			di4 := ci3 - cr4
			dr5 := cr2 + ci5
			dr2 := cr2 - ci5
			di5 := ci2 - cr5
			di2 := ci2 + cr5
			ch3.set(i-1, k, 1, wa1[i-2]*dr2-wa1[i-1]*di2)
			ch3.set(i, k, 1, wa1[i-2]*di2+wa1[i-1]*dr2)
			ch3.set(i-1, k, 2, wa2[i-2]*dr3-wa2[i-1]*di3)
			ch3.set(i, k, 2, wa2[i-2]*di3+wa2[i-1]*dr3)
			ch3.set(i-1, k, 3, wa3[i-2]*dr4-wa3[i-1]*di4)
			ch3.set(i, k, 3, wa3[i-2]*di4+wa3[i-1]*dr4)
			ch3.set(i-1, k, 4, wa4[i-2]*dr5-wa4[i-1]*di5)
			ch3.set(i, k, 4, wa4[i-2]*di5+wa4[i-1]*dr5)
		}
	}
}

func radbg(ido, ip, l1, idl1 int, cc, c1, c2, ch, ch2, wa []float64) {
	cc3 := newThreeArray(ido, ip, l1, cc)
	c13 := newThreeArray(ido, l1, ip, c1)
	ch3 := newThreeArray(ido, l1, ip, ch)
	c2m := newTwoArray(idl1, ip, c2)
	ch2m := newTwoArray(idl1, ip, ch2)

	arg := 2 * math.Pi / float64(ip)
	dcp := math.Cos(arg)
	dsp := math.Sin(arg)
	ipph := (ip + 1) / 2
	nbd := (ido - 1) / 2

	if ido < l1 {
		for i := 0; i < ido; i++ {
			for k := 0; k < l1; k++ {
				ch3.set(i, k, 0, cc3.at(i, 0, k))
			}
		}
	} else {
		for k := 0; k < l1; k++ {
			for i := 0; i < ido; i++ {
				ch3.set(i, k, 0, cc3.at(i, 0, k))
			}
		}
	}

	for j := 1; j < ipph; j++ {
		jc := ip - j
		j2 := 2 * j
		for k := 0; k < l1; k++ {
			ch3.set(0, k, j, cc3.at(ido-1, j2-1, k)+cc3.at(ido-1, j2-1, k))
			ch3.set(0, k, jc, cc3.at(0, j2, k)+cc3.at(0, j2, k))
		}
	}

	if ido != 1 {
		if nbd < l1 {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				j2 := 2 * j
				for i := 2; i < ido; i += 2 {
					ic := ido - i
					for k := 0; k < l1; k++ {
						ch3.set(i-1, k, j, cc3.at(i-1, j2, k)+cc3.at(ic-1, j2-1, k))
						ch3.set(i-1, k, jc, cc3.at(i-1, j2, k)-cc3.at(ic-1, j2-1, k))
						ch3.set(i, k, j, cc3.at(i, j2, k)-cc3.at(ic, j2-1, k))
						ch3.set(i, k, jc, cc3.at(i, j2, k)+cc3.at(ic, j2-1, k))
					}
				}
			}
		} else {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				j2 := 2 * j
				for k := 0; k < l1; k++ {
					for i := 2; i < ido; i += 2 {
						ic := ido - i
						ch3.set(i-1, k, j, cc3.at(i-1, j2, k)+cc3.at(ic-1, j2-1, k))
						ch3.set(i-1, k, jc, cc3.at(i-1, j2, k)-cc3.at(ic-1, j2-1, k))
						ch3.set(i, k, j, cc3.at(i, j2, k)-cc3.at(ic, j2-1, k))
						ch3.set(i, k, jc, cc3.at(i, j2, k)+cc3.at(ic, j2-1, k))
					}
				}
			}
		}
	}

	ar1 := 1.0
	ai1 := 0.0
	for l := 1; l < ipph; l++ {
		lc := ip - l
		ar1h := dcp*ar1 - dsp*ai1
		ai1 = dcp*ai1 + dsp*ar1
		ar1 = ar1h
		for ik := 0; ik < idl1; ik++ {
			c2m.set(ik, l, ch2m.at(ik, 0)+ar1*ch2m.at(ik, 1))
			c2m.set(ik, lc, ai1*ch2m.at(ik, ip-1))
		}
		dc2 := ar1
		ds2 := ai1
		ar2 := ar1
		ai2 := ai1
		for j := 2; j < ipph; j++ {
			jc := ip - j
			ar2h := dc2*ar2 - ds2*ai2
			ai2 = dc2*ai2 + ds2*ar2
			ar2 = ar2h
			for ik := 0; ik < idl1; ik++ {
				c2m.add(ik, l, ar2*ch2m.at(ik, j))
				c2m.add(ik, lc, ai2*ch2m.at(ik, jc))
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
		for k := 0; k < l1; k++ {
			ch3.set(0, k, j, c13.at(0, k, j)-c13.at(0, k, jc))
			ch3.set(0, k, jc, c13.at(0, k, j)+c13.at(0, k, jc))
		}
	}

	if ido != 1 {
		if nbd < l1 {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				for i := 2; i < ido; i += 2 {
					for k := 0; k < l1; k++ {
						ch3.set(i-1, k, j, c13.at(i-1, k, j)-c13.at(i, k, jc))
						ch3.set(i-1, k, jc, c13.at(i-1, k, j)+c13.at(i, k, jc))
						ch3.set(i, k, j, c13.at(i, k, j)+c13.at(i-1, k, jc))
						ch3.set(i, k, jc, c13.at(i, k, j)-c13.at(i-1, k, jc))
					}
				}
			}
		} else {
			for j := 1; j < ipph; j++ {
				jc := ip - j
				for k := 0; k < l1; k++ {
					for i := 2; i < ido; i += 2 {
						ch3.set(i-1, k, j, c13.at(i-1, k, j)-c13.at(i, k, jc))
						ch3.set(i-1, k, jc, c13.at(i-1, k, j)+c13.at(i, k, jc))
						ch3.set(i, k, j, c13.at(i, k, j)+c13.at(i-1, k, jc))
						ch3.set(i, k, jc, c13.at(i, k, j)-c13.at(i-1, k, jc))
					}
				}
			}
		}
	}

	if ido == 1 {
		return
	}
	for ik := 0; ik < idl1; ik++ {
		c2m.set(ik, 0, ch2m.at(ik, 0))
	}
	for j := 1; j < ip; j++ {
		for k := 0; k < l1; k++ {
			c13.set(0, k, j, ch3.at(0, k, j))
		}
	}

	is := -ido - 1
	if nbd > l1 {
		for j := 1; j < ip; j++ {
			is += ido
			for k := 0; k < l1; k++ {
				idij := is
				for i := 2; i < ido; i += 2 {
					idij += 2
					c13.set(i-1, k, j, wa[idij-1]*ch3.at(i-1, k, j)-wa[idij]*ch3.at(i, k, j))
					c13.set(i, k, j, wa[idij-1]*ch3.at(i, k, j)+wa[idij]*ch3.at(i-1, k, j))
				}
			}
		}
		return
	}
	for j := 1; j < ip; j++ {
		is += ido
		idij := is
		for i := 2; i < ido; i += 2 {
			idij += 2
			for k := 0; k < l1; k++ {
				c13.set(i-1, k, j, wa[idij-1]*ch3.at(i-1, k, j)-wa[idij]*ch3.at(i, k, j))
				c13.set(i, k, j, wa[idij-1]*ch3.at(i, k, j)+wa[idij]*ch3.at(i-1, k, j))
			}
		}
	}
}
