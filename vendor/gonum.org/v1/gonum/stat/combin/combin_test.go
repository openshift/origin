// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package combin

import (
	"math/big"
	"reflect"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// intSosMatch returns true if the two slices of slices are equal.
func intSosMatch(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i, s := range a {
		if len(s) != len(b[i]) {
			return false
		}
		for j, v := range s {
			if v != b[i][j] {
				return false
			}
		}
	}
	return true
}

var binomialTests = []struct {
	n, k, ans int
}{
	{0, 0, 1},
	{5, 0, 1},
	{5, 1, 5},
	{5, 2, 10},
	{5, 3, 10},
	{5, 4, 5},
	{5, 5, 1},

	{6, 0, 1},
	{6, 1, 6},
	{6, 2, 15},
	{6, 3, 20},
	{6, 4, 15},
	{6, 5, 6},
	{6, 6, 1},

	{20, 0, 1},
	{20, 1, 20},
	{20, 2, 190},
	{20, 3, 1140},
	{20, 4, 4845},
	{20, 5, 15504},
	{20, 6, 38760},
	{20, 7, 77520},
	{20, 8, 125970},
	{20, 9, 167960},
	{20, 10, 184756},
	{20, 11, 167960},
	{20, 12, 125970},
	{20, 13, 77520},
	{20, 14, 38760},
	{20, 15, 15504},
	{20, 16, 4845},
	{20, 17, 1140},
	{20, 18, 190},
	{20, 19, 20},
	{20, 20, 1},
}

func TestBinomial(t *testing.T) {
	for cas, test := range binomialTests {
		ans := Binomial(test.n, test.k)
		if ans != test.ans {
			t.Errorf("Case %v: Binomial mismatch. Got %v, want %v.", cas, ans, test.ans)
		}
	}
	var (
		n    = 61
		want big.Int
		got  big.Int
	)
	for k := 0; k <= n; k++ {
		want.Binomial(int64(n), int64(k))
		got.SetInt64(int64(Binomial(n, k)))
		if want.Cmp(&got) != 0 {
			t.Errorf("Case n=%v,k=%v: Binomial mismatch for large n. Got %v, want %v.", n, k, got, want)
		}
	}
}

func TestGeneralizedBinomial(t *testing.T) {
	for cas, test := range binomialTests {
		ans := GeneralizedBinomial(float64(test.n), float64(test.k))
		if !floats.EqualWithinAbsOrRel(ans, float64(test.ans), 1e-14, 1e-14) {
			t.Errorf("Case %v: Binomial mismatch. Got %v, want %v.", cas, ans, test.ans)
		}
	}
}

func TestCombinations(t *testing.T) {
	for cas, test := range []struct {
		n, k int
		data [][]int
	}{
		{
			n:    1,
			k:    1,
			data: [][]int{{0}},
		},
		{
			n:    2,
			k:    1,
			data: [][]int{{0}, {1}},
		},
		{
			n:    2,
			k:    2,
			data: [][]int{{0, 1}},
		},
		{
			n:    3,
			k:    1,
			data: [][]int{{0}, {1}, {2}},
		},
		{
			n:    3,
			k:    2,
			data: [][]int{{0, 1}, {0, 2}, {1, 2}},
		},
		{
			n:    3,
			k:    3,
			data: [][]int{{0, 1, 2}},
		},
		{
			n:    4,
			k:    1,
			data: [][]int{{0}, {1}, {2}, {3}},
		},
		{
			n:    4,
			k:    2,
			data: [][]int{{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}},
		},
		{
			n:    4,
			k:    3,
			data: [][]int{{0, 1, 2}, {0, 1, 3}, {0, 2, 3}, {1, 2, 3}},
		},
		{
			n:    4,
			k:    4,
			data: [][]int{{0, 1, 2, 3}},
		},
	} {
		data := Combinations(test.n, test.k)
		if !intSosMatch(data, test.data) {
			t.Errorf("Cas %v: Generated combinations mismatch. Got %v, want %v.", cas, data, test.data)
		}
	}
}

func TestCombinationGenerator(t *testing.T) {
	for n := 0; n <= 10; n++ {
		for k := 1; k <= n; k++ {
			combinations := Combinations(n, k)
			cg := NewCombinationGenerator(n, k)
			genCombs := make([][]int, 0, len(combinations))
			for cg.Next() {
				genCombs = append(genCombs, cg.Combination(nil))
			}
			if !intSosMatch(combinations, genCombs) {
				t.Errorf("Combinations and generated combinations do not match. n = %v, k = %v", n, k)
			}
		}
	}
}

func TestCartesian(t *testing.T) {
	// First, test with a known return.
	data := [][]float64{
		{1, 2},
		{3, 4},
		{5, 6},
	}
	want := mat.NewDense(8, 3, []float64{
		1, 3, 5,
		1, 3, 6,
		1, 4, 5,
		1, 4, 6,
		2, 3, 5,
		2, 3, 6,
		2, 4, 5,
		2, 4, 6,
	})
	got := Cartesian(nil, data)
	if !mat.Equal(want, got) {
		t.Errorf("cartesian data mismatch.\nwant:\n%v\ngot:\n%v", mat.Formatted(want), mat.Formatted(got))
	}
	gotTo := mat.NewDense(8, 3, nil)
	Cartesian(gotTo, data)
	if !mat.Equal(want, got) {
		t.Errorf("cartesian data mismatch with supplied.\nwant:\n%v\ngot:\n%v", mat.Formatted(want), mat.Formatted(gotTo))
	}

	// Test that Cartesian generates unique vectors.
	for cas, data := range [][][]float64{
		{{1}, {2, 3}, {8, 9, 10}},
		{{1, 10}, {2, 3}, {8, 9, 10}},
		{{1, 10, 11}, {2, 3}, {8}},
	} {
		cart := Cartesian(nil, data)
		r, c := cart.Dims()
		if c != len(data) {
			t.Errorf("Case %v: wrong number of columns. Want %v, got %v", cas, len(data), c)
		}
		wantRows := 1
		for _, v := range data {
			wantRows *= len(v)
		}
		if r != wantRows {
			t.Errorf("Case %v: wrong number of rows. Want %v, got %v", cas, wantRows, r)
		}
		for i := 0; i < r; i++ {
			for j := i + 1; j < r; j++ {
				if floats.Equal(cart.RawRowView(i), cart.RawRowView(j)) {
					t.Errorf("Cas %v: rows %d and %d are equal", cas, i, j)
				}
			}
		}

		cartTo := mat.NewDense(r, c, nil)
		Cartesian(cartTo, data)
		if !mat.Equal(cart, cartTo) {
			t.Errorf("cartesian data mismatch with supplied.\nwant:\n%v\ngot:\n%v", mat.Formatted(cart), mat.Formatted(cartTo))
		}
	}
}

func TestIdxSubFor(t *testing.T) {
	for cas, dims := range [][]int{
		{2, 2},
		{3, 1, 6},
		{2, 4, 6, 7},
	} {
		// Loop over all of the indexes. Confirm that the subscripts make sense
		// and that IdxFor is the converse of SubFor.
		maxIdx := 1
		for _, v := range dims {
			maxIdx *= v
		}
		into := make([]int, len(dims))
		for idx := 0; idx < maxIdx; idx++ {
			sub := SubFor(nil, idx, dims)
			for i := range sub {
				if sub[i] < 0 || sub[i] >= dims[i] {
					t.Errorf("cas %v: bad subscript. dims: %v, sub: %v", cas, dims, sub)
				}
			}
			SubFor(into, idx, dims)
			if !reflect.DeepEqual(sub, into) {
				t.Errorf("cas %v: subscript mismatch with supplied slice. Got %v, want %v", cas, into, sub)
			}
			idxOut := IdxFor(sub, dims)
			if idxOut != idx {
				t.Errorf("cas %v: returned index mismatch. Got %v, want %v", cas, idxOut, idx)
			}
		}
	}
}
