// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spatial

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

func simpleAdjacency(n, wide int, diag bool) mat.Matrix {
	m := mat.NewDense(n, n, nil)
	for i := 0; i < n; i++ {
		for j := 1; j <= wide; j++ {
			if j > i {
				continue
			}
			m.Set(i-j, i, 1)
			m.Set(i, i-j, 1)
		}
		if diag {
			m.Set(i, i, 1)
		}
	}
	return m
}

func simpleAdjacencyBand(n, wide int, diag bool) mat.Matrix {
	m := mat.NewBandDense(n, n, wide, wide, nil)
	for i := 0; i < n; i++ {
		for j := 1; j <= wide; j++ {
			if j > i {
				continue
			}
			m.SetBand(i-j, i, 1)
			m.SetBand(i, i-j, 1)
		}
		if diag {
			m.SetBand(i, i, 1)
		}
	}
	return m
}

var spatialTests = []struct {
	from, to float64
	n, wide  int
	fn       func(float64, int, *rand.Rand) float64
	locality func(n, wide int, diag bool) mat.Matrix

	// Values for MoranI and z-score are obtained from
	// an R reference implementation.
	wantMoranI float64
	wantZ      float64

	// The value for expected number of significant
	// segments is obtained from visual inspection
	// of the plotted data.
	wantSegs int
}{
	// Dense matrix locality.
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, _ int, rnd *rand.Rand) float64 {
			return rnd.Float64()
		},
		locality: simpleAdjacency,

		wantMoranI: -0.04387221370785312,
		wantZ:      -1.3543515772206267,
		wantSegs:   0,
	},
	{
		from: -math.Pi / 2, to: 3 * math.Pi / 2, n: 1000, wide: 1,
		fn: func(x float64, _ int, _ *rand.Rand) float64 {
			y := math.Sin(x)
			if math.Abs(y) > 0.5 {
				y *= 1/math.Abs(y) - 1
			}
			return y * math.Sin(x*2)
		},
		locality: simpleAdjacency,

		wantMoranI: 1.0008149537991464,
		wantZ:      31.648547078779092,
		wantSegs:   4,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, _ int, rnd *rand.Rand) float64 {
			return rnd.NormFloat64()
		},
		locality: simpleAdjacency,

		wantMoranI: 0.0259414094549987,
		wantZ:      0.8511426395944303,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(x float64, _ int, rnd *rand.Rand) float64 {
			if rnd.Float64() < 0.5 {
				return rnd.NormFloat64() + 5
			}
			return rnd.NormFloat64()
		},
		locality: simpleAdjacency,

		wantMoranI: -0.0003533345592575677,
		wantZ:      0.0204605353504713,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(x float64, i int, rnd *rand.Rand) float64 {
			if i%2 == 0 {
				return rnd.NormFloat64() + 5
			}
			return rnd.NormFloat64()
		},
		locality: simpleAdjacency,

		wantMoranI: -0.8587138204405251,
		wantZ:      -27.09614459007475,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, i int, _ *rand.Rand) float64 {
			return float64(i % 2)
		},
		locality: simpleAdjacency,

		wantMoranI: -1,
		wantZ:      -31.559531064275987,
		wantSegs:   0,
	},

	// Band matrix locality.
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, _ int, rnd *rand.Rand) float64 {
			return rnd.Float64()
		},
		locality: simpleAdjacencyBand,

		wantMoranI: -0.04387221370785312,
		wantZ:      -1.3543515772206267,
		wantSegs:   0,
	},
	{
		from: -math.Pi / 2, to: 3 * math.Pi / 2, n: 1000, wide: 1,
		fn: func(x float64, _ int, _ *rand.Rand) float64 {
			y := math.Sin(x)
			if math.Abs(y) > 0.5 {
				y *= 1/math.Abs(y) - 1
			}
			return y * math.Sin(x*2)
		},
		locality: simpleAdjacencyBand,

		wantMoranI: 1.0008149537991464,
		wantZ:      31.648547078779092,
		wantSegs:   4,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, _ int, rnd *rand.Rand) float64 {
			return rnd.NormFloat64()
		},
		locality: simpleAdjacencyBand,

		wantMoranI: 0.0259414094549987,
		wantZ:      0.8511426395944303,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(x float64, _ int, rnd *rand.Rand) float64 {
			if rnd.Float64() < 0.5 {
				return rnd.NormFloat64() + 5
			}
			return rnd.NormFloat64()
		},
		locality: simpleAdjacencyBand,

		wantMoranI: -0.0003533345592575677,
		wantZ:      0.0204605353504713,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(x float64, i int, rnd *rand.Rand) float64 {
			if i%2 == 0 {
				return rnd.NormFloat64() + 5
			}
			return rnd.NormFloat64()
		},
		locality: simpleAdjacencyBand,

		wantMoranI: -0.8587138204405251,
		wantZ:      -27.09614459007475,
		wantSegs:   0,
	},
	{
		from: 0, to: 1, n: 1000, wide: 1,
		fn: func(_ float64, i int, _ *rand.Rand) float64 {
			return float64(i % 2)
		},
		locality: simpleAdjacencyBand,

		wantMoranI: -1,
		wantZ:      -31.559531064275987,
		wantSegs:   0,
	},
}

func TestGetisOrd(t *testing.T) {
	for ti, test := range spatialTests {
		rnd := rand.New(rand.NewSource(1))
		data := make([]float64, test.n)
		step := (test.to - test.from) / float64(test.n)
		for i := range data {
			data[i] = test.fn(test.from+step*float64(i), i, rnd)
		}
		locality := test.locality(test.n, test.wide, true)

		nseg := getisOrdSegments(data, nil, locality)
		if nseg != test.wantSegs {
			t.Errorf("unexpected number of significant segments for test %d: got:%d want:%d",
				ti, nseg, test.wantSegs)
		}
	}
}

// getisOrdSegments returns the number of contiguously significant G*i segemtns in
// data. This allows an intuitive validation of the function in lieu of a reference
// implementation.
func getisOrdSegments(data, weight []float64, locality mat.Matrix) int {
	const thresh = 2
	var nseg int
	segstart := -1
	for i := range data {
		gi := GetisOrdGStar(i, data, weight, locality)
		if segstart != -1 {
			if math.Abs(gi) < thresh {
				// Filter short segments.
				if i-segstart < 5 {
					segstart = -1
					continue
				}

				segstart = -1
				nseg++
			}
			continue
		}
		if math.Abs(gi) >= thresh {
			segstart = i
		}
	}
	if segstart != -1 && len(data)-segstart >= 5 {
		nseg++
	}
	return nseg
}

func TestGlobalMoransI(t *testing.T) {
	const tol = 1e-14
	for ti, test := range spatialTests {
		rnd := rand.New(rand.NewSource(1))
		data := make([]float64, test.n)
		step := (test.to - test.from) / float64(test.n)
		for i := range data {
			data[i] = test.fn(test.from+step*float64(i), i, rnd)
		}
		locality := test.locality(test.n, test.wide, false)

		gotI, _, gotZ := GlobalMoransI(data, nil, locality)

		if !floats.EqualWithinAbsOrRel(gotI, test.wantMoranI, tol, tol) {
			t.Errorf("unexpected Moran's I value for test %d: got:%v want:%v", ti, gotI, test.wantMoranI)
		}
		if !floats.EqualWithinAbsOrRel(gotZ, test.wantZ, tol, tol) {
			t.Errorf("unexpected Moran's I z-score for test %d: got:%v want:%v", ti, gotZ, test.wantZ)
		}
	}
}
