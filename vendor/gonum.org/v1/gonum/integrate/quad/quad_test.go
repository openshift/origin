// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quad

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat/distuv"
)

func TestFixed(t *testing.T) {
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
		{
			f:   distuv.UnitNormal.Prob,
			min: math.Inf(-1),
			max: math.Inf(1),
			n:   []int{15, 16, 50, 51, 300, 301},
			tol: []float64{5e-3, 1e-3, 1e-7, 1e-7, 1e-14, 1e-14},
			ans: 1,
		},
		{
			f:   func(x float64) float64 { return math.Exp(-x) },
			min: 5,
			max: math.Inf(1),
			n:   []int{15, 16, 50, 51, 300, 301},
			tol: []float64{5e-3, 1e-3, 1e-7, 1e-7, 1e-14, 1e-14},
			ans: math.Exp(-5),
		},
		{
			f:   func(x float64) float64 { return math.Exp(x) },
			min: math.Inf(-1),
			max: -5,
			n:   []int{15, 16, 50, 51, 300, 301},
			tol: []float64{5e-3, 1e-3, 1e-7, 1e-7, 1e-14, 1e-14},
			ans: math.Exp(-5),
		},
		{
			f:   func(x float64) float64 { return math.Exp(x) },
			min: 3,
			max: 3,
			n:   []int{15, 16, 50, 51, 300, 301},
			tol: []float64{0, 0, 0, 0, 0, 0},
			ans: 0,
		},
	} {
		for j, n := range test.n {
			ans := Fixed(test.f, test.min, test.max, n, nil, 0)
			if !floats.EqualWithinAbsOrRel(ans, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Case %d, n = %d: Mismatch. Want %v, got %v", i, n, test.ans, ans)
			}
			ans2 := Fixed(test.f, test.min, test.max, n, nil, 3)
			if !floats.EqualWithinAbsOrRel(ans2, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Case %d, n = %d: Mismatch concurrent. Want %v, got %v", i, n, test.ans, ans)
			}
		}
	}
}

// legendreNonSingle wraps Legendre but does not implement FixedLocationSingle.
type legendreNonSingle struct {
	Legendre Legendre
}

func (l legendreNonSingle) FixedLocations(x, weight []float64, min, max float64) {
	l.Legendre.FixedLocations(x, weight, min, max)
}

func TestFixedNonSingle(t *testing.T) {
	// TODO(btracey): Add tests with infinite bounds when we have native support
	// for indefinite integrals.
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
		{
			f:   func(x float64) float64 { return math.Exp(x) },
			min: 3,
			max: 3,
			n:   []int{3, 4, 6, 7, 15, 16, 300, 301},
			tol: []float64{0, 0, 0, 0, 0, 0, 0, 0},
			ans: 0,
		},
	} {
		for j, n := range test.n {
			ans := Fixed(test.f, test.min, test.max, n, legendreNonSingle{}, 0)
			if !floats.EqualWithinAbsOrRel(ans, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Case = %d, n = %d: Mismatch. Want %v, got %v", i, n, test.ans, ans)
			}
			ans2 := Fixed(test.f, test.min, test.max, n, legendreNonSingle{}, 3)
			if !floats.EqualWithinAbsOrRel(ans2, test.ans, test.tol[j], test.tol[j]) {
				t.Errorf("Case = %d, n = %d: Mismatch concurrent. Want %v, got %v", i, n, test.ans, ans)
			}
		}
	}
}
