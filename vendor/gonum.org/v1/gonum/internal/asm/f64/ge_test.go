// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package f64

import (
	"fmt"
	"testing"
)

var gerTests = []struct {
	x, y, a []float64
	want    []float64
}{ // m x n ( kernels executed )
	{ // 1 x 1 (1x1)
		x:    []float64{2},
		y:    []float64{4.4},
		a:    []float64{10},
		want: []float64{18.8},
	},
	{ // 3 x 2 ( 2x2, 1x2 )
		x: []float64{-2, -3, 0},
		y: []float64{-1.1, 5},
		a: []float64{
			1.3, 2.4,
			2.6, 2.8,
			-1.3, -4.3,
		},
		want: []float64{3.5, -7.6, 5.9, -12.2, -1.3, -4.3},
	},
	{ // 3 x 3 ( 2x2, 2x1, 1x2, 1x1 )
		x: []float64{-2, 7, 12},
		y: []float64{-1.1, 0, 6},
		a: []float64{
			1.3, 2.4, 3.5,
			2.6, 2.8, 3.3,
			-1.3, -4.3, -9.7,
		},
		want: []float64{3.5, 2.4, -8.5, -5.1, 2.8, 45.3, -14.5, -4.3, 62.3},
	},
	{ // 5 x 3 ( 4x2, 4x1, 1x2, 1x1 )
		x: []float64{-2, -3, 0, 1, 2},
		y: []float64{-1.1, 5, 0},
		a: []float64{
			1.3, 2.4, 3.5,
			2.6, 2.8, 3.3,
			-1.3, -4.3, -9.7,
			8, 9, -10,
			-12, -14, -6,
		},
		want: []float64{3.5, -7.6, 3.5, 5.9, -12.2, 3.3, -1.3, -4.3, -9.7, 6.9, 14, -10, -14.2, -4, -6},
	},
	{ // 3 x 6 ( 2x4, 2x2, 1x4, 1x2 )
		x: []float64{-2, -3, 0},
		y: []float64{-1.1, 5, 0, 9, 19, 22},
		a: []float64{
			1.3, 2.4, 3.5, 4.8, 1.11, -9,
			2.6, 2.8, 3.3, -3.4, 6.2, -8.7,
			-1.3, -4.3, -9.7, -3.1, 8.9, 8.9,
		},
		want: []float64{3.5, -7.6, 3.5, -13.2, -36.89, -53, 5.9, -12.2, 3.3, -30.4, -50.8, -74.7, -1.3, -4.3, -9.7, -3.1, 8.9, 8.9},
	},
	{ // 5 x 5 ( 4x4, 4x1, 1x4, 1x1)
		x: []float64{-2, 0, 2, 0, 7},
		y: []float64{-1.1, 8, 7, 3, 5},
		a: []float64{
			1.3, 2.4, 3.5, 2.2, 8.3,
			2.6, 2.8, 3.3, 4.4, -1.5,
			-1.3, -4.3, -9.7, -8.8, 6.2,
			8, 9, -10, -11, 12,
			-12, -14, -6, -2, 4,
		},
		want: []float64{
			3.5, -13.6, -10.5, -3.8, -1.7,
			2.6, 2.8, 3.3, 4.4, -1.5,
			-3.5, 11.7, 4.3, -2.8, 16.2,
			8, 9, -10, -11, 12,
			-19.700000000000003, 42, 43, 19, 39,
		},
	},
	{ // 7 x 7 ( 4x4, 4x2, 4x1, 2x4, 2x2, 2x1, 1x4, 1x2, 1x1 ) < nan test >
		x: []float64{-2, 8, 9, -3, -1.2, 5, 4.5},
		y: []float64{-1.1, nan, 19, 11, -9.22, 7, 3.3},
		a: []float64{
			1.3, 2.4, 3.5, 4.8, 1.11, -9, 2.2,
			2.6, 2.8, 3.3, -3.4, 6.2, -8.7, 5.1,
			-1.3, -4.3, -9.7, -3.1, 8.9, 8.9, 8,
			5, -2.5, 1.8, -3.6, 2.8, 4.9, 7,
			-1.3, -4.3, -9.7, -3.1, 8.9, 8.9, 8,
			2.6, 2.8, 3.3, -3.4, 6.2, -8.7, 5.1,
			1.3, 2.4, 3.5, 4.8, 1.11, -9, 2.2,
		},
		want: []float64{
			3.5, nan, -34.5, -17.2, 19.55, -23, -4.4,
			-6.2, nan, 155.3, 84.6, -67.56, 47.3, 31.5,
			-11.2, nan, 161.3, 95.9, -74.08, 71.9, 37.7,
			8.3, nan, -55.2, -36.6, 30.46, -16.1, -2.9,
			0.02, nan, -32.5, -16.3, 19.964, 0.5, 4.04,
			-2.9, nan, 98.3, 51.6, -39.9, 26.3, 21.6,
			-3.65, nan, 89, 54.3, -40.38, 22.5, 17.05,
		},
	},
}

func TestGer(t *testing.T) {
	const (
		xGdVal, yGdVal, aGdVal = -0.5, 1.5, 10
		gdLn                   = 4
	)
	for i, test := range gerTests {
		m, n := len(test.x), len(test.y)
		for _, align := range align2 {
			prefix := fmt.Sprintf("Test %v (%vx%v) align(x:%v,y:%v,a:%v)",
				i, m, n, align.x, align.y, align.x^align.y)
			xgLn, ygLn, agLn := gdLn+align.x, gdLn+align.y, gdLn+align.x^align.y
			xg, yg := guardVector(test.x, xGdVal, xgLn), guardVector(test.y, yGdVal, ygLn)
			x, y := xg[xgLn:len(xg)-xgLn], yg[ygLn:len(yg)-ygLn]
			ag := guardVector(test.a, aGdVal, agLn)
			a := ag[agLn : len(ag)-agLn]

			alpha := 1.0
			Ger(uintptr(m), uintptr(n), alpha, x, 1, y, 1, a, uintptr(n))
			for i := range test.want {
				if !within(a[i], test.want[i]) {
					t.Errorf(msgVal, prefix, i, a[i], test.want[i])
					t.Error(a)
					return
				}
			}
			if !isValidGuard(xg, xGdVal, xgLn) {
				t.Errorf(msgGuard, prefix, "x", xg[:xgLn], xg[len(xg)-xgLn:])
			}
			if !isValidGuard(yg, yGdVal, ygLn) {
				t.Errorf(msgGuard, prefix, "y", yg[:ygLn], yg[len(yg)-ygLn:])
			}
			if !isValidGuard(ag, aGdVal, agLn) {
				t.Errorf(msgGuard, prefix, "a", ag[:agLn], ag[len(ag)-agLn:])
			}
			if !equalStrided(test.x, x, 1) {
				t.Errorf(msgReadOnly, prefix, "x")
			}
			if !equalStrided(test.y, y, 1) {
				t.Errorf(msgReadOnly, prefix, "y")
			}
		}

		for _, inc := range newIncSet(1, 2) {
			prefix := fmt.Sprintf("Test %v (%vx%v) inc(x:%v,y:%v)", i, m, n, inc.x, inc.y)
			xg := guardIncVector(test.x, xGdVal, inc.x, gdLn)
			yg := guardIncVector(test.y, yGdVal, inc.y, gdLn)
			x, y := xg[gdLn:len(xg)-gdLn], yg[gdLn:len(yg)-gdLn]
			ag := guardVector(test.a, aGdVal, gdLn)
			a := ag[gdLn : len(ag)-gdLn]

			alpha := 3.5
			Ger(uintptr(m), uintptr(n), alpha, x, uintptr(inc.x), y, uintptr(inc.y), a, uintptr(n))
			for i := range test.want {
				want := alpha*(test.want[i]-test.a[i]) + test.a[i]
				if !within(a[i], want) {
					t.Errorf(msgVal, prefix, i, a[i], want)
				}
			}
			checkValidIncGuard(t, xg, xGdVal, inc.x, gdLn)
			checkValidIncGuard(t, yg, yGdVal, inc.y, gdLn)
			if !isValidGuard(ag, aGdVal, gdLn) {
				t.Errorf(msgGuard, prefix, "a", ag[:gdLn], ag[len(ag)-gdLn:])
			}
			if !equalStrided(test.x, x, inc.x) {
				t.Errorf(msgReadOnly, prefix, "x")
			}
			if !equalStrided(test.y, y, inc.y) {
				t.Errorf(msgReadOnly, prefix, "y")
			}
		}
	}
}

func BenchmarkGer(t *testing.B) {
	const alpha = 3
	for _, dims := range newIncSet(3, 10, 30, 100, 300, 1e3, 3e3, 1e4) {
		m, n := dims.x, dims.y
		if m/n >= 100 || n/m >= 100 {
			continue
		}
		for _, inc := range newIncSet(1, 3, 4, 10) {
			t.Run(fmt.Sprintf("Dger %dx%d (%d %d)", m, n, inc.x, inc.y), func(b *testing.B) {
				x, y, a := gerData(m, n, inc.x, inc.y)
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					Ger(uintptr(m), uintptr(n), alpha, x, uintptr(inc.x), y, uintptr(inc.y), a, uintptr(n))
				}
			})

		}
	}
}

func gerData(m, n, incX, incY int) (x, y, a []float64) {
	x = make([]float64, m*incX)
	y = make([]float64, n*incY)
	a = make([]float64, m*n)
	ln := len(x)
	if len(y) > ln {
		ln = len(y)
	}
	if len(a) > ln {
		ln = len(a)
	}
	for i := 0; i < ln; i++ {
		v := float64(i)
		if i < len(a) {
			a[i] = v
		}
		if i < len(x) {
			x[i] = v
		}
		if i < len(y) {
			y[i] = v
		}
	}
	return x, y, a
}
