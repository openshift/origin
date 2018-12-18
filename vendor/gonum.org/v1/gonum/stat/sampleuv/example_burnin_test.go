// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import "gonum.org/v1/gonum/stat/distuv"

type ProposalDist struct {
	Sigma float64
}

func (p ProposalDist) ConditionalRand(y float64) float64 {
	return distuv.Normal{Mu: y, Sigma: p.Sigma}.Rand()
}

func (p ProposalDist) ConditionalLogProb(x, y float64) float64 {
	return distuv.Normal{Mu: y, Sigma: p.Sigma}.LogProb(x)
}

func ExampleMetropolisHastings_burnin() {
	n := 1000    // The number of samples to generate.
	burnin := 50 // Number of samples to ignore at the start.
	var initial float64
	// target is the distribution from which we would like to sample.
	target := distuv.Weibull{K: 5, Lambda: 0.5}
	// proposal is the proposal distribution. Here, we are choosing
	// a tight Gaussian distribution around the current location. In
	// typical problems, if Sigma is too small, it takes a lot of samples
	// to move around the distribution. If Sigma is too large, it can be hard
	// to find acceptable samples.
	proposal := ProposalDist{Sigma: 0.2}

	samples := make([]float64, n)
	mh := MetropolisHastings{Initial: initial, Target: target, Proposal: proposal, BurnIn: burnin}
	mh.Sample(samples)
}
