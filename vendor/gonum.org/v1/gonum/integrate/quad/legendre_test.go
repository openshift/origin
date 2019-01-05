// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quad

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

func TestLegendre(t *testing.T) {
	for i, test := range []struct {
		f        func(float64) float64
		min, max float64
		n        []int
		tol      []float64
		ans      float64
	}{
		// Tolerances determined from intuition and a bit of post-hoc tweaking.
		{
			f:   func(x float64) float64 { return math.Exp(x) },
			min: -3,
			max: 5,
			n:   []int{3, 4, 6, 7, 15, 16, 300, 301},
			tol: []float64{5e-2, 5e-3, 5e-6, 1e-7, 1e-14, 1e-14, 1e-14, 1e-14},
			ans: math.Exp(5) - math.Exp(-3),
		},
	} {
		for j, n := range test.n {
			ans := Fixed(test.f, test.min, test.max, n, Legendre{}, 0)
			if !floats.EqualWithinAbsOrRel(ans, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Mismatch. Case = %d, n = %d. Want %v, got %v", i, n, test.ans, ans)
			}
			ans2 := Fixed(test.f, test.min, test.max, n, Legendre{}, 3)
			if !floats.EqualWithinAbsOrRel(ans2, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Mismatch concurrent. Case = %d, n = %d. Want %v, got %v", i, n, test.ans, ans)
			}
		}
	}
}

func TestLegendreSingle(t *testing.T) {
	for c, test := range []struct {
		n        int
		min, max float64
	}{
		{
			n:   100,
			min: -1,
			max: 1,
		},
		{
			n:   50,
			min: -3,
			max: -1,
		},
		{
			n:   1000,
			min: 2,
			max: 7,
		},
	} {
		l := Legendre{}
		n := test.n
		xs := make([]float64, n)
		weights := make([]float64, n)
		l.FixedLocations(xs, weights, test.min, test.max)

		xsSingle := make([]float64, n)
		weightsSingle := make([]float64, n)
		for i := range xsSingle {
			xsSingle[i], weightsSingle[i] = l.FixedLocationSingle(n, i, test.min, test.max)
		}
		if !floats.Equal(xs, xsSingle) {
			t.Errorf("Case %d: xs mismatch batch and single", c)
		}
		if !floats.Equal(weights, weightsSingle) {
			t.Errorf("Case %d: weights mismatch batch and single", c)
		}
	}
}
