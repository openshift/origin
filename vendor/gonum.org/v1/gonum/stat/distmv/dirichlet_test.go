// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distmv

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/mat"
)

func TestDirichlet(t *testing.T) {
	// Data from Scipy.
	for cas, test := range []struct {
		Dir  *Dirichlet
		x    []float64
		prob float64
	}{
		{
			NewDirichlet([]float64{1, 1, 1}, nil),
			[]float64{0.2, 0.3, 0.5},
			2.0,
		},
		{
			NewDirichlet([]float64{0.6, 10, 8.7}, nil),
			[]float64{0.2, 0.3, 0.5},
			0.24079612737071665,
		},
	} {
		p := test.Dir.Prob(test.x)
		if math.Abs(p-test.prob) > 1e-14 {
			t.Errorf("Probablility mismatch. Case %v. Got %v, want %v", cas, p, test.prob)
		}
	}

	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		Dir *Dirichlet
	}{
		{
			NewDirichlet([]float64{1, 1, 1}, rnd),
		},
		{
			NewDirichlet([]float64{2, 3}, rnd),
		},
		{
			NewDirichlet([]float64{0.2, 0.3}, rnd),
		},
		{
			NewDirichlet([]float64{0.2, 4}, rnd),
		},
		{
			NewDirichlet([]float64{0.1, 4, 20}, rnd),
		},
	} {
		const n = 1e5
		d := test.Dir
		dim := d.Dim()
		x := mat.NewDense(n, dim, nil)
		generateSamples(x, d)
		checkMean(t, cas, x, d, 1e-2)
		checkCov(t, cas, x, d, 1e-2)
	}
}
