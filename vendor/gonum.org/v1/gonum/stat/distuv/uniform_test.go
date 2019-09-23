// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"
	"gonum.org/v1/gonum/floats"
)

func TestUniformProb(t *testing.T) {
	for _, test := range []struct {
		min, max, x, want float64
	}{
		{0, 1, 1, 1},
		{2, 4, 0, 0},
		{2, 4, 5, 0},
		{2, 4, 3, 0.5},
		{0, 100, 1, 0.01},
	} {
		pdf := Uniform{test.min, test.max, nil}.Prob(test.x)
		if !floats.EqualWithinAbsOrRel(pdf, test.want, 1e-15, 1e-15) {
			t.Errorf("Pdf mismatch, x = %v, min = %v, max = %v. Got %v, want %v", test.x, test.min, test.max, pdf, test.want)
		}
	}
}

func TestUniformCDF(t *testing.T) {
	for _, test := range []struct {
		min, max, x, want float64
	}{
		{0, 1, 1, 1},
		{0, 100, 100, 1},
		{0, 100, 0, 0},
		{0, 100, 50, 0.5},
		{0, 50, 10, 0.2},
	} {
		cdf := Uniform{test.min, test.max, nil}.CDF(test.x)
		if !floats.EqualWithinAbsOrRel(cdf, test.want, 1e-15, 1e-15) {
			t.Errorf("CDF mismatch, x = %v, min = %v, max = %v. Got %v, want %v", test.x, test.min, test.max, cdf, test.want)
		}
	}
}

func TestUniform(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, b := range []Uniform{
		{1, 2, src},
		{0, 100, src},
		{50, 60, src},
	} {
		testUniform(t, b, i)
	}
}

func testUniform(t *testing.T, u Uniform, i int) {
	tol := 1e-2
	const n = 1e5
	const bins = 50
	x := make([]float64, n)
	generateSamples(x, u)
	sort.Float64s(x)

	testRandLogProbContinuous(t, i, 0, x, u, tol, bins)
	checkMean(t, i, x, u, tol)
	checkVarAndStd(t, i, x, u, tol)
	checkExKurtosis(t, i, x, u, 7e-2)
	checkProbContinuous(t, i, x, u, 1e-3)
	checkQuantileCDFSurvival(t, i, x, u, 1e-2)
}
