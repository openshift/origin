// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import "gonum.org/v1/gonum/stat/distuv"

func max(a, b int) int {
	if a < b {
		return b
	}
	return a
}

func ExampleMetropolisHastings_samplingRate() {
	// See Burnin example for a description of these quantities.
	n := 1000
	burnin := 300
	var initial float64
	target := distuv.Weibull{K: 5, Lambda: 0.5}
	proposal := ProposalDist{Sigma: 0.2}

	// Successive samples are correlated with one another through the
	// Markov Chain defined by the proposal distribution. One may use
	// a sampling rate to decrease the correlation in the samples for
	// an increase in computation cost. The rate parameter specifies
	// that for every accepted sample stored in `samples`, rate - 1 accepted
	// samples are not stored in `samples`.
	rate := 50

	mh := MetropolisHastings{
		Initial:  initial,
		Target:   target,
		Proposal: proposal,
		BurnIn:   burnin,
		Rate:     rate,
	}

	samples := make([]float64, n)
	mh.Sample(samples)
}
