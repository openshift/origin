// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dualquat_test

import (
	"fmt"

	"gonum.org/v1/gonum/num/dualquat"
	"gonum.org/v1/gonum/num/quat"
)

func Example_displace() {
	// Displace a point [3, 4, 5] by [4, 2, 6].
	// See http://www.euclideanspace.com/maths/algebra/realNormedAlgebra/other/dualQuaternion/index.htm

	// Point to be transformed in the dual imaginary vector.
	p := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 3, Jmag: 4, Kmag: 5}}

	// Displacement vector, half [4, 2, 6], in the dual imaginary vector.
	d := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 2, Jmag: 1, Kmag: 3}}

	fmt.Println(dualquat.Mul(dualquat.Mul(d, p), dualquat.Conj(d)).Dual)

	// Output:
	//
	// (0+7i+6j+11k)
}

func Example_rotate() {
	// Rotate a point [3, 4, 5] by 180° around the x axis.
	// See http://www.euclideanspace.com/maths/algebra/realNormedAlgebra/other/dualQuaternion/index.htm

	// Point to be transformed in the dual imaginary vector.
	p := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 3, Jmag: 4, Kmag: 5}}

	// Rotation in the real quaternion.
	r := dualquat.Number{Real: quat.Number{Real: 0, Imag: 1}}

	fmt.Println(dualquat.Mul(dualquat.Mul(r, p), dualquat.Conj(r)).Dual)

	// Output:
	//
	// (0+3i-4j-5k)
}

func Example_displaceAndRotate() {
	// Displace a point [3, 4, 5] by [4, 2, 6] and then rotate
	// by 180° around the x axis.
	// See http://www.euclideanspace.com/maths/algebra/realNormedAlgebra/other/dualQuaternion/index.htm

	// Point to be transformed in the dual imaginary vector.
	p := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 3, Jmag: 4, Kmag: 5}}

	// Displacement vector, half [4, 2, 6], in the dual imaginary vector.
	d := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 2, Jmag: 1, Kmag: 3}}

	// Rotation in the real quaternion.
	r := dualquat.Number{Real: quat.Number{Real: 0, Imag: 1}}

	// Combine the rotation and displacement so
	// the displacement is performed first.
	q := dualquat.Mul(r, d)

	fmt.Println(dualquat.Mul(dualquat.Mul(q, p), dualquat.Conj(q)).Dual)

	// Output:
	//
	// (0+7i-6j-11k)
}

func Example_rotateAndDisplace() {
	// Rotate a point [3, 4, 5] by 180° around the x axis and then
	// displace by [4, 2, 6]

	// Point to be transformed in the dual imaginary vector.
	p := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 3, Jmag: 4, Kmag: 5}}

	// Displacement vector, half [4, 2, 6], in the dual imaginary vector.
	d := dualquat.Number{Real: quat.Number{Real: 1}, Dual: quat.Number{Imag: 2, Jmag: 1, Kmag: 3}}

	// Rotation in the real quaternion.
	r := dualquat.Number{Real: quat.Number{Real: 0, Imag: 1}}

	// Combine the rotation and displacement so
	// the rotations is performed first.
	q := dualquat.Mul(d, r)

	fmt.Println(dualquat.Mul(dualquat.Mul(q, p), dualquat.Conj(q)).Dual)

	// Output:
	//
	// (0+7i-2j+1k)
}
