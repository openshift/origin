// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quad

import (
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mathext"
)

// Hermite generates sample locations and weights for performing quadrature with
// with a squared-exponential weight
//  int_-inf^inf e^(-x^2) f(x) dx .
type Hermite struct{}

func (h Hermite) FixedLocations(x, weight []float64, min, max float64) {
	// TODO(btracey): Implement the case where x > 20, x < 200 so that we don't
	// need to store all of that data.

	// Algorithm adapted from Chebfun http://www.chebfun.org/.
	//
	// References:
	// Algorithm:
	// G. H. Golub and J. A. Welsch, "Calculation of Gauss quadrature rules",
	// Math. Comp. 23:221-230, 1969.
	// A. Glaser, X. Liu and V. Rokhlin, "A fast algorithm for the
	// calculation of the roots of special functions", SIAM Journal
	// on Scientific Computing", 29(4):1420-1438:, 2007.
	// A. Townsend, T. Trogdon, and S.Olver, Fast computation of Gauss quadrature
	// nodes and weights on the whole real line, IMA J. Numer. Anal., 36: 337–358,
	// 2016. http://arxiv.org/abs/1410.5286

	if len(x) != len(weight) {
		panic("hermite: slice length mismatch")
	}
	if min >= max {
		panic("hermite: min >= max")
	}
	if !math.IsInf(min, -1) || !math.IsInf(max, 1) {
		panic("hermite: non-infinite bound")
	}
	h.locations(x, weight)
}

func (h Hermite) locations(x, weights []float64) {
	n := len(x)
	switch {
	case 0 < n && n <= 200:
		copy(x, xCacheHermite[n-1])
		copy(weights, wCacheHermite[n-1])
	case n > 200:
		h.locationsAsy(x, weights)
	}
}

// Algorithm adapted from Chebfun http://www.chebfun.org/. Specific code
// https://github.com/chebfun/chebfun/blob/development/hermpts.m.

// Original Copyright Notice:

/*
Copyright (c) 2015, The Chancellor, Masters and Scholars of the University
of Oxford, and the Chebfun Developers. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:
    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above copyright
      notice, this list of conditions and the following disclaimer in the
      documentation and/or other materials provided with the distribution.
    * Neither the name of the University of Oxford nor the names of its
      contributors may be used to endorse or promote products derived from
      this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

// locationAsy returns the node locations and weights of a Hermite quadrature rule
// with len(x) points.
func (h Hermite) locationsAsy(x, w []float64) {
	// A. Townsend, T. Trogdon, and S.Olver, Fast computation of Gauss quadrature
	// nodes and weights the whole real line, IMA J. Numer. Anal.,
	// 36: 337–358, 2016. http://arxiv.org/abs/1410.5286

	// Find the positive locations and weights.
	n := len(x)
	l := n / 2
	xa := x[l:]
	wa := w[l:]
	for i := range xa {
		xa[i], wa[i] = h.locationsAsy0(i, n)
	}
	// Flip around zero -- copy the negative x locations with the corresponding
	// weights.
	if n%2 == 0 {
		l--
	}
	for i, v := range xa {
		x[l-i] = -v
	}
	for i, v := range wa {
		w[l-i] = v
	}
	sumW := floats.Sum(w)
	c := math.SqrtPi / sumW
	floats.Scale(c, w)
}

// locationsAsy0 returns the location and weight for location i in an n-point
// quadrature rule. The rule is symmetric, so i should be <= n/2 + n%2.
func (h Hermite) locationsAsy0(i, n int) (x, w float64) {
	const convTol = 1e-16
	const convIter = 20
	theta0 := h.hermiteInitialGuess(i, n)
	t0 := theta0 / math.Sqrt(2*float64(n)+1)
	theta0 = math.Acos(t0)
	sqrt2np1 := math.Sqrt(2*float64(n) + 1)
	var vali, dvali float64
	for k := 0; k < convIter; k++ {
		vali, dvali = h.hermpolyAsyAiry(i, n, theta0)
		dt := -vali / (math.Sqrt2 * sqrt2np1 * dvali * math.Sin(theta0))
		theta0 -= dt
		if math.Abs(theta0) < convTol {
			break
		}
	}
	x = sqrt2np1 * math.Cos(theta0)
	ders := x*vali + math.Sqrt2*dvali
	w = math.Exp(-x*x) / (ders * ders)
	return x, w
}

// hermpolyAsyAiry evaluates the Hermite polynomials using the Airy asymptotic
// formula in theta-space.
func (h Hermite) hermpolyAsyAiry(i, n int, t float64) (valVec, dvalVec float64) {
	musq := 2*float64(n) + 1
	cosT := math.Cos(t)
	sinT := math.Sin(t)
	sin2T := 2 * cosT * sinT
	eta := 0.5*t - 0.25*sin2T
	chi := -math.Pow(3*eta/2, 2.0/3)
	phi := math.Pow(-chi/(sinT*sinT), 1.0/4)
	cnst := 2 * math.SqrtPi * math.Pow(musq, 1.0/6) * phi
	airy0 := real(mathext.AiryAi(complex(math.Pow(musq, 2.0/3)*chi, 0)))
	airy1 := real(mathext.AiryAiDeriv(complex(math.Pow(musq, 2.0/3)*chi, 0)))

	// Terms in 12.10.43:
	const (
		a1 = 15.0 / 144
		b1 = -7.0 / 5 * a1
		a2 = 5.0 * 7 * 9 * 11.0 / 2.0 / 144.0 / 144.0
		b2 = -13.0 / 11 * a2
		a3 = 7.0 * 9 * 11 * 13 * 15 * 17 / 6.0 / 144.0 / 144.0 / 144.0
		b3 = -19.0 / 17 * a3
	)

	// Pre-compute terms.
	cos2T := cosT * cosT
	cos3T := cos2T * cosT
	cos4T := cos3T * cosT
	cos5T := cos4T * cosT
	cos7T := cos5T * cos2T
	cos9T := cos7T * cos2T

	chi2 := chi * chi
	chi3 := chi2 * chi
	chi4 := chi3 * chi
	chi5 := chi4 * chi

	phi6 := math.Pow(phi, 6)
	phi12 := phi6 * phi6
	phi18 := phi12 * phi6

	// u polynomials in 12.10.9.
	u1 := (cos3T - 6*cosT) / 24.0
	u2 := (-9*cos4T + 249*cos2T + 145) / 1152.0
	u3 := (-4042*cos9T + 18189*cos7T - 28287*cos5T - 151995*cos3T - 259290*cosT) / 414720.0

	val := airy0
	B0 := -(phi6*u1 + a1) / chi2
	val += B0 * airy1 / math.Pow(musq, 4.0/3)
	A1 := (phi12*u2 + b1*phi6*u1 + b2) / chi3
	val += A1 * airy0 / (musq * musq)
	B1 := -(phi18*u3 + a1*phi12*u2 + a2*phi6*u1 + a3) / chi5
	val += B1 * airy1 / math.Pow(musq, 4.0/3+2)
	val *= cnst

	// Derivative.
	eta = 0.5*t - 0.25*sin2T
	chi = -math.Pow(3*eta/2, 2.0/3)
	phi = math.Pow(-chi/(sinT*sinT), 1.0/4)
	cnst = math.Sqrt2 * math.SqrtPi * math.Pow(musq, 1.0/3) / phi

	// v polynomials in 12.10.10.
	v1 := (cos3T + 6*cosT) / 24
	v2 := (15*cos4T - 327*cos2T - 143) / 1152
	v3 := (259290*cosT + 238425*cos3T - 36387*cos5T + 18189*cos7T - 4042*cos9T) / 414720

	C0 := -(phi6*v1 + b1) / chi
	dval := C0 * airy0 / math.Pow(musq, 2.0/3)
	dval += airy1
	C1 := -(phi18*v3 + b1*phi12*v2 + b2*phi6*v1 + b3) / chi4
	dval += C1 * airy0 / math.Pow(musq, 2.0/3+2)
	D1 := (phi12*v2 + a1*phi6*v1 + a2) / chi3
	dval += D1 * airy1 / (musq * musq)
	dval *= cnst
	return val, dval
}

// hermiteInitialGuess returns the initial guess for node i in an n-point Hermite
// quadrature rule. The rule is symmetric, so i should be <= n/2 + n%2.
func (h Hermite) hermiteInitialGuess(i, n int) float64 {
	// There are two different formulas for the initial guesses of the hermite
	// quadrature locations. The first uses the Gatteschi formula and is good
	// near x = sqrt(n+0.5)
	//  [1] L. Gatteschi, Asymptotics and bounds for the zeros of Laguerre
	//  polynomials: a survey, J. Comput. Appl. Math., 144 (2002), pp. 7-27.
	// The second is the Tricomi initial guesses, good near x = 0. This is
	// equation 2.1 in [1] and is originally from
	//  [2] F. G. Tricomi, Sugli zeri delle funzioni di cui si conosce una
	//  rappresentazione asintotica, Ann. Mat. Pura Appl. 26 (1947), pp. 283-300.

	// If the number of points is odd, there is a quadrature point at 1, which
	// has an initial guess of 0.
	if n%2 == 1 {
		if i == 0 {
			return 0
		}
		i--
	}

	m := n / 2
	a := -0.5
	if n%2 == 1 {
		a = 0.5
	}
	nu := 4*float64(m) + 2*a + 2

	// Find the split between Gatteschi guesses and Tricomi guesses.
	p := 0.4985 + math.SmallestNonzeroFloat64
	pidx := int(math.Floor(p * float64(n)))

	// Use the Tricomi initial guesses in the first half where x is nearer to zero.
	// Note: zeros of besselj(+/-.5,x) are integer and half-integer multiples of pi.
	if i < pidx {
		rhs := math.Pi * (4*float64(m) - 4*(float64(i)+1) + 3) / nu
		tnk := math.Pi / 2
		for k := 0; k < 7; k++ {
			val := tnk - math.Sin(tnk) - rhs
			dval := 1 - math.Cos(tnk)
			dTnk := val / dval
			tnk -= dTnk
			if math.Abs(dTnk) < 1e-14 {
				break
			}
		}
		vc := math.Cos(tnk / 2)
		t := vc * vc
		return math.Sqrt(nu*t - (5.0/(4.0*(1-t)*(1-t))-1.0/(1-t)-1+3*a*a)/3/nu)
	}

	// Use Gatteschi guesses in the second half where x is nearer to sqrt(n+0.5)
	i = i + 1 - m
	var ar float64
	if i < len(airyRtsExact) {
		ar = airyRtsExact[i]
	} else {
		t := 3.0 / 8 * math.Pi * (4*(float64(i)+1) - 1)
		ar = math.Pow(t, 2.0/3) * (1 +
			5.0/48*math.Pow(t, -2) -
			5.0/36*math.Pow(t, -4) +
			77125.0/82944*math.Pow(t, -6) -
			108056875.0/6967296*math.Pow(t, -8) +
			162375596875.0/334430208*math.Pow(t, -10))
	}
	r := nu + math.Pow(2, 2.0/3)*ar*math.Pow(nu, 1.0/3) +
		0.2*math.Pow(2, 4.0/3)*ar*ar*math.Pow(nu, -1.0/3) +
		(11.0/35-a*a-12.0/175*ar*ar*ar)/nu +
		(16.0/1575*ar+92.0/7875*math.Pow(ar, 4))*math.Pow(2, 2.0/3)*math.Pow(nu, -5.0/3) -
		(15152.0/3031875*math.Pow(ar, 5)+1088.0/121275*ar*ar)*math.Pow(2, 1.0/3)*math.Pow(nu, -7.0/3)
	if r < 0 {
		ar = 0
	} else {
		ar = math.Sqrt(r)
	}
	return ar
}

// airyRtsExact are the first airy roots.
var airyRtsExact = []float64{
	-2.338107410459762,
	-4.087949444130970,
	-5.520559828095555,
	-6.786708090071765,
	-7.944133587120863,
	-9.022650853340979,
	-10.040174341558084,
	-11.008524303733260,
	-11.936015563236262,
	-12.828776752865757,
}
