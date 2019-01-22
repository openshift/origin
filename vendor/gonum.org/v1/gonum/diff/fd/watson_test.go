// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import "gonum.org/v1/gonum/mat"

// Watson implements the Watson's function.
// Dimension of the problem should be 2 <= dim <= 31. For dim == 9, the problem
// of minimizing the function is very ill conditioned.
//
// This is copied from gonum.org/v1/optimize/functions for testing Hessian-like
// derivative methods.
//
// References:
//  - Kowalik, J.S., Osborne, M.R.: Methods for Unconstrained Optimization
//    Problems. Elsevier North-Holland, New York, 1968
//  - More, J., Garbow, B.S., Hillstrom, K.E.: Testing unconstrained
//    optimization software. ACM Trans Math Softw 7 (1981), 17-41
type Watson struct{}

func (Watson) Func(x []float64) (sum float64) {
	for i := 1; i <= 29; i++ {
		d1 := float64(i) / 29

		d2 := 1.0
		var s1 float64
		for j := 1; j < len(x); j++ {
			s1 += float64(j) * d2 * x[j]
			d2 *= d1
		}

		d2 = 1.0
		var s2 float64
		for _, v := range x {
			s2 += d2 * v
			d2 *= d1
		}

		t := s1 - s2*s2 - 1
		sum += t * t
	}
	t := x[1] - x[0]*x[0] - 1
	sum += x[0]*x[0] + t*t
	return sum
}

func (Watson) Grad(grad, x []float64) {
	if len(x) != len(grad) {
		panic("incorrect size of the gradient")
	}

	for i := range grad {
		grad[i] = 0
	}
	for i := 1; i <= 29; i++ {
		d1 := float64(i) / 29

		d2 := 1.0
		var s1 float64
		for j := 1; j < len(x); j++ {
			s1 += float64(j) * d2 * x[j]
			d2 *= d1
		}

		d2 = 1.0
		var s2 float64
		for _, v := range x {
			s2 += d2 * v
			d2 *= d1
		}

		t := s1 - s2*s2 - 1
		s3 := 2 * d1 * s2
		d2 = 2 / d1
		for j := range x {
			grad[j] += d2 * (float64(j) - s3) * t
			d2 *= d1
		}
	}
	t := x[1] - x[0]*x[0] - 1
	grad[0] += x[0] * (2 - 4*t)
	grad[1] += 2 * t
}

func (Watson) Hess(hess mat.MutableSymmetric, x []float64) {
	dim := len(x)
	if dim != hess.Symmetric() {
		panic("incorrect size of the Hessian")
	}

	for j := 0; j < dim; j++ {
		for k := j; k < dim; k++ {
			hess.SetSym(j, k, 0)
		}
	}
	for i := 1; i <= 29; i++ {
		d1 := float64(i) / 29
		d2 := 1.0
		var s1 float64
		for j := 1; j < dim; j++ {
			s1 += float64(j) * d2 * x[j]
			d2 *= d1
		}

		d2 = 1.0
		var s2 float64
		for _, v := range x {
			s2 += d2 * v
			d2 *= d1
		}

		t := s1 - s2*s2 - 1
		s3 := 2 * d1 * s2
		d2 = 2 / d1
		th := 2 * d1 * d1 * t
		for j := 0; j < dim; j++ {
			v := float64(j) - s3
			d3 := 1 / d1
			for k := 0; k <= j; k++ {
				hess.SetSym(k, j, hess.At(k, j)+d2*d3*(v*(float64(k)-s3)-th))
				d3 *= d1
			}
			d2 *= d1
		}
	}
	t1 := x[1] - x[0]*x[0] - 1
	hess.SetSym(0, 0, hess.At(0, 0)+8*x[0]*x[0]+2-4*t1)
	hess.SetSym(0, 1, hess.At(0, 1)-4*x[0])
	hess.SetSym(1, 1, hess.At(1, 1)+2)
}
