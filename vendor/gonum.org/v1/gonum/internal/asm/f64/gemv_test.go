// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package f64

import (
	"fmt"
	"testing"
)

type DgemvCase struct {
	m int
	n int
	A []float64
	x []float64
	y []float64

	NoTrans []DgemvSubcase
	Trans   []DgemvSubcase
}

type DgemvSubcase struct {
	alpha     float64
	beta      float64
	want      []float64
	wantRevX  []float64
	wantRevY  []float64
	wantRevXY []float64
}

var DgemvCases = []DgemvCase{
	{ // 1x1
		m: 1,
		n: 1,
		A: []float64{4.1},
		x: []float64{2.2},
		y: []float64{6.8},

		NoTrans: []DgemvSubcase{ // (1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0},
				wantRevX:  []float64{0},
				wantRevY:  []float64{0},
				wantRevXY: []float64{0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{6.8},
				wantRevX:  []float64{6.8},
				wantRevY:  []float64{6.8},
				wantRevXY: []float64{6.8},
			},
			{alpha: 1, beta: 0,
				want:      []float64{9.02},
				wantRevX:  []float64{9.02},
				wantRevY:  []float64{9.02},
				wantRevXY: []float64{9.02},
			},
			{alpha: 8, beta: -6,
				want:      []float64{31.36},
				wantRevX:  []float64{31.36},
				wantRevY:  []float64{31.36},
				wantRevXY: []float64{31.36},
			},
		},

		Trans: []DgemvSubcase{ // (1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0},
				wantRevX:  []float64{0},
				wantRevY:  []float64{0},
				wantRevXY: []float64{0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{2.2},
				wantRevX:  []float64{2.2},
				wantRevY:  []float64{2.2},
				wantRevXY: []float64{2.2},
			},
			{alpha: 1, beta: 0,
				want:      []float64{27.88},
				wantRevX:  []float64{27.88},
				wantRevY:  []float64{27.88},
				wantRevXY: []float64{27.88},
			},
			{alpha: 8, beta: -6,
				want:      []float64{209.84},
				wantRevX:  []float64{209.84},
				wantRevY:  []float64{209.84},
				wantRevXY: []float64{209.84},
			},
		},
	},

	{ // 3x2
		m: 3,
		n: 2,
		A: []float64{
			4.67, 2.75,
			0.48, 1.21,
			2.28, 2.82,
		},
		x: []float64{3.38, 3},
		y: []float64{2.8, 1.71, 2.64},

		NoTrans: []DgemvSubcase{ // (2x2, 1x2)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0},
				wantRevX:  []float64{0, 0, 0},
				wantRevY:  []float64{0, 0, 0},
				wantRevXY: []float64{0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{2.8, 1.71, 2.64},
				wantRevX:  []float64{2.8, 1.71, 2.64},
				wantRevY:  []float64{2.8, 1.71, 2.64},
				wantRevXY: []float64{2.8, 1.71, 2.64},
			},
			{alpha: 1, beta: 0,
				want:      []float64{24.0346, 5.2524, 16.1664},
				wantRevX:  []float64{23.305, 5.5298, 16.3716},
				wantRevY:  []float64{16.1664, 5.2524, 24.0346},
				wantRevXY: []float64{16.3716, 5.5298, 23.305},
			},
			{alpha: 8, beta: -6,
				want:      []float64{175.4768, 31.7592, 113.4912},
				wantRevX:  []float64{169.64, 33.9784, 115.1328},
				wantRevY:  []float64{112.5312, 31.7592, 176.4368},
				wantRevXY: []float64{114.1728, 33.9784, 170.6},
			},
		},

		Trans: []DgemvSubcase{ // (2x2)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0},
				wantRevX:  []float64{0, 0},
				wantRevY:  []float64{0, 0},
				wantRevXY: []float64{0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{3.38, 3},
				wantRevX:  []float64{3.38, 3},
				wantRevY:  []float64{3.38, 3},
				wantRevXY: []float64{3.38, 3},
			},
			{alpha: 1, beta: 0,
				want:      []float64{19.916, 17.2139},
				wantRevX:  []float64{19.5336, 17.2251},
				wantRevY:  []float64{17.2139, 19.916},
				wantRevXY: []float64{17.2251, 19.5336},
			},
			{alpha: 8, beta: -6,
				want:      []float64{139.048, 119.7112},
				wantRevX:  []float64{135.9888, 119.8008},
				wantRevY:  []float64{117.4312, 141.328},
				wantRevXY: []float64{117.5208, 138.2688},
			},
		},
	},

	{ // 3x3
		m: 3,
		n: 3,
		A: []float64{
			4.38, 4.4, 4.26,
			4.18, 0.56, 2.57,
			2.59, 2.07, 0.46,
		},
		x: []float64{4.82, 1.82, 1.12},
		y: []float64{0.24, 1.41, 3.45},

		NoTrans: []DgemvSubcase{ // (2x2, 2x1, 1x2, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0},
				wantRevX:  []float64{0, 0, 0},
				wantRevY:  []float64{0, 0, 0},
				wantRevXY: []float64{0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{0.24, 1.41, 3.45},
				wantRevX:  []float64{0.24, 1.41, 3.45},
				wantRevY:  []float64{0.24, 1.41, 3.45},
				wantRevXY: []float64{0.24, 1.41, 3.45},
			},
			{alpha: 1, beta: 0,
				want:      []float64{33.8908, 24.0452, 16.7664},
				wantRevX:  []float64{33.4468, 18.0882, 8.8854},
				wantRevY:  []float64{16.7664, 24.0452, 33.8908},
				wantRevXY: []float64{8.8854, 18.0882, 33.4468},
			},
			{alpha: 8, beta: -6,
				want:      []float64{269.6864, 183.9016, 113.4312},
				wantRevX:  []float64{266.1344, 136.2456, 50.3832},
				wantRevY:  []float64{132.6912, 183.9016, 250.4264},
				wantRevXY: []float64{69.6432, 136.2456, 246.8744},
			},
		},

		Trans: []DgemvSubcase{ // (2x2, 1x2, 2x1, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0},
				wantRevX:  []float64{0, 0, 0},
				wantRevY:  []float64{0, 0, 0},
				wantRevXY: []float64{0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{4.82, 1.82, 1.12},
				wantRevX:  []float64{4.82, 1.82, 1.12},
				wantRevY:  []float64{4.82, 1.82, 1.12},
				wantRevXY: []float64{4.82, 1.82, 1.12},
			},
			{alpha: 1, beta: 0,
				want:      []float64{15.8805, 8.9871, 6.2331},
				wantRevX:  []float64{21.6264, 16.4664, 18.4311},
				wantRevY:  []float64{6.2331, 8.9871, 15.8805},
				wantRevXY: []float64{18.4311, 16.4664, 21.6264},
			},
			{alpha: 8, beta: -6,
				want:      []float64{98.124, 60.9768, 43.1448},
				wantRevX:  []float64{144.0912, 120.8112, 140.7288},
				wantRevY:  []float64{20.9448, 60.9768, 120.324},
				wantRevXY: []float64{118.5288, 120.8112, 166.2912},
			},
		},
	},

	{ // 5x3
		m: 5,
		n: 3,
		A: []float64{
			4.1, 6.2, 8.1,
			9.6, 3.5, 9.1,
			10, 7, 3,
			1, 1, 2,
			9, 2, 5,
		},
		x: []float64{1, 2, 3},
		y: []float64{7, 8, 9, 10, 11},

		NoTrans: []DgemvSubcase{ //(4x2, 4x1, 1x2, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, 0, 0},
				wantRevX:  []float64{0, 0, 0, 0, 0},
				wantRevY:  []float64{0, 0, 0, 0, 0},
				wantRevXY: []float64{0, 0, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{7, 8, 9, 10, 11},
				wantRevX:  []float64{7, 8, 9, 10, 11},
				wantRevY:  []float64{7, 8, 9, 10, 11},
				wantRevXY: []float64{7, 8, 9, 10, 11},
			},
			{alpha: 1, beta: 0,
				want:      []float64{40.8, 43.9, 33, 9, 28},
				wantRevX:  []float64{32.8, 44.9, 47, 7, 36},
				wantRevY:  []float64{28, 9, 33, 43.9, 40.8},
				wantRevXY: []float64{36, 7, 47, 44.9, 32.8},
			},
			{alpha: 8, beta: -6,
				want:      []float64{284.4, 303.2, 210, 12, 158},
				wantRevX:  []float64{220.4, 311.2, 322, -4, 222},
				wantRevY:  []float64{182, 24, 210, 291.2, 260.4},
				wantRevXY: []float64{246, 8, 322, 299.2, 196.4},
			},
		},

		Trans: []DgemvSubcase{ //( 2x4, 1x4, 2x1, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0},
				wantRevX:  []float64{0, 0, 0},
				wantRevY:  []float64{0, 0, 0},
				wantRevXY: []float64{0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{1, 2, 3},
				wantRevX:  []float64{1, 2, 3},
				wantRevY:  []float64{1, 2, 3},
				wantRevXY: []float64{1, 2, 3},
			},
			{alpha: 1, beta: 0,
				want:      []float64{304.5, 166.4, 231.5},
				wantRevX:  []float64{302.1, 188.2, 258.1},
				wantRevY:  []float64{231.5, 166.4, 304.5},
				wantRevXY: []float64{258.1, 188.2, 302.1},
			},
			{alpha: 8, beta: -6,
				want:      []float64{2430, 1319.2, 1834},
				wantRevX:  []float64{2410.8, 1493.6, 2046.8},
				wantRevY:  []float64{1846, 1319.2, 2418},
				wantRevXY: []float64{2058.8, 1493.6, 2398.8},
			},
		},
	},

	{ // 3x5
		m: 3,
		n: 5,
		A: []float64{
			1.4, 2.34, 3.96, 0.96, 2.3,
			3.43, 0.62, 1.09, 0.2, 3.56,
			1.15, 0.58, 3.8, 1.16, 0.01,
		},
		x: []float64{2.34, 2.82, 4.73, 0.22, 3.91},
		y: []float64{2.46, 2.22, 4.75},

		NoTrans: []DgemvSubcase{ // (2x4, 2x1, 1x4, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0},
				wantRevX:  []float64{0, 0, 0},
				wantRevY:  []float64{0, 0, 0},
				wantRevXY: []float64{0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{2.46, 2.22, 4.75},
				wantRevX:  []float64{2.46, 2.22, 4.75},
				wantRevY:  []float64{2.46, 2.22, 4.75},
				wantRevXY: []float64{2.46, 2.22, 4.75},
			},
			{alpha: 1, beta: 0,
				want:      []float64{37.8098, 28.8939, 22.5949},
				wantRevX:  []float64{32.8088, 27.5978, 25.8927},
				wantRevY:  []float64{22.5949, 28.8939, 37.8098},
				wantRevXY: []float64{25.8927, 27.5978, 32.8088},
			},
			{alpha: 8, beta: -6,
				want:      []float64{287.7184, 217.8312, 152.2592},
				wantRevX:  []float64{247.7104, 207.4624, 178.6416},
				wantRevY:  []float64{165.9992, 217.8312, 273.9784},
				wantRevXY: []float64{192.3816, 207.4624, 233.9704},
			},
		},

		Trans: []DgemvSubcase{ // (4x2, 1x2, 4x1, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, 0, 0},
				wantRevX:  []float64{0, 0, 0, 0, 0},
				wantRevY:  []float64{0, 0, 0, 0, 0},
				wantRevXY: []float64{0, 0, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{2.34, 2.82, 4.73, 0.22, 3.91},
				wantRevX:  []float64{2.34, 2.82, 4.73, 0.22, 3.91},
				wantRevY:  []float64{2.34, 2.82, 4.73, 0.22, 3.91},
				wantRevXY: []float64{2.34, 2.82, 4.73, 0.22, 3.91},
			},
			{alpha: 1, beta: 0,
				want:      []float64{16.5211, 9.8878, 30.2114, 8.3156, 13.6087},
				wantRevX:  []float64{17.0936, 13.9182, 30.5778, 7.8576, 18.8528},
				wantRevY:  []float64{13.6087, 8.3156, 30.2114, 9.8878, 16.5211},
				wantRevXY: []float64{18.8528, 7.8576, 30.5778, 13.9182, 17.0936},
			},
			{alpha: 8, beta: -6,
				want:      []float64{118.1288, 62.1824, 213.3112, 65.2048, 85.4096},
				wantRevX:  []float64{122.7088, 94.4256, 216.2424, 61.5408, 127.3624},
				wantRevY:  []float64{94.8296, 49.6048, 213.3112, 77.7824, 108.7088},
				wantRevXY: []float64{136.7824, 45.9408, 216.2424, 110.0256, 113.2888},
			},
		},
	},

	{ // 7x7 & nan test
		m: 7,
		n: 7,
		A: []float64{
			0.9, 2.6, 0.5, 1.8, 2.3, 0.6, 0.2,
			1.6, 0.6, 1.3, 2.1, 1.4, 0.4, 0.8,
			2.9, 0.9, 2.3, 2.5, 1.4, 1.8, 1.6,
			2.6, 2.8, 2.1, 0.3, nan, 2.2, 1.3,
			0.2, 2.2, 1.8, 1.8, 2.1, 1.3, 1.4,
			1.7, 1.4, 2.3, 2., 1., 0., 1.4,
			2.1, 1.9, 0.8, 2.9, 1.3, 0.3, 1.3,
		},
		x: []float64{0.4, 2.8, 3.5, 0.3, 0.6, 2.5, 3.1},
		y: []float64{3.2, 4.4, 5., 4.3, 4.1, 1.4, 0.2},

		NoTrans: []DgemvSubcase{ // (4x4, 4x2, 4x1, 2x4, 2x2, 2x1, 1x4, 1x2, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, nan, 0, 0, 0},
				wantRevX:  []float64{0, 0, 0, nan, 0, 0, 0},
				wantRevY:  []float64{0, 0, 0, nan, 0, 0, 0},
				wantRevXY: []float64{0, 0, 0, nan, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{3.2, 4.4, 5., nan, 4.1, 1.4, 0.2},
				wantRevX:  []float64{3.2, 4.4, 5., nan, 4.1, 1.4, 0.2},
				wantRevY:  []float64{3.2, 4.4, 5., nan, 4.1, 1.4, 0.2},
				wantRevXY: []float64{3.2, 4.4, 5., nan, 4.1, 1.4, 0.2},
			},
			{alpha: 1, beta: 0,
				want:      []float64{13.43, 11.82, 22.78, nan, 21.93, 18.19, 15.39},
				wantRevX:  []float64{19.94, 14.21, 23.95, nan, 19.29, 14.81, 18.52},
				wantRevY:  []float64{15.39, 18.19, 21.93, nan, 22.78, 11.82, 13.43},
				wantRevXY: []float64{18.52, 14.81, 19.29, nan, 23.95, 14.21, 19.94},
			},
			{alpha: 8, beta: -6,
				want:      []float64{88.24, 68.16, 152.24, nan, 150.84, 137.12, 121.92},
				wantRevX:  []float64{140.32, 87.28, 161.6, nan, 129.72, 110.08, 146.96},
				wantRevY:  []float64{103.92, 119.12, 145.44, nan, 157.64, 86.16, 106.24},
				wantRevXY: []float64{128.96, 92.08, 124.32, nan, 167., 105.28, 158.32},
			},
		},

		Trans: []DgemvSubcase{ // (4x4, 2x4, 1x4, 4x2, 2x2, 1x2, 4x1, 2x1, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, 0, nan, 0, 0},
				wantRevX:  []float64{0, 0, 0, 0, nan, 0, 0},
				wantRevY:  []float64{0, 0, nan, 0, 0, 0, 0},
				wantRevXY: []float64{0, 0, nan, 0, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{0.4, 2.8, 3.5, 0.3, nan, 2.5, 3.1},
				wantRevX:  []float64{0.4, 2.8, 3.5, 0.3, nan, 2.5, 3.1},
				wantRevY:  []float64{0.4, 2.8, nan, 0.3, 0.6, 2.5, 3.1},
				wantRevXY: []float64{0.4, 2.8, nan, 0.3, 0.6, 2.5, 3.1},
			},
			{alpha: 1, beta: 0,
				want:      []float64{39.22, 38.86, 38.61, 39.55, nan, 27.53, 25.71},
				wantRevX:  []float64{40.69, 40.33, 42.06, 41.92, nan, 24.98, 30.63},
				wantRevY:  []float64{25.71, 27.53, nan, 39.55, 38.61, 38.86, 39.22},
				wantRevXY: []float64{30.63, 24.98, nan, 41.92, 42.06, 40.33, 40.69},
			},
			{alpha: 8, beta: -6,
				want:      []float64{311.36, 294.08, 287.88, 314.6, nan, 205.24, 187.08},
				wantRevX:  []float64{323.12, 305.84, 315.48, 333.56, nan, 184.84, 226.44},
				wantRevY:  []float64{203.28, 203.44, nan, 314.6, 305.28, 295.88, 295.16},
				wantRevXY: []float64{242.64, 183.04, nan, 333.56, 332.88, 307.64, 306.92},
			},
		},
	},
	{ // 11x11
		m: 11,
		n: 11,
		A: []float64{
			0.4, 3., 2.5, 2., 0.4, 2., 2., 1., 0.1, 0.3, 2.,
			1.7, 0.7, 2.6, 1.6, 0.5, 2.4, 3., 0.9, 0.1, 2.8, 1.3,
			1.1, 2.2, 1.5, 0.8, 2.9, 0.4, 0.5, 1.7, 0.8, 2.6, 0.7,
			2.2, 1.7, 0.8, 2.9, 0.7, 0.7, 1.7, 1.8, 1.9, 2.4, 1.9,
			0.3, 0.5, 1.6, 1.5, 1.5, 2.4, 1.7, 1.2, 1.9, 2.8, 1.2,
			1.4, 2.2, 1.7, 1.4, 2.7, 1.4, 0.9, 1.8, 0.5, 1.2, 1.9,
			0.8, 2.3, 1.7, 1.3, 2., 2.8, 2.6, 0.4, 2.5, 1.3, 0.5,
			2.4, 2.8, 1.1, 0.2, 0.4, 2.8, 0.5, 0.5, 0., 2.8, 1.9,
			2.3, 1.8, 2.3, 1.7, 1.1, 0.1, 1.4, 1.2, 1.9, 0.5, 0.6,
			0.6, 2.4, 1.2, 0.3, 1.4, 1.3, 2.5, 2.6, 0., 1.3, 2.6,
			0.7, 1.5, 0.2, 1.4, 1.1, 1.8, 0.2, 1., 1., 0.6, 1.2,
		},
		x: []float64{2.5, 1.2, 0.8, 2.9, 3.4, 1.8, 4.6, 3.3, 3.8, 0.9, 1.1},
		y: []float64{3.8, 3.4, 1.6, 4.8, 4.3, 0.5, 2., 2.5, 1.5, 2.8, 3.9},

		NoTrans: []DgemvSubcase{ // (4x4, 4x2, 4x1, 2x4, 2x2, 2x1, 1x4, 1x2, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevX:  []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevY:  []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevXY: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{3.8, 3.4, 1.6, 4.8, 4.3, 0.5, 2., 2.5, 1.5, 2.8, 3.9},
				wantRevX:  []float64{3.8, 3.4, 1.6, 4.8, 4.3, 0.5, 2., 2.5, 1.5, 2.8, 3.9},
				wantRevY:  []float64{3.8, 3.4, 1.6, 4.8, 4.3, 0.5, 2., 2.5, 1.5, 2.8, 3.9},
				wantRevXY: []float64{3.8, 3.4, 1.6, 4.8, 4.3, 0.5, 2., 2.5, 1.5, 2.8, 3.9},
			},
			{alpha: 1, beta: 0,
				want:      []float64{32.71, 38.93, 33.55, 45.46, 39.24, 38.41, 46.23, 25.78, 37.33, 37.42, 24.63},
				wantRevX:  []float64{39.82, 43.78, 37.73, 41.19, 40.17, 44.41, 42.75, 28.14, 35.6, 41.25, 23.9},
				wantRevY:  []float64{24.63, 37.42, 37.33, 25.78, 46.23, 38.41, 39.24, 45.46, 33.55, 38.93, 32.71},
				wantRevXY: []float64{23.9, 41.25, 35.6, 28.14, 42.75, 44.41, 40.17, 41.19, 37.73, 43.78, 39.82},
			},
			{alpha: 8, beta: -6,
				want:      []float64{238.88, 291.04, 258.8, 334.88, 288.12, 304.28, 357.84, 191.24, 289.64, 282.56, 173.64},
				wantRevX:  []float64{295.76, 329.84, 292.24, 300.72, 295.56, 352.28, 330., 210.12, 275.8, 313.2, 167.8},
				wantRevY:  []float64{174.24, 278.96, 289.04, 177.44, 344.04, 304.28, 301.92, 348.68, 259.4, 294.64, 238.28},
				wantRevXY: []float64{168.4, 309.6, 275.2, 196.32, 316.2, 352.28, 309.36, 314.52, 292.84, 333.44, 295.16},
			},
		},

		Trans: []DgemvSubcase{ // (4x4, 2x4, 1x4, 4x2, 2x2, 1x2, 4x1, 2x1, 1x1)
			{alpha: 0, beta: 0,
				want:      []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevX:  []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevY:  []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
				wantRevXY: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			},
			{alpha: 0, beta: 1,
				want:      []float64{2.5, 1.2, 0.8, 2.9, 3.4, 1.8, 4.6, 3.3, 3.8, 0.9, 1.1},
				wantRevX:  []float64{2.5, 1.2, 0.8, 2.9, 3.4, 1.8, 4.6, 3.3, 3.8, 0.9, 1.1},
				wantRevY:  []float64{2.5, 1.2, 0.8, 2.9, 3.4, 1.8, 4.6, 3.3, 3.8, 0.9, 1.1},
				wantRevXY: []float64{2.5, 1.2, 0.8, 2.9, 3.4, 1.8, 4.6, 3.3, 3.8, 0.9, 1.1},
			},
			{alpha: 1, beta: 0,
				want:      []float64{37.07, 55.58, 46.05, 47.34, 33.88, 54.19, 50.85, 39.31, 31.29, 55.31, 46.98},
				wantRevX:  []float64{38.11, 63.38, 46.44, 40.04, 34.63, 59.27, 50.13, 35.45, 28.26, 51.64, 46.22},
				wantRevY:  []float64{46.98, 55.31, 31.29, 39.31, 50.85, 54.19, 33.88, 47.34, 46.05, 55.58, 37.07},
				wantRevXY: []float64{46.22, 51.64, 28.26, 35.45, 50.13, 59.27, 34.63, 40.04, 46.44, 63.38, 38.11},
			},
			{alpha: 8, beta: -6,
				want:      []float64{281.56, 437.44, 363.6, 361.32, 250.64, 422.72, 379.2, 294.68, 227.52, 437.08, 369.24},
				wantRevX:  []float64{289.88, 499.84, 366.72, 302.92, 256.64, 463.36, 373.44, 263.8, 203.28, 407.72, 363.16},
				wantRevY:  []float64{360.84, 435.28, 245.52, 297.08, 386.4, 422.72, 243.44, 358.92, 345.6, 439.24, 289.96},
				wantRevXY: []float64{354.76, 405.92, 221.28, 266.2, 380.64, 463.36, 249.44, 300.52, 348.72, 501.64, 298.28},
			},
		},
	},
}

func TestGemv(t *testing.T) {
	for _, test := range DgemvCases {
		t.Run(fmt.Sprintf("(%vx%v)", test.m, test.n), func(tt *testing.T) {
			for i, cas := range test.NoTrans {
				tt.Run(fmt.Sprintf("NoTrans case %v", i), func(st *testing.T) {
					dgemvcomp(st, test, false, cas, i)
				})
			}
			for i, cas := range test.Trans {
				tt.Run(fmt.Sprintf("Trans case %v", i), func(st *testing.T) {
					dgemvcomp(st, test, true, cas, i)
				})
			}
		})
	}
}

func dgemvcomp(t *testing.T, test DgemvCase, trans bool, cas DgemvSubcase, i int) {
	const (
		xGdVal, yGdVal, aGdVal = 0.5, 1.5, 10
		gdLn                   = 4
	)
	if trans {
		test.x, test.y = test.y, test.x
	}
	prefix := fmt.Sprintf("Test (%vx%v) t:%v (a:%v,b:%v)", test.m, test.n, trans, cas.alpha, cas.beta)
	xg, yg := guardVector(test.x, xGdVal, gdLn), guardVector(test.y, yGdVal, gdLn)
	x, y := xg[gdLn:len(xg)-gdLn], yg[gdLn:len(yg)-gdLn]
	ag := guardVector(test.A, aGdVal, gdLn)
	a := ag[gdLn : len(ag)-gdLn]

	lda := uintptr(test.n)
	if trans {
		GemvT(uintptr(test.m), uintptr(test.n), cas.alpha, a, lda, x, 1, cas.beta, y, 1)
	} else {
		GemvN(uintptr(test.m), uintptr(test.n), cas.alpha, a, lda, x, 1, cas.beta, y, 1)
	}
	for i := range cas.want {
		if !within(y[i], cas.want[i]) {
			t.Errorf(msgVal, prefix, i, y[i], cas.want[i])
		}
	}

	if !isValidGuard(xg, xGdVal, gdLn) {
		t.Errorf(msgGuard, prefix, "x", xg[:gdLn], xg[len(xg)-gdLn:])
	}
	if !isValidGuard(yg, yGdVal, gdLn) {
		t.Errorf(msgGuard, prefix, "y", yg[:gdLn], yg[len(yg)-gdLn:])
	}
	if !isValidGuard(ag, aGdVal, gdLn) {
		t.Errorf(msgGuard, prefix, "a", ag[:gdLn], ag[len(ag)-gdLn:])
	}
	if !equalStrided(test.x, x, 1) {
		t.Errorf(msgReadOnly, prefix, "x")
	}
	if !equalStrided(test.A, a, 1) {
		t.Errorf(msgReadOnly, prefix, "a")
	}

	for _, inc := range newIncSet(-1, 1, 2, 3, 90) {
		incPrefix := fmt.Sprintf("%s inc(x:%v, y:%v)", prefix, inc.x, inc.y)
		want, incY := cas.want, inc.y
		switch {
		case inc.x < 0 && inc.y < 0:
			want = cas.wantRevXY
			incY = -inc.y
		case inc.x < 0:
			want = cas.wantRevX
		case inc.y < 0:
			want = cas.wantRevY
			incY = -inc.y
		}
		xg, yg := guardIncVector(test.x, xGdVal, inc.x, gdLn), guardIncVector(test.y, yGdVal, inc.y, gdLn)
		x, y := xg[gdLn:len(xg)-gdLn], yg[gdLn:len(yg)-gdLn]
		ag := guardVector(test.A, aGdVal, gdLn)
		a := ag[gdLn : len(ag)-gdLn]

		if trans {
			GemvT(uintptr(test.m), uintptr(test.n), cas.alpha,
				a, lda, x, uintptr(inc.x),
				cas.beta, y, uintptr(inc.y))
		} else {
			GemvN(uintptr(test.m), uintptr(test.n), cas.alpha,
				a, lda, x, uintptr(inc.x),
				cas.beta, y, uintptr(inc.y))
		}
		for i := range want {
			if !within(y[i*incY], want[i]) {
				t.Errorf(msgVal, incPrefix, i, y[i*incY], want[i])
				t.Error(y[i*incY] - want[i])
			}
		}

		checkValidIncGuard(t, xg, xGdVal, inc.x, gdLn)
		checkValidIncGuard(t, yg, yGdVal, inc.y, gdLn)
		if !isValidGuard(ag, aGdVal, gdLn) {
			t.Errorf(msgGuard, incPrefix, "a", ag[:gdLn], ag[len(ag)-gdLn:])
		}
		if !equalStrided(test.x, x, inc.x) {
			t.Errorf(msgReadOnly, incPrefix, "x")
		}
		if !equalStrided(test.A, a, 1) {
			t.Errorf(msgReadOnly, incPrefix, "a")
		}
	}
}
