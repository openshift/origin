// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dlartger interface {
	Dlartg(f, g float64) (cs, sn, r float64)
}

func DlartgTest(t *testing.T, impl Dlartger) {
	const tol = 1e-14
	// safmn2 and safmx2 are copied from native.Dlartg.
	// safmn2 ~ 2*10^{-146}
	safmn2 := math.Pow(dlamchB, math.Trunc(math.Log(dlamchS/dlamchE)/math.Log(dlamchB)/2))
	// safmx2 ~ 5*10^145
	safmx2 := 1 / safmn2
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		// Generate randomly huge, tiny, and "normal" input arguments to Dlartg.
		var f float64
		var fHuge bool
		switch rnd.Intn(3) {
		case 0:
			// Huge f.
			fHuge = true
			// scale is in the range (10^{-10}, 10^10].
			scale := math.Pow(10, 10-20*rnd.Float64())
			// f is in the range (5*10^135, 5*10^155].
			f = scale * safmx2
		case 1:
			// Tiny f.
			// f is in the range (2*10^{-156}, 2*10^{-136}].
			f = math.Pow(10, 10-20*rnd.Float64()) * safmn2
		default:
			f = rnd.NormFloat64()
		}
		if rnd.Intn(2) == 0 {
			// Sometimes change the sign of f.
			f *= -1
		}

		var g float64
		var gHuge bool
		switch rnd.Intn(3) {
		case 0:
			// Huge g.
			gHuge = true
			g = math.Pow(10, 10-20*rnd.Float64()) * safmx2
		case 1:
			// Tiny g.
			g = math.Pow(10, 10-20*rnd.Float64()) * safmn2
		default:
			g = rnd.NormFloat64()
		}
		if rnd.Intn(2) == 0 {
			g *= -1
		}

		// Generate a plane rotation so that
		//  [ cs sn] * [f] = [r]
		//  [-sn cs]   [g] = [0]
		cs, sn, r := impl.Dlartg(f, g)

		// Check that the first equation holds.
		rWant := cs*f + sn*g
		if !floats.EqualWithinAbsOrRel(math.Abs(rWant), math.Abs(r), tol, tol) {
			t.Errorf("Case f=%v,g=%v: unexpected r. Want %v, got %v", f, g, rWant, r)
		}
		// Check that cs and sn define a plane rotation. The 2×2 matrix
		// has orthogonal columns by construction, so only check that
		// the columns/rows have unit norm.
		oneTest := cs*cs + sn*sn
		if math.Abs(oneTest-1) > tol {
			t.Errorf("Case f=%v,g=%v: expected cs^2+sn^2==1, got %v", f, g, oneTest)
		}
		if !fHuge && !gHuge {
			// Check that the second equation holds.
			// If both numbers are huge, cancellation errors make
			// this test unreliable.
			zeroTest := -sn*f + cs*g
			if math.Abs(zeroTest) > tol {
				t.Errorf("Case f=%v,g=%v: expected zero, got %v", f, g, zeroTest)
			}
		}
		// Check that cs is positive as documented.
		if math.Abs(f) > math.Abs(g) && cs < 0 {
			t.Errorf("Case f=%v,g=%v: unexpected negative cs %v", f, g, cs)
		}
	}
	// Check other documented special cases.
	for i := 0; i < 100; i++ {
		cs, sn, _ := impl.Dlartg(rnd.NormFloat64(), 0)
		if cs != 1 {
			t.Errorf("Unexpected cs for g=0. Want 1, got %v", cs)
		}
		if sn != 0 {
			t.Errorf("Unexpected sn for g=0. Want 0, got %v", sn)
		}
	}
	for i := 0; i < 100; i++ {
		cs, sn, _ := impl.Dlartg(0, rnd.NormFloat64())
		if cs != 0 {
			t.Errorf("Unexpected cs for f=0. Want 0, got %v", cs)
		}
		if sn != 1 {
			t.Errorf("Unexpected sn for f=0. Want 1, got %v", sn)
		}
	}
}
