// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

func TestInverseGamma(t *testing.T) {
	// Values extracted from a comparison with scipy
	for _, test := range []struct {
		x, alpha, want float64
	}{
		{0.9, 4.5, 0.050521067785046482},
		{0.04, 45, 0.10550644842525572},
		{20, 0.4, 0.0064691988681571536},
	} {
		pdf := InverseGamma{Alpha: test.alpha, Beta: 1}.Prob(test.x)
		if !floats.EqualWithinAbsOrRel(pdf, test.want, 1e-10, 1e-10) {
			t.Errorf("Pdf mismatch. Got %v, want %v", pdf, test.want)
		}
	}
	src := rand.NewSource(1)
	for i, g := range []InverseGamma{
		{Alpha: 5.6, Beta: 0.5, Src: src},
		{Alpha: 30, Beta: 1.7, Src: src},
		{Alpha: 30.2, Beta: 1.7, Src: src},
	} {
		testInverseGamma(t, g, i)
	}
}

func testInverseGamma(t *testing.T, f InverseGamma, i int) {
	tol := 1e-2
	const n = 1e6
	const bins = 50
	x := make([]float64, n)
	generateSamples(x, f)
	sort.Float64s(x)

	testRandLogProbContinuous(t, i, 0, x, f, tol, bins)
	checkMean(t, i, x, f, tol)
	checkVarAndStd(t, i, x, f, 2e-2)
	checkExKurtosis(t, i, x, f, 2e-1)
	checkProbContinuous(t, i, x, f, 1e-3)
	checkQuantileCDFSurvival(t, i, x, f, 5e-2)
}
