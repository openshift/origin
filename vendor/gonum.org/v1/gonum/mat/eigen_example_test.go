// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat_test

import (
	"fmt"
	"log"

	"gonum.org/v1/gonum/mat"
)

func ExampleEigenSym() {
	a := mat.NewSymDense(2, []float64{
		7, 0.5,
		0.5, 1,
	})
	fmt.Printf("A = %v\n\n", mat.Formatted(a, mat.Prefix("    ")))

	var eigsym mat.EigenSym
	ok := eigsym.Factorize(a, true)
	if !ok {
		log.Fatal("Symmetric eigendecomposition failed")
	}
	fmt.Printf("Eigenvalues of A:\n%1.3f\n\n", eigsym.Values(nil))

	var ev mat.Dense
	eigsym.VectorsTo(&ev)
	fmt.Printf("Eigenvectors of A:\n%1.3f\n\n", mat.Formatted(&ev))

	// Output:
	// A = ⎡  7  0.5⎤
	//     ⎣0.5    1⎦
	//
	// Eigenvalues of A:
	// [0.959 7.041]
	//
	// Eigenvectors of A:
	// ⎡ 0.082  -0.997⎤
	// ⎣-0.997  -0.082⎦
	//
}

func ExampleEigen() {
	a := mat.NewDense(2, 2, []float64{
		1, -1,
		1, 1,
	})
	fmt.Printf("A = %v\n\n", mat.Formatted(a, mat.Prefix("    ")))

	var eig mat.Eigen
	ok := eig.Factorize(a, mat.EigenLeft)
	if !ok {
		log.Fatal("Eigendecomposition failed")
	}
	fmt.Printf("Eigenvalues of A:\n%v\n", eig.Values(nil))

	// Output:
	// A = ⎡ 1  -1⎤
	//     ⎣ 1   1⎦
	//
	// Eigenvalues of A:
	// [(1+1i) (1-1i)]
}
