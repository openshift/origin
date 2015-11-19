// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"fmt"
	"testing"

	"gopkg.in/check.v1"
)

// Tests
func Test(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func leaksPanic(fn Panicker) (panicked bool) {
	defer func() {
		r := recover()
		panicked = r != nil
	}()
	Maybe(fn)
	return
}

func panics(fn func()) (panicked bool, message string) {
	defer func() {
		r := recover()
		panicked = r != nil
		message = fmt.Sprint(r)
	}()
	fn()
	return
}

func flatten(f [][]float64) (r, c int, d []float64) {
	r = len(f)
	if r == 0 {
		panic("bad test: no row")
	}
	c = len(f[0])
	d = make([]float64, 0, r*c)
	for _, row := range f {
		if len(row) != c {
			panic("bad test: ragged input")
		}
		d = append(d, row...)
	}
	return r, c, d
}

func unflatten(r, c int, d []float64) [][]float64 {
	m := make([][]float64, r)
	for i := 0; i < r; i++ {
		m[i] = d[i*c : (i+1)*c]
	}
	return m
}

func eye() *Dense {
	return NewDense(3, 3, []float64{
		1, 0, 0,
		0, 1, 0,
		0, 0, 1,
	})
}

func (s *S) TestMaybe(c *check.C) {
	for i, test := range []struct {
		fn     Panicker
		panics bool
	}{
		{
			func() {},
			false,
		},
		{
			func() { panic("panic") },
			true,
		},
		{
			func() { panic(Error{"panic"}) },
			false,
		},
	} {
		c.Check(leaksPanic(test.fn), check.Equals, test.panics, check.Commentf("Test %d", i))
	}
}

func (s *S) TestSolve(c *check.C) {
	for _, test := range []struct {
		name     string
		panics   bool
		singular bool
		a        [][]float64
		b        [][]float64
		x        [][]float64
	}{
		{
			name:   "OneElement",
			panics: false,
			a:      [][]float64{{6}},
			b:      [][]float64{{3}},
			x:      [][]float64{{0.5}},
		},
		{
			name:   "SquareIdentity",
			panics: false,
			a: [][]float64{
				{1, 0, 0},
				{0, 1, 0},
				{0, 0, 1},
			},
			b: [][]float64{
				{3},
				{2},
				{1},
			},
			x: [][]float64{
				{3},
				{2},
				{1},
			},
		},
		{
			name:   "Square",
			panics: false,
			a: [][]float64{
				{0.8147, 0.9134, 0.5528},
				{0.9058, 0.6324, 0.8723},
				{0.1270, 0.0975, 0.7612},
			},
			b: [][]float64{
				{0.278},
				{0.547},
				{0.958},
			},
			x: [][]float64{
				{-0.932687281002860},
				{0.303963920182067},
				{1.375216503507109},
			},
		},
		{
			name:   "ColumnMismatch",
			panics: true,
			a: [][]float64{
				{0.6046602879796196, 0.9405090880450124, 0.6645600532184904},
				{0.4377141871869802, 0.4246374970712657, 0.6868230728671094},
			},
			b: [][]float64{
				{0.30091186058528707},
				{0.5152126285020654},
				{0.8136399609900968},
				{0.12345},
			},
			x: [][]float64{
				{-26.618512183136257},
				{8.730387239011677},
				{12.316510032082446},
				{0.1234},
			},
		},
		{
			name:   "WideMatrix",
			panics: false,
			a: [][]float64{
				{0.8147, 0.9134, 0.5528},
				{0.9058, 0.6324, 0.8723},
			},
			b: [][]float64{
				{0.278},
				{0.547},
			},
			x: [][]float64{
				{0.25919787248965376},
				{-0.25560256266441034},
				{0.5432324059702451},
			},
		},

		{
			name:   "Skinny1",
			panics: false,
			a: [][]float64{
				{0.8147, 0.9134, 0.9},
				{0.9058, 0.6324, 0.9},
				{0.1270, 0.0975, 0.1},
				{1.6, 2.8, -3.5},
			},
			b: [][]float64{
				{0.278},
				{0.547},
				{-0.958},
				{1.452},
			},
			x: [][]float64{
				{0.820970340787782},
				{-0.218604626527306},
				{-0.212938815234215},
			},
		},
		{
			name:   "Skinny2",
			panics: false,
			a: [][]float64{
				{0.8147, 0.9134, 0.231, -1.65},
				{0.9058, 0.6324, 0.9, 0.72},
				{0.1270, 0.0975, 0.1, 1.723},
				{1.6, 2.8, -3.5, 0.987},
				{7.231, 9.154, 1.823, 0.9},
			},
			b: [][]float64{
				{0.278, 8.635},
				{0.547, 9.125},
				{-0.958, -0.762},
				{1.452, 1.444},
				{1.999, -7.234},
			},
			x: [][]float64{
				{1.863006789511373, 44.467887791812750},
				{-1.127270935407224, -34.073794226035126},
				{-0.527926457947330, -8.032133759788573},
				{-0.248621916204897, -2.366366415805275},
			},
		},
		{
			name:     "Singular square",
			singular: true,
			a: [][]float64{
				{0, 0},
				{0, 0},
			},
			b: [][]float64{
				{3},
				{2},
			},
			x: nil,
		},
		{
			name:     "Singular tall",
			singular: true,
			a: [][]float64{
				{0, 0},
				{0, 0},
				{0, 0},
			},
			b: [][]float64{
				{3},
				{2},
			},
			x: nil,
		},
		{
			name:     "Singular wide",
			singular: true,
			a: [][]float64{
				{0, 0, 0},
				{0, 0, 0},
			},
			b: [][]float64{
				{3},
				{2},
				{1},
			},
			x: nil,
		},
	} {
		a := NewDense(flatten(test.a))
		b := NewDense(flatten(test.b))

		var x *Dense
		fn := func() {
			var err error
			x, err = Solve(a, b)
			if (err == ErrSingular) != test.singular {
				c.Errorf("Unexpected error for Solve %v: got:%v", test.name, err)
			}
		}

		panicked, message := panics(fn)
		if test.singular {
			c.Check(x, check.Equals, (*Dense)(nil))
			continue
		}
		if panicked {
			c.Check(panicked, check.Equals, test.panics, check.Commentf("Test %v panicked: %s", test.name, message))
			continue
		}

		trueX := NewDense(flatten(test.x))
		c.Check(x.EqualsApprox(trueX, 1e-13), check.Equals, true, check.Commentf("Test %v solution mismatch: Found %v, expected %v ", test.name, x, trueX))
	}
}
