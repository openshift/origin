// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package spatial_test

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/spatial"
)

func ExampleGlobalMoransI_linear() {
	data := []float64{0, 0, 0, 1, 1, 1, 0, 1, 0, 0}

	// The locality here describes spatial neighbor
	// relationships.
	locality := mat.NewDense(10, 10, []float64{
		0, 1, 0, 0, 0, 0, 0, 0, 0, 0,
		1, 0, 1, 0, 0, 0, 0, 0, 0, 0,
		0, 1, 0, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 0, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 0, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 0, 1, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 0, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 0, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 1, 0, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 1, 0,
	})

	i, _, z := spatial.GlobalMoransI(data, nil, locality)

	fmt.Printf("Moran's I=%.4v z-score=%.4v\n", i, z)

	// Output:
	//
	// Moran's I=0.1111 z-score=0.6335
}

func ExampleGlobalMoransI_banded() {
	data := []float64{0, 0, 0, 1, 1, 1, 0, 1, 0, 0}

	// The locality here describes spatial neighbor
	// relationships.
	// This example uses the band matrix representation
	// to improve time and space efficiency.
	locality := mat.NewBandDense(10, 10, 1, 1, []float64{
		0, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 1,
		1, 0, 0,
	})

	i, _, z := spatial.GlobalMoransI(data, nil, locality)

	fmt.Printf("Moran's I=%.4v z-score=%.4v\n", i, z)

	// Output:
	//
	// Moran's I=0.1111 z-score=0.6335
}

func ExampleGetisOrdGStar() {
	data := []float64{0, 0, 0, 1, 1, 1, 0, 1, 0, 0}

	// The locality here describes spatial neighbor
	// relationships including self.
	locality := mat.NewDense(10, 10, []float64{
		1, 1, 0, 0, 0, 0, 0, 0, 0, 0,
		1, 1, 1, 0, 0, 0, 0, 0, 0, 0,
		0, 1, 1, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 1, 1, 1, 0, 0, 0, 0, 0,
		0, 0, 0, 1, 1, 1, 0, 0, 0, 0,
		0, 0, 0, 0, 1, 1, 1, 0, 0, 0,
		0, 0, 0, 0, 0, 1, 1, 1, 0, 0,
		0, 0, 0, 0, 0, 0, 1, 1, 1, 0,
		0, 0, 0, 0, 0, 0, 0, 1, 1, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 1, 1,
	})

	for i, v := range data {
		fmt.Printf("v=%v G*i=% .4v\n", v, spatial.GetisOrdGStar(i, data, nil, locality))
	}

	// Output:
	//
	// v=0 G*i=-1.225
	// v=0 G*i=-1.604
	// v=0 G*i=-0.2673
	// v=1 G*i= 1.069
	// v=1 G*i= 2.405
	// v=1 G*i= 1.069
	// v=0 G*i= 1.069
	// v=1 G*i=-0.2673
	// v=0 G*i=-0.2673
	// v=0 G*i=-1.225
}

func ExampleGetisOrdGStar_banded() {
	data := []float64{0, 0, 0, 1, 1, 1, 0, 1, 0, 0}

	// The locality here describes spatial neighbor
	// relationships including self.
	// This example uses the band matrix representation
	// to improve time and space efficiency.
	locality := mat.NewBandDense(10, 10, 1, 1, []float64{
		0, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
		1, 1, 0,
	})

	for i, v := range data {
		fmt.Printf("v=%v G*i=% .4v\n", v, spatial.GetisOrdGStar(i, data, nil, locality))
	}

	// Output:
	//
	// v=0 G*i=-1.225
	// v=0 G*i=-1.604
	// v=0 G*i=-0.2673
	// v=1 G*i= 1.069
	// v=1 G*i= 2.405
	// v=1 G*i= 1.069
	// v=0 G*i= 1.069
	// v=1 G*i=-0.2673
	// v=0 G*i=-0.2673
	// v=0 G*i=-1.225
}
