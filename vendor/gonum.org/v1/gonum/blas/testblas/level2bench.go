// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testblas

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
)

func DgemvBenchmark(b *testing.B, impl Dgemver, tA blas.Transpose, m, n, incX, incY int) {
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX = n
		lenY = m
	} else {
		lenX = m
		lenY = n
	}
	xr := make([]float64, lenX)
	for i := range xr {
		xr[i] = rand.Float64()
	}
	x := makeIncremented(xr, incX, 0)
	yr := make([]float64, lenY)
	for i := range yr {
		yr[i] = rand.Float64()
	}
	y := makeIncremented(yr, incY, 0)
	a := make([]float64, m*n)
	for i := range a {
		a[i] = rand.Float64()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		impl.Dgemv(tA, m, n, 2, a, n, x, incX, 3, y, incY)
	}
}

func DgerBenchmark(b *testing.B, impl Dgerer, m, n, incX, incY int) {
	xr := make([]float64, m)
	for i := range xr {
		xr[i] = rand.Float64()
	}
	x := makeIncremented(xr, incX, 0)
	yr := make([]float64, n)
	for i := range yr {
		yr[i] = rand.Float64()
	}
	y := makeIncremented(yr, incY, 0)
	a := make([]float64, m*n)
	for i := range a {
		a[i] = rand.Float64()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		impl.Dger(m, n, 2, x, incX, y, incY, a, n)
	}
}

type Sgerer interface {
	Sger(m, n int, alpha float32, x []float32, incX int, y []float32, incY int, a []float32, lda int)
}

func SgerBenchmark(b *testing.B, blasser Sgerer, m, n, incX, incY int) {
	xr := make([]float32, m)
	for i := range xr {
		xr[i] = rand.Float32()
	}
	x := makeIncremented32(xr, incX, 0)
	yr := make([]float32, n)
	for i := range yr {
		yr[i] = rand.Float32()
	}
	y := makeIncremented32(yr, incY, 0)
	a := make([]float32, m*n)
	for i := range a {
		a[i] = rand.Float32()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blasser.Sger(m, n, 2, x, incX, y, incY, a, n)
	}
}
