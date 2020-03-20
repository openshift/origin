// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

type Dlassqer interface {
	Dlassq(n int, x []float64, incx int, scale, ssq float64) (float64, float64)
}

func DlassqTest(t *testing.T, impl Dlassqer) {
	const tol = 1e-14

	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 2, 3, 4, 5, 10} {
		for _, incx := range []int{1, 3} {
			name := fmt.Sprintf("n=%v,incx=%v", n, incx)

			// Allocate a slice of minimum length and fill it with
			// random numbers.
			x := make([]float64, max(0, 1+(n-1)*incx))
			for i := range x {
				x[i] = rnd.Float64()
			}

			// Fill the referenced elements of x and compute the
			// expected result in a non-sophisticated way.
			scale := rnd.Float64()
			ssq := rnd.Float64()
			want := scale * scale * ssq
			for i := 0; i < n; i++ {
				xi := rnd.NormFloat64()
				x[i*incx] = xi
				want += xi * xi
			}

			xCopy := make([]float64, len(x))
			copy(xCopy, x)

			// Update scale and ssq so that
			//  scale_out^2 * ssq_out = x[0]^2 + ... + x[n-1]^2 + scale_in^2*ssq_in
			scale, ssq = impl.Dlassq(n, x, incx, scale, ssq)
			if !floats.Equal(x, xCopy) {
				t.Fatalf("%v: unexpected modification of x", name)
			}

			// Check the result.
			got := scale * scale * ssq
			if math.Abs(got-want) >= tol {
				t.Errorf("%v: unexpected result; got %v, want %v", name, got, want)
			}
		}
	}
}
