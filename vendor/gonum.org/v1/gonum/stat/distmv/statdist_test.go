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
)

func TestBhattacharyyaNormal(t *testing.T) {
	for cas, test := range []struct {
		am, bm  []float64
		ac, bc  *mat.SymDense
		samples int
		tol     float64
	}{
		{
			am:      []float64{2, 3},
			ac:      mat.NewSymDense(2, []float64{3, -1, -1, 2}),
			bm:      []float64{-1, 1},
			bc:      mat.NewSymDense(2, []float64{1.5, 0.2, 0.2, 0.9}),
			samples: 100000,
			tol:     3e-1,
		},
	} {
		rnd := rand.New(rand.NewSource(1))
		a, ok := NewNormal(test.am, test.ac, rnd)
		if !ok {
			panic("bad test")
		}
		b, ok := NewNormal(test.bm, test.bc, rnd)
		if !ok {
			panic("bad test")
		}
		want := bhattacharyyaSample(a.Dim(), test.samples, a, b)
		got := Bhattacharyya{}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Bhattacharyya mismatch, case %d: got %v, want %v", cas, got, want)
		}

		// Bhattacharyya should by symmetric
		got2 := Bhattacharyya{}.DistNormal(b, a)
		if math.Abs(got-got2) > 1e-14 {
			t.Errorf("Bhattacharyya distance not symmetric")
		}
	}
}

func TestBhattacharyyaUniform(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		a, b    *Uniform
		samples int
		tol     float64
	}{
		{
			a:       NewUniform([]Bound{{-3, 2}, {-5, 8}}, rnd),
			b:       NewUniform([]Bound{{-4, 1}, {-7, 10}}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
		{
			a:       NewUniform([]Bound{{-3, 2}, {-5, 8}}, rnd),
			b:       NewUniform([]Bound{{-5, -4}, {-7, 10}}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
	} {
		a, b := test.a, test.b
		want := bhattacharyyaSample(a.Dim(), test.samples, a, b)
		got := Bhattacharyya{}.DistUniform(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Bhattacharyya mismatch, case %d: got %v, want %v", cas, got, want)
		}
		// Bhattacharyya should by symmetric
		got2 := Bhattacharyya{}.DistUniform(b, a)
		if math.Abs(got-got2) > 1e-14 {
			t.Errorf("Bhattacharyya distance not symmetric")
		}
	}
}

// bhattacharyyaSample finds an estimate of the Bhattacharyya coefficient through
// sampling.
func bhattacharyyaSample(dim, samples int, l RandLogProber, r LogProber) float64 {
	lBhatt := make([]float64, samples)
	x := make([]float64, dim)
	for i := 0; i < samples; i++ {
		// Do importance sampling over a: \int sqrt(a*b)/a * a dx
		l.Rand(x)
		pa := l.LogProb(x)
		pb := r.LogProb(x)
		lBhatt[i] = 0.5*pb - 0.5*pa
	}
	logBc := floats.LogSumExp(lBhatt) - math.Log(float64(samples))
	return -logBc
}

func TestCrossEntropyNormal(t *testing.T) {
	for cas, test := range []struct {
		am, bm  []float64
		ac, bc  *mat.SymDense
		samples int
		tol     float64
	}{
		{
			am:      []float64{2, 3},
			ac:      mat.NewSymDense(2, []float64{3, -1, -1, 2}),
			bm:      []float64{-1, 1},
			bc:      mat.NewSymDense(2, []float64{1.5, 0.2, 0.2, 0.9}),
			samples: 100000,
			tol:     1e-2,
		},
	} {
		rnd := rand.New(rand.NewSource(1))
		a, ok := NewNormal(test.am, test.ac, rnd)
		if !ok {
			panic("bad test")
		}
		b, ok := NewNormal(test.bm, test.bc, rnd)
		if !ok {
			panic("bad test")
		}
		var ce float64
		x := make([]float64, a.Dim())
		for i := 0; i < test.samples; i++ {
			a.Rand(x)
			ce -= b.LogProb(x)
		}
		ce /= float64(test.samples)
		got := CrossEntropy{}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(ce, got, test.tol, test.tol) {
			t.Errorf("CrossEntropy mismatch, case %d: got %v, want %v", cas, got, ce)
		}
	}
}

func TestHellingerNormal(t *testing.T) {
	for cas, test := range []struct {
		am, bm  []float64
		ac, bc  *mat.SymDense
		samples int
		tol     float64
	}{
		{
			am:      []float64{2, 3},
			ac:      mat.NewSymDense(2, []float64{3, -1, -1, 2}),
			bm:      []float64{-1, 1},
			bc:      mat.NewSymDense(2, []float64{1.5, 0.2, 0.2, 0.9}),
			samples: 100000,
			tol:     5e-1,
		},
	} {
		rnd := rand.New(rand.NewSource(1))
		a, ok := NewNormal(test.am, test.ac, rnd)
		if !ok {
			panic("bad test")
		}
		b, ok := NewNormal(test.bm, test.bc, rnd)
		if !ok {
			panic("bad test")
		}
		lAitchEDoubleHockeySticks := make([]float64, test.samples)
		x := make([]float64, a.Dim())
		for i := 0; i < test.samples; i++ {
			// Do importance sampling over a: \int (\sqrt(a)-\sqrt(b))^2/a * a dx
			a.Rand(x)
			pa := a.LogProb(x)
			pb := b.LogProb(x)
			d := math.Exp(0.5*pa) - math.Exp(0.5*pb)
			d = d * d
			lAitchEDoubleHockeySticks[i] = math.Log(d) - pa
		}
		want := math.Sqrt(0.5 * math.Exp(floats.LogSumExp(lAitchEDoubleHockeySticks)-math.Log(float64(test.samples))))
		got := Hellinger{}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Hellinger mismatch, case %d: got %v, want %v", cas, got, want)
		}
	}
}

func TestKullbackLeiblerDirichlet(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		a, b    *Dirichlet
		samples int
		tol     float64
	}{
		{
			a:       NewDirichlet([]float64{2, 3, 4}, rnd),
			b:       NewDirichlet([]float64{4, 2, 1.1}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
		{
			a:       NewDirichlet([]float64{2, 3, 4, 0.1, 8}, rnd),
			b:       NewDirichlet([]float64{2, 2, 6, 0.5, 9}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
	} {
		a, b := test.a, test.b
		want := klSample(a.Dim(), test.samples, a, b)
		got := KullbackLeibler{}.DistDirichlet(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Kullback-Leibler mismatch, case %d: got %v, want %v", cas, got, want)
		}
	}
}

func TestKullbackLeiblerNormal(t *testing.T) {
	for cas, test := range []struct {
		am, bm  []float64
		ac, bc  *mat.SymDense
		samples int
		tol     float64
	}{
		{
			am:      []float64{2, 3},
			ac:      mat.NewSymDense(2, []float64{3, -1, -1, 2}),
			bm:      []float64{-1, 1},
			bc:      mat.NewSymDense(2, []float64{1.5, 0.2, 0.2, 0.9}),
			samples: 10000,
			tol:     1e-2,
		},
	} {
		rnd := rand.New(rand.NewSource(1))
		a, ok := NewNormal(test.am, test.ac, rnd)
		if !ok {
			panic("bad test")
		}
		b, ok := NewNormal(test.bm, test.bc, rnd)
		if !ok {
			panic("bad test")
		}
		want := klSample(a.Dim(), test.samples, a, b)
		got := KullbackLeibler{}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Case %d, KL mismatch: got %v, want %v", cas, got, want)
		}
	}
}

func TestKullbackLeiblerUniform(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		a, b    *Uniform
		samples int
		tol     float64
	}{
		{
			a:       NewUniform([]Bound{{-5, 2}, {-7, 12}}, rnd),
			b:       NewUniform([]Bound{{-4, 1}, {-7, 10}}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
		{
			a:       NewUniform([]Bound{{-5, 2}, {-7, 12}}, rnd),
			b:       NewUniform([]Bound{{-9, -6}, {-7, 10}}, rnd),
			samples: 100000,
			tol:     1e-2,
		},
	} {
		a, b := test.a, test.b
		want := klSample(a.Dim(), test.samples, a, b)
		got := KullbackLeibler{}.DistUniform(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Kullback-Leibler mismatch, case %d: got %v, want %v", cas, got, want)
		}
	}
}

// klSample finds an estimate of the Kullback-Leibler divergence through sampling.
func klSample(dim, samples int, l RandLogProber, r LogProber) float64 {
	var klmc float64
	x := make([]float64, dim)
	for i := 0; i < samples; i++ {
		l.Rand(x)
		pa := l.LogProb(x)
		pb := r.LogProb(x)
		klmc += pa - pb
	}
	return klmc / float64(samples)
}

func TestRenyiNormal(t *testing.T) {
	for cas, test := range []struct {
		am, bm  []float64
		ac, bc  *mat.SymDense
		alpha   float64
		samples int
		tol     float64
	}{
		{
			am:      []float64{2, 3},
			ac:      mat.NewSymDense(2, []float64{3, -1, -1, 2}),
			bm:      []float64{-1, 1},
			bc:      mat.NewSymDense(2, []float64{1.5, 0.2, 0.2, 0.9}),
			alpha:   0.3,
			samples: 10000,
			tol:     3e-1,
		},
	} {
		rnd := rand.New(rand.NewSource(1))
		a, ok := NewNormal(test.am, test.ac, rnd)
		if !ok {
			panic("bad test")
		}
		b, ok := NewNormal(test.bm, test.bc, rnd)
		if !ok {
			panic("bad test")
		}
		want := renyiSample(a.Dim(), test.samples, test.alpha, a, b)
		got := Renyi{Alpha: test.alpha}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, test.tol, test.tol) {
			t.Errorf("Case %d: Renyi sampling mismatch: got %v, want %v", cas, got, want)
		}

		// Compare with Bhattacharyya.
		want = 2 * Bhattacharyya{}.DistNormal(a, b)
		got = Renyi{Alpha: 0.5}.DistNormal(a, b)
		if !floats.EqualWithinAbsOrRel(want, got, 1e-10, 1e-10) {
			t.Errorf("Case %d: Renyi mismatch with Bhattacharyya: got %v, want %v", cas, got, want)
		}

		// Compare with KL in both directions.
		want = KullbackLeibler{}.DistNormal(a, b)
		got = Renyi{Alpha: 0.9999999}.DistNormal(a, b) // very close to 1 but not equal to 1.
		if !floats.EqualWithinAbsOrRel(want, got, 1e-6, 1e-6) {
			t.Errorf("Case %d: Renyi mismatch with KL(a||b): got %v, want %v", cas, got, want)
		}
		want = KullbackLeibler{}.DistNormal(b, a)
		got = Renyi{Alpha: 0.9999999}.DistNormal(b, a) // very close to 1 but not equal to 1.
		if !floats.EqualWithinAbsOrRel(want, got, 1e-6, 1e-6) {
			t.Errorf("Case %d: Renyi mismatch with KL(b||a): got %v, want %v", cas, got, want)
		}
	}
}

// renyiSample finds an estimate of the Rényi divergence through sampling.
// Note that this sampling procedure only works if l has broader support than r.
func renyiSample(dim, samples int, alpha float64, l RandLogProber, r LogProber) float64 {
	rmcs := make([]float64, samples)
	x := make([]float64, dim)
	for i := 0; i < samples; i++ {
		l.Rand(x)
		pa := l.LogProb(x)
		pb := r.LogProb(x)
		rmcs[i] = (alpha-1)*pa + (1-alpha)*pb
	}
	return 1 / (alpha - 1) * (floats.LogSumExp(rmcs) - math.Log(float64(samples)))
}
