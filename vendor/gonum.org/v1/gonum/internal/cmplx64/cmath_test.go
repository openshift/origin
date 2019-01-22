// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cmplx64

import (
	"testing"

	math "gonum.org/v1/gonum/internal/math32"
)

// The higher-precision values in vc26 were used to derive the
// input arguments vc (see also comment below). For reference
// only (do not delete).
var vc26 = []complex64{
	(4.97901192488367350108546816 + 7.73887247457810456552351752i),
	(7.73887247457810456552351752 - 0.27688005719200159404635997i),
	(-0.27688005719200159404635997 - 5.01060361827107492160848778i),
	(-5.01060361827107492160848778 + 9.63629370719841737980004837i),
	(9.63629370719841737980004837 + 2.92637723924396464525443662i),
	(2.92637723924396464525443662 + 5.22908343145930665230025625i),
	(5.22908343145930665230025625 + 2.72793991043601025126008608i),
	(2.72793991043601025126008608 + 1.82530809168085506044576505i),
	(1.82530809168085506044576505 - 8.68592476857560136238589621i),
	(-8.68592476857560136238589621 + 4.97901192488367350108546816i),
}

var vc = []complex64{
	(4.9790119248836735e+00 + 7.7388724745781045e+00i),
	(7.7388724745781045e+00 - 2.7688005719200159e-01i),
	(-2.7688005719200159e-01 - 5.0106036182710749e+00i),
	(-5.0106036182710749e+00 + 9.6362937071984173e+00i),
	(9.6362937071984173e+00 + 2.9263772392439646e+00i),
	(2.9263772392439646e+00 + 5.2290834314593066e+00i),
	(5.2290834314593066e+00 + 2.7279399104360102e+00i),
	(2.7279399104360102e+00 + 1.8253080916808550e+00i),
	(1.8253080916808550e+00 - 8.6859247685756013e+00i),
	(-8.6859247685756013e+00 + 4.9790119248836735e+00i),
}

// The expected results below were computed by the high precision calculators
// at http://keisan.casio.com/.  More exact input values (array vc[], above)
// were obtained by printing them with "%.26f".  The answers were calculated
// to 26 digits (by using the "Digit number" drop-down control of each
// calculator).

var abs = []float32{
	9.2022120669932650313380972e+00,
	7.7438239742296106616261394e+00,
	5.0182478202557746902556648e+00,
	1.0861137372799545160704002e+01,
	1.0070841084922199607011905e+01,
	5.9922447613166942183705192e+00,
	5.8978784056736762299945176e+00,
	3.2822866700678709020367184e+00,
	8.8756430028990417290744307e+00,
	1.0011785496777731986390856e+01,
}

var conj = []complex64{
	(4.9790119248836735e+00 - 7.7388724745781045e+00i),
	(7.7388724745781045e+00 + 2.7688005719200159e-01i),
	(-2.7688005719200159e-01 + 5.0106036182710749e+00i),
	(-5.0106036182710749e+00 - 9.6362937071984173e+00i),
	(9.6362937071984173e+00 - 2.9263772392439646e+00i),
	(2.9263772392439646e+00 - 5.2290834314593066e+00i),
	(5.2290834314593066e+00 - 2.7279399104360102e+00i),
	(2.7279399104360102e+00 - 1.8253080916808550e+00i),
	(1.8253080916808550e+00 + 8.6859247685756013e+00i),
	(-8.6859247685756013e+00 - 4.9790119248836735e+00i),
}

var sqrt = []complex64{
	(2.6628203086086130543813948e+00 + 1.4531345674282185229796902e+00i),
	(2.7823278427251986247149295e+00 - 4.9756907317005224529115567e-02i),
	(1.5397025302089642757361015e+00 - 1.6271336573016637535695727e+00i),
	(1.7103411581506875260277898e+00 + 2.8170677122737589676157029e+00i),
	(3.1390392472953103383607947e+00 + 4.6612625849858653248980849e-01i),
	(2.1117080764822417640789287e+00 + 1.2381170223514273234967850e+00i),
	(2.3587032281672256703926939e+00 + 5.7827111903257349935720172e-01i),
	(1.7335262588873410476661577e+00 + 5.2647258220721269141550382e-01i),
	(2.3131094974708716531499282e+00 - 1.8775429304303785570775490e+00i),
	(8.1420535745048086240947359e-01 + 3.0575897587277248522656113e+00i),
}

// special cases
var vcAbsSC = []complex64{
	NaN(),
}
var absSC = []float32{
	math.NaN(),
}
var vcConjSC = []complex64{
	NaN(),
}
var conjSC = []complex64{
	NaN(),
}
var vcIsNaNSC = []complex64{
	complex(math.Inf(-1), math.Inf(-1)),
	complex(math.Inf(-1), math.NaN()),
	complex(math.NaN(), math.Inf(-1)),
	complex(0, math.NaN()),
	complex(math.NaN(), 0),
	complex(math.Inf(1), math.Inf(1)),
	complex(math.Inf(1), math.NaN()),
	complex(math.NaN(), math.Inf(1)),
	complex(math.NaN(), math.NaN()),
}
var isNaNSC = []bool{
	false,
	false,
	false,
	true,
	true,
	false,
	false,
	false,
	true,
}
var vcSqrtSC = []complex64{
	NaN(),
}
var sqrtSC = []complex64{
	NaN(),
}

// functions borrowed from pkg/math/all_test.go
func tolerance(a, b, e float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}

	// note: b is correct (expected) value, a is actual value.
	// make error tolerance a fraction of b, not a.
	if b != 0 {
		e = e * b
		if e < 0 {
			e = -e
		}
	}
	return d < e
}
func veryclose(a, b float32) bool { return tolerance(a, b, 1e-7) }
func alike(a, b float32) bool {
	switch {
	case a != a && b != b: // math.IsNaN(a) && math.IsNaN(b):
		return true
	case a == b:
		return math.Signbit(a) == math.Signbit(b)
	}
	return false
}

func cTolerance(a, b complex64, e float32) bool {
	d := Abs(a - b)
	if b != 0 {
		e = e * Abs(b)
		if e < 0 {
			e = -e
		}
	}
	return d < e
}
func cVeryclose(a, b complex64) bool { return cTolerance(a, b, 1e-7) }
func cAlike(a, b complex64) bool {
	switch {
	case IsNaN(a) && IsNaN(b):
		return true
	case a == b:
		return math.Signbit(real(a)) == math.Signbit(real(b)) && math.Signbit(imag(a)) == math.Signbit(imag(b))
	}
	return false
}

func TestAbs(t *testing.T) {
	for i := 0; i < len(vc); i++ {
		if f := Abs(vc[i]); !veryclose(abs[i], f) {
			t.Errorf("Abs(%g) = %g, want %g", vc[i], f, abs[i])
		}
	}
	for i := 0; i < len(vcAbsSC); i++ {
		if f := Abs(vcAbsSC[i]); !alike(absSC[i], f) {
			t.Errorf("Abs(%g) = %g, want %g", vcAbsSC[i], f, absSC[i])
		}
	}
}
func TestConj(t *testing.T) {
	for i := 0; i < len(vc); i++ {
		if f := Conj(vc[i]); !cVeryclose(conj[i], f) {
			t.Errorf("Conj(%g) = %g, want %g", vc[i], f, conj[i])
		}
	}
	for i := 0; i < len(vcConjSC); i++ {
		if f := Conj(vcConjSC[i]); !cAlike(conjSC[i], f) {
			t.Errorf("Conj(%g) = %g, want %g", vcConjSC[i], f, conjSC[i])
		}
	}
}
func TestIsNaN(t *testing.T) {
	for i := 0; i < len(vcIsNaNSC); i++ {
		if f := IsNaN(vcIsNaNSC[i]); isNaNSC[i] != f {
			t.Errorf("IsNaN(%v) = %v, want %v", vcIsNaNSC[i], f, isNaNSC[i])
		}
	}
}
func TestSqrt(t *testing.T) {
	for i := 0; i < len(vc); i++ {
		if f := Sqrt(vc[i]); !cVeryclose(sqrt[i], f) {
			t.Errorf("Sqrt(%g) = %g, want %g", vc[i], f, sqrt[i])
		}
	}
	for i := 0; i < len(vcSqrtSC); i++ {
		if f := Sqrt(vcSqrtSC[i]); !cAlike(sqrtSC[i], f) {
			t.Errorf("Sqrt(%g) = %g, want %g", vcSqrtSC[i], f, sqrtSC[i])
		}
	}
}

func BenchmarkAbs(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Abs(complex(2.5, 3.5))
	}
}
func BenchmarkConj(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Conj(complex(2.5, 3.5))
	}
}
func BenchmarkSqrt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Sqrt(complex(2.5, 3.5))
	}
}
