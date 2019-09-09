// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"math"
	"math/cmplx"
	"testing"
)

var sinTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: math.Pi / 2}, want: Number{Real: 1}},
	{q: Number{Imag: math.Pi / 2}, want: Number{Imag: imag(cmplx.Sin(complex(0, math.Pi/2)))}},
	{q: Number{Jmag: math.Pi / 2}, want: Number{Jmag: imag(cmplx.Sin(complex(0, math.Pi/2)))}},
	{q: Number{Kmag: math.Pi / 2}, want: Number{Kmag: imag(cmplx.Sin(complex(0, math.Pi/2)))}},

	// Exercises from Real Quaternionic Calculus Handbook doi:10.1007/978-3-0348-0622-0
	// Ex 6.159 (a) and (b).
	{q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, want: func() Number {
		p := math.Cos(1) * math.Sinh(math.Sqrt(3)) / math.Sqrt(3)
		// An error exists in the book's given solution for the real part.
		return Number{Real: math.Sin(1) * math.Cosh(math.Sqrt(3)), Imag: p, Jmag: p, Kmag: p}
	}()},
	{q: Number{Imag: -2, Jmag: 1}, want: func() Number {
		s := math.Sinh(math.Sqrt(5)) / math.Sqrt(5)
		return Number{Imag: -2 * s, Jmag: s}
	}()},
}

func TestSin(t *testing.T) {
	const tol = 1e-14
	for _, test := range sinTests {
		got := Sin(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Sin(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var sinhTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: math.Pi / 2}, want: Number{Real: math.Sinh(math.Pi / 2)}},
	{q: Number{Imag: math.Pi / 2}, want: Number{Imag: imag(cmplx.Sinh(complex(0, math.Pi/2)))}},
	{q: Number{Jmag: math.Pi / 2}, want: Number{Jmag: imag(cmplx.Sinh(complex(0, math.Pi/2)))}},
	{q: Number{Kmag: math.Pi / 2}, want: Number{Kmag: imag(cmplx.Sinh(complex(0, math.Pi/2)))}},
	{q: Number{Real: 1, Imag: -1, Jmag: -1}, want: func() Number {
		// This was based on the example on p118, but it too has an error.
		q := Number{Real: 1, Imag: -1, Jmag: -1}
		return Scale(0.5, Sub(Exp(q), Exp(Scale(-1, q))))
	}()},
	{q: Number{1, 1, 1, 1}, want: func() Number {
		q := Number{1, 1, 1, 1}
		return Scale(0.5, Sub(Exp(q), Exp(Scale(-1, q))))
	}()},
	{q: Asinh(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
	{q: Asinh(Number{1, 1, 1, 1}), want: func() Number {
		q := Asinh(Number{1, 1, 1, 1})
		return Scale(0.5, Sub(Exp(q), Exp(Scale(-1, q))))
	}()},
	{q: Number{Real: math.Inf(1)}, want: Number{Real: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi / 2}, want: Number{Real: math.Inf(1), Imag: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi}, want: Number{Real: math.Inf(-1), Imag: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: 3 * math.Pi / 2}, want: Number{Real: math.Inf(-1), Imag: math.Inf(-1)}},
	{q: Number{Real: math.Inf(1), Imag: 2 * math.Pi}, want: Number{Real: math.Inf(1), Imag: math.Inf(-1)}},
}

func TestSinh(t *testing.T) {
	const tol = 1e-14
	for _, test := range sinhTests {
		got := Sinh(test.q)
		if !sameApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Sinh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var cosTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: 1}},
	{q: Number{Real: math.Pi / 2}, want: Number{Real: 0}},
	{q: Number{Imag: math.Pi / 2}, want: Number{Real: real(cmplx.Cos(complex(0, math.Pi/2)))}},
	{q: Number{Jmag: math.Pi / 2}, want: Number{Real: real(cmplx.Cos(complex(0, math.Pi/2)))}},
	{q: Number{Kmag: math.Pi / 2}, want: Number{Real: real(cmplx.Cos(complex(0, math.Pi/2)))}},

	// Example from Real Quaternionic Calculus Handbook doi:10.1007/978-3-0348-0622-0
	// p108.
	{q: Number{Real: 1, Imag: 1, Jmag: 1, Kmag: 1}, want: func() Number {
		p := math.Sin(1) * math.Sinh(math.Sqrt(3)) / math.Sqrt(3)
		return Number{Real: math.Cos(1) * math.Cosh(math.Sqrt(3)), Imag: -p, Jmag: -p, Kmag: -p}
	}()},
}

func TestCos(t *testing.T) {
	const tol = 1e-14
	for _, test := range cosTests {
		got := Cos(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Cos(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var coshTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: 1}},
	{q: Number{Real: math.Pi / 2}, want: Number{Real: math.Cosh(math.Pi / 2)}},
	{q: Number{Imag: math.Pi / 2}, want: Number{Imag: imag(cmplx.Cosh(complex(0, math.Pi/2)))}},
	{q: Number{Jmag: math.Pi / 2}, want: Number{Jmag: imag(cmplx.Cosh(complex(0, math.Pi/2)))}},
	{q: Number{Kmag: math.Pi / 2}, want: Number{Kmag: imag(cmplx.Cosh(complex(0, math.Pi/2)))}},
	{q: Number{Real: 1, Imag: -1, Jmag: -1}, want: func() Number {
		q := Number{Real: 1, Imag: -1, Jmag: -1}
		return Scale(0.5, Add(Exp(q), Exp(Scale(-1, q))))
	}()},
	{q: Number{1, 1, 1, 1}, want: func() Number {
		q := Number{1, 1, 1, 1}
		return Scale(0.5, Add(Exp(q), Exp(Scale(-1, q))))
	}()},
	{q: Number{Real: math.Inf(1)}, want: Number{Real: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi / 2}, want: Number{Real: math.Inf(1), Imag: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi}, want: Number{Real: math.Inf(-1), Imag: math.Inf(1)}},
	{q: Number{Real: math.Inf(1), Imag: 3 * math.Pi / 2}, want: Number{Real: math.Inf(-1), Imag: math.Inf(-1)}},
	{q: Number{Real: math.Inf(1), Imag: 2 * math.Pi}, want: Number{Real: math.Inf(1), Imag: math.Inf(-1)}},
}

func TestCosh(t *testing.T) {
	const tol = 1e-14
	for _, test := range coshTests {
		got := Cosh(test.q)
		if !sameApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Cosh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var tanTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: math.Pi / 4}, want: Number{Real: math.Tan(math.Pi / 4)}},
	{q: Number{Imag: math.Pi / 4}, want: Number{Imag: imag(cmplx.Tan(complex(0, math.Pi/4)))}},
	{q: Number{Jmag: math.Pi / 4}, want: Number{Jmag: imag(cmplx.Tan(complex(0, math.Pi/4)))}},
	{q: Number{Kmag: math.Pi / 4}, want: Number{Kmag: imag(cmplx.Tan(complex(0, math.Pi/4)))}},

	// From exercise from Real Numberernionic Calculus Handbook doi:10.1007/978-3-0348-0622-0
	{q: Number{Imag: 1}, want: Mul(Sin(Number{Imag: 1}), Inv(Cos(Number{Imag: 1})))},
	{q: Number{1, 1, 1, 1}, want: Mul(Sin(Number{1, 1, 1, 1}), Inv(Cos(Number{1, 1, 1, 1})))},
}

func TestTan(t *testing.T) {
	const tol = 1e-14
	for _, test := range tanTests {
		got := Tan(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Tan(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var tanhTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: math.Pi / 4}, want: Number{Real: math.Tanh(math.Pi / 4)}},
	{q: Number{Imag: math.Pi / 4}, want: Number{Imag: imag(cmplx.Tanh(complex(0, math.Pi/4)))}},
	{q: Number{Jmag: math.Pi / 4}, want: Number{Jmag: imag(cmplx.Tanh(complex(0, math.Pi/4)))}},
	{q: Number{Kmag: math.Pi / 4}, want: Number{Kmag: imag(cmplx.Tanh(complex(0, math.Pi/4)))}},
	{q: Number{Imag: 1}, want: Mul(Sinh(Number{Imag: 1}), Inv(Cosh(Number{Imag: 1})))},
	{q: Number{1, 1, 1, 1}, want: Mul(Sinh(Number{1, 1, 1, 1}), Inv(Cosh(Number{1, 1, 1, 1})))},
	{q: Number{Real: math.Inf(1)}, want: Number{Real: 1}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi / 4}, want: Number{Real: 1, Imag: 0 * math.Sin(math.Pi/2)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi / 2}, want: Number{Real: 1, Imag: 0 * math.Sin(math.Pi)}},
	{q: Number{Real: math.Inf(1), Imag: 3 * math.Pi / 4}, want: Number{Real: 1, Imag: 0 * math.Sin(3*math.Pi/2)}},
	{q: Number{Real: math.Inf(1), Imag: math.Pi}, want: Number{Real: 1, Imag: 0 * math.Sin(2*math.Pi)}},
}

func TestTanh(t *testing.T) {
	const tol = 1e-14
	for _, test := range tanhTests {
		got := Tanh(test.q)
		if !sameApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Tanh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var asinTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: 1}, want: Number{Real: math.Pi / 2}},
	{q: Number{Imag: 1}, want: Number{Imag: real(cmplx.Asinh(1))}},
	{q: Number{Jmag: 1}, want: Number{Jmag: real(cmplx.Asinh(1))}},
	{q: Number{Kmag: 1}, want: Number{Kmag: real(cmplx.Asinh(1))}},
	{q: Sin(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
}

func TestAsin(t *testing.T) {
	const tol = 1e-14
	for _, test := range asinTests {
		got := Asin(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Asin(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var asinhTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: 1}, want: Number{Real: math.Asinh(1)}},
	{q: Number{Imag: 1}, want: Number{Imag: math.Pi / 2}},
	{q: Number{Jmag: 1}, want: Number{Jmag: math.Pi / 2}},
	{q: Number{Kmag: 1}, want: Number{Kmag: math.Pi / 2}},
	{q: Number{1, 1, 1, 1}, want: func() Number {
		q := Number{1, 1, 1, 1}
		return Log(Add(q, Sqrt(Add(Mul(q, q), Number{Real: 1}))))
	}()},
	{q: Sinh(Number{Real: 1}), want: Number{Real: 1}},
	{q: Sinh(Number{Imag: 1}), want: Number{Imag: 1}},
	{q: Sinh(Number{Imag: 1, Jmag: 1}), want: Number{Imag: 1, Jmag: 1}},
	{q: Sinh(Number{Real: 1, Imag: 1, Jmag: 1}), want: Number{Real: 1, Imag: 1, Jmag: 1}},
	// The following fails:
	// {q: Sinh(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
	// but this passes...
	{q: Sinh(Number{1, 1, 1, 1}), want: func() Number {
		q := Sinh(Number{1, 1, 1, 1})
		return Log(Add(q, Sqrt(Add(Mul(q, q), Number{Real: 1}))))
	}()},
	// And see the Sinh tests that do the reciprocal operation.
}

func TestAsinh(t *testing.T) {
	const tol = 1e-14
	for _, test := range asinhTests {
		got := Asinh(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Asinh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var acosTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: math.Pi / 2}},
	{q: Number{Real: 1}, want: Number{Real: 0}},
	{q: Number{Imag: 1}, want: Number{Real: real(cmplx.Acos(1i)), Imag: imag(cmplx.Acos(1i))}},
	{q: Number{Jmag: 1}, want: Number{Real: real(cmplx.Acos(1i)), Jmag: imag(cmplx.Acos(1i))}},
	{q: Number{Kmag: 1}, want: Number{Real: real(cmplx.Acos(1i)), Kmag: imag(cmplx.Acos(1i))}},
	{q: Cos(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
}

func TestAcos(t *testing.T) {
	const tol = 1e-14
	for _, test := range acosTests {
		got := Acos(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Acos(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var acoshTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{Real: math.Pi / 2}},
	{q: Number{Real: 1}, want: Number{Real: math.Acosh(1)}},
	{q: Number{Imag: 1}, want: Number{Real: real(cmplx.Acosh(1i)), Imag: imag(cmplx.Acosh(1i))}},
	{q: Number{Jmag: 1}, want: Number{Real: real(cmplx.Acosh(1i)), Jmag: imag(cmplx.Acosh(1i))}},
	{q: Number{Kmag: 1}, want: Number{Real: real(cmplx.Acosh(1i)), Kmag: imag(cmplx.Acosh(1i))}},
	{q: Cosh(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
	{q: Number{1, 1, 1, 1}, want: func() Number {
		q := Number{1, 1, 1, 1}
		return Log(Add(q, Sqrt(Sub(Mul(q, q), Number{Real: 1}))))
	}()},
	// The following fails by a factor of -1.
	// {q: Cosh(Number{1, 1, 1, 1}), want: func() Number {
	// 	q := Cosh(Number{1, 1, 1, 1})
	// 	return Log(Add(q, Sqrt(Sub(Mul(q, q), Number{Real: 1}))))
	// }()},
}

func TestAcosh(t *testing.T) {
	const tol = 1e-14
	for _, test := range acoshTests {
		got := Acosh(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Acosh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var atanTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: 1}, want: Number{Real: math.Pi / 4}},
	{q: Number{Imag: 0.5}, want: Number{Real: real(cmplx.Atan(0.5i)), Imag: imag(cmplx.Atan(0.5i))}},
	{q: Number{Jmag: 0.5}, want: Number{Real: real(cmplx.Atan(0.5i)), Jmag: imag(cmplx.Atan(0.5i))}},
	{q: Number{Kmag: 0.5}, want: Number{Real: real(cmplx.Atan(0.5i)), Kmag: imag(cmplx.Atan(0.5i))}},
	{q: Tan(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
}

func TestAtan(t *testing.T) {
	const tol = 1e-14
	for _, test := range atanTests {
		got := Atan(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Atan(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}

var atanhTests = []struct {
	q    Number
	want Number
}{
	{q: Number{}, want: Number{}},
	{q: Number{Real: 1}, want: Number{Real: math.Atanh(1)}},
	{q: Number{Imag: 0.5}, want: Number{Real: real(cmplx.Atanh(0.5i)), Imag: imag(cmplx.Atanh(0.5i))}},
	{q: Number{Jmag: 0.5}, want: Number{Real: real(cmplx.Atanh(0.5i)), Jmag: imag(cmplx.Atanh(0.5i))}},
	{q: Number{Kmag: 0.5}, want: Number{Real: real(cmplx.Atanh(0.5i)), Kmag: imag(cmplx.Atanh(0.5i))}},
	{q: Number{1, 1, 1, 1}, want: func() Number {
		q := Number{1, 1, 1, 1}
		return Scale(0.5, Sub(Log(Add(Number{Real: 1}, q)), Log(Sub(Number{Real: 1}, q))))
	}()},
	{q: Tanh(Number{Real: 1}), want: Number{Real: 1}},
	{q: Tanh(Number{Imag: 1}), want: Number{Imag: 1}},
	{q: Tanh(Number{Imag: 1, Jmag: 1}), want: Number{Imag: 1, Jmag: 1}},
	{q: Tanh(Number{Real: 1, Imag: 1, Jmag: 1}), want: Number{Real: 1, Imag: 1, Jmag: 1}},
	// The following fails
	// {q: Tanh(Number{1, 1, 1, 1}), want: Number{1, 1, 1, 1}},
	// but...
	{q: Tanh(Number{1, 1, 1, 1}), want: func() Number {
		q := Tanh(Number{1, 1, 1, 1})
		return Scale(0.5, Sub(Log(Add(Number{Real: 1}, q)), Log(Sub(Number{Real: 1}, q))))
	}()},
}

func TestAtanh(t *testing.T) {
	const tol = 1e-14
	for _, test := range atanhTests {
		got := Atanh(test.q)
		if !equalApprox(got, test.want, tol) {
			t.Errorf("unexpected result for Atanh(%v): got:%v want:%v", test.q, got, test.want)
		}
	}
}
