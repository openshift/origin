// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c64

// DotcUnitary is
//  for i, v := range x {
//  	sum += y[i] * conj(v)
//  }
//  return sum
func DotcUnitary(x, y []complex64) (sum complex64) {
	for i, v := range x {
		sum += y[i] * conj(v)
	}
	return sum
}

// DotcInc is
//  for i := 0; i < int(n); i++ {
//  	sum += y[iy] * conj(x[ix])
//  	ix += incX
//  	iy += incY
//  }
//  return sum
func DotcInc(x, y []complex64, n, incX, incY, ix, iy uintptr) (sum complex64) {
	for i := 0; i < int(n); i++ {
		sum += y[iy] * conj(x[ix])
		ix += incX
		iy += incY
	}
	return sum
}
