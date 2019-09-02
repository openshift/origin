// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/quat"
)

// point is a 3-dimensional point/vector.
type point struct {
	x, y, z float64
}

// raise raises the dimensionality of a point to a quaternion.
func raise(p point) quat.Number {
	return quat.Number{Imag: p.x, Jmag: p.y, Kmag: p.z}
}

// rotate performs the quaternion rotation of p by the given quaternion
// and scaling by the scale factor.
func rotate(p point, by quat.Number, scale float64) point {
	// Ensure the modulus of by is correctly scaled.
	if len := quat.Abs(by); len != scale {
		by = quat.Scale(math.Sqrt(scale)/len, by)
	}

	// Perform the rotation/scaling.
	pp := quat.Mul(quat.Mul(by, raise(p)), quat.Conj(by))

	// Extract the point.
	return point{x: pp.Imag, y: pp.Jmag, z: pp.Kmag}
}

// Rotate a cube 120° around the diagonal vector [1, 1, 1].
func Example_rotate() {
	alpha := 2 * math.Pi / 3
	q := raise(point{1, 1, 1})
	scale := 1.0

	q = quat.Scale(math.Sin(alpha/2)/quat.Abs(q), q)
	q.Real += math.Cos(alpha / 2)

	for i, p := range []point{
		{x: 0, y: 0, z: 0},
		{x: 0, y: 0, z: 1},
		{x: 0, y: 1, z: 0},
		{x: 0, y: 1, z: 1},
		{x: 1, y: 0, z: 0},
		{x: 1, y: 0, z: 1},
		{x: 1, y: 1, z: 0},
		{x: 1, y: 1, z: 1},
	} {
		pp := rotate(p, q, scale)

		// Clean up floating point error for clarity.
		pp.x = floats.Round(pp.x, 2)
		pp.y = floats.Round(pp.y, 2)
		pp.z = floats.Round(pp.z, 2)

		fmt.Printf("%d %+v -> %+v\n", i, p, pp)
	}

	// Output:
	//
	// 0 {x:0 y:0 z:0} -> {x:0 y:0 z:0}
	// 1 {x:0 y:0 z:1} -> {x:1 y:0 z:0}
	// 2 {x:0 y:1 z:0} -> {x:0 y:0 z:1}
	// 3 {x:0 y:1 z:1} -> {x:1 y:0 z:1}
	// 4 {x:1 y:0 z:0} -> {x:0 y:1 z:0}
	// 5 {x:1 y:0 z:1} -> {x:1 y:1 z:0}
	// 6 {x:1 y:1 z:0} -> {x:0 y:1 z:1}
	// 7 {x:1 y:1 z:1} -> {x:1 y:1 z:1}
}
