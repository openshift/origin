// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

// ConstFunc is a constant function returning the value held by the type.
type ConstFunc float64

func (c ConstFunc) Func(x []float64) float64 {
	return float64(c)
}

func (c ConstFunc) Grad(grad, x []float64) {
	for i := range grad {
		grad[i] = 0
	}
}

func (c ConstFunc) Hess(dst mat.MutableSymmetric, x []float64) {
	n := len(x)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			dst.SetSym(i, j, 0)
		}
	}
}

// LinearFunc is a linear function returning w*x+c.
type LinearFunc struct {
	w []float64
	c float64
}

func (l LinearFunc) Func(x []float64) float64 {
	return floats.Dot(l.w, x) + l.c
}

func (l LinearFunc) Grad(grad, x []float64) {
	copy(grad, l.w)
}

func (l LinearFunc) Hess(dst mat.MutableSymmetric, x []float64) {
	n := len(x)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			dst.SetSym(i, j, 0)
		}
	}
}

// QuadFunc is a quadratic function returning 0.5*x'*a*x + b*x + c.
type QuadFunc struct {
	a *mat.SymDense
	b *mat.VecDense
	c float64
}

func (q QuadFunc) Func(x []float64) float64 {
	v := mat.NewVecDense(len(x), x)
	var tmp mat.VecDense
	tmp.MulVec(q.a, v)
	return 0.5*mat.Dot(&tmp, v) + mat.Dot(q.b, v) + q.c
}

func (q QuadFunc) Grad(grad, x []float64) {
	var tmp mat.VecDense
	v := mat.NewVecDense(len(x), x)
	tmp.MulVec(q.a, v)
	for i := range grad {
		grad[i] = tmp.At(i, 0) + q.b.At(i, 0)
	}
}

func (q QuadFunc) Hess(dst mat.MutableSymmetric, x []float64) {
	n := len(x)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			dst.SetSym(i, j, q.a.At(i, j))
		}
	}
}
