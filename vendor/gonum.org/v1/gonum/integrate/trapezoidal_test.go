// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package integrate

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

func TestTrapezoidal(t *testing.T) {
	const N = 1e6
	x := floats.Span(make([]float64, N), 0, 1)
	for i, test := range []struct {
		x    []float64
		f    func(x float64) float64
		want float64
	}{
		{
			x:    x,
			f:    func(x float64) float64 { return x },
			want: 0.5,
		},
		{
			x:    floats.Span(make([]float64, N), -1, 1),
			f:    func(x float64) float64 { return x },
			want: 0,
		},
		{
			x:    x,
			f:    func(x float64) float64 { return x + 10 },
			want: 10.5,
		},
		{
			x:    x,
			f:    func(x float64) float64 { return 3*x*x + 10 },
			want: 11,
		},
		{
			x:    x,
			f:    func(x float64) float64 { return math.Exp(x) },
			want: 1.7182818284591876,
		},
		{
			x:    floats.Span(make([]float64, N), 0, math.Pi),
			f:    func(x float64) float64 { return math.Cos(x) },
			want: 0,
		},
		{
			x:    floats.Span(make([]float64, N), 0, 2*math.Pi),
			f:    func(x float64) float64 { return math.Cos(x) },
			want: 0,
		},
		{
			x:    floats.Span(make([]float64, N*10), 0, math.Pi),
			f:    func(x float64) float64 { return math.Sin(x) },
			want: 2,
		},
		{
			x:    floats.Span(make([]float64, N*10), 0, 0.5*math.Pi),
			f:    func(x float64) float64 { return math.Sin(x) },
			want: 1,
		},
		{
			x:    floats.Span(make([]float64, N), 0, 2*math.Pi),
			f:    func(x float64) float64 { return math.Sin(x) },
			want: 0,
		},
	} {
		y := make([]float64, len(test.x))
		for i, v := range test.x {
			y[i] = test.f(v)
		}
		v := Trapezoidal(test.x, y)
		if !floats.EqualWithinAbs(v, test.want, 1e-12) {
			t.Errorf("test #%d: got=%v want=%v\n", i, v, test.want)
		}
	}
}
