// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distmv

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

func TestStudentTProbs(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for _, test := range []struct {
		nu    float64
		mu    []float64
		sigma *mat.SymDense

		x     [][]float64
		probs []float64
	}{
		{
			nu:    3,
			mu:    []float64{0, 0},
			sigma: mat.NewSymDense(2, []float64{1, 0, 0, 1}),

			x: [][]float64{
				{0, 0},
				{1, -1},
				{3, 4},
				{-1, -2},
			},
			// Outputs compared with WolframAlpha.
			probs: []float64{
				0.159154943091895335768883,
				0.0443811199724279860006777747927,
				0.0005980371870904696541052658,
				0.01370560783418571283428283,
			},
		},
		{
			nu:    4,
			mu:    []float64{2, -3},
			sigma: mat.NewSymDense(2, []float64{8, -1, -1, 5}),

			x: [][]float64{
				{0, 0},
				{1, -1},
				{3, 4},
				{-1, -2},
				{2, -3},
			},
			// Outputs compared with WolframAlpha.
			probs: []float64{
				0.007360810111491788657953608191001,
				0.0143309905845607117740440592999,
				0.0005307774290578041397794096037035009801668903,
				0.0115657422475668739943625904793879,
				0.0254851872062589062995305736215,
			},
		},
	} {
		s, ok := NewStudentsT(test.mu, test.sigma, test.nu, src)
		if !ok {
			t.Fatal("bad test")
		}
		for i, x := range test.x {
			xcpy := make([]float64, len(x))
			copy(xcpy, x)
			p := s.Prob(x)
			if !floats.Same(x, xcpy) {
				t.Errorf("X modified during call to prob, %v, %v", x, xcpy)
			}
			if !floats.EqualWithinAbsOrRel(p, test.probs[i], 1e-10, 1e-10) {
				t.Errorf("Probability mismatch. X = %v. Got %v, want %v.", x, p, test.probs[i])
			}
		}
	}
}

func TestStudentsTRand(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		mean   []float64
		cov    *mat.SymDense
		nu     float64
		tolcov float64
	}{
		{
			mean:   []float64{0, 0},
			cov:    mat.NewSymDense(2, []float64{1, 0, 0, 1}),
			nu:     4,
			tolcov: 1e-2,
		},
		{
			mean:   []float64{3, 4},
			cov:    mat.NewSymDense(2, []float64{5, 1.2, 1.2, 6}),
			nu:     8,
			tolcov: 1e-2,
		},
		{
			mean:   []float64{3, 4, -2},
			cov:    mat.NewSymDense(3, []float64{5, 1.2, -0.8, 1.2, 6, 0.4, -0.8, 0.4, 2}),
			nu:     8,
			tolcov: 1e-2,
		},
	} {
		s, ok := NewStudentsT(test.mean, test.cov, test.nu, src)
		if !ok {
			t.Fatal("bad test")
		}
		const nSamples = 1e6
		dim := len(test.mean)
		samps := mat.NewDense(nSamples, dim, nil)
		for i := 0; i < nSamples; i++ {
			s.Rand(samps.RawRowView(i))
		}
		estMean := make([]float64, dim)
		for i := range estMean {
			estMean[i] = stat.Mean(mat.Col(nil, i, samps), nil)
		}
		mean := s.Mean(nil)
		if !floats.EqualApprox(estMean, mean, 1e-2) {
			t.Errorf("Mean mismatch: want: %v, got %v", test.mean, estMean)
		}
		cov := s.CovarianceMatrix(nil)
		estCov := stat.CovarianceMatrix(nil, samps, nil)
		if !mat.EqualApprox(estCov, cov, test.tolcov) {
			t.Errorf("Case %d: Cov mismatch: want: %v, got %v", cas, cov, estCov)
		}
	}
}

func TestStudentsTConditional(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for _, test := range []struct {
		mean []float64
		cov  *mat.SymDense
		nu   float64

		idx    []int
		value  []float64
		tolcov float64
	}{
		{
			mean:  []float64{3, 4, -2},
			cov:   mat.NewSymDense(3, []float64{5, 1.2, -0.8, 1.2, 6, 0.4, -0.8, 0.4, 2}),
			nu:    8,
			idx:   []int{0},
			value: []float64{6},

			tolcov: 1e-2,
		},
	} {
		s, ok := NewStudentsT(test.mean, test.cov, test.nu, src)
		if !ok {
			t.Fatal("bad test")
		}

		sUp, ok := s.ConditionStudentsT(test.idx, test.value, src)
		if !ok {
			t.Error("unexpected failure of ConditionStudentsT")
		}

		// Compute the other values by hand the inefficient way to compare
		newNu := test.nu + float64(len(test.idx))
		if newNu != sUp.nu {
			t.Errorf("Updated nu mismatch. Got %v, want %v", s.nu, newNu)
		}
		dim := len(test.mean)
		unob := findUnob(test.idx, dim)
		ob := test.idx

		muUnob := make([]float64, len(unob))
		for i, v := range unob {
			muUnob[i] = test.mean[v]
		}
		muOb := make([]float64, len(ob))
		for i, v := range ob {
			muOb[i] = test.mean[v]
		}

		var sig11, sig22 mat.SymDense
		sig11.SubsetSym(&s.sigma, unob)
		sig22.SubsetSym(&s.sigma, ob)

		sig12 := mat.NewDense(len(unob), len(ob), nil)
		for i := range unob {
			for j := range ob {
				sig12.Set(i, j, s.sigma.At(unob[i], ob[j]))
			}
		}

		shift := make([]float64, len(ob))
		copy(shift, test.value)
		floats.Sub(shift, muOb)

		newMu := make([]float64, len(muUnob))
		newMuVec := mat.NewVecDense(len(muUnob), newMu)
		shiftVec := mat.NewVecDense(len(shift), shift)
		var tmp mat.VecDense
		tmp.SolveVec(&sig22, shiftVec)
		newMuVec.MulVec(sig12, &tmp)
		floats.Add(newMu, muUnob)

		if !floats.EqualApprox(newMu, sUp.mu, 1e-10) {
			t.Errorf("Mu mismatch. Got %v, want %v", sUp.mu, newMu)
		}

		var tmp2 mat.Dense
		tmp2.Solve(&sig22, sig12.T())

		var tmp3 mat.Dense
		tmp3.Mul(sig12, &tmp2)
		tmp3.Sub(&sig11, &tmp3)

		dot := mat.Dot(shiftVec, &tmp)
		tmp3.Scale((test.nu+dot)/(test.nu+float64(len(ob))), &tmp3)
		if !mat.EqualApprox(&tmp3, &sUp.sigma, 1e-10) {
			t.Errorf("Sigma mismatch")
		}
	}
}

func TestStudentsTMarginalSingle(t *testing.T) {
	for _, test := range []struct {
		mu    []float64
		sigma *mat.SymDense
		nu    float64
	}{
		{
			mu:    []float64{2, 3, 4},
			sigma: mat.NewSymDense(3, []float64{2, 0.5, 3, 0.5, 1, 0.6, 3, 0.6, 10}),
			nu:    5,
		},
		{
			mu:    []float64{2, 3, 4, 5},
			sigma: mat.NewSymDense(4, []float64{2, 0.5, 3, 0.1, 0.5, 1, 0.6, 0.2, 3, 0.6, 10, 0.3, 0.1, 0.2, 0.3, 3}),
			nu:    6,
		},
	} {
		studentst, ok := NewStudentsT(test.mu, test.sigma, test.nu, nil)
		if !ok {
			t.Fatalf("Bad test, covariance matrix not positive definite")
		}
		for i, mean := range test.mu {
			st := studentst.MarginalStudentsTSingle(i, nil)
			if st.Mean() != mean {
				t.Errorf("Mean mismatch nil Sigma, idx %v: want %v, got %v.", i, mean, st.Mean())
			}
			std := math.Sqrt(test.sigma.At(i, i))
			if math.Abs(st.Sigma-std) > 1e-14 {
				t.Errorf("StdDev mismatch nil Sigma, idx %v: want %v, got %v.", i, std, st.StdDev())
			}
			if st.Nu != test.nu {
				t.Errorf("Nu mismatch nil Sigma, idx %v: want %v, got %v ", i, test.nu, st.Nu)
			}
		}
	}
}
