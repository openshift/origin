// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat_test

import (
	"fmt"

	"gonum.org/v1/gonum/mat"
)

func ExampleDense_Add() {
	// Initialize two matrices, a and b.
	a := mat.NewDense(2, 2, []float64{
		1, 0,
		1, 0,
	})
	b := mat.NewDense(2, 2, []float64{
		0, 1,
		0, 1,
	})

	// Add a and b, placing the result into c.
	// Notice that the size is automatically adjusted
	// when the receiver has zero size.
	var c mat.Dense
	c.Add(a, b)

	// Print the result using the formatter.
	fc := mat.Formatted(&c, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("c = %v", fc)

	// Output:
	//
	// c = ⎡1  1⎤
	//     ⎣1  1⎦
}

func ExampleDense_Sub() {
	// Initialize two matrices, a and b.
	a := mat.NewDense(2, 2, []float64{
		1, 1,
		1, 1,
	})
	b := mat.NewDense(2, 2, []float64{
		1, 0,
		0, 1,
	})

	// Subtract b from a, placing the result into a.
	a.Sub(a, b)

	// Print the result using the formatter.
	fa := mat.Formatted(a, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("a = %v", fa)

	// Output:
	//
	// a = ⎡0  1⎤
	//     ⎣1  0⎦
}

func ExampleDense_MulElem() {
	// Initialize two matrices, a and b.
	a := mat.NewDense(2, 2, []float64{
		1, 2,
		3, 4,
	})
	b := mat.NewDense(2, 2, []float64{
		1, 2,
		3, 4,
	})

	// Multiply the elements of a and b, placing the result into a.
	a.MulElem(a, b)

	// Print the result using the formatter.
	fa := mat.Formatted(a, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("a = %v", fa)

	// Output:
	//
	// a = ⎡1   4⎤
	//     ⎣9  16⎦
}

func ExampleDense_DivElem() {
	// Initialize two matrices, a and b.
	a := mat.NewDense(2, 2, []float64{
		5, 10,
		15, 20,
	})
	b := mat.NewDense(2, 2, []float64{
		5, 5,
		5, 5,
	})

	// Divide the elements of a by b, placing the result into a.
	a.DivElem(a, b)

	// Print the result using the formatter.
	fa := mat.Formatted(a, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("a = %v", fa)

	// Output:
	//
	// a = ⎡1  2⎤
	//     ⎣3  4⎦
}

func ExampleDense_Inverse() {
	// Initialize two matrices, a and ia.
	a := mat.NewDense(2, 2, []float64{
		4, 0,
		0, 4,
	})
	var ia mat.Dense

	// Take the inverse of a and place the result in ia.
	ia.Inverse(a)

	// Print the result using the formatter.
	fa := mat.Formatted(&ia, mat.Prefix("     "), mat.Squeeze())
	fmt.Printf("ia = %.2g\n\n", fa)

	// Confirm that A * A^-1 = I
	var r mat.Dense
	r.Mul(a, &ia)
	fr := mat.Formatted(&r, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("r = %v\n\n", fr)

	// The Inverse operation, however, is numerically unstable,
	// and should typically be avoided.
	// For example, a common need is to find x = A^-1 * b.
	// In this case, the SolveVec method of VecDense
	// (if b is a Vector) or Solve method of Dense (if b is a
	// matrix) should used instead of computing the Inverse of A.
	b := mat.NewDense(2, 2, []float64{
		2, 0,
		0, 2,
	})
	var x mat.Dense
	x.Solve(a, b)

	// Print the result using the formatter.
	fx := mat.Formatted(&x, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("x = %v", fx)

	// Output:
	//
	// ia = ⎡0.25    -0⎤
	//      ⎣   0  0.25⎦
	//
	// r = ⎡1  0⎤
	//     ⎣0  1⎦
	//
	// x = ⎡0.5    0⎤
	//     ⎣  0  0.5⎦
}

func ExampleDense_Mul() {
	// Initialize two matrices, a and b.
	a := mat.NewDense(2, 2, []float64{
		4, 0,
		0, 4,
	})
	b := mat.NewDense(2, 3, []float64{
		4, 0, 0,
		0, 0, 4,
	})

	// Take the matrix product of a and b and place the result in c.
	var c mat.Dense
	c.Mul(a, b)

	// Print the result using the formatter.
	fc := mat.Formatted(&c, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("c = %v", fc)

	// Output:
	//
	// c = ⎡16  0   0⎤
	//     ⎣ 0  0  16⎦
}

func ExampleDense_Exp() {
	// Initialize a matrix a with some data.
	a := mat.NewDense(2, 2, []float64{
		1, 0,
		0, 1,
	})

	// Take the exponential of the matrix and place the result in m.
	var m mat.Dense
	m.Exp(a)

	// Print the result using the formatter.
	fm := mat.Formatted(&m, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("m = %4.2f", fm)

	// Output:
	//
	// m = ⎡2.72  0.00⎤
	//     ⎣0.00  2.72⎦
}

func ExampleDense_Pow() {
	// Initialize a matrix with some data.
	a := mat.NewDense(2, 2, []float64{
		4, 4,
		4, 4,
	})

	// Take the second power of matrix a and place the result in m.
	var m mat.Dense
	m.Pow(a, 2)

	// Print the result using the formatter.
	fm := mat.Formatted(&m, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("m = %v\n\n", fm)

	// Take the zeroth power of matrix a and place the result in n.
	// We expect an identity matrix of the same size as matrix a.
	var n mat.Dense
	n.Pow(a, 0)

	// Print the result using the formatter.
	fn := mat.Formatted(&n, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("n = %v", fn)

	// Output:
	//
	// m = ⎡32  32⎤
	//     ⎣32  32⎦
	//
	// n = ⎡1  0⎤
	//     ⎣0  1⎦
}

func ExampleDense_Scale() {
	// Initialize a matrix with some data.
	a := mat.NewDense(2, 2, []float64{
		4, 4,
		4, 4,
	})

	// Scale the matrix by a factor of 0.25 and place the result in m.
	var m mat.Dense
	m.Scale(0.25, a)

	// Print the result using the formatter.
	fm := mat.Formatted(&m, mat.Prefix("    "), mat.Squeeze())
	fmt.Printf("m = %4.3f", fm)

	// Output:
	//
	// m = ⎡1.000  1.000⎤
	//     ⎣1.000  1.000⎦
}
