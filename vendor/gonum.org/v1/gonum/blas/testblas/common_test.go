// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"math"
	"math/cmplx"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/floats"
)

func TestFlattenBanded(t *testing.T) {
	for i, test := range []struct {
		dense     [][]float64
		ku        int
		kl        int
		condensed [][]float64
	}{
		{
			dense:     [][]float64{{3}},
			ku:        0,
			kl:        0,
			condensed: [][]float64{{3}},
		},
		{
			dense: [][]float64{
				{3, 4, 0},
			},
			ku: 1,
			kl: 0,
			condensed: [][]float64{
				{3, 4},
			},
		},
		{
			dense: [][]float64{
				{3, 4, 0, 0, 0},
			},
			ku: 1,
			kl: 0,
			condensed: [][]float64{
				{3, 4},
			},
		},
		{
			dense: [][]float64{
				{3, 4, 0},
				{0, 5, 8},
				{0, 0, 2},
				{0, 0, 0},
				{0, 0, 0},
			},
			ku: 1,
			kl: 0,
			condensed: [][]float64{
				{3, 4},
				{5, 8},
				{2, math.NaN()},
				{math.NaN(), math.NaN()},
				{math.NaN(), math.NaN()},
			},
		},
		{
			dense: [][]float64{
				{3, 4, 6},
				{0, 5, 8},
				{0, 0, 2},
				{0, 0, 0},
				{0, 0, 0},
			},
			ku: 2,
			kl: 0,
			condensed: [][]float64{
				{3, 4, 6},
				{5, 8, math.NaN()},
				{2, math.NaN(), math.NaN()},
				{math.NaN(), math.NaN(), math.NaN()},
				{math.NaN(), math.NaN(), math.NaN()},
			},
		},
		{
			dense: [][]float64{
				{3, 4, 6},
				{1, 5, 8},
				{0, 6, 2},
				{0, 0, 7},
				{0, 0, 0},
			},
			ku: 2,
			kl: 1,
			condensed: [][]float64{
				{math.NaN(), 3, 4, 6},
				{1, 5, 8, math.NaN()},
				{6, 2, math.NaN(), math.NaN()},
				{7, math.NaN(), math.NaN(), math.NaN()},
				{math.NaN(), math.NaN(), math.NaN(), math.NaN()},
			},
		},
		{
			dense: [][]float64{
				{1, 2, 0},
				{3, 4, 5},
				{6, 7, 8},
				{0, 9, 10},
				{0, 0, 11},
			},
			ku: 1,
			kl: 2,
			condensed: [][]float64{
				{math.NaN(), math.NaN(), 1, 2},
				{math.NaN(), 3, 4, 5},
				{6, 7, 8, math.NaN()},
				{9, 10, math.NaN(), math.NaN()},
				{11, math.NaN(), math.NaN(), math.NaN()},
			},
		},
		{
			dense: [][]float64{
				{1, 0, 0},
				{3, 4, 0},
				{6, 7, 8},
				{0, 9, 10},
				{0, 0, 11},
			},
			ku: 0,
			kl: 2,
			condensed: [][]float64{
				{math.NaN(), math.NaN(), 1},
				{math.NaN(), 3, 4},
				{6, 7, 8},
				{9, 10, math.NaN()},
				{11, math.NaN(), math.NaN()},
			},
		},
		{
			dense: [][]float64{
				{1, 0, 0, 0, 0},
				{3, 4, 0, 0, 0},
				{1, 3, 5, 0, 0},
			},
			ku: 0,
			kl: 2,
			condensed: [][]float64{
				{math.NaN(), math.NaN(), 1},
				{math.NaN(), 3, 4},
				{1, 3, 5},
			},
		},
	} {
		condensed := flattenBanded(test.dense, test.ku, test.kl)
		correct := flatten(test.condensed)
		if !floats.Same(condensed, correct) {
			t.Errorf("Case %v mismatch. Want %v, got %v.", i, correct, condensed)
		}
	}
}

func TestFlattenTriangular(t *testing.T) {
	for i, test := range []struct {
		a   [][]float64
		ans []float64
		ul  blas.Uplo
	}{
		{
			a: [][]float64{
				{1, 2, 3},
				{0, 4, 5},
				{0, 0, 6},
			},
			ul:  blas.Upper,
			ans: []float64{1, 2, 3, 4, 5, 6},
		},
		{
			a: [][]float64{
				{1, 0, 0},
				{2, 3, 0},
				{4, 5, 6},
			},
			ul:  blas.Lower,
			ans: []float64{1, 2, 3, 4, 5, 6},
		},
	} {
		a := flattenTriangular(test.a, test.ul)
		if !floats.Equal(a, test.ans) {
			t.Errorf("Case %v. Want %v, got %v.", i, test.ans, a)
		}
	}
}

func TestPackUnpackAsHermitian(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for _, uplo := range []blas.Uplo{blas.Upper, blas.Lower} {
		for _, n := range []int{1, 2, 5, 50} {
			for _, lda := range []int{max(1, n), n + 11} {
				a := makeZGeneral(nil, n, n, lda)
				for i := 0; i < n; i++ {
					for j := i; j < n; j++ {
						a[i*lda+j] = complex(rnd.NormFloat64(), rnd.NormFloat64())
						if i != j {
							a[j*lda+i] = cmplx.Conj(a[i*lda+j])
						}
					}
				}
				aCopy := make([]complex128, len(a))
				copy(aCopy, a)

				ap := zPack(uplo, n, a, lda)
				if !zsame(a, aCopy) {
					t.Errorf("Case uplo=%v,n=%v,lda=%v: zPack modified a", uplo, n, lda)
				}

				apCopy := make([]complex128, len(ap))
				copy(apCopy, ap)

				art := zUnpackAsHermitian(uplo, n, ap)
				if !zsame(ap, apCopy) {
					t.Errorf("Case uplo=%v,n=%v,lda=%v: zUnpackAsHermitian modified ap", uplo, n, lda)
				}

				// Copy the round-tripped A into a matrix with the same stride
				// as the original.
				got := makeZGeneral(nil, n, n, lda)
				for i := 0; i < n; i++ {
					copy(got[i*lda:i*lda+n], art[i*n:i*n+n])
				}
				if !zsame(got, a) {
					t.Errorf("Case uplo=%v,n=%v,lda=%v: zPack and zUnpackAsHermitian do not roundtrip", uplo, n, lda)
				}
			}
		}
	}
}
