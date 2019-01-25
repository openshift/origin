// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file must be kept in sync with array_bound_checks.go.

// +build !bounds

package fftpack

// The types in array.go implement Fortran-like arrays for bootstrapping
// the implementation of the FFT functions translated from FFTPACK; they
// are column-major.

type twoArray struct {
	jStride int
	data    []float64
}

func newTwoArray(i, j int, data []float64) twoArray {
	if len(data) < i*j {
		panic("fourier: short data")
	}
	return twoArray{
		jStride: i,
		data:    data[:i*j],
	}
}

func (a twoArray) at(i, j int) float64 {
	return a.data[i+a.jStride*j]
}

func (a twoArray) set(i, j int, v float64) {
	a.data[i+a.jStride*j] = v
}

func (a twoArray) add(i, j int, v float64) {
	a.data[i+a.jStride*j] += v
}

type threeArray struct {
	jStride, kStride int
	data             []float64
}

func newThreeArray(i, j, k int, data []float64) threeArray {
	if len(data) < i*j*k {
		panic("fourier: short data")
	}
	return threeArray{
		jStride: i,
		kStride: i * j,
		data:    data[:i*j*k],
	}
}

func (a threeArray) at(i, j, k int) float64 {
	return a.data[i+a.jStride*j+a.kStride*k]
}

func (a threeArray) set(i, j, k int, v float64) {
	a.data[i+a.jStride*j+a.kStride*k] = v
}
