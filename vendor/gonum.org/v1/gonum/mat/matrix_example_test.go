// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat_test

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

func ExampleCol() {
	// This example copies the second column of a matrix into col, allocating a new slice of float64.
	m := mat.NewDense(3, 3, []float64{
		2.0, 9.0, 3.0,
		4.5, 6.7, 8.0,
		1.2, 3.0, 6.0,
	})

	col := mat.Col(nil, 1, m)

	fmt.Printf("col = %#v", col)
	// Output:
	//
	// col = []float64{9, 6.7, 3}
}

func ExampleRow() {
	// This example copies the third row of a matrix into row, allocating a new slice of float64.
	m := mat.NewDense(3, 3, []float64{
		2.0, 9.0, 3.0,
		4.5, 6.7, 8.0,
		1.2, 3.0, 6.0,
	})

	row := mat.Row(nil, 2, m)

	fmt.Printf("row = %#v", row)
	// Output:
	//
	// row = []float64{1.2, 3, 6}
}
