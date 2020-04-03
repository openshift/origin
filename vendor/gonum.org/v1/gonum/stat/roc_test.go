// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stat

import (
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
)

func TestROC(t *testing.T) {
	cases := []struct {
		y          []float64
		c          []bool
		w          []float64
		cutoffs    []float64
		wantTPR    []float64
		wantFPR    []float64
		wantThresh []float64
	}{
		// Test cases were calculated using sklearn metrics.roc_curve when
		// cutoffs is nil. Where cutoffs is not nil, a visual inspection is
		// used.
		// Some differences exist between unweighted ROCs from our function
		// and metrics.roc_curve which appears to use integer cutoffs in that
		// case. sklearn also appears to do some magic that trims leading zeros
		// sometimes.
		{ // 0
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			wantTPR:    []float64{0, 0.25, 0.5, 0.75, 0.75, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 7.5, 6, 5, 3, 0},
		},
		{ // 1
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			wantTPR:    []float64{0, 0.25, 0.5, 0.875, 0.875, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 7.5, 6, 5, 3, 0},
		},
		{ // 2
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			cutoffs:    []float64{-1, 2, 4, 6, 8},
			wantTPR:    []float64{0, 0.5, 0.75, 1, 1},
			wantFPR:    []float64{0, 0, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 4, 2},
		},
		{ // 3
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			cutoffs:    []float64{-1, 1, 2, 3, 4, 5, 6, 7, 8},
			wantTPR:    []float64{0, 0.5, 0.5, 0.75, 0.75, 0.75, 1, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0, 0.5, 0.5, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 7, 6, 5, 4, 3, 2, 1},
		},
		{ // 4
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			cutoffs:    []float64{-1, 2, 4, 6, 8},
			wantTPR:    []float64{0, 0.5, 0.875, 1, 1},
			wantFPR:    []float64{0, 0, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 4, 2},
		},
		{ // 5
			y:          []float64{0, 3, 5, 6, 7.5, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			cutoffs:    []float64{-1, 1, 2, 3, 4, 5, 6, 7, 8},
			wantTPR:    []float64{0, 0.5, 0.5, 0.875, 0.875, 0.875, 1, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0, 0.6, 0.6, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 7, 6, 5, 4, 3, 2, 1},
		},
		{ // 6
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			wantTPR:    []float64{0, 0.25, 0.75, 1, 1},
			wantFPR:    []float64{0, 0, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 3, 0},
		},
		{ // 7
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			wantTPR:    []float64{0, 0.25, 0.875, 1, 1},
			wantFPR:    []float64{0, 0, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 3, 0},
		},
		{ // 8
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			cutoffs:    []float64{-1, 2, 4, 6, 8},
			wantTPR:    []float64{0, 0.25, 0.75, 1, 1},
			wantFPR:    []float64{0, 0, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 4, 2},
		},
		{ // 9
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			cutoffs:    []float64{-1, 1, 2, 3, 4, 5, 6, 7, 8},
			wantTPR:    []float64{0, 0.25, 0.25, 0.75, 0.75, 0.75, 1, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0.5, 0.5, 0.5, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 8, 7, 6, 5, 4, 3, 2, 1},
		},
		{ // 10
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			cutoffs:    []float64{-1, 2, 4, 6, 8},
			wantTPR:    []float64{0, 0.25, 0.875, 1, 1},
			wantFPR:    []float64{0, 0, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 6, 4, 2},
		},
		{ // 11
			y:          []float64{0, 3, 6, 6, 6, 8},
			c:          []bool{false, true, false, true, true, true},
			w:          []float64{4, 1, 6, 3, 2, 2},
			cutoffs:    []float64{-1, 1, 2, 3, 4, 5, 6, 7, 8},
			wantTPR:    []float64{0, 0.25, 0.25, 0.875, 0.875, 0.875, 1, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0.6, 0.6, 0.6, 0.6, 0.6, 1},
			wantThresh: []float64{math.Inf(1), 8, 7, 6, 5, 4, 3, 2, 1},
		},
		{ // 12
			y:          []float64{0.1, 0.35, 0.4, 0.8},
			c:          []bool{true, false, true, false},
			wantTPR:    []float64{0, 0, 0.5, 0.5, 1},
			wantFPR:    []float64{0, 0.5, 0.5, 1, 1},
			wantThresh: []float64{math.Inf(1), 0.8, 0.4, 0.35, 0.1},
		},
		{ // 13
			y:          []float64{0.1, 0.35, 0.4, 0.8},
			c:          []bool{false, false, true, true},
			wantTPR:    []float64{0, 0.5, 1, 1, 1},
			wantFPR:    []float64{0, 0, 0, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 0.8, 0.4, 0.35, 0.1},
		},
		{ // 14
			y:          []float64{0.01, 0.02, 0.03, 0.04, 0.05, 0.06, 10},
			c:          []bool{false, true, false, false, true, true, false},
			cutoffs:    []float64{-1, 2.5, 5, 7.5, 10},
			wantTPR:    []float64{0, 0, 0, 0, 1},
			wantFPR:    []float64{0, 0.25, 0.25, 0.25, 1},
			wantThresh: []float64{math.Inf(1), 10, 7.5, 5, 2.5},
		},
		{ // 15
			y:          []float64{1, 2},
			c:          []bool{false, false},
			wantTPR:    []float64{math.NaN(), math.NaN(), math.NaN()},
			wantFPR:    []float64{0, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 2, 1},
		},
		{ // 16
			y:          []float64{1, 2},
			c:          []bool{false, false},
			cutoffs:    []float64{-1, 2},
			wantTPR:    []float64{math.NaN(), math.NaN()},
			wantFPR:    []float64{0, 1},
			wantThresh: []float64{math.Inf(1), 2},
		},
		{ // 17
			y:          []float64{1, 2},
			c:          []bool{false, false},
			cutoffs:    []float64{0, 1.2, 1.4, 1.6, 1.8, 2},
			wantTPR:    []float64{math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN()},
			wantFPR:    []float64{0, 0.5, 0.5, 0.5, 0.5, 1},
			wantThresh: []float64{math.Inf(1), 2, 1.8, 1.6, 1.4, 1.2},
		},
		{ // 18
			y:          []float64{1},
			c:          []bool{false},
			wantTPR:    []float64{math.NaN(), math.NaN()},
			wantFPR:    []float64{0, 1},
			wantThresh: []float64{math.Inf(1), 1},
		},
		{ // 19
			y:          []float64{1},
			c:          []bool{false},
			cutoffs:    []float64{-1, 1},
			wantTPR:    []float64{math.NaN(), math.NaN()},
			wantFPR:    []float64{0, 1},
			wantThresh: []float64{math.Inf(1), 1},
		},
		{ // 20
			y:          []float64{1},
			c:          []bool{true},
			wantTPR:    []float64{0, 1},
			wantFPR:    []float64{math.NaN(), math.NaN()},
			wantThresh: []float64{math.Inf(1), 1},
		},
		{ // 21
			y:          []float64{},
			c:          []bool{},
			wantTPR:    nil,
			wantFPR:    nil,
			wantThresh: nil,
		},
		{ // 22
			y:          []float64{},
			c:          []bool{},
			cutoffs:    []float64{-1, 2.5, 5, 7.5, 10},
			wantTPR:    nil,
			wantFPR:    nil,
			wantThresh: nil,
		},
	}
	for i, test := range cases {
		gotTPR, gotFPR, gotThresh := ROC(test.cutoffs, test.y, test.c, test.w)
		if !floats.Same(gotTPR, test.wantTPR) {
			t.Errorf("%d: unexpected TPR got:%v want:%v", i, gotTPR, test.wantTPR)
		}
		if !floats.Same(gotFPR, test.wantFPR) {
			t.Errorf("%d: unexpected FPR got:%v want:%v", i, gotFPR, test.wantFPR)
		}
		if !floats.Same(gotThresh, test.wantThresh) {
			t.Errorf("%d: unexpected thresholds got:%#v want:%v", i, gotThresh, test.wantThresh)
		}
	}
}
