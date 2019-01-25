// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spatial_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/spatial"
)

// Euclid is a mat.Matrix whose elements refects the Euclidean
// distance between a series of unit-separated points strided
// to be arranged in an x by y grid.
type Euclid struct{ x, y int }

func (e Euclid) Dims() (r, c int) { return e.x * e.y, e.x * e.y }
func (e Euclid) At(i, j int) float64 {
	d := e.x * e.y
	if i < 0 || d <= i || j < 0 || d <= j {
		panic("bounds error")
	}
	if i == j {
		return 0
	}
	x := float64(j%e.x - i%e.x)
	y := float64(j/e.x - i/e.x)
	return 1 / math.Hypot(x, y)
}
func (e Euclid) T() mat.Matrix { return mat.Transpose{e} }

func ExampleGlobalMoransI_areal() {
	locality := Euclid{10, 10}

	data1 := []float64{
		1, 0, 0, 1, 0, 0, 1, 0, 0, 0,
		0, 1, 1, 0, 0, 1, 0, 0, 0, 0,
		1, 0, 0, 1, 0, 0, 0, 0, 1, 0,
		0, 0, 1, 0, 1, 0, 1, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0, 1, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 0, 0, 1, 0, 1, 0,
		1, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 1, 0, 1, 0, 0, 0,
		1, 0, 0, 0, 0, 0, 0, 0, 1, 0,
	}
	i1, _, z1 := spatial.GlobalMoransI(data1, nil, locality)

	data2 := []float64{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 1, 0, 0, 0, 0, 0,
		0, 0, 1, 1, 1, 1, 0, 0, 0, 0,
		0, 0, 1, 1, 1, 1, 0, 0, 0, 0,
		0, 0, 0, 1, 1, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 0, 0, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 1, 1, 1,
	}
	i2, _, z2 := spatial.GlobalMoransI(data2, nil, locality)

	fmt.Printf("%v scattered points Moran's I=%.4v z-score=%.4v\n", floats.Sum(data1), i1, z1)
	fmt.Printf("%v clustered points Moran's I=%.4v z-score=%.4v\n", floats.Sum(data2), i2, z2)

	// Output:
	//
	// 24 scattered points Moran's I=-0.02999 z-score=-1.913
	// 24 clustered points Moran's I=0.09922 z-score=10.52
}
