// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c64

// ScalUnitary is
//  for i := range x {
//  	x[i] *= alpha
//  }
func ScalUnitary(alpha complex64, x []complex64) {
	for i := range x {
		x[i] *= alpha
	}
}

// ScalUnitaryTo is
//  for i, v := range x {
//  	dst[i] = alpha * v
//  }
func ScalUnitaryTo(dst []complex64, alpha complex64, x []complex64) {
	for i, v := range x {
		dst[i] = alpha * v
	}
}

// ScalInc is
//  var ix uintptr
//  for i := 0; i < int(n); i++ {
//  	x[ix] *= alpha
//  	ix += incX
//  }
func ScalInc(alpha complex64, x []complex64, n, incX uintptr) {
	var ix uintptr
	for i := 0; i < int(n); i++ {
		x[ix] *= alpha
		ix += incX
	}
}

// ScalIncTo is
//  var idst, ix uintptr
//  for i := 0; i < int(n); i++ {
//  	dst[idst] = alpha * x[ix]
//  	ix += incX
//  	idst += incDst
//  }
func ScalIncTo(dst []complex64, incDst uintptr, alpha complex64, x []complex64, n, incX uintptr) {
	var idst, ix uintptr
	for i := 0; i < int(n); i++ {
		dst[idst] = alpha * x[ix]
		ix += incX
		idst += incDst
	}
}
