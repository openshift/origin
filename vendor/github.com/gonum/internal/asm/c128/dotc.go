// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c128

import "math/cmplx"

// DotcUnitary is
//  for i, v := range x {
//  	sum += y[i] * cmplx.Conj(v)
//  }
//  return sum
func DotcUnitary(x, y []complex128) (sum complex128) {
	for i, v := range x {
		sum += y[i] * cmplx.Conj(v)
	}
	return sum
}

// DotcInc is
//  for i := 0; i < int(n); i++ {
//  	sum += y[iy] * cmplx.Conj(x[ix])
//  	ix += incX
//  	iy += incY
//  }
//  return sum
func DotcInc(x, y []complex128, n, incX, incY, ix, iy uintptr) (sum complex128) {
	for i := 0; i < int(n); i++ {
		sum += y[iy] * cmplx.Conj(x[ix])
		ix += incX
		iy += incY
	}
	return sum
}
