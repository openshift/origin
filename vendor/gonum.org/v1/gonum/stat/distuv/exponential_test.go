// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"math"
	"sort"
	"testing"

	"golang.org/x/exp/rand"
)

func TestExponentialProb(t *testing.T) {
	pts := []univariateProbPoint{
		{
			loc:     0,
			prob:    1,
			cumProb: 0,
			logProb: 0,
		},
		{
			loc:     -1,
			prob:    0,
			cumProb: 0,
			logProb: math.Inf(-1),
		},
		{
			loc:     1,
			prob:    1 / (math.E),
			cumProb: 0.6321205588285576784044762298385391325541888689682321654921631983025385042551001966428527256540803563,
			logProb: -1,
		},
		{
			loc:     20,
			prob:    math.Exp(-20),
			cumProb: 0.999999997938846377561442172034059619844179023624192724400896307027755338370835976215440646720089072,
			logProb: -20,
		},
	}
	testDistributionProbs(t, Exponential{Rate: 1}, "Exponential", pts)
}

func TestExponentialFitPrior(t *testing.T) {
	testConjugateUpdate(t, func() ConjugateUpdater { return &Exponential{Rate: 13.7} })
}

func TestExponential(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, dist := range []Exponential{
		{Rate: 3, Src: src},
		{Rate: 1.5, Src: src},
		{Rate: 0.9, Src: src},
	} {
		testExponential(t, dist, i)
	}
}

func testExponential(t *testing.T, dist Exponential, i int) {
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

func TestExponentialScore(t *testing.T) {
	for _, test := range []*Exponential{
		{
			Rate: 1,
		},
		{
			Rate: 0.35,
		},
		{
			Rate: 4.6,
		},
	} {
		testDerivParam(t, test)
	}
}

func TestExponentialFitPanic(t *testing.T) {
	e := Exponential{Rate: 2}
	defer func() {
		r := recover()
		if r != nil {
			t.Errorf("unexpected panic for Fit call: %v", r)
		}
	}()
	e.Fit(make([]float64, 10), nil)
}
