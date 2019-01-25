// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dlasrter interface {
	Dlasrt(s lapack.Sort, n int, d []float64)
}

func DlasrtTest(t *testing.T, impl Dlasrter) {
	for ti, test := range []struct {
		data    []float64
		wantInc []float64
		wantDec []float64
	}{
		{
			data:    nil,
			wantInc: nil,
			wantDec: nil,
		},
		{
			data:    []float64{},
			wantInc: []float64{},
			wantDec: []float64{},
		},
		{
			data:    []float64{1},
			wantInc: []float64{1},
			wantDec: []float64{1},
		},
		{
			data:    []float64{1, 2},
			wantInc: []float64{1, 2},
			wantDec: []float64{2, 1},
		},
		{
			data:    []float64{1, 2, -3},
			wantInc: []float64{-3, 1, 2},
			wantDec: []float64{2, 1, -3},
		},
		{
			data:    []float64{-5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5},
			wantInc: []float64{-5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5},
			wantDec: []float64{5, 4, 3, 2, 1, 0, -1, -2, -3, -4, -5},
		},
		{
			data:    []float64{5, 4, 3, 2, 1, 0, -1, -2, -3, -4, -5},
			wantInc: []float64{-5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5},
			wantDec: []float64{5, 4, 3, 2, 1, 0, -1, -2, -3, -4, -5},
		},
		{
			data:    []float64{-2, 4, -1, 2, -4, 0, 3, 5, -5, 1, -3},
			wantInc: []float64{-5, -4, -3, -2, -1, 0, 1, 2, 3, 4, 5},
			wantDec: []float64{5, 4, 3, 2, 1, 0, -1, -2, -3, -4, -5},
		},
	} {
		n := len(test.data)
		ds := make([]float64, n)

		copy(ds, test.data)
		impl.Dlasrt(lapack.SortIncreasing, n, ds)
		if !floats.Equal(ds, test.wantInc) {
			t.Errorf("Case #%v: unexpected result of SortIncreasing", ti)
		}

		copy(ds, test.data)
		impl.Dlasrt(lapack.SortDecreasing, n, ds)
		if !floats.Equal(ds, test.wantDec) {
			t.Errorf("Case #%v: unexpected result of SortIncreasing", ti)
		}
	}
}
