// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"math"
	"sort"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/floats"
)

func TestLaplaceProb(t *testing.T) {
	pts := []univariateProbPoint{
		{
			loc:     0,
			prob:    0.5,
			cumProb: 0.5,
			logProb: math.Log(0.5),
		},
		{
			loc:     -1,
			prob:    1 / (2 * math.E),
			cumProb: 0.1839397205857211607977618850807304337229055655158839172539184008487307478724499016785736371729598219,
			logProb: math.Log(1 / (2 * math.E)),
		},
		{
			loc:     1,
			prob:    1 / (2 * math.E),
			cumProb: 0.8160602794142788392022381149192695662770944344841160827460815991512692521275500983214263628270401781,
			logProb: math.Log(1 / (2 * math.E)),
		},
		{
			loc:     -7,
			prob:    1 / (2 * math.Pow(math.E, 7)),
			cumProb: 0.0004559409827772581040015680422046413132368622637180269204080667109447399446551532646631395032324502210,
			logProb: math.Log(1 / (2 * math.Pow(math.E, 7))),
		},
		{
			loc:     7,
			prob:    1 / (2 * math.Pow(math.E, 7)),
			cumProb: 0.9995440590172227418959984319577953586867631377362819730795919332890552600553448467353368604967675498,
			logProb: math.Log(1 / (2 * math.Pow(math.E, 7))),
		},
		{
			loc:     -20,
			prob:    math.Exp(-20.69314718055994530941723212145817656807550013436025525412068000949339362196969471560586332699641869),
			cumProb: 1.030576811219278913982970190077910488187903637799551846486122330814582011892279676639955463952790684 * 1e-9,
			logProb: -20.69314718055994530941723212145817656807550013436025525412068000949339362196969471560586332699641869,
		},
		{
			loc:     20,
			prob:    math.Exp(-20.69314718055994530941723212145817656807550013436025525412068000949339362196969471560586332699641869),
			cumProb: 0.999999998969423188780721086017029809922089511812096362200448153513877669185417988107720323360044536,
			logProb: -20.69314718055994530941723212145817656807550013436025525412068000949339362196969471560586332699641869,
		},
	}
	testDistributionProbs(t, Laplace{Mu: 0, Scale: 1}, "Laplace", pts)
}

func TestLaplace(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, dist := range []Laplace{
		{Mu: 0, Scale: 3, Src: src},
		{Mu: 1, Scale: 1.5, Src: src},
		{Mu: -1, Scale: 0.9, Src: src},
	} {
		testLaplace(t, dist, i)
	}
}

func testLaplace(t *testing.T, dist Laplace, i int) {
	const (
		tol  = 1e-2
		n    = 3e6
		bins = 50
	)
	x := make([]float64, n)
	generateSamples(x, dist)
	sort.Float64s(x)

	checkMean(t, i, x, dist, tol)
	checkVarAndStd(t, i, x, dist, tol)
	checkEntropy(t, i, x, dist, tol)
	checkExKurtosis(t, i, x, dist, tol)
	checkSkewness(t, i, x, dist, tol)
	checkMedian(t, i, x, dist, tol)
	checkQuantileCDFSurvival(t, i, x, dist, tol)
	checkProbContinuous(t, i, x, dist, 1e-10)
	checkProbQuantContinuous(t, i, x, dist, tol)
}

func TestLaplaceFit(t *testing.T) {
	cases := []struct {
		samples   []float64
		weights   []float64
		wantMu    float64
		wantScale float64
	}{
		{
			samples:   []float64{10, 1, 1},
			weights:   nil,
			wantMu:    1,
			wantScale: 3,
		},
		{
			samples:   []float64{10, 1, 1},
			weights:   []float64{10, 10, 10},
			wantMu:    1,
			wantScale: 3,
		},
		{
			samples:   []float64{10, 1, 1},
			weights:   []float64{0, 1, 1},
			wantMu:    1,
			wantScale: 0,
		},
	}
	for i, test := range cases {
		d := Laplace{}
		d.Fit(test.samples, test.weights)
		if !floats.EqualWithinAbsOrRel(d.Mu, test.wantMu, 1e-10, 1e-10) {
			t.Errorf("unexpected location result for test %d: got:%f, want:%f", i, d.Mu, test.wantMu)
		}
		if !floats.EqualWithinAbsOrRel(d.Scale, test.wantScale, 1e-10, 1e-10) {
			t.Errorf("unexpected scale result for test %d: got:%f, want:%f", i, d.Scale, test.wantScale)
		}
	}
}

func TestLaplaceFitRandomSamples(t *testing.T) {

	nSamples := 100000
	src := rand.New(rand.NewSource(1))
	l := Laplace{
		Mu:    3,
		Scale: 5,
		Src:   src,
	}
	samples := make([]float64, nSamples)
	for i := range samples {
		samples[i] = l.Rand()
	}
	le := Laplace{}
	le.Fit(samples, nil)
	if !floats.EqualWithinAbsOrRel(le.Mu, l.Mu, 1e-2, 1e-2) {
		t.Errorf("unexpected location result for random test got:%f, want:%f", le.Mu, l.Mu)
	}
	if !floats.EqualWithinAbsOrRel(le.Scale, l.Scale, 1e-2, 1e-2) {
		t.Errorf("unexpected scale result for random test got:%f, want:%f", le.Scale, l.Scale)
	}
}
