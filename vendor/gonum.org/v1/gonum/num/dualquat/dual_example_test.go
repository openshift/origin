// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualquat_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/dualquat"
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

// raiseDual raises the dimensionality of a point to a dual quaternion.
func raiseDual(p point) dualquat.Number {
	return dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: raise(p),
	}
}

// transform performs the transformation of p by the given dual quaternions.
// The transformations are normalized to unit vectors.
func transform(p point, by ...dualquat.Number) point {
	if len(by) == 0 {
		return p
	}

	// Ensure the modulus of by is correctly scaled.
	for i := range by {
		if len := quat.Abs(by[i].Real); len != 1 {
			by[i].Real = quat.Scale(1/len, by[i].Real)
		}
	}

	// Perform the transformations.
	q := by[0]
	for _, o := range by[1:] {
		q = dualquat.Mul(o, q)
	}
	pp := dualquat.Mul(dualquat.Mul(q, raiseDual(p)), dualquat.Conj(q))

	// Extract the point.
	return point{x: pp.Dual.Imag, y: pp.Dual.Jmag, z: pp.Dual.Kmag}
}

func Example() {
	// Translate a 1×1×1 cube by [3, 4, 5] and rotate it 120° around the
	// diagonal vector [1, 1, 1].
	fmt.Println("cube:")

	// Construct a displacement.
	displace := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Scale(0.5, raise(point{3, 4, 5})),
	}

	// Construct a rotations.
	alpha := 2 * math.Pi / 3
	axis := raise(point{1, 1, 1})
	rotate := dualquat.Number{Real: axis}
	rotate.Real = quat.Scale(math.Sin(alpha/2)/quat.Abs(rotate.Real), rotate.Real)
	rotate.Real.Real += math.Cos(alpha / 2)

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
		pp := transform(p,
			displace, rotate,
		)

		// Clean up floating point error for clarity.
		pp.x = floats.Round(pp.x, 2)
		pp.y = floats.Round(pp.y, 2)
		pp.z = floats.Round(pp.z, 2)

		fmt.Printf(" %d %+v -> %+v\n", i, p, pp)
	}

	// Rotate a line segment from {[2, 1, 1], [2, 1, 2]} 120° around
	// the diagonal vector [1, 1, 1] at its lower end.
	fmt.Println("\nline segment:")

	// Construct an displacement to the origin from the lower end...
	origin := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Scale(0.5, raise(point{-2, -1, -1})),
	}
	// ... and back from the origin to the lower end.
	replace := dualquat.Number{
		Real: quat.Number{Real: 1},
		Dual: quat.Scale(-1, origin.Dual),
	}

	for i, p := range []point{
		{x: 2, y: 1, z: 1},
		{x: 2, y: 1, z: 2},
	} {
		pp := transform(p,
			origin,  // Displace to origin.
			rotate,  // Rotate around axis.
			replace, // Displace back to original location.
		)

		// Clean up floating point error for clarity.
		pp.x = floats.Round(pp.x, 2)
		pp.y = floats.Round(pp.y, 2)
		pp.z = floats.Round(pp.z, 2)

		fmt.Printf(" %d %+v -> %+v\n", i, p, pp)
	}

	// Output:
	//
	// cube:
	//  0 {x:0 y:0 z:0} -> {x:5 y:3 z:4}
	//  1 {x:0 y:0 z:1} -> {x:6 y:3 z:4}
	//  2 {x:0 y:1 z:0} -> {x:5 y:3 z:5}
	//  3 {x:0 y:1 z:1} -> {x:6 y:3 z:5}
	//  4 {x:1 y:0 z:0} -> {x:5 y:4 z:4}
	//  5 {x:1 y:0 z:1} -> {x:6 y:4 z:4}
	//  6 {x:1 y:1 z:0} -> {x:5 y:4 z:5}
	//  7 {x:1 y:1 z:1} -> {x:6 y:4 z:5}
	//
	// line segment:
	//  0 {x:2 y:1 z:1} -> {x:2 y:1 z:1}
	//  1 {x:2 y:1 z:2} -> {x:3 y:1 z:1}
}
