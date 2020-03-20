// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualcmplx_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/num/dualcmplx"
)

// Example point, displacement and rotation from Euclidean Space Dual Complex Number page:
// http://www.euclideanspace.com/maths/algebra/realNormedAlgebra/other/dualComplex/index.htm

func Example_displace() {
	// Displace a point [3, 4] by [4, 3].

	// Point to be transformed in the dual imaginary vector.
	p := dualcmplx.Number{Real: 1, Dual: 3 + 4i}

	// Displacement vector, half [4, 3], in the dual imaginary vector.
	d := dualcmplx.Number{Real: 1, Dual: 2 + 1.5i}

	fmt.Printf("%.0f\n", dualcmplx.Mul(dualcmplx.Mul(d, p), dualcmplx.Conj(d)).Dual)

	// Output:
	//
	// (7+7i)
}

func Example_rotate() {
	// Rotate a point [3, 4] by 90° around the origin.

	// Point to be transformed in the dual imaginary vector.
	p := dualcmplx.Number{Real: 1, Dual: 3 + 4i}

	// Half the rotation in the real complex number.
	r := dualcmplx.Number{Real: complex(math.Cos(math.Pi/4), math.Sin(math.Pi/4))}

	fmt.Printf("%.0f\n", dualcmplx.Mul(dualcmplx.Mul(r, p), dualcmplx.Conj(r)).Dual)

	// Output:
	//
	// (-4+3i)
}

func Example_displaceAndRotate() {
	// Displace a point [3, 4] by [4, 3] and then rotate
	// by 90° around the origin.

	// Point to be transformed in the dual imaginary vector.
	p := dualcmplx.Number{Real: 1, Dual: 3 + 4i}

	// Displacement vector, half [4, 3], in the dual imaginary vector.
	d := dualcmplx.Number{Real: 1, Dual: 2 + 1.5i}

	// Rotation in the real complex number.
	r := dualcmplx.Number{Real: complex(math.Cos(math.Pi/4), math.Sin(math.Pi/4))}

	// Combine the rotation and displacement so
	// the displacement is performed first.
	q := dualcmplx.Mul(r, d)

	fmt.Printf("%.0f\n", dualcmplx.Mul(dualcmplx.Mul(q, p), dualcmplx.Conj(q)).Dual)

	// Output:
	//
	// (-7+7i)
}

func Example_rotateAndDisplace() {
	// Rotate a point [3, 4] by 90° around the origin and then
	// displace by [4, 3].

	// Point to be transformed in the dual imaginary vector.
	p := dualcmplx.Number{Real: 1, Dual: 3 + 4i}

	// Displacement vector, half [4, 3], in the dual imaginary vector.
	d := dualcmplx.Number{Real: 1, Dual: 2 + 1.5i}

	// Rotation in the real complex number.
	r := dualcmplx.Number{Real: complex(math.Cos(math.Pi/4), math.Sin(math.Pi/4))}

	// Combine the rotation and displacement so
	// the displacement is performed first.
	q := dualcmplx.Mul(d, r)

	fmt.Printf("%.0f\n", dualcmplx.Mul(dualcmplx.Mul(q, p), dualcmplx.Conj(q)).Dual)

	// Output:
	//
	// (0+6i)
}
