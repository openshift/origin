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

func TestBinomialProb(t *testing.T) {
	const tol = 1e-10
	for i, tt := range []struct {
		k    float64
		n    float64
		p    float64
		want float64
	}{
		// Probabilities computed with Wolfram|Alpha (http://wwww.wolframalpha.com)
		{0, 10, 0.5, 0.0009765625},
		{1, 10, 0.5, 0.009765625},
		{2, 10, 0.5, 0.0439453125},
		{3, 10, 0.5, 0.1171875},
		{4, 10, 0.5, 0.205078125},
		{5, 10, 0.75, 5.839920043945313e-02},
		{6, 10, 0.75, 0.1459980010986328},
		{7, 10, 0.75, 0.2502822875976563},
		{8, 10, 0.75, 0.2815675735473633},
		{9, 10, 0.75, 0.1877117156982422},
		{10, 10, 0.75, 5.6313514709472656e-02},

		{0, 25, 0.25, 7.525434581650003e-04},
		{2, 25, 0.25, 2.508478193883334e-02},
		{5, 25, 0.25, 0.1645375881987921},
		{7, 25, 0.25, 0.1654081574485211},
		{10, 25, 0.25, 4.165835076481272e-02},
		{12, 25, 0.01, 4.563372575901533e-18},
		{15, 25, 0.01, 2.956207951505780e-24},
		{17, 25, 0.01, 9.980175928758777e-29},
		{20, 25, 0.99, 4.345539559454088e-06},
		{22, 25, 0.99, 1.843750355939806e-03},
		{25, 25, 0.99, 0.7778213593991468},

		{0.5, 25, 0.5, 0},
		{1.5, 25, 0.5, 0},
		{2.5, 25, 0.5, 0},
		{3.5, 25, 0.5, 0},
		{4.5, 25, 0.5, 0},
		{5.5, 25, 0.5, 0},
		{6.5, 25, 0.5, 0},
		{7.5, 25, 0.5, 0},
		{8.5, 25, 0.5, 0},
		{9.5, 25, 0.5, 0},
	} {
		b := Binomial{N: tt.n, P: tt.p}
		got := b.Prob(tt.k)
		if !floats.EqualWithinRel(got, tt.want, tol) {
			t.Errorf("test-%d: got=%e. want=%e\n", i, got, tt.want)
		}
	}
}

func TestBinomialCDF(t *testing.T) {
	const tol = 1e-10
	for i, tt := range []struct {
		k    float64
		n    float64
		p    float64
		want float64
	}{
		// Cumulative probabilities computed with SciPy
		{0, 10, 0.5, 9.765625e-04},
		{1, 10, 0.5, 1.0742187499999998e-02},
		{2, 10, 0.5, 5.468749999999999e-02},
		{3, 10, 0.5, 1.7187499999999994e-01},
		{4, 10, 0.5, 3.769531249999999e-01},
		{5, 10, 0.25, 9.802722930908203e-01},
		{6, 10, 0.25, 9.964942932128906e-01},
		{7, 10, 0.25, 9.995841979980469e-01},
		{8, 10, 0.25, 9.999704360961914e-01},
		{9, 10, 0.25, 9.999990463256836e-01},
		{10, 10, 0.25, 1.0},

		{0, 25, 0.75, 8.881784197001252e-16},
		{2.5, 25, 0.75, 2.4655832930875472e-12},
		{5, 25, 0.75, 1.243460090449844e-08},
		{7.5, 25, 0.75, 1.060837565347583e-06},
		{10, 25, 0.75, 2.1451240486669576e-04},
		{12.5, 25, 0.01, 9.999999999999999e-01},
		{15, 25, 0.01, 9.999999999999999e-01},
		{17.5, 25, 0.01, 9.999999999999999e-01},
		{20, 25, 0.99, 4.495958469027147e-06},
		{22.5, 25, 0.99, 1.9506768897388268e-03},
		{25, 25, 0.99, 1.0},
	} {
		b := Binomial{N: tt.n, P: tt.p}
		got := b.CDF(tt.k)
		if !floats.EqualWithinRel(got, tt.want, tol) {
			t.Errorf("test-%d: got=%e. want=%e\n", i, got, tt.want)
		}
	}
}

func TestBinomial(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, b := range []Binomial{
		{100, 0.5, src},
		{15, 0.25, src},
		{10, 0.75, src},
		{9000, 0.102, src},
		{1e6, 0.001, src},
		{25, 0.02, src},
		{3, 0.8, src},
	} {
		testBinomial(t, b, i)
	}
}

func testBinomial(t *testing.T, b Binomial, i int) {
	const (
		tol = 1e-2
		n   = 1e6
	)
	x := make([]float64, n)
	generateSamples(x, b)
	sort.Float64s(x)

	checkMean(t, i, x, b, tol)
	checkVarAndStd(t, i, x, b, tol)
	checkExKurtosis(t, i, x, b, 7e-2)
}
