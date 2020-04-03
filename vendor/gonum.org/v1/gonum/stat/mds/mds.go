// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mds

import (
	"math"

	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/mat"
)

// TorgersonScaling converts a dissimilarity matrix to a matrix containing
// Euclidean coordinates. TorgersonScaling places the coordinates in dst and
// returns it and the number of positive Eigenvalues if successful.
// If the scaling is not successful, dst will be empty upon return.
// When the scaling is successful, dst will be resized to k columns wide.
// Eigenvalues will be copied into eigdst and returned as eig if it is provided.
//
// TorgersonScaling will panic if dst is not empty.
func TorgersonScaling(dst *mat.Dense, eigdst []float64, dis mat.Symmetric) (k int, eig []float64) {
	// https://doi.org/10.1007/0-387-28981-X_12

	n := dis.Symmetric()
	if dst.IsEmpty() {
		dst.ReuseAs(n, n)
	} else {
		panic("mds: receiver matrix not empty")
	}

	b := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			v := dis.At(i, j)
			v *= v
			b.SetSym(i, j, v)
		}
	}
	c := mat.NewSymDense(n, nil)
	s := -1 / float64(n)
	for i := 0; i < n; i++ {
		c.SetSym(i, i, 1+s)
		for j := i + 1; j < n; j++ {
			c.SetSym(i, j, s)
		}
	}
	dst.Product(c, b, c)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			b.SetSym(i, j, -0.5*dst.At(i, j))
		}
	}

	var ed mat.EigenSym
	ok := ed.Factorize(b, true)
	if !ok {
		return 0, eigdst
	}
	ed.VectorsTo(dst)
	vals := ed.Values(nil)
	reverse(vals, dst.RawMatrix())
	copy(eigdst, vals)

	for i, v := range vals {
		if v < 0 {
			vals[i] = 0
			continue
		}
		k = i + 1
		vals[i] = math.Sqrt(v)
	}

	var tmp mat.Dense
	tmp.Mul(dst, mat.NewDiagonalRect(n, k, vals[:k]))
	*dst = *dst.Slice(0, n, 0, k).(*mat.Dense)
	dst.Copy(&tmp)

	return k, eigdst
}

func reverse(values []float64, vectors blas64.General) {
	for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
		values[i], values[j] = values[j], values[i]
		blas64.Swap(blas64.Vector{N: vectors.Rows, Inc: vectors.Stride, Data: vectors.Data[i:]},
			blas64.Vector{N: vectors.Rows, Inc: vectors.Stride, Data: vectors.Data[j:]})
	}
}
