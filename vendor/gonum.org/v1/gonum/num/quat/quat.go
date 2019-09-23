// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

import (
	"fmt"
	"strconv"
	"strings"
)

var zero Number

// Number is a float64 precision quaternion.
type Number struct {
	Real, Imag, Jmag, Kmag float64
}

// Format implements fmt.Formatter.
func (q Number) Format(fs fmt.State, c rune) {
	prec, pOk := fs.Precision()
	if !pOk {
		prec = -1
	}
	width, wOk := fs.Width()
	if !wOk {
		width = -1
	}
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "%T{Real:%#v, Imag:%#v, Jmag:%#v, Kmag:%#v}", q, q.Real, q.Imag, q.Jmag, q.Kmag)
			return
		}
		if fs.Flag('+') {
			fmt.Fprintf(fs, "{Real:%+v, Imag:%+v, Jmag:%+v, Kmag:%+v}", q.Real, q.Imag, q.Jmag, q.Kmag)
			return
		}
		c = 'g'
		prec = -1
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		fre := fmtString(fs, c, prec, width, false)
		fim := fmtString(fs, c, prec, width, true)
		fmt.Fprintf(fs, fmt.Sprintf("(%s%[2]si%[2]sj%[2]sk)", fre, fim), q.Real, q.Imag, q.Jmag, q.Kmag)
	default:
		fmt.Fprintf(fs, "%%!%c(%T=%[2]v)", c, q)
		return
	}
}

// This is horrible, but it's what we have.
func fmtString(fs fmt.State, c rune, prec, width int, wantPlus bool) string {
	var b strings.Builder
	b.WriteByte('%')
	for _, f := range "0+- " {
		if fs.Flag(int(f)) || (f == '+' && wantPlus) {
			b.WriteByte(byte(f))
		}
	}
	if width >= 0 {
		fmt.Fprint(&b, width)
	}
	if prec >= 0 {
		b.WriteByte('.')
		if prec > 0 {
			fmt.Fprint(&b, prec)
		}
	}
	b.WriteRune(c)
	return b.String()
}

// Add returns the sum of x and y.
func Add(x, y Number) Number {
	return Number{
		Real: x.Real + y.Real,
		Imag: x.Imag + y.Imag,
		Jmag: x.Jmag + y.Jmag,
		Kmag: x.Kmag + y.Kmag,
	}
}

// Sub returns the difference of x and y, x-y.
func Sub(x, y Number) Number {
	return Number{
		Real: x.Real - y.Real,
		Imag: x.Imag - y.Imag,
		Jmag: x.Jmag - y.Jmag,
		Kmag: x.Kmag - y.Kmag,
	}
}

// Mul returns the Hamiltonian product of x and y.
func Mul(x, y Number) Number {
	return Number{
		Real: x.Real*y.Real - x.Imag*y.Imag - x.Jmag*y.Jmag - x.Kmag*y.Kmag,
		Imag: x.Real*y.Imag + x.Imag*y.Real + x.Jmag*y.Kmag - x.Kmag*y.Jmag,
		Jmag: x.Real*y.Jmag - x.Imag*y.Kmag + x.Jmag*y.Real + x.Kmag*y.Imag,
		Kmag: x.Real*y.Kmag + x.Imag*y.Jmag - x.Jmag*y.Imag + x.Kmag*y.Real,
	}
}

// Scale returns q scaled by f.
func Scale(f float64, q Number) Number {
	return Number{Real: f * q.Real, Imag: f * q.Imag, Jmag: f * q.Jmag, Kmag: f * q.Kmag}
}

// Parse converts the string s to a Number. The string may be parenthesized and
// has the format [±]N±Ni±Nj±Nk. The order of the components is not strict.
func Parse(s string) (Number, error) {
	if len(s) == 0 {
		return Number{}, parseError{state: -1}
	}
	orig := s

	wantClose := s[0] == '('
	if wantClose {
		if s[len(s)-1] != ')' {
			return Number{}, parseError{string: orig, state: -1}
		}
		s = s[1 : len(s)-1]
	}
	if len(s) == 0 {
		return Number{}, parseError{string: orig, state: -1}
	}
	switch s[0] {
	case 'n', 'N':
		if strings.ToLower(s) == "nan" {
			return NaN(), nil
		}
	case 'i', 'I':
		if strings.ToLower(s) == "inf" {
			return Inf(), nil
		}
	}

	var q Number
	var parts byte
	for i := 0; i < 4; i++ {
		beg, end, p, err := floatPart(s)
		if err != nil {
			return q, parseError{string: orig, state: -1}
		}
		if parts&(1<<p) != 0 {
			return q, parseError{string: orig, state: -1}
		}
		parts |= 1 << p
		var v float64
		switch s[:end] {
		case "-":
			if len(s[end:]) == 0 {
				return q, parseError{string: orig, state: -1}
			}
			v = -1
		case "+":
			if len(s[end:]) == 0 {
				return q, parseError{string: orig, state: -1}
			}
			v = 1
		default:
			v, err = strconv.ParseFloat(s[beg:end], 64)
			if err != nil {
				return q, err
			}
		}
		s = s[end:]
		switch p {
		case 0:
			q.Real = v
		case 1:
			q.Imag = v
			s = s[1:]
		case 2:
			q.Jmag = v
			s = s[1:]
		case 3:
			q.Kmag = v
			s = s[1:]
		}
		if len(s) == 0 {
			return q, nil
		}
		if !isSign(rune(s[0])) {
			return q, parseError{string: orig, state: -1}
		}
	}

	return q, parseError{string: orig, state: -1}
}

func floatPart(s string) (beg, end int, part uint, err error) {
	const (
		wantMantSign = iota
		wantMantIntInit
		wantMantInt
		wantMantFrac
		wantExpSign
		wantExpInt

		wantInfN
		wantInfF
		wantCloseInf

		wantNaNA
		wantNaNN
		wantCloseNaN
	)
	var i, state int
	var r rune
	for i, r = range s {
		switch state {
		case wantMantSign:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isSign(r):
				state = wantMantIntInit
			case isDigit(r):
				state = wantMantInt
			case isDot(r):
				state = wantMantFrac
			case r == 'i', r == 'I':
				state = wantInfN
			case r == 'n', r == 'N':
				state = wantNaNA
			}

		case wantMantIntInit:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isDigit(r):
				state = wantMantInt
			case isDot(r):
				state = wantMantFrac
			case r == 'i':
				// We need to sneak a look-ahead here.
				if i == len(s)-1 || s[i+1] == '-' || s[i+1] == '+' {
					return 0, i, 1, nil
				}
				fallthrough
			case r == 'I':
				state = wantInfN
			case r == 'n', r == 'N':
				state = wantNaNA
			}

		case wantMantInt:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isDigit(r):
				// Do nothing
			case isDot(r):
				state = wantMantFrac
			case isExponent(r):
				state = wantExpSign
			case isSign(r):
				return 0, i, 0, nil
			case r == 'i':
				return 0, i, 1, nil
			case r == 'j':
				return 0, i, 2, nil
			case r == 'k':
				return 0, i, 3, nil
			}

		case wantMantFrac:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isDigit(r):
				// Do nothing
			case isExponent(r):
				state = wantExpSign
			case isSign(r):
				return 0, i, 0, nil
			case r == 'i':
				return 0, i, 1, nil
			case r == 'j':
				return 0, i, 2, nil
			case r == 'k':
				return 0, i, 3, nil
			}

		case wantExpSign:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isSign(r) || isDigit(r):
				state = wantExpInt
			}

		case wantExpInt:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isDigit(r):
				// Do nothing
			case isSign(r):
				return 0, i, 0, nil
			case r == 'i':
				return 0, i, 1, nil
			case r == 'j':
				return 0, i, 2, nil
			case r == 'k':
				return 0, i, 3, nil
			}

		case wantInfN:
			if r != 'n' && r != 'N' {
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			}
			state = wantInfF
		case wantInfF:
			if r != 'f' && r != 'F' {
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			}
			state = wantCloseInf
		case wantCloseInf:
			switch {
			default:
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			case isSign(r):
				return 0, i, 0, nil
			case r == 'i':
				return 0, i, 1, nil
			case r == 'j':
				return 0, i, 2, nil
			case r == 'k':
				return 0, i, 3, nil
			}

		case wantNaNA:
			if r != 'a' && r != 'A' {
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			}
			state = wantNaNN
		case wantNaNN:
			if r != 'n' && r != 'N' {
				return 0, i, 0, parseError{string: s, state: state, rune: r}
			}
			state = wantCloseNaN
		case wantCloseNaN:
			if isSign(rune(s[0])) {
				beg = 1
			}
			switch {
			default:
				return beg, i, 0, parseError{string: s, state: state, rune: r}
			case isSign(r):
				return beg, i, 0, nil
			case r == 'i':
				return beg, i, 1, nil
			case r == 'j':
				return beg, i, 2, nil
			case r == 'k':
				return beg, i, 3, nil
			}
		}
	}
	switch state {
	case wantMantSign, wantExpSign, wantExpInt:
		if state == wantExpInt && isDigit(r) {
			break
		}
		return 0, i, 0, parseError{string: s, state: state, rune: r}
	}
	return 0, len(s), 0, nil
}

func isSign(r rune) bool {
	return r == '+' || r == '-'
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func isExponent(r rune) bool {
	return r == 'e' || r == 'E'
}

func isDot(r rune) bool {
	return r == '.'
}

type parseError struct {
	string string
	state  int
	rune   rune
}

func (e parseError) Error() string {
	if e.state < 0 {
		return fmt.Sprintf("quat: failed to parse: %q", e.string)
	}
	return fmt.Sprintf("quat: failed to parse in state %d with %q: %q", e.state, e.rune, e.string)
}
