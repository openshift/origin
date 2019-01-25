// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import (
	"sort"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distuv"
)

const tol = 1e-2

type lhDist interface {
	Quantile(float64) float64
	CDF(float64) float64
}

func TestLatinHypercube(t *testing.T) {
	for _, nSamples := range []int{1, 2, 5, 10, 20} {
		samples := make([]float64, nSamples)
		for _, dist := range []lhDist{
			distuv.Uniform{Min: 0, Max: 1},
			distuv.Uniform{Min: 0, Max: 10},
			distuv.Normal{Mu: 5, Sigma: 3},
		} {
			LatinHypercube{Q: dist}.Sample(samples)
			sort.Float64s(samples)
			for i, v := range samples {
				p := dist.CDF(v)
				if p < float64(i)/float64(nSamples) || p > float64(i+1)/float64(nSamples) {
					t.Errorf("probability out of bounds")
				}
			}
		}
	}
}

func TestImportance(t *testing.T) {
	// Test by finding the expected value of a Normal.
	trueMean := 3.0
	target := distuv.Normal{Mu: trueMean, Sigma: 2}
	proposal := distuv.Normal{Mu: 0, Sigma: 5}
	nSamples := 100000
	x := make([]float64, nSamples)
	weights := make([]float64, nSamples)
	Importance{Target: target, Proposal: proposal}.SampleWeighted(x, weights)
	ev := stat.Mean(x, weights)
	if !floats.EqualWithinAbsOrRel(ev, trueMean, tol, tol) {
		t.Errorf("Mean mismatch: Want %v, got %v", trueMean, ev)
	}
}

func TestRejection(t *testing.T) {
	// Test by finding the expected value of a Normal.
	trueMean := 3.0
	target := distuv.Normal{Mu: trueMean, Sigma: 2}
	proposal := distuv.Normal{Mu: 0, Sigma: 5}

	nSamples := 20000
	x := make([]float64, nSamples)
	r := &Rejection{Target: target, Proposal: proposal, C: 100}
	r.Sample(x)
	ev := stat.Mean(x, nil)
	if !floats.EqualWithinAbsOrRel(ev, trueMean, tol, tol) {
		t.Errorf("Mean mismatch: Want %v, got %v", trueMean, ev)
	}
}

type condNorm struct {
	Sigma float64
}

func (c condNorm) ConditionalRand(y float64) float64 {
	return distuv.Normal{Mu: y, Sigma: c.Sigma}.Rand()
}

func (c condNorm) ConditionalLogProb(x, y float64) float64 {
	return distuv.Normal{Mu: y, Sigma: c.Sigma}.LogProb(x)
}

func TestMetropolisHastings(t *testing.T) {
	// Test by finding the expected value of a Normal.
	trueMean := 3.0
	target := distuv.Normal{Mu: trueMean, Sigma: 2}
	proposal := condNorm{Sigma: 5}

	burnin := 500
	nSamples := 100000 + burnin
	x := make([]float64, nSamples)
	mh := MetropolisHastings{
		Initial:  100,
		Target:   target,
		Proposal: proposal,

		BurnIn: burnin,
	}
	mh.Sample(x)

	ev := stat.Mean(x, nil)
	if !floats.EqualWithinAbsOrRel(ev, trueMean, tol, tol) {
		t.Errorf("Mean mismatch: Want %v, got %v", trueMean, ev)
	}
}
