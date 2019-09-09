// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"
)

func TestBernouilli(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, dist := range []Bernoulli{
		{P: 0.5, Src: src},
		{P: 0.9, Src: src},
		{P: 0.2, Src: src},
	} {
		testBernouilli(t, dist, i)
	}
}

func testBernouilli(t *testing.T, dist Bernoulli, i int) {
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
	checkProbDiscrete(t, i, x, dist, tol)
}
