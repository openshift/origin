// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distmv

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/diff/fd"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

type mvTest struct {
	Mu      []float64
	Sigma   *mat.SymDense
	Loc     []float64
	Logprob float64
	Prob    float64
}

func TestNormProbs(t *testing.T) {
	dist1, ok := NewNormal([]float64{0, 0}, mat.NewSymDense(2, []float64{1, 0, 0, 1}), nil)
	if !ok {
		t.Errorf("bad test")
	}
	dist2, ok := NewNormal([]float64{6, 7}, mat.NewSymDense(2, []float64{8, 2, 0, 4}), nil)
	if !ok {
		t.Errorf("bad test")
	}
	testProbability(t, []probCase{
		{
			dist:    dist1,
			loc:     []float64{0, 0},
			logProb: -1.837877066409345,
		},
		{
			dist:    dist2,
			loc:     []float64{6, 7},
			logProb: -3.503979321496947,
		},
		{
			dist:    dist2,
			loc:     []float64{1, 2},
			logProb: -7.075407892925519,
		},
	})
}

func TestNewNormalChol(t *testing.T) {
	for _, test := range []struct {
		mean []float64
		cov  *mat.SymDense
	}{
		{
			mean: []float64{2, 3},
			cov:  mat.NewSymDense(2, []float64{1, 0.1, 0.1, 1}),
		},
	} {
		var chol mat.Cholesky
		ok := chol.Factorize(test.cov)
		if !ok {
			panic("bad test")
		}
		n := NewNormalChol(test.mean, &chol, nil)
		// Generate a random number and calculate probability to ensure things
		// have been set properly. See issue #426.
		x := n.Rand(nil)
		_ = n.Prob(x)
	}
}

func TestNormRand(t *testing.T) {
	for _, test := range []struct {
		mean []float64
		cov  []float64
	}{
		{
			mean: []float64{0, 0},
			cov: []float64{
				1, 0,
				0, 1,
			},
		},
		{
			mean: []float64{0, 0},
			cov: []float64{
				1, 0.9,
				0.9, 1,
			},
		},
		{
			mean: []float64{6, 7},
			cov: []float64{
				5, 0.9,
				0.9, 2,
			},
		},
	} {
		dim := len(test.mean)
		cov := mat.NewSymDense(dim, test.cov)
		n, ok := NewNormal(test.mean, cov, nil)
		if !ok {
			t.Errorf("bad covariance matrix")
		}

		nSamples := 1000000
		samps := mat.NewDense(nSamples, dim, nil)
		for i := 0; i < nSamples; i++ {
			n.Rand(samps.RawRowView(i))
		}
		estMean := make([]float64, dim)
		for i := range estMean {
			estMean[i] = stat.Mean(mat.Col(nil, i, samps), nil)
		}
		if !floats.EqualApprox(estMean, test.mean, 1e-2) {
			t.Errorf("Mean mismatch: want: %v, got %v", test.mean, estMean)
		}
		estCov := stat.CovarianceMatrix(nil, samps, nil)
		if !mat.EqualApprox(estCov, cov, 1e-2) {
			t.Errorf("Cov mismatch: want: %v, got %v", cov, estCov)
		}
	}
}

func TestNormalQuantile(t *testing.T) {
	for _, test := range []struct {
		mean []float64
		cov  []float64
	}{
		{
			mean: []float64{6, 7},
			cov: []float64{
				5, 0.9,
				0.9, 2,
			},
		},
	} {
		dim := len(test.mean)
		cov := mat.NewSymDense(dim, test.cov)
		n, ok := NewNormal(test.mean, cov, nil)
		if !ok {
			t.Errorf("bad covariance matrix")
		}

		nSamples := 1000000
		rnd := rand.New(rand.NewSource(1))
		samps := mat.NewDense(nSamples, dim, nil)
		tmp := make([]float64, dim)
		for i := 0; i < nSamples; i++ {
			for j := range tmp {
				tmp[j] = rnd.Float64()
			}
			n.Quantile(samps.RawRowView(i), tmp)
		}
		estMean := make([]float64, dim)
		for i := range estMean {
			estMean[i] = stat.Mean(mat.Col(nil, i, samps), nil)
		}
		if !floats.EqualApprox(estMean, test.mean, 1e-2) {
			t.Errorf("Mean mismatch: want: %v, got %v", test.mean, estMean)
		}
		estCov := stat.CovarianceMatrix(nil, samps, nil)
		if !mat.EqualApprox(estCov, cov, 1e-2) {
			t.Errorf("Cov mismatch: want: %v, got %v", cov, estCov)
		}
	}
}

func TestConditionNormal(t *testing.T) {
	// Uncorrelated values shouldn't influence the updated values.
	for _, test := range []struct {
		mu       []float64
		sigma    *mat.SymDense
		observed []int
		values   []float64

		newMu    []float64
		newSigma *mat.SymDense
	}{
		{
			mu:       []float64{2, 3},
			sigma:    mat.NewSymDense(2, []float64{2, 0, 0, 5}),
			observed: []int{0},
			values:   []float64{10},

			newMu:    []float64{3},
			newSigma: mat.NewSymDense(1, []float64{5}),
		},
		{
			mu:       []float64{2, 3},
			sigma:    mat.NewSymDense(2, []float64{2, 0, 0, 5}),
			observed: []int{1},
			values:   []float64{10},

			newMu:    []float64{2},
			newSigma: mat.NewSymDense(1, []float64{2}),
		},
		{
			mu:       []float64{2, 3, 4},
			sigma:    mat.NewSymDense(3, []float64{2, 0, 0, 0, 5, 0, 0, 0, 10}),
			observed: []int{1},
			values:   []float64{10},

			newMu:    []float64{2, 4},
			newSigma: mat.NewSymDense(2, []float64{2, 0, 0, 10}),
		},
		{
			mu:       []float64{2, 3, 4},
			sigma:    mat.NewSymDense(3, []float64{2, 0, 0, 0, 5, 0, 0, 0, 10}),
			observed: []int{0, 1},
			values:   []float64{10, 15},

			newMu:    []float64{4},
			newSigma: mat.NewSymDense(1, []float64{10}),
		},
		{
			mu:       []float64{2, 3, 4, 5},
			sigma:    mat.NewSymDense(4, []float64{2, 0.5, 0, 0, 0.5, 5, 0, 0, 0, 0, 10, 2, 0, 0, 2, 3}),
			observed: []int{0, 1},
			values:   []float64{10, 15},

			newMu:    []float64{4, 5},
			newSigma: mat.NewSymDense(2, []float64{10, 2, 2, 3}),
		},
	} {
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Fatalf("Bad test, original sigma not positive definite")
		}
		newNormal, ok := normal.ConditionNormal(test.observed, test.values, nil)
		if !ok {
			t.Fatalf("Bad test, update failure")
		}

		if !floats.EqualApprox(test.newMu, newNormal.mu, 1e-12) {
			t.Errorf("Updated mean mismatch. Want %v, got %v.", test.newMu, newNormal.mu)
		}

		var sigma mat.SymDense
		newNormal.chol.ToSym(&sigma)
		if !mat.EqualApprox(test.newSigma, &sigma, 1e-12) {
			t.Errorf("Updated sigma mismatch\n.Want:\n% v\nGot:\n% v\n", test.newSigma, sigma)
		}
	}

	// Test bivariate case where the update rule is analytic
	for _, test := range []struct {
		mu    []float64
		std   []float64
		rho   float64
		value float64
	}{
		{
			mu:    []float64{2, 3},
			std:   []float64{3, 5},
			rho:   0.9,
			value: 1000,
		},
		{
			mu:    []float64{2, 3},
			std:   []float64{3, 5},
			rho:   -0.9,
			value: 1000,
		},
	} {
		std := test.std
		rho := test.rho
		sigma := mat.NewSymDense(2, []float64{std[0] * std[0], std[0] * std[1] * rho, std[0] * std[1] * rho, std[1] * std[1]})
		normal, ok := NewNormal(test.mu, sigma, nil)
		if !ok {
			t.Fatalf("Bad test, original sigma not positive definite")
		}
		newNormal, ok := normal.ConditionNormal([]int{1}, []float64{test.value}, nil)
		if !ok {
			t.Fatalf("Bad test, update failed")
		}
		var newSigma mat.SymDense
		newNormal.chol.ToSym(&newSigma)
		trueMean := test.mu[0] + rho*(std[0]/std[1])*(test.value-test.mu[1])
		if math.Abs(trueMean-newNormal.mu[0]) > 1e-14 {
			t.Errorf("Mean mismatch. Want %v, got %v", trueMean, newNormal.mu[0])
		}
		trueVar := (1 - rho*rho) * std[0] * std[0]
		if math.Abs(trueVar-newSigma.At(0, 0)) > 1e-14 {
			t.Errorf("Std mismatch. Want %v, got %v", trueMean, newNormal.mu[0])
		}
	}

	// Test via sampling.
	for _, test := range []struct {
		mu         []float64
		sigma      *mat.SymDense
		observed   []int
		unobserved []int
		value      []float64
	}{
		// The indices in unobserved must be in ascending order for this test.
		{
			mu:    []float64{2, 3, 4},
			sigma: mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),

			observed:   []int{0},
			unobserved: []int{1, 2},
			value:      []float64{1.9},
		},
		{
			mu:    []float64{2, 3, 4, 5},
			sigma: mat.NewSymDense(4, []float64{2, 0.5, 3, 0.1, 0.5, 1, 0.6, 0.2, 3, 0.6, 10, 0.3, 0.1, 0.2, 0.3, 3}),

			observed:   []int{0, 3},
			unobserved: []int{1, 2},
			value:      []float64{1.9, 2.9},
		},
	} {
		totalSamp := 4000000
		var nSamp int
		samples := mat.NewDense(totalSamp, len(test.mu), nil)
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Errorf("bad test")
		}
		sample := make([]float64, len(test.mu))
		for i := 0; i < totalSamp; i++ {
			normal.Rand(sample)
			isClose := true
			for i, v := range test.observed {
				if math.Abs(sample[v]-test.value[i]) > 1e-1 {
					isClose = false
					break
				}
			}
			if isClose {
				samples.SetRow(nSamp, sample)
				nSamp++
			}
		}

		if nSamp < 100 {
			t.Errorf("bad test, not enough samples")
			continue
		}
		samples = samples.Slice(0, nSamp, 0, len(test.mu)).(*mat.Dense)

		// Compute mean and covariance matrix.
		estMean := make([]float64, len(test.mu))
		for i := range estMean {
			estMean[i] = stat.Mean(mat.Col(nil, i, samples), nil)
		}
		estCov := stat.CovarianceMatrix(nil, samples, nil)

		// Compute update rule.
		newNormal, ok := normal.ConditionNormal(test.observed, test.value, nil)
		if !ok {
			t.Fatalf("Bad test, update failure")
		}

		var subEstMean []float64
		for _, v := range test.unobserved {

			subEstMean = append(subEstMean, estMean[v])
		}
		subEstCov := mat.NewSymDense(len(test.unobserved), nil)
		for i := 0; i < len(test.unobserved); i++ {
			for j := i; j < len(test.unobserved); j++ {
				subEstCov.SetSym(i, j, estCov.At(test.unobserved[i], test.unobserved[j]))
			}
		}

		for i, v := range subEstMean {
			if math.Abs(newNormal.mu[i]-v) > 5e-2 {
				t.Errorf("Mean mismatch. Want %v, got %v.", newNormal.mu[i], v)
			}
		}
		var sigma mat.SymDense
		newNormal.chol.ToSym(&sigma)
		if !mat.EqualApprox(&sigma, subEstCov, 1e-1) {
			t.Errorf("Covariance mismatch. Want:\n%0.8v\nGot:\n%0.8v\n", subEstCov, sigma)
		}
	}
}

func TestCovarianceMatrix(t *testing.T) {
	for _, test := range []struct {
		mu    []float64
		sigma *mat.SymDense
	}{
		{
			mu:    []float64{2, 3, 4},
			sigma: mat.NewSymDense(3, []float64{1, 0.5, 3, 0.5, 8, -1, 3, -1, 15}),
		},
	} {
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Fatalf("Bad test, covariance matrix not positive definite")
		}
		cov := normal.CovarianceMatrix(nil)
		if !mat.EqualApprox(cov, test.sigma, 1e-14) {
			t.Errorf("Covariance mismatch with nil input")
		}
		dim := test.sigma.Symmetric()
		cov = mat.NewSymDense(dim, nil)
		normal.CovarianceMatrix(cov)
		if !mat.EqualApprox(cov, test.sigma, 1e-14) {
			t.Errorf("Covariance mismatch with supplied input")
		}
	}
}

func TestMarginal(t *testing.T) {
	for _, test := range []struct {
		mu       []float64
		sigma    *mat.SymDense
		marginal []int
	}{
		{
			mu:       []float64{2, 3, 4},
			sigma:    mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),
			marginal: []int{0},
		},
		{
			mu:       []float64{2, 3, 4},
			sigma:    mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),
			marginal: []int{0, 2},
		},
		{
			mu:    []float64{2, 3, 4, 5},
			sigma: mat.NewSymDense(4, []float64{2, 0.5, 3, 0.1, 0.5, 1, 0.6, 0.2, 3, 0.6, 10, 0.3, 0.1, 0.2, 0.3, 3}),

			marginal: []int{0, 3},
		},
	} {
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Fatalf("Bad test, covariance matrix not positive definite")
		}
		marginal, ok := normal.MarginalNormal(test.marginal, nil)
		if !ok {
			t.Fatalf("Bad test, marginal matrix not positive definite")
		}
		dim := normal.Dim()
		nSamples := 1000000
		samps := mat.NewDense(nSamples, dim, nil)
		for i := 0; i < nSamples; i++ {
			normal.Rand(samps.RawRowView(i))
		}
		estMean := make([]float64, dim)
		for i := range estMean {
			estMean[i] = stat.Mean(mat.Col(nil, i, samps), nil)
		}
		for i, v := range test.marginal {
			if math.Abs(marginal.mu[i]-estMean[v]) > 1e-2 {
				t.Errorf("Mean mismatch: want: %v, got %v", estMean[v], marginal.mu[i])
			}
		}

		marginalCov := marginal.CovarianceMatrix(nil)
		estCov := stat.CovarianceMatrix(nil, samps, nil)
		for i, v1 := range test.marginal {
			for j, v2 := range test.marginal {
				c := marginalCov.At(i, j)
				ec := estCov.At(v1, v2)
				if math.Abs(c-ec) > 5e-2 {
					t.Errorf("Cov mismatch element i = %d, j = %d: want: %v, got %v", i, j, c, ec)
				}
			}
		}
	}
}

func TestMarginalSingle(t *testing.T) {
	for _, test := range []struct {
		mu    []float64
		sigma *mat.SymDense
	}{
		{
			mu:    []float64{2, 3, 4},
			sigma: mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),
		},
		{
			mu:    []float64{2, 3, 4, 5},
			sigma: mat.NewSymDense(4, []float64{2, 0.5, 3, 0.1, 0.5, 1, 0.6, 0.2, 3, 0.6, 10, 0.3, 0.1, 0.2, 0.3, 3}),
		},
	} {
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Fatalf("Bad test, covariance matrix not positive definite")
		}
		for i, mean := range test.mu {
			norm := normal.MarginalNormalSingle(i, nil)
			if norm.Mean() != mean {
				t.Errorf("Mean mismatch nil Sigma, idx %v: want %v, got %v.", i, mean, norm.Mean())
			}
			std := math.Sqrt(test.sigma.At(i, i))
			if math.Abs(norm.StdDev()-std) > 1e-14 {
				t.Errorf("StdDev mismatch nil Sigma, idx %v: want %v, got %v.", i, std, norm.StdDev())
			}
		}
	}

	// Test matching with TestMarginal.
	rnd := rand.New(rand.NewSource(1))
	for cas := 0; cas < 10; cas++ {
		dim := rnd.Intn(10) + 1
		mu := make([]float64, dim)
		for i := range mu {
			mu[i] = rnd.Float64()
		}
		x := make([]float64, dim*dim)
		for i := range x {
			x[i] = rnd.Float64()
		}
		matrix := mat.NewDense(dim, dim, x)
		var sigma mat.SymDense
		sigma.SymOuterK(1, matrix)

		normal, ok := NewNormal(mu, &sigma, nil)
		if !ok {
			t.Fatal("bad test")
		}
		for i := 0; i < dim; i++ {
			single := normal.MarginalNormalSingle(i, nil)
			mult, ok := normal.MarginalNormal([]int{i}, nil)
			if !ok {
				t.Fatal("bad test")
			}
			if math.Abs(single.Mean()-mult.Mean(nil)[0]) > 1e-14 {
				t.Errorf("Mean mismatch")
			}
			if math.Abs(single.Variance()-mult.CovarianceMatrix(nil).At(0, 0)) > 1e-14 {
				t.Errorf("Variance mismatch")
			}
		}
	}
}

func TestNormalScoreInput(t *testing.T) {
	for cas, test := range []struct {
		mu    []float64
		sigma *mat.SymDense
		x     []float64
	}{
		{
			mu:    []float64{2, 3, 4},
			sigma: mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),
			x:     []float64{1, 3.1, -2},
		},
		{
			mu:    []float64{2, 3, 4, 5},
			sigma: mat.NewSymDense(4, []float64{2, 0.5, 3, 0.1, 0.5, 1, 0.6, 0.2, 3, 0.6, 10, 0.3, 0.1, 0.2, 0.3, 3}),
			x:     []float64{1, 3.1, -2, 5},
		},
	} {
		normal, ok := NewNormal(test.mu, test.sigma, nil)
		if !ok {
			t.Fatalf("Bad test, covariance matrix not positive definite")
		}
		x := make([]float64, len(test.x))
		copy(x, test.x)
		score := normal.ScoreInput(nil, x)
		if !floats.Equal(x, test.x) {
			t.Errorf("x modified during call to ScoreInput")
		}
		scoreFD := fd.Gradient(nil, normal.LogProb, x, nil)
		if !floats.EqualApprox(score, scoreFD, 1e-4) {
			t.Errorf("Case %d: derivative mismatch. Got %v, want %v", cas, score, scoreFD)
		}
	}
}
