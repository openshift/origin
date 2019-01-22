// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distuv

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

func TestPoissonProb(t *testing.T) {
	const tol = 1e-10
	for i, tt := range []struct {
		k      float64
		lambda float64
		want   float64
	}{
		{0, 1, 3.678794411714423e-01},
		{1, 1, 3.678794411714423e-01},
		{2, 1, 1.839397205857211e-01},
		{3, 1, 6.131324019524039e-02},
		{4, 1, 1.532831004881010e-02},
		{5, 1, 3.065662009762020e-03},
		{6, 1, 5.109436682936698e-04},
		{7, 1, 7.299195261338139e-05},
		{8, 1, 9.123994076672672e-06},
		{9, 1, 1.013777119630298e-06},

		{0, 2.5, 8.208499862389880e-02},
		{1, 2.5, 2.052124965597470e-01},
		{2, 2.5, 2.565156206996838e-01},
		{3, 2.5, 2.137630172497365e-01},
		{4, 2.5, 1.336018857810853e-01},
		{5, 2.5, 6.680094289054267e-02},
		{6, 2.5, 2.783372620439277e-02},
		{7, 2.5, 9.940616501568845e-03},
		{8, 2.5, 3.106442656740263e-03},
		{9, 2.5, 8.629007379834082e-04},

		{0.5, 2.5, 0},
		{1.5, 2.5, 0},
		{2.5, 2.5, 0},
		{3.5, 2.5, 0},
		{4.5, 2.5, 0},
		{5.5, 2.5, 0},
		{6.5, 2.5, 0},
		{7.5, 2.5, 0},
		{8.5, 2.5, 0},
		{9.5, 2.5, 0},
	} {
		p := Poisson{Lambda: tt.lambda}
		got := p.Prob(tt.k)
		if !floats.EqualWithinAbs(got, tt.want, tol) {
			t.Errorf("test-%d: got=%e. want=%e\n", i, got, tt.want)
		}
	}
}

func TestPoissonCDF(t *testing.T) {
	const tol = 1e-10
	for i, tt := range []struct {
		k      float64
		lambda float64
		want   float64
	}{
		{0, 1, 0.367879441171442},
		{1, 1, 0.735758882342885},
		{2, 1, 0.919698602928606},
		{3, 1, 0.981011843123846},
		{4, 1, 0.996340153172656},
		{5, 1, 0.999405815182418},
		{6, 1, 0.999916758850712},
		{7, 1, 0.999989750803325},
		{8, 1, 0.999998874797402},
		{9, 1, 0.999999888574522},

		{0, 2.5, 0.082084998623899},
		{1, 2.5, 0.287297495183646},
		{2, 2.5, 0.543813115883329},
		{3, 2.5, 0.757576133133066},
		{4, 2.5, 0.891178018914151},
		{5, 2.5, 0.957978961804694},
		{6, 2.5, 0.985812688009087},
		{7, 2.5, 0.995753304510655},
		{8, 2.5, 0.998859747167396},
		{9, 2.5, 0.999722647905379},
	} {
		p := Poisson{Lambda: tt.lambda}
		got := p.CDF(tt.k)
		if !floats.EqualWithinAbs(got, tt.want, tol) {
			t.Errorf("test-%d: got=%e. want=%e\n", i, got, tt.want)
		}
	}
}

func TestPoisson(t *testing.T) {
	src := rand.New(rand.NewSource(1))
	for i, b := range []Poisson{
		{100, src},
		{15, src},
		{10, src},
		{9.9, src},
		{3, src},
		{1.5, src},
		{0.9, src},
	} {
		testPoisson(t, b, i)
	}
}

func testPoisson(t *testing.T, p Poisson, i int) {
	tol := 1e-2
	const n = 1e6
	x := make([]float64, n)
	generateSamples(x, p)
	sort.Float64s(x)

	checkMean(t, i, x, p, tol)
	checkVarAndStd(t, i, x, p, tol)
	checkExKurtosis(t, i, x, p, 7e-2)
}
