// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c64

import "testing"

var tests = []struct {
	incX, incY, incDst int
	ix, iy, idst       uintptr
	a                  complex64
	dst, x, y          []complex64
	ex                 []complex64
}{
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 + 1i,
		dst: []complex64{5},
		x:   []complex64{1},
		y:   []complex64{1i},
		ex:  []complex64{1 + 2i}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 + 2i,
		dst: []complex64{0, 0, 0},
		x:   []complex64{0, 0, 0},
		y:   []complex64{1, 1, 1},
		ex:  []complex64{1, 1, 1}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 + 2i,
		dst: []complex64{0, 0, 0},
		x:   []complex64{0, 0},
		y:   []complex64{1, 1, 1},
		ex:  []complex64{1, 1}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 + 2i,
		dst: []complex64{1i, 1i, 1i},
		x:   []complex64{1i, 1i, 1i},
		y:   []complex64{1, 2, 1},
		ex:  []complex64{-1 + 1i, 1i, -1 + 1i}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   -1i,
		dst: []complex64{1i, 1i, 1i},
		x:   []complex64{1i, 1i, 1i},
		y:   []complex64{1, 2, 1},
		ex:  []complex64{2, 3, 2}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   -1i,
		dst: []complex64{1i, 1i, 1i},
		x:   []complex64{1i, 1i, 1i, 1i, 1i}[1:4],
		y:   []complex64{1, 1, 2, 1, 1}[1:4],
		ex:  []complex64{2, 3, 2}},
	{incX: 2, incY: 4, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   -2,
		dst: []complex64{1i, 1i, 1i, 1i, 1i},
		x:   []complex64{2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i},
		y:   []complex64{1, 1, 2, 1, 1},
		ex:  []complex64{-3 - 2i, -3 - 2i, -2 - 2i, -3 - 2i, -3 - 2i}},
	// Run big test twice, once aligned once unaligned.
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 - 1i,
		dst: make([]complex64, 10),
		x:   []complex64{1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i},
		y:   []complex64{1, 1, 2, 1, 1, 1, 1, 2, 1, 1},
		ex:  []complex64{2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i}},
	{incX: 2, incY: 2, incDst: 3, ix: 0, iy: 0, idst: 0,
		a:   1 - 1i,
		dst: make([]complex64, 10),
		x:   []complex64{1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i},
		y:   []complex64{1, 1, 2, 1, 1, 1, 1, 2, 1, 1},
		ex:  []complex64{2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i}},
	{incX: -2, incY: -2, incDst: -3, ix: 18, iy: 18, idst: 27,
		a:   1 - 1i,
		dst: make([]complex64, 10),
		x:   []complex64{1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i},
		y:   []complex64{1, 1, 2, 1, 1, 1, 1, 2, 1, 1},
		ex:  []complex64{2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i}},
	{incX: -2, incY: 2, incDst: -3, ix: 18, iy: 0, idst: 27,
		a:   1 - 1i,
		dst: make([]complex64, 10),
		x:   []complex64{1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i, 1i},
		y:   []complex64{1, 1, 2, 1, 1, 1, 1, 2, 1, 1},
		ex:  []complex64{2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 2 + 1i, 3 + 1i, 2 + 1i, 2 + 1i}},
}

func TestAxpyUnitary(t *testing.T) {
	var x_gd, y_gd complex64 = 1, 1
	for cas, test := range tests {
		xg_ln, yg_ln := 4+cas%2, 4+cas%3
		test.x, test.y = guardVector(test.x, x_gd, xg_ln), guardVector(test.y, y_gd, yg_ln)
		x, y := test.x[xg_ln:len(test.x)-xg_ln], test.y[yg_ln:len(test.y)-yg_ln]
		AxpyUnitary(test.a, x, y)
		for i := range test.ex {
			if y[i] != test.ex[i] {
				t.Errorf("Test %d Unexpected result at %d Got: %v Expected: %v", cas, i, y[i], test.ex[i])
			}
		}
		if !isValidGuard(test.x, x_gd, xg_ln) {
			t.Errorf("Test %d Guard violated in x vector %v %v", cas, test.x[:xg_ln], test.x[len(test.x)-xg_ln:])
		}
		if !isValidGuard(test.y, y_gd, yg_ln) {
			t.Errorf("Test %d Guard violated in y vector %v %v", cas, test.y[:yg_ln], test.y[len(test.y)-yg_ln:])
		}
	}
}

func TestAxpyUnitaryTo(t *testing.T) {
	var x_gd, y_gd, dst_gd complex64 = 1, 1, 0
	for cas, test := range tests {
		xg_ln, yg_ln := 4+cas%2, 4+cas%3
		test.x, test.y = guardVector(test.x, x_gd, xg_ln), guardVector(test.y, y_gd, yg_ln)
		test.dst = guardVector(test.dst, dst_gd, xg_ln)
		x, y := test.x[xg_ln:len(test.x)-xg_ln], test.y[yg_ln:len(test.y)-yg_ln]
		dst := test.dst[xg_ln : len(test.dst)-xg_ln]
		AxpyUnitaryTo(dst, test.a, x, y)
		for i := range test.ex {
			if dst[i] != test.ex[i] {
				t.Errorf("Test %d Unexpected result at %d Got: %v Expected: %v", cas, i, dst[i], test.ex[i])
			}
		}
		if !isValidGuard(test.x, x_gd, xg_ln) {
			t.Errorf("Test %d Guard violated in x vector %v %v", cas, test.x[:xg_ln], test.x[len(test.x)-xg_ln:])
		}
		if !isValidGuard(test.y, y_gd, yg_ln) {
			t.Errorf("Test %d Guard violated in y vector %v %v", cas, test.y[:yg_ln], test.y[len(test.y)-yg_ln:])
		}
		if !isValidGuard(test.dst, dst_gd, xg_ln) {
			t.Errorf("Test %d Guard violated in dst vector %v %v", cas, test.dst[:xg_ln], test.dst[len(test.dst)-xg_ln:])
		}
	}
}

func TestAxpyInc(t *testing.T) {
	var x_gd, y_gd complex64 = 1, 1
	for cas, test := range tests {
		xg_ln, yg_ln := 4+cas%2, 4+cas%3
		test.x, test.y = guardIncVector(test.x, x_gd, test.incX, xg_ln), guardIncVector(test.y, y_gd, test.incY, yg_ln)
		x, y := test.x[xg_ln:len(test.x)-xg_ln], test.y[yg_ln:len(test.y)-yg_ln]
		AxpyInc(test.a, x, y, uintptr(len(test.ex)), uintptr(test.incX), uintptr(test.incY), test.ix, test.iy)
		for i := range test.ex {
			if y[int(test.iy)+i*int(test.incY)] != test.ex[i] {
				t.Errorf("Test %d Unexpected result at %d Got: %v Expected: %v", cas, i, y[i*int(test.incY)], test.ex[i])
			}
		}
		checkValidIncGuard(t, test.x, x_gd, test.incX, xg_ln)
		checkValidIncGuard(t, test.y, y_gd, test.incY, yg_ln)
	}
}

func TestAxpyIncTo(t *testing.T) {
	var x_gd, y_gd, dst_gd complex64 = 1, 1, 0
	for cas, test := range tests {
		xg_ln, yg_ln := 4+cas%2, 4+cas%3
		test.x, test.y = guardIncVector(test.x, x_gd, test.incX, xg_ln), guardIncVector(test.y, y_gd, test.incY, yg_ln)
		test.dst = guardIncVector(test.dst, dst_gd, test.incDst, xg_ln)
		x, y := test.x[xg_ln:len(test.x)-xg_ln], test.y[yg_ln:len(test.y)-yg_ln]
		dst := test.dst[xg_ln : len(test.dst)-xg_ln]
		AxpyIncTo(dst, uintptr(test.incDst), test.idst, test.a, x, y, uintptr(len(test.ex)), uintptr(test.incX), uintptr(test.incY), test.ix, test.iy)
		for i := range test.ex {
			if dst[int(test.idst)+i*int(test.incDst)] != test.ex[i] {
				t.Errorf("Test %d Unexpected result at %d Got: %v Expected: %v", cas, i, dst[i*int(test.incDst)], test.ex[i])
			}
		}
		checkValidIncGuard(t, test.x, x_gd, test.incX, xg_ln)
		checkValidIncGuard(t, test.y, y_gd, test.incY, yg_ln)
		checkValidIncGuard(t, test.dst, dst_gd, test.incDst, xg_ln)
	}
}
