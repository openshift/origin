// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distmv

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

type prober interface {
	Prob(x []float64) float64
	LogProb(x []float64) float64
}

type probCase struct {
	dist    prober
	loc     []float64
	logProb float64
}

func testProbability(t *testing.T, cases []probCase) {
	for _, test := range cases {
		logProb := test.dist.LogProb(test.loc)
		if math.Abs(logProb-test.logProb) > 1e-14 {
			t.Errorf("LogProb mismatch: want: %v, got: %v", test.logProb, logProb)
		}
		prob := test.dist.Prob(test.loc)
		if math.Abs(prob-math.Exp(test.logProb)) > 1e-14 {
			t.Errorf("Prob mismatch: want: %v, got: %v", math.Exp(test.logProb), prob)
		}
	}
}

func generateSamples(x *mat.Dense, r Rander) {
	n, _ := x.Dims()
	for i := 0; i < n; i++ {
		r.Rand(x.RawRowView(i))
	}
}

type Meaner interface {
	Mean([]float64) []float64
}

func checkMean(t *testing.T, cas int, x *mat.Dense, m Meaner, tol float64) {
	mean := m.Mean(nil)

	// Check that the answer is identical when using nil or non-nil.
	mean2 := make([]float64, len(mean))
	m.Mean(mean2)
	if !floats.Equal(mean, mean2) {
		t.Errorf("Mean mismatch when providing nil and slice. Case %v", cas)
	}

	// Check that the mean matches the samples.
	r, _ := x.Dims()
	col := make([]float64, r)
	meanEst := make([]float64, len(mean))
	for i := range meanEst {
		meanEst[i] = stat.Mean(mat.Col(col, i, x), nil)
	}
	if !floats.EqualApprox(mean, meanEst, tol) {
		t.Errorf("Returned mean and sample mean mismatch. Case %v. Empirical %v, returned %v", cas, meanEst, mean)
	}
}

type Cover interface {
	CovarianceMatrix(*mat.SymDense) *mat.SymDense
}

func checkCov(t *testing.T, cas int, x *mat.Dense, c Cover, tol float64) {
	cov := c.CovarianceMatrix(nil)
	n := cov.Symmetric()
	cov2 := mat.NewSymDense(n, nil)
	c.CovarianceMatrix(cov2)
	if !mat.Equal(cov, cov2) {
		t.Errorf("Cov mismatch when providing nil and matrix. Case %v", cas)
	}
	var cov3 mat.SymDense
	c.CovarianceMatrix(&cov3)
	if !mat.Equal(cov, &cov3) {
		t.Errorf("Cov mismatch when providing zero matrix. Case %v", cas)
	}

	// Check that the covariance matrix matches the samples
	covEst := stat.CovarianceMatrix(nil, x, nil)
	if !mat.EqualApprox(covEst, cov, tol) {
		t.Errorf("Return cov and sample cov mismatch. Cas %v.\nGot:\n%0.4v\nWant:\n%0.4v", cas, mat.Formatted(cov), mat.Formatted(covEst))
	}
}
