// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualcmplx_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/num/dualcmplx"
)

// point is a 2-dimensional point/vector.
type point struct {
	x, y float64
}

// raise raises the dimensionality of a point to a complex.
func raise(p point) complex128 {
	return complex(p.x, p.y)
}

// raiseDual raises the dimensionality of a point to a dual complex number.
func raiseDual(p point) dualcmplx.Number {
	return dualcmplx.Number{
		Real: 1,
		Dual: complex(p.x, p.y),
	}
}

// transform performs the transformation of p by the given dual complex numbers.
// The transformations are normalized to unit vectors.
func transform(p point, by ...dualcmplx.Number) point {
	if len(by) == 0 {
		return p
	}

	// Ensure the modulus of by is correctly scaled.
	for i := range by {
		if len := dualcmplx.Abs(by[i]); len != 1 {
			by[i].Real *= complex(1/len, 0)
		}
	}

	// Perform the transformations.
	z := by[0]
	for _, o := range by[1:] {
		z = dualcmplx.Mul(o, z)
	}
	pp := dualcmplx.Mul(dualcmplx.Mul(z, raiseDual(p)), dualcmplx.Conj(z))

	// Extract the point.
	return point{x: real(pp.Dual), y: imag(pp.Dual)}
}

func Example() {
	// Translate a 1×1 square by [3, 4] and rotate it 90° around the
	// origin.
	fmt.Println("square:")

	// Construct a displacement.
	displace := dualcmplx.Number{
		Real: 1,
		Dual: 0.5 * raise(point{3, 4}),
	}

	// Construct a rotation.
	alpha := math.Pi / 2
	rotate := dualcmplx.Number{Real: complex(math.Cos(alpha/2), math.Sin(alpha/2))}

	for i, p := range []point{
		{x: 0, y: 0},
		{x: 0, y: 1},
		{x: 1, y: 0},
		{x: 1, y: 1},
	} {
		pp := transform(p,
			displace, rotate,
		)

		// Clean up floating point error for clarity.
		pp.x = floats.Round(pp.x, 2)
		pp.y = floats.Round(pp.y, 2)

		fmt.Printf(" %d %+v -> %+v\n", i, p, pp)
	}

	// Rotate a line segment 90° around its lower end [2, 2].
	fmt.Println("\nline segment:")

	// Construct a displacement to the origin from the lower end...
	origin := dualcmplx.Number{
		Real: 1,
		Dual: 0.5 * raise(point{-2, -2}),
	}
	// ... and back from the origin to the lower end.
	replace := dualcmplx.Number{
		Real: 1,
		Dual: -origin.Dual,
	}

	for i, p := range []point{
		{x: 2, y: 2},
		{x: 2, y: 3},
	} {
		pp := transform(p,
			origin,  // Displace to origin.
			rotate,  // Rotate around axis.
			replace, // Displace back to original location.
		)

		// Clean up floating point error for clarity.
		pp.x = floats.Round(pp.x, 2)
		pp.y = floats.Round(pp.y, 2)

		fmt.Printf(" %d %+v -> %+v\n", i, p, pp)
	}

	// Output:
	//
	// square:
	//  0 {x:0 y:0} -> {x:-4 y:3}
	//  1 {x:0 y:1} -> {x:-5 y:3}
	//  2 {x:1 y:0} -> {x:-4 y:4}
	//  3 {x:1 y:1} -> {x:-5 y:4}
	//
	// line segment:
	//  0 {x:2 y:2} -> {x:2 y:2}
	//  1 {x:2 y:3} -> {x:1 y:2}
}
