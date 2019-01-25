// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"math"
	"testing"
)

type Dgerer interface {
	Dger(m, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64, lda int)
}

func DgerTest(t *testing.T, blasser Dgerer) {
	for _, test := range []struct {
		name string
		a    [][]float64
		m    int
		n    int
		x    []float64
		y    []float64
		incX int
		incY int

		want [][]float64
	}{
		{
			name: "M gt N inc 1",
			m:    5,
			n:    3,
			a: [][]float64{
				{1.3, 2.4, 3.5},
				{2.6, 2.8, 3.3},
				{-1.3, -4.3, -9.7},
				{8, 9, -10},
				{-12, -14, -6},
			},
			x:    []float64{-2, -3, 0, 1, 2},
			y:    []float64{-1.1, 5, 0},
			incX: 1,
			incY: 1,
			want: [][]float64{{3.5, -7.6, 3.5}, {5.9, -12.2, 3.3}, {-1.3, -4.3, -9.7}, {6.9, 14, -10}, {-14.2, -4, -6}},
		},
		{
			name: "M eq N inc 1",
			m:    3,
			n:    3,
			a: [][]float64{
				{1.3, 2.4, 3.5},
				{2.6, 2.8, 3.3},
				{-1.3, -4.3, -9.7},
			},
			x:    []float64{-2, -3, 0},
			y:    []float64{-1.1, 5, 0},
			incX: 1,
			incY: 1,
			want: [][]float64{{3.5, -7.6, 3.5}, {5.9, -12.2, 3.3}, {-1.3, -4.3, -9.7}},
		},

		{
			name: "M lt N inc 1",
			m:    3,
			n:    6,
			a: [][]float64{
				{1.3, 2.4, 3.5, 4.8, 1.11, -9},
				{2.6, 2.8, 3.3, -3.4, 6.2, -8.7},
				{-1.3, -4.3, -9.7, -3.1, 8.9, 8.9},
			},
			x:    []float64{-2, -3, 0},
			y:    []float64{-1.1, 5, 0, 9, 19, 22},
			incX: 1,
			incY: 1,
			want: [][]float64{{3.5, -7.6, 3.5, -13.2, -36.89, -53}, {5.9, -12.2, 3.3, -30.4, -50.8, -74.7}, {-1.3, -4.3, -9.7, -3.1, 8.9, 8.9}},
		},
		{
			name: "M gt N inc not 1",
			m:    5,
			n:    3,
			a: [][]float64{
				{1.3, 2.4, 3.5},
				{2.6, 2.8, 3.3},
				{-1.3, -4.3, -9.7},
				{8, 9, -10},
				{-12, -14, -6},
			},
			x:    []float64{-2, -3, 0, 1, 2, 6, 0, 9, 7},
			y:    []float64{-1.1, 5, 0, 8, 7, -5, 7},
			incX: 2,
			incY: 3,
			want: [][]float64{{3.5, -13.6, -10.5}, {2.6, 2.8, 3.3}, {-3.5, 11.7, 4.3}, {8, 9, -10}, {-19.700000000000003, 42, 43}},
		},
		{
			name: "M eq N inc not 1",
			m:    3,
			n:    3,
			a: [][]float64{
				{1.3, 2.4, 3.5},
				{2.6, 2.8, 3.3},
				{-1.3, -4.3, -9.7},
			},
			x:    []float64{-2, -3, 0, 8, 7, -9, 7, -6, 12, 6, 6, 6, -11},
			y:    []float64{-1.1, 5, 0, 0, 9, 8, 6},
			incX: 4,
			incY: 3,
			want: [][]float64{{3.5, 2.4, -8.5}, {-5.1, 2.8, 45.3}, {-14.5, -4.3, 62.3}},
		},
		{
			name: "M lt N inc not 1",
			m:    3,
			n:    6,
			a: [][]float64{
				{1.3, 2.4, 3.5, 4.8, 1.11, -9},
				{2.6, 2.8, 3.3, -3.4, 6.2, -8.7},
				{-1.3, -4.3, -9.7, -3.1, 8.9, 8.9},
			},
			x:    []float64{-2, -3, 0, 0, 8, 0, 9, -3},
			y:    []float64{-1.1, 5, 0, 9, 19, 22, 11, -8.11, -9.22, 9.87, 7},
			incX: 3,
			incY: 2,
			want: [][]float64{{3.5, 2.4, -34.5, -17.2, 19.55, -23}, {2.6, 2.8, 3.3, -3.4, 6.2, -8.7}, {-11.2, -4.3, 161.3, 95.9, -74.08, 71.9}},
		},
		{
			name: "Y NaN element",
			m:    1,
			n:    1,
			a:    [][]float64{{1.3}},
			x:    []float64{1.3},
			y:    []float64{math.NaN()},
			incX: 1,
			incY: 1,
			want: [][]float64{{math.NaN()}},
		},
		{
			name: "M eq N large inc 1",
			m:    7,
			n:    7,
			x:    []float64{6.2, -5, 88.68, 43.4, -30.5, -40.2, 19.9},
			y:    []float64{1.5, 21.7, -28.7, -11.9, 18.1, 3.1, 21},
			a: [][]float64{
				{-20.5, 17.1, -8.4, -23.8, 3.9, 7.7, 6.25},
				{2.9, -0.29, 25.6, -9.4, 36.5, 9.7, 2.3},
				{4.1, -34.1, 10.3, 4.5, -42.05, 9.4, 4},
				{19.2, 9.8, -32.7, 4.1, 4.4, -22.5, -7.8},
				{3.6, -24.5, 21.7, 8.6, -13.82, 38.05, -2.29},
				{39.4, -40.5, 7.9, -2.5, -7.7, 18.1, -25.5},
				{-18.5, 43.2, 2.1, 30.1, 3.02, -31.1, -7.6},
			},
			incX: 1,
			incY: 1,
			want: [][]float64{
				{-11.2, 151.64, -186.34, -97.58, 116.12, 26.92, 136.45},
				{-4.6, -108.79, 169.1, 50.1, -54, -5.8, -102.7},
				{137.12, 1890.256, -2534.816, -1050.792, 1563.058, 284.308, 1866.28},
				{84.3, 951.58, -1278.28, -512.36, 789.94, 112.04, 903.6},
				{-42.15, -686.35, 897.05, 371.55, -565.87, -56.5, -642.79},
				{-20.9, -912.84, 1161.64, 475.88, -735.32, -106.52, -869.7},
				{11.35, 475.03, -569.03, -206.71, 363.21, 30.59, 410.3},
			},
		},
		{
			name: "M eq N large inc not 1",
			m:    7,
			n:    7,
			x:    []float64{6.2, 100, 200, -5, 300, 400, 88.68, 100, 200, 43.4, 300, 400, -30.5, 100, 200, -40.2, 300, 400, 19.9},
			y:    []float64{1.5, 100, 200, 300, 21.7, 100, 200, 300, -28.7, 100, 200, 300, -11.9, 100, 200, 300, 18.1, 100, 200, 300, 3.1, 100, 200, 300, 21},
			a: [][]float64{
				{-20.5, 17.1, -8.4, -23.8, 3.9, 7.7, 6.25},
				{2.9, -0.29, 25.6, -9.4, 36.5, 9.7, 2.3},
				{4.1, -34.1, 10.3, 4.5, -42.05, 9.4, 4},
				{19.2, 9.8, -32.7, 4.1, 4.4, -22.5, -7.8},
				{3.6, -24.5, 21.7, 8.6, -13.82, 38.05, -2.29},
				{39.4, -40.5, 7.9, -2.5, -7.7, 18.1, -25.5},
				{-18.5, 43.2, 2.1, 30.1, 3.02, -31.1, -7.6},
			},
			incX: 3,
			incY: 4,
			want: [][]float64{
				{-11.2, 151.64, -186.34, -97.58, 116.12, 26.92, 136.45},
				{-4.6, -108.79, 169.1, 50.1, -54, -5.8, -102.7},
				{137.12, 1890.256, -2534.816, -1050.792, 1563.058, 284.308, 1866.28},
				{84.3, 951.58, -1278.28, -512.36, 789.94, 112.04, 903.6},
				{-42.15, -686.35, 897.05, 371.55, -565.87, -56.5, -642.79},
				{-20.9, -912.84, 1161.64, 475.88, -735.32, -106.52, -869.7},
				{11.35, 475.03, -569.03, -206.71, 363.21, 30.59, 410.3},
			},
		},
	} {
		// TODO: Add tests where a is longer
		// TODO: Add panic tests
		// TODO: Add negative increment tests

		x := sliceCopy(test.x)
		y := sliceCopy(test.y)

		a := sliceOfSliceCopy(test.a)

		// Test with row major
		alpha := 1.0
		aFlat := flatten(a)
		blasser.Dger(test.m, test.n, alpha, x, test.incX, y, test.incY, aFlat, test.n)
		ans := unflatten(aFlat, test.m, test.n)
		dgercomp(t, x, test.x, y, test.y, ans, test.want, test.name+" row maj")

		// Test with different alpha
		alpha = 4.0
		aFlat = flatten(a)
		blasser.Dger(test.m, test.n, alpha, x, test.incX, y, test.incY, aFlat, test.n)
		ans = unflatten(aFlat, test.m, test.n)
		trueCopy := sliceOfSliceCopy(test.want)
		for i := range trueCopy {
			for j := range trueCopy[i] {
				trueCopy[i][j] = alpha*(trueCopy[i][j]-a[i][j]) + a[i][j]
			}
		}
		dgercomp(t, x, test.x, y, test.y, ans, trueCopy, test.name+" row maj alpha")
	}
}

func dgercomp(t *testing.T, x, xCopy, y, yCopy []float64, ans [][]float64, trueAns [][]float64, name string) {
	if !dSliceEqual(x, xCopy) {
		t.Errorf("case %v: x modified during call to dger\n%v\n%v", name, x, xCopy)
	}
	if !dSliceEqual(y, yCopy) {
		t.Errorf("case %v: y modified during call to dger\n%v\n%v", name, y, yCopy)
	}

	for i := range ans {
		if !dSliceTolEqual(ans[i], trueAns[i]) {
			t.Errorf("case %v: answer mismatch at %v.\nExpected %v,\nFound %v", name, i, trueAns, ans)
			break
		}
	}
}
