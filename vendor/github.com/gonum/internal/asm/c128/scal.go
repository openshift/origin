// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package c128

// ScalUnitary is
//  for i := range x {
//  	x[i] *= alpha
//  }
func ScalUnitary(alpha complex128, x []complex128) {
	for i := range x {
		x[i] *= alpha
	}
}

// ScalUnitaryTo is
//  for i, v := range x {
//  	dst[i] = alpha * v
//  }
func ScalUnitaryTo(dst []complex128, alpha complex128, x []complex128) {
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
func ScalInc(alpha complex128, x []complex128, n, incX uintptr) {
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
func ScalIncTo(dst []complex128, incDst uintptr, alpha complex128, x []complex128, n, incX uintptr) {
	var idst, ix uintptr
	for i := 0; i < int(n); i++ {
		dst[idst] = alpha * x[ix]
		ix += incX
		idst += incDst
	}
}
