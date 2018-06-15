// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"
	"math/rand"
	"testing"

	"github.com/gonum/floats"
)

type Dlartger interface {
	Dlartg(f, g float64) (cs, sn, r float64)
}

func DlartgTest(t *testing.T, impl Dlartger) {
	const tol = 1e-14
	// safmn2 and safmx2 are copied from native.Dlartg.
	safmn2 := math.Pow(dlamchB, math.Trunc(math.Log(dlamchS/dlamchE)/math.Log(dlamchB)/2))
	safmx2 := 1 / safmn2
	rnd := rand.New(rand.NewSource(1))
	for i := 0; i < 1000; i++ {
		var f float64
		var fHuge bool
		switch rnd.Intn(3) {
		case 0:
			// Huge f.
			fHuge = true
			f = math.Pow(10, 10-20*rnd.Float64()) * safmx2
		case 1:
			// Tiny f.
			f = math.Pow(10, 10-20*rnd.Float64()) * safmn2
		default:
			f = rnd.NormFloat64()
		}
		if rnd.Intn(2) == 0 {
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

		cs, sn, r := impl.Dlartg(f, g)

		rWant := cs*f + sn*g
		if !floats.EqualWithinAbsOrRel(math.Abs(rWant), math.Abs(r), tol, tol) {
			t.Errorf("Case f=%v,g=%v: unexpected r. Want %v, got %v", f, g, rWant, r)
		}
		oneTest := cs*cs + sn*sn
		if math.Abs(oneTest-1) > tol {
			t.Errorf("Case f=%v,g=%v: expected cs^2+sn^2==1, got %v", f, g, oneTest)
		}
		if !fHuge && !gHuge {
			zeroTest := -sn*f + cs*g
			if math.Abs(zeroTest) > tol {
				t.Errorf("Case f=%v,g=%v: expected zero, got %v", f, g, zeroTest)
			}
		}
		if math.Abs(f) > math.Abs(g) && cs < 0 {
			t.Errorf("Case f=%v,g=%v: unexpected negative cs %v", f, g, cs)
		}
	}
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
