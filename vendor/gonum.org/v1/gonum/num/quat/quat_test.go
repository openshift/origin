// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

var arithTests = []struct {
	x, y Number
	f    float64

	wantAdd   Number
	wantSub   Number
	wantMul   Number
	wantScale Number
}{
	{
		x: Number{1, 1, 1, 1}, y: Number{1, 1, 1, 1},
		f: 2,

		wantAdd:   Number{2, 2, 2, 2},
		wantSub:   Number{0, 0, 0, 0},
		wantMul:   Number{-2, 2, 2, 2},
		wantScale: Number{2, 2, 2, 2},
	},
	{
		x: Number{1, 1, 1, 1}, y: Number{2, -1, 1, -1},
		f: -2,

		wantAdd:   Number{3, 0, 2, 0},
		wantSub:   Number{-1, 2, 0, 2},
		wantMul:   Number{3, -1, 3, 3},
		wantScale: Number{-2, -2, -2, -2},
	},
	{
		x: Number{1, 2, 3, 4}, y: Number{4, -3, 2, -1},
		f: 2,

		wantAdd:   Number{5, -1, 5, 3},
		wantSub:   Number{-3, 5, 1, 5},
		wantMul:   Number{8, -6, 4, 28},
		wantScale: Number{2, 4, 6, 8},
	},
	{
		x: Number{1, 2, 3, 4}, y: Number{-4, 3, -2, 1},
		f: -2,

		wantAdd:   Number{-3, 5, 1, 5},
		wantSub:   Number{5, -1, 5, 3},
		wantMul:   Number{-8, 6, -4, -28},
		wantScale: Number{-2, -4, -6, -8},
	},
	{
		x: Number{-4, 3, -2, 1}, y: Number{1, 2, 3, 4},
		f: 0.5,

		wantAdd:   Number{-3, 5, 1, 5},
		wantSub:   Number{-5, 1, -5, -3},
		wantMul:   Number{-8, -16, -24, -2},
		wantScale: Number{-2, 1.5, -1, 0.5},
	},
}

func TestArithmetic(t *testing.T) {
	for _, test := range arithTests {
		gotAdd := Add(test.x, test.y)
		if gotAdd != test.wantAdd {
			t.Errorf("unexpected result for %v+%v: got:%v, want:%v", test.x, test.y, gotAdd, test.wantAdd)
		}
		gotSub := Sub(test.x, test.y)
		if gotSub != test.wantSub {
			t.Errorf("unexpected result for %v-%v: got:%v, want:%v", test.x, test.y, gotSub, test.wantSub)
		}
		gotMul := Mul(test.x, test.y)
		if gotMul != test.wantMul {
			t.Errorf("unexpected result for %v*%v: got:%v, want:%v", test.x, test.y, gotMul, test.wantMul)
		}
		gotScale := Scale(test.f, test.x)
		if gotScale != test.wantScale {
			t.Errorf("unexpected result for %v*%v: got:%v, want:%v", test.f, test.x, gotScale, test.wantScale)
		}
	}
}

var formatTests = []struct {
	q      Number
	format string
	want   string
}{
	{q: Number{1.1, 2.1, 3.1, 4.1}, format: "%#v", want: "quat.Number{Real:1.1, Imag:2.1, Jmag:3.1, Kmag:4.1}"},         // Bootstrap test.
	{q: Number{-1.1, -2.1, -3.1, -4.1}, format: "%#v", want: "quat.Number{Real:-1.1, Imag:-2.1, Jmag:-3.1, Kmag:-4.1}"}, // Bootstrap test.
	{q: Number{1.1, 2.1, 3.1, 4.1}, format: "%+v", want: "{Real:1.1, Imag:2.1, Jmag:3.1, Kmag:4.1}"},
	{q: Number{-1.1, -2.1, -3.1, -4.1}, format: "%+v", want: "{Real:-1.1, Imag:-2.1, Jmag:-3.1, Kmag:-4.1}"},
	{q: Number{1, 2, 3, 4}, format: "%v", want: "(1+2i+3j+4k)"},
	{q: Number{-1, -2, -3, -4}, format: "%v", want: "(-1-2i-3j-4k)"},
	{q: Number{1, 2, 3, 4}, format: "%g", want: "(1+2i+3j+4k)"},
	{q: Number{-1, -2, -3, -4}, format: "%g", want: "(-1-2i-3j-4k)"},
	{q: Number{1, 2, 3, 4}, format: "%e", want: "(1.000000e+00+2.000000e+00i+3.000000e+00j+4.000000e+00k)"},
	{q: Number{-1, -2, -3, -4}, format: "%e", want: "(-1.000000e+00-2.000000e+00i-3.000000e+00j-4.000000e+00k)"},
	{q: Number{1, 2, 3, 4}, format: "%E", want: "(1.000000E+00+2.000000E+00i+3.000000E+00j+4.000000E+00k)"},
	{q: Number{-1, -2, -3, -4}, format: "%E", want: "(-1.000000E+00-2.000000E+00i-3.000000E+00j-4.000000E+00k)"},
	{q: Number{1, 2, 3, 4}, format: "%f", want: "(1.000000+2.000000i+3.000000j+4.000000k)"},
	{q: Number{-1, -2, -3, -4}, format: "%f", want: "(-1.000000-2.000000i-3.000000j-4.000000k)"},
}

func TestFormat(t *testing.T) {
	for _, test := range formatTests {
		got := fmt.Sprintf(test.format, test.q)
		if got != test.want {
			t.Errorf("unexpected result for fmt.Sprintf(%q, %#v): got:%q, want:%q", test.format, test.q, got, test.want)
		}
	}
}

var parseTests = []struct {
	s       string
	want    Number
	wantErr error
}{
	// Simple error states:
	{s: "", wantErr: parseError{state: -1}},
	{s: "()", wantErr: parseError{string: "()", state: -1}},
	{s: "(1", wantErr: parseError{string: "(1", state: -1}},
	{s: "1)", wantErr: parseError{string: "1)", state: -1}},

	// Ambiguous parse error states:
	{s: "1+2i+3i", wantErr: parseError{string: "1+2i+3i", state: -1}},
	{s: "1+2i3j", wantErr: parseError{string: "1+2i3j", state: -1}},
	{s: "1e-4i-4k+10.3e6j+", wantErr: parseError{string: "1e-4i-4k+10.3e6j+", state: -1}},
	{s: "1e-4i-4k+10.3e6j-", wantErr: parseError{string: "1e-4i-4k+10.3e6j-", state: -1}},

	// Valid input:
	{s: "1+4i", want: Number{Real: 1, Imag: 4}},
	{s: "4i+1", want: Number{Real: 1, Imag: 4}},
	{s: "+1+4i", want: Number{Real: 1, Imag: 4}},
	{s: "+4i+1", want: Number{Real: 1, Imag: 4}},
	{s: "1e-4-4k+10.3e6j+1i", want: Number{Real: 1e-4, Imag: 1, Jmag: 10.3e6, Kmag: -4}},
	{s: "1e-4-4k+10.3e6j+i", want: Number{Real: 1e-4, Imag: 1, Jmag: 10.3e6, Kmag: -4}},
	{s: "1e-4-4k+10.3e6j-i", want: Number{Real: 1e-4, Imag: -1, Jmag: 10.3e6, Kmag: -4}},
	{s: "1e-4i-4k+10.3e6j-1", want: Number{Real: -1, Imag: 1e-4, Jmag: 10.3e6, Kmag: -4}},
	{s: "1e-4i-4k+10.3e6j+1", want: Number{Real: 1, Imag: 1e-4, Jmag: 10.3e6, Kmag: -4}},
	{s: "(1+4i)", want: Number{Real: 1, Imag: 4}},
	{s: "(4i+1)", want: Number{Real: 1, Imag: 4}},
	{s: "(+1+4i)", want: Number{Real: 1, Imag: 4}},
	{s: "(+4i+1)", want: Number{Real: 1, Imag: 4}},
	{s: "(1e-4-4k+10.3e6j+1i)", want: Number{Real: 1e-4, Imag: 1, Jmag: 10.3e6, Kmag: -4}},
	{s: "(1e-4-4k+10.3e6j+i)", want: Number{Real: 1e-4, Imag: 1, Jmag: 10.3e6, Kmag: -4}},
	{s: "(1e-4-4k+10.3e6j-i)", want: Number{Real: 1e-4, Imag: -1, Jmag: 10.3e6, Kmag: -4}},
	{s: "(1e-4i-4k+10.3e6j-1)", want: Number{Real: -1, Imag: 1e-4, Jmag: 10.3e6, Kmag: -4}},
	{s: "(1e-4i-4k+10.3e6j+1)", want: Number{Real: 1, Imag: 1e-4, Jmag: 10.3e6, Kmag: -4}},
	{s: "NaN", want: NaN()},
	{s: "nan", want: NaN()},
	{s: "Inf", want: Inf()},
	{s: "inf", want: Inf()},
	{s: "(Inf+Infi)", want: Number{Real: math.Inf(1), Imag: math.Inf(1)}},
	{s: "(-Inf+Infi)", want: Number{Real: math.Inf(-1), Imag: math.Inf(1)}},
	{s: "(+Inf-Infi)", want: Number{Real: math.Inf(1), Imag: math.Inf(-1)}},
	{s: "(inf+infi)", want: Number{Real: math.Inf(1), Imag: math.Inf(1)}},
	{s: "(-inf+infi)", want: Number{Real: math.Inf(-1), Imag: math.Inf(1)}},
	{s: "(+inf-infi)", want: Number{Real: math.Inf(1), Imag: math.Inf(-1)}},
	{s: "(nan+nani)", want: Number{Real: math.NaN(), Imag: math.NaN()}},
	{s: "(nan-nani)", want: Number{Real: math.NaN(), Imag: math.NaN()}},
	{s: "(nan+nani+1k)", want: Number{Real: math.NaN(), Imag: math.NaN(), Kmag: 1}},
	{s: "(nan-nani+1k)", want: Number{Real: math.NaN(), Imag: math.NaN(), Kmag: 1}},
}

func TestParse(t *testing.T) {
	for _, test := range parseTests {
		got, err := Parse(test.s)
		if err != test.wantErr {
			t.Errorf("unexpected error for Parse(%q): got:%#v, want:%#v", test.s, err, test.wantErr)
		}
		if err != nil {
			continue
		}
		if !sameNumber(got, test.want) {
			t.Errorf("unexpected result for Parse(%q): got:%v, want:%v", test.s, got, test.want)
		}
	}
}

func equalApprox(a, b Number, tol float64) bool {
	return floats.EqualWithinAbsOrRel(a.Real, b.Real, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Imag, b.Imag, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Jmag, b.Jmag, tol, tol) &&
		floats.EqualWithinAbsOrRel(a.Kmag, b.Kmag, tol, tol)
}

func sameApprox(a, b Number, tol float64) bool {
	switch {
	case a.Real == 0 && b.Real == 0:
		return math.Signbit(a.Real) == math.Signbit(b.Real)
	case a.Imag == 0 && b.Imag == 0:
		return math.Signbit(a.Imag) == math.Signbit(b.Imag)
	case a.Jmag == 0 && b.Jmag == 0:
		return math.Signbit(a.Jmag) == math.Signbit(b.Jmag)
	case a.Kmag == 0 && b.Kmag == 0:
		return math.Signbit(a.Kmag) == math.Signbit(b.Kmag)
	}
	return (sameFloat(a.Real, b.Real) || floats.EqualWithinAbsOrRel(a.Real, b.Real, tol, tol)) &&
		(sameFloat(a.Imag, b.Imag) || floats.EqualWithinAbsOrRel(a.Imag, b.Imag, tol, tol)) &&
		(sameFloat(a.Jmag, b.Jmag) || floats.EqualWithinAbsOrRel(a.Jmag, b.Jmag, tol, tol)) &&
		(sameFloat(a.Kmag, b.Kmag) || floats.EqualWithinAbsOrRel(a.Kmag, b.Kmag, tol, tol))
}

func sameNumber(a, b Number) bool {
	return sameFloat(a.Real, b.Real) &&
		sameFloat(a.Imag, b.Imag) &&
		sameFloat(a.Jmag, b.Jmag) &&
		sameFloat(a.Kmag, b.Kmag)
}

func sameFloat(a, b float64) bool {
	return a == b || (math.IsNaN(a) && math.IsNaN(b))
}
