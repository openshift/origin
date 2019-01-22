// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dznrm2er interface {
	Dznrm2(n int, x []complex128, incX int) float64
	Dnrm2er
}

func Dznrm2Test(t *testing.T, impl Dznrm2er) {
	tol := 1e-12
	for tc, test := range []struct {
		x    []complex128
		want float64
	}{
		{
			x:    nil,
			want: 0,
		},
		{
			x:    []complex128{1 + 2i},
			want: 2.2360679774998,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i},
			want: 5.4772255750517,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i},
			want: 9.5393920141695,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i},
			want: 1.4282856857086e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i},
			want: 1.9621416870349e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i},
			want: 2.5495097567964e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i},
			want: 3.1859064644148e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i},
			want: 3.8678159211627e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i},
			want: 4.5923850012820e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i},
			want: 5.3572380943915e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i},
			want: 6.1603571325046e+01,
		},
		{
			x:    []complex128{1 + 2i, 3 + 4i, 5 + 6i, 7 + 8i, 9 + 10i, 11 + 12i, 13 + 14i, 15 + 16i, 17 + 18i, 19 + 20i, 21 + 22i, 23 + 24i},
			want: 70,
		},
	} {
		n := len(test.x)
		for _, incX := range []int{-10, -1, 1, 2, 9, 17} {
			x := makeZVector(test.x, incX)
			xCopy := make([]complex128, len(x))
			copy(xCopy, x)

			got := impl.Dznrm2(n, x, incX)

			prefix := fmt.Sprintf("Case %v (n=%v,incX=%v):", tc, n, incX)

			if !zsame(x, xCopy) {
				t.Errorf("%v: unexpected modification of x", prefix)
			}

			if incX < 0 {
				if got != 0 {
					t.Errorf("%v: non-zero result when incX < 0. got %v", prefix, got)
				}
				continue
			}

			if !floats.EqualWithinAbsOrRel(test.want, got, tol, tol) {
				t.Errorf("%v: unexpected result. want %v, got %v", prefix, test.want, got)
			}
		}
	}

	tol = 1e-14
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{10, 50, 100} {
		for _, incX := range []int{1, 2, 10} {
			re := make([]float64, n)
			for i := range re {
				re[i] = rnd.NormFloat64()
			}
			im := make([]float64, n)
			for i := range im {
				im[i] = rnd.NormFloat64()
			}
			want := math.Hypot(impl.Dnrm2(n, re, 1), impl.Dnrm2(n, im, 1))

			x := make([]complex128, (n-1)*incX+1)
			for i := range x {
				x[i] = znan
			}
			for i := range re {
				x[i*incX] = complex(re[i], im[i])
			}

			got := impl.Dznrm2(n, x, incX)

			if !floats.EqualWithinAbsOrRel(want, got, tol, tol) {
				t.Errorf("Case n=%v,incX=%v: unexpected result using Dnrm2. want %v, got %v", n, incX, want, got)
			}
		}
	}
}
