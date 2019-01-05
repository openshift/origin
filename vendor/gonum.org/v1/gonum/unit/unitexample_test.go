// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit_test

import (
	"fmt"

	"gonum.org/v1/gonum/unit"
)

func ExampleNew() {
	// Create an acceleration of 3 m/s^2
	accel := unit.New(3.0, unit.Dimensions{unit.LengthDim: 1, unit.TimeDim: -2})
	fmt.Println(accel)

	// Output: 3 m s^-2
}

func ExampleNewDimension() {
	// Create a "trees" dimension
	// Typically, this should be used within an init function
	treeDim := unit.NewDimension("tree")
	countPerArea := unit.New(0.1, unit.Dimensions{treeDim: 1, unit.LengthDim: -2})
	fmt.Println(countPerArea)

	// Output: 0.1 tree m^-2
}
