// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package samplemv

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/gonum/stat/distmv"
)

type lhDist interface {
	Quantile(x, p []float64) []float64
	CDF(p, x []float64) []float64
	Dim() int
}

func TestLatinHypercube(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for _, nSamples := range []int{1, 2, 5, 10, 20} {
		for _, dist := range []lhDist{
			distmv.NewUniform([]distmv.Bound{{0, 3}}, src),
			distmv.NewUniform([]distmv.Bound{{0, 3}, {-1, 5}, {-4, -1}}, src),
		} {
			dim := dist.Dim()
			batch := mat.NewDense(nSamples, dim, nil)
			LatinHypercube{Src: src, Q: dist}.Sample(batch)
			// Latin hypercube should have one entry per hyperrow.
			present := make([][]bool, nSamples)
			for i := range present {
				present[i] = make([]bool, dim)
			}
			cdf := make([]float64, dim)
			for i := 0; i < nSamples; i++ {
				dist.CDF(cdf, batch.RawRowView(i))
				for j := 0; j < dim; j++ {
					p := cdf[j]
					quadrant := int(math.Floor(p * float64(nSamples)))
					present[quadrant][j] = true
				}
			}
			allPresent := true
			for i := 0; i < nSamples; i++ {
				for j := 0; j < dim; j++ {
					if !present[i][j] {
						allPresent = false
					}
				}
			}
			if !allPresent {
				t.Errorf("All quadrants not present")
			}
		}
	}
}

func TestImportance(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	// Test by finding the expected value of a multi-variate normal.
	dim := 3
	target, ok := randomNormal(dim, src)
	if !ok {
		t.Fatal("bad test, sigma not pos def")
	}

	muImp := make([]float64, dim)
	sigmaImp := mat.NewSymDense(dim, nil)
	for i := 0; i < dim; i++ {
		sigmaImp.SetSym(i, i, 3)
	}
	proposal, ok := distmv.NewNormal(muImp, sigmaImp, src)
	if !ok {
		t.Fatal("bad test, sigma not pos def")
	}

	nSamples := 200000
	batch := mat.NewDense(nSamples, dim, nil)
	weights := make([]float64, nSamples)
	Importance{Target: target, Proposal: proposal}.SampleWeighted(batch, weights)

	compareNormal(t, target, batch, weights, 5e-2, 5e-2)
}

func TestRejection(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	// Test by finding the expected value of a uniform.
	dim := 3
	bounds := make([]distmv.Bound, dim)
	for i := 0; i < dim; i++ {
		min := src.NormFloat64()
		max := src.NormFloat64()
		if min > max {
			min, max = max, min
		}
		bounds[i].Min = min
		bounds[i].Max = max
	}
	target := distmv.NewUniform(bounds, src)
	mu := target.Mean(nil)

	muImp := make([]float64, dim)
	sigmaImp := mat.NewSymDense(dim, nil)
	for i := 0; i < dim; i++ {
		sigmaImp.SetSym(i, i, 6)
	}
	proposal, ok := distmv.NewNormal(muImp, sigmaImp, src)
	if !ok {
		t.Fatal("bad test, sigma not pos def")
	}

	nSamples := 1000
	batch := mat.NewDense(nSamples, dim, nil)
	weights := make([]float64, nSamples)
	rej := Rejection{Target: target, Proposal: proposal, C: 1000, Src: src}
	rej.Sample(batch)
	err := rej.Err()
	if err != nil {
		t.Error("Bad test, nan samples")
	}

	for i := 0; i < dim; i++ {
		col := mat.Col(nil, i, batch)
		ev := stat.Mean(col, weights)
		if math.Abs(ev-mu[i]) > 1e-2 {
			t.Errorf("Mean mismatch: Want %v, got %v", mu[i], ev)
		}
	}
}

func TestMetropolisHastings(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	// Test by finding the expected value of a normal distribution.
	dim := 3
	target, ok := randomNormal(dim, src)
	if !ok {
		t.Fatal("bad test, sigma not pos def")
	}

	sigmaImp := mat.NewSymDense(dim, nil)
	for i := 0; i < dim; i++ {
		sigmaImp.SetSym(i, i, 0.25)
	}
	proposal, ok := NewProposalNormal(sigmaImp, src)
	if !ok {
		t.Fatal("bad test, sigma not pos def")
	}

	nSamples := 100000
	burnin := 5000
	batch := mat.NewDense(nSamples, dim, nil)
	initial := make([]float64, dim)
	metropolisHastings(batch, initial, target, proposal, src)
	batch = batch.Slice(burnin, nSamples, 0, dim).(*mat.Dense)

	compareNormal(t, target, batch, nil, 5e-1, 5e-1)
}

// randomNormal constructs a random Normal distribution.
func randomNormal(dim int, src *rand.Rand) (*distmv.Normal, bool) {
	data := make([]float64, dim*dim)
	for i := range data {
		data[i] = rand.Float64()
	}
	a := mat.NewDense(dim, dim, data)
	var sigma mat.SymDense
	sigma.SymOuterK(1, a)
	mu := make([]float64, dim)
	for i := range mu {
		mu[i] = rand.NormFloat64()
	}
	return distmv.NewNormal(mu, &sigma, src)
}

func compareNormal(t *testing.T, want *distmv.Normal, batch *mat.Dense, weights []float64, meanTol, covTol float64) {
	dim := want.Dim()
	mu := want.Mean(nil)
	sigma := want.CovarianceMatrix(nil)
	n, _ := batch.Dims()
	if weights == nil {
		weights = make([]float64, n)
		for i := range weights {
			weights[i] = 1
		}
	}
	for i := 0; i < dim; i++ {
		col := mat.Col(nil, i, batch)
		ev := stat.Mean(col, weights)
		if math.Abs(ev-mu[i]) > meanTol {
			t.Errorf("Mean mismatch: Want %v, got %v", mu[i], ev)
		}
	}

	cov := stat.CovarianceMatrix(nil, batch, weights)
	if !mat.EqualApprox(cov, sigma, covTol) {
		t.Errorf("Covariance matrix mismatch")
	}
}

func TestMetropolisHastingser(t *testing.T) {
	for _, test := range []struct {
		dim, burnin, rate, samples int
	}{
		{3, 10, 1, 1},
		{3, 10, 2, 1},
		{3, 10, 1, 2},
		{3, 10, 3, 2},
		{3, 10, 7, 4},
		{3, 10, 7, 4},

		{3, 11, 51, 103},
		{3, 11, 103, 51},
		{3, 51, 11, 103},
		{3, 51, 103, 11},
		{3, 103, 11, 51},
		{3, 103, 51, 11},
	} {
		dim := test.dim

		initial := make([]float64, dim)
		target, ok := randomNormal(dim, nil)
		if !ok {
			t.Fatal("bad test, sigma not pos def")
		}

		sigmaImp := mat.NewSymDense(dim, nil)
		for i := 0; i < dim; i++ {
			sigmaImp.SetSym(i, i, 0.25)
		}

		// Test the Metropolis Hastingser by generating all the samples, then generating
		// the same samples with a burnin and rate.
		src := rand.New(rand.NewSource(1))
		proposal, ok := NewProposalNormal(sigmaImp, src)
		if !ok {
			t.Fatal("bad test, sigma not pos def")
		}

		mh := MetropolisHastingser{
			Initial:  initial,
			Target:   target,
			Proposal: proposal,
			Src:      src,
			BurnIn:   0,
			Rate:     0,
		}
		samples := test.samples
		burnin := test.burnin
		rate := test.rate
		fullBatch := mat.NewDense(1+burnin+rate*(samples-1), dim, nil)
		mh.Sample(fullBatch)

		src = rand.New(rand.NewSource(1))
		proposal, _ = NewProposalNormal(sigmaImp, src)
		mh = MetropolisHastingser{
			Initial:  initial,
			Target:   target,
			Proposal: proposal,
			Src:      src,
			BurnIn:   burnin,
			Rate:     rate,
		}
		batch := mat.NewDense(samples, dim, nil)
		mh.Sample(batch)

		same := true
		count := burnin
		for i := 0; i < samples; i++ {
			if !floats.Equal(batch.RawRowView(i), fullBatch.RawRowView(count)) {
				fmt.Println("sample ", i, "is different")
				same = false
				break
			}
			count += rate
		}

		if !same {
			fmt.Printf("%v\n", mat.Formatted(batch))
			fmt.Printf("%v\n", mat.Formatted(fullBatch))

			t.Errorf("sampling mismatch: dim = %v, burnin = %v, rate = %v, samples = %v", dim, burnin, rate, samples)
		}
	}
}
