// Do not manually edit this file. It was created by the generate_blas.go from cblas.h.

// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

/*
#cgo CFLAGS: -g -O2
#include "cblas.h"
*/
import "C"

import (
	"unsafe"

	"github.com/gonum/blas"
)

// Type check assertions:
var (
	_ blas.Float32    = Implementation{}
	_ blas.Float64    = Implementation{}
	_ blas.Complex64  = Implementation{}
	_ blas.Complex128 = Implementation{}
)

// Type order is used to specify the matrix storage format. We still interact with
// an API that allows client calls to specify order, so this is here to document that fact.
type order int

const (
	rowMajor order = 101 + iota
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type Implementation struct{}

// Special cases...

type srotmParams struct {
	flag float32
	h    [4]float32
}

type drotmParams struct {
	flag float64
	h    [4]float64
}

func (Implementation) Srotg(a float32, b float32) (c float32, s float32, r float32, z float32) {
	C.cblas_srotg((*C.float)(&a), (*C.float)(&b), (*C.float)(&c), (*C.float)(&s))
	return c, s, a, b
}
func (Implementation) Srotmg(d1 float32, d2 float32, b1 float32, b2 float32) (p blas.SrotmParams, rd1 float32, rd2 float32, rb1 float32) {
	var pi srotmParams
	C.cblas_srotmg((*C.float)(&d1), (*C.float)(&d2), (*C.float)(&b1), C.float(b2), (*C.float)(unsafe.Pointer(&pi)))
	return blas.SrotmParams{Flag: blas.Flag(pi.flag), H: pi.h}, d1, d2, b1
}
func (Implementation) Srotm(n int, x []float32, incX int, y []float32, incY int, p blas.SrotmParams) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if p.Flag < blas.Identity || p.Flag > blas.Diagonal {
		panic("blas: illegal blas.Flag value")
	}
	if n == 0 {
		return
	}
	pi := srotmParams{
		flag: float32(p.Flag),
		h:    p.H,
	}
	C.cblas_srotm(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY), (*C.float)(unsafe.Pointer(&pi)))
}
func (Implementation) Drotg(a float64, b float64) (c float64, s float64, r float64, z float64) {
	C.cblas_drotg((*C.double)(&a), (*C.double)(&b), (*C.double)(&c), (*C.double)(&s))
	return c, s, a, b
}
func (Implementation) Drotmg(d1 float64, d2 float64, b1 float64, b2 float64) (p blas.DrotmParams, rd1 float64, rd2 float64, rb1 float64) {
	var pi drotmParams
	C.cblas_drotmg((*C.double)(&d1), (*C.double)(&d2), (*C.double)(&b1), C.double(b2), (*C.double)(unsafe.Pointer(&pi)))
	return blas.DrotmParams{Flag: blas.Flag(pi.flag), H: pi.h}, d1, d2, b1
}
func (Implementation) Drotm(n int, x []float64, incX int, y []float64, incY int, p blas.DrotmParams) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if p.Flag < blas.Identity || p.Flag > blas.Diagonal {
		panic("blas: illegal blas.Flag value")
	}
	if n == 0 {
		return
	}
	pi := drotmParams{
		flag: float64(p.Flag),
		h:    p.H,
	}
	C.cblas_drotm(C.int(n), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY), (*C.double)(unsafe.Pointer(&pi)))
}
func (Implementation) Cdotu(n int, x []complex64, incX int, y []complex64, incY int) (dotu complex64) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	C.cblas_cdotu_sub(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&dotu))
	return dotu
}
func (Implementation) Cdotc(n int, x []complex64, incX int, y []complex64, incY int) (dotc complex64) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	C.cblas_cdotc_sub(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&dotc))
	return dotc
}
func (Implementation) Zdotu(n int, x []complex128, incX int, y []complex128, incY int) (dotu complex128) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	C.cblas_zdotu_sub(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&dotu))
	return dotu
}
func (Implementation) Zdotc(n int, x []complex128, incX int, y []complex128, incY int) (dotc complex128) {
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	C.cblas_zdotc_sub(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&dotc))
	return dotc
}

// Generated cases ...

// Sdsdot computes the dot product of the two vectors plus a constant
//  alpha + \sum_i x[i]*y[i]
func (Implementation) Sdsdot(n int, alpha float32, x []float32, incX int, y []float32, incY int) float32 {
	// declared at cblas.h:24:8 float cblas_sdsdot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_sdsdot(C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY)))
}

// Dsdot computes the dot product of the two vectors
//  \sum_i x[i]*y[i]
func (Implementation) Dsdot(n int, x []float32, incX int, y []float32, incY int) float64 {
	// declared at cblas.h:26:8 double cblas_dsdot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_dsdot(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY)))
}

// Sdot computes the dot product of the two vectors
//  \sum_i x[i]*y[i]
func (Implementation) Sdot(n int, x []float32, incX int, y []float32, incY int) float32 {
	// declared at cblas.h:28:8 float cblas_sdot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_sdot(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY)))
}

// Ddot computes the dot product of the two vectors
//  \sum_i x[i]*y[i]
func (Implementation) Ddot(n int, x []float64, incX int, y []float64, incY int) float64 {
	// declared at cblas.h:30:8 double cblas_ddot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_ddot(C.int(n), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY)))
}

// Snrm2 computes the Euclidean norm of a vector,
//  sqrt(\sum_i x[i] * x[i]).
// This function returns 0 if incX is negative.
func (Implementation) Snrm2(n int, x []float32, incX int) float32 {
	// declared at cblas.h:49:8 float cblas_snrm2 ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_snrm2(C.int(n), (*C.float)(&x[0]), C.int(incX)))
}

// Sasum computes the sum of the absolute values of the elements of x.
//  \sum_i |x[i]|
// Sasum returns 0 if incX is negative.
func (Implementation) Sasum(n int, x []float32, incX int) float32 {
	// declared at cblas.h:50:8 float cblas_sasum ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_sasum(C.int(n), (*C.float)(&x[0]), C.int(incX)))
}

// Dnrm2 computes the Euclidean norm of a vector,
//  sqrt(\sum_i x[i] * x[i]).
// This function returns 0 if incX is negative.
func (Implementation) Dnrm2(n int, x []float64, incX int) float64 {
	// declared at cblas.h:52:8 double cblas_dnrm2 ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_dnrm2(C.int(n), (*C.double)(&x[0]), C.int(incX)))
}

// Dasum computes the sum of the absolute values of the elements of x.
//  \sum_i |x[i]|
// Dasum returns 0 if incX is negative.
func (Implementation) Dasum(n int, x []float64, incX int) float64 {
	// declared at cblas.h:53:8 double cblas_dasum ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_dasum(C.int(n), (*C.double)(&x[0]), C.int(incX)))
}

func (Implementation) Scnrm2(n int, x []complex64, incX int) float32 {
	// declared at cblas.h:55:8 float cblas_scnrm2 ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_scnrm2(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

func (Implementation) Scasum(n int, x []complex64, incX int) float32 {
	// declared at cblas.h:56:8 float cblas_scasum ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float32(C.cblas_scasum(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

func (Implementation) Dznrm2(n int, x []complex128, incX int) float64 {
	// declared at cblas.h:58:8 double cblas_dznrm2 ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_dznrm2(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

func (Implementation) Dzasum(n int, x []complex128, incX int) float64 {
	// declared at cblas.h:59:8 double cblas_dzasum ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return 0
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return 0
	}
	return float64(C.cblas_dzasum(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

// Isamax returns the index of an element of x with the largest absolute value.
// If there are multiple such indices the earliest is returned.
// Isamax returns -1 if n == 0.
func (Implementation) Isamax(n int, x []float32, incX int) int {
	// declared at cblas.h:65:13 int cblas_isamax ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n == 0 || incX < 0 {
		return -1
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return -1
	}
	return int(C.cblas_isamax(C.int(n), (*C.float)(&x[0]), C.int(incX)))
}

// Idamax returns the index of an element of x with the largest absolute value.
// If there are multiple such indices the earliest is returned.
// Idamax returns -1 if n == 0.
func (Implementation) Idamax(n int, x []float64, incX int) int {
	// declared at cblas.h:66:13 int cblas_idamax ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n == 0 || incX < 0 {
		return -1
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return -1
	}
	return int(C.cblas_idamax(C.int(n), (*C.double)(&x[0]), C.int(incX)))
}

func (Implementation) Icamax(n int, x []complex64, incX int) int {
	// declared at cblas.h:67:13 int cblas_icamax ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n == 0 || incX < 0 {
		return -1
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return -1
	}
	return int(C.cblas_icamax(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

func (Implementation) Izamax(n int, x []complex128, incX int) int {
	// declared at cblas.h:68:13 int cblas_izamax ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n == 0 || incX < 0 {
		return -1
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return -1
	}
	return int(C.cblas_izamax(C.int(n), unsafe.Pointer(&x[0]), C.int(incX)))
}

// Sswap exchanges the elements of two vectors.
//  x[i], y[i] = y[i], x[i] for all i
func (Implementation) Sswap(n int, x []float32, incX int, y []float32, incY int) {
	// declared at cblas.h:79:6 void cblas_sswap ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_sswap(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY))
}

// Scopy copies the elements of x into the elements of y.
//  y[i] = x[i] for all i
func (Implementation) Scopy(n int, x []float32, incX int, y []float32, incY int) {
	// declared at cblas.h:81:6 void cblas_scopy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_scopy(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY))
}

// Saxpy adds alpha times x to y
//  y[i] += alpha * x[i] for all i
func (Implementation) Saxpy(n int, alpha float32, x []float32, incX int, y []float32, incY int) {
	// declared at cblas.h:83:6 void cblas_saxpy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_saxpy(C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY))
}

// Dswap exchanges the elements of two vectors.
//  x[i], y[i] = y[i], x[i] for all i
func (Implementation) Dswap(n int, x []float64, incX int, y []float64, incY int) {
	// declared at cblas.h:90:6 void cblas_dswap ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dswap(C.int(n), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY))
}

// Dcopy copies the elements of x into the elements of y.
//  y[i] = x[i] for all i
func (Implementation) Dcopy(n int, x []float64, incX int, y []float64, incY int) {
	// declared at cblas.h:92:6 void cblas_dcopy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dcopy(C.int(n), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY))
}

// Daxpy adds alpha times x to y
//  y[i] += alpha * x[i] for all i
func (Implementation) Daxpy(n int, alpha float64, x []float64, incX int, y []float64, incY int) {
	// declared at cblas.h:94:6 void cblas_daxpy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_daxpy(C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY))
}

func (Implementation) Cswap(n int, x []complex64, incX int, y []complex64, incY int) {
	// declared at cblas.h:101:6 void cblas_cswap ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_cswap(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Ccopy(n int, x []complex64, incX int, y []complex64, incY int) {
	// declared at cblas.h:103:6 void cblas_ccopy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_ccopy(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Caxpy(n int, alpha complex64, x []complex64, incX int, y []complex64, incY int) {
	// declared at cblas.h:105:6 void cblas_caxpy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_caxpy(C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zswap(n int, x []complex128, incX int, y []complex128, incY int) {
	// declared at cblas.h:112:6 void cblas_zswap ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zswap(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zcopy(n int, x []complex128, incX int, y []complex128, incY int) {
	// declared at cblas.h:114:6 void cblas_zcopy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zcopy(C.int(n), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zaxpy(n int, alpha complex128, x []complex128, incX int, y []complex128, incY int) {
	// declared at cblas.h:116:6 void cblas_zaxpy ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zaxpy(C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY))
}

// Srot applies a plane transformation.
//  x[i] = c * x[i] + s * y[i]
//  y[i] = c * y[i] - s * x[i]
func (Implementation) Srot(n int, x []float32, incX int, y []float32, incY int, c, s float32) {
	// declared at cblas.h:129:6 void cblas_srot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_srot(C.int(n), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY), C.float(c), C.float(s))
}

// Drot applies a plane transformation.
//  x[i] = c * x[i] + s * y[i]
//  y[i] = c * y[i] - s * x[i]
func (Implementation) Drot(n int, x []float64, incX int, y []float64, incY int, c, s float64) {
	// declared at cblas.h:136:6 void cblas_drot ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_drot(C.int(n), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY), C.double(c), C.double(s))
}

// Sscal scales x by alpha.
//  x[i] *= alpha
// Sscal has no effect if incX < 0.
func (Implementation) Sscal(n int, alpha float32, x []float32, incX int) {
	// declared at cblas.h:145:6 void cblas_sscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_sscal(C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX))
}

// Dscal scales x by alpha.
//  x[i] *= alpha
// Dscal has no effect if incX < 0.
func (Implementation) Dscal(n int, alpha float64, x []float64, incX int) {
	// declared at cblas.h:146:6 void cblas_dscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dscal(C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX))
}

func (Implementation) Cscal(n int, alpha complex64, x []complex64, incX int) {
	// declared at cblas.h:147:6 void cblas_cscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_cscal(C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Zscal(n int, alpha complex128, x []complex128, incX int) {
	// declared at cblas.h:148:6 void cblas_zscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zscal(C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Csscal(n int, alpha float32, x []complex64, incX int) {
	// declared at cblas.h:149:6 void cblas_csscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incX < 0 {
		return
	}
	if incX > 0 && (n-1)*incX >= len(x) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_csscal(C.int(n), C.float(alpha), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Zdscal(n int, alpha float64, x []complex128, incX int) {
	// declared at cblas.h:150:6 void cblas_zdscal ...

	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zdscal(C.int(n), C.double(alpha), unsafe.Pointer(&x[0]), C.int(incX))
}

// Sgemv computes
//  y = alpha * a * x + beta * y if tA = blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA = blas.Trans or blas.ConjTrans
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Sgemv(tA blas.Transpose, m, n int, alpha float32, a []float32, lda int, x []float32, incX int, beta float32, y []float32, incY int) {
	// declared at cblas.h:171:6 void cblas_sgemv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_sgemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX), C.float(beta), (*C.float)(&y[0]), C.int(incY))
}

// Sgbmv computes
//  y = alpha * A * x + beta * y if tA == blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA == blas.Trans or blas.ConjTrans
// where a is an m×n band matrix kL subdiagonals and kU super-diagonals, and
// m and n refer to the size of the full dense matrix it represents.
// x and y are vectors, and alpha and beta are scalars.
func (Implementation) Sgbmv(tA blas.Transpose, m, n, kL, kU int, alpha float32, a []float32, lda int, x []float32, incX int, beta float32, y []float32, incY int) {
	// declared at cblas.h:176:6 void cblas_sgbmv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if kL < 0 {
		panic("blas: kL < 0")
	}
	if kU < 0 {
		panic("blas: kU < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+kL+kU+1 > len(a) || lda < kL+kU+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_sgbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.int(kL), C.int(kU), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX), C.float(beta), (*C.float)(&y[0]), C.int(incY))
}

// Strmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// A is an n×n Triangular matrix and x is a vector.
func (Implementation) Strmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float32, lda int, x []float32, incX int) {
	// declared at cblas.h:181:6 void cblas_strmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_strmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX))
}

// Stbmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// where A is an n×n triangular banded matrix with k diagonals, and x is a vector.
func (Implementation) Stbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float32, lda int, x []float32, incX int) {
	// declared at cblas.h:185:6 void cblas_stbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_stbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX))
}

// Stpmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// where A is an n×n unit triangular matrix in packed format, and x is a vector.
func (Implementation) Stpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []float32, incX int) {
	// declared at cblas.h:189:6 void cblas_stpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_stpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.float)(&ap[0]), (*C.float)(&x[0]), C.int(incX))
}

// Strsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans or blas.ConjTrans
// A is an n×n triangular matrix and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Strsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float32, lda int, x []float32, incX int) {
	// declared at cblas.h:192:6 void cblas_strsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_strsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX))
}

// Stbsv solves
//  A * x = b
// where A is an n×n triangular banded matrix with k diagonals in packed format,
// and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Stbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float32, lda int, x []float32, incX int) {
	// declared at cblas.h:196:6 void cblas_stbsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_stbsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX))
}

// Stpsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans or blas.ConjTrans
// where A is an n×n triangular matrix in packed format and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Stpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []float32, incX int) {
	// declared at cblas.h:200:6 void cblas_stpsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_stpsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.float)(&ap[0]), (*C.float)(&x[0]), C.int(incX))
}

// Dgemv computes
//  y = alpha * a * x + beta * y if tA = blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA = blas.Trans or blas.ConjTrans
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Dgemv(tA blas.Transpose, m, n int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	// declared at cblas.h:204:6 void cblas_dgemv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dgemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX), C.double(beta), (*C.double)(&y[0]), C.int(incY))
}

// Dgbmv computes
//  y = alpha * A * x + beta * y if tA == blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA == blas.Trans or blas.ConjTrans
// where a is an m×n band matrix kL subdiagonals and kU super-diagonals, and
// m and n refer to the size of the full dense matrix it represents.
// x and y are vectors, and alpha and beta are scalars.
func (Implementation) Dgbmv(tA blas.Transpose, m, n, kL, kU int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	// declared at cblas.h:209:6 void cblas_dgbmv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if kL < 0 {
		panic("blas: kL < 0")
	}
	if kU < 0 {
		panic("blas: kU < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+kL+kU+1 > len(a) || lda < kL+kU+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_dgbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.int(kL), C.int(kU), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX), C.double(beta), (*C.double)(&y[0]), C.int(incY))
}

// Dtrmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// A is an n×n Triangular matrix and x is a vector.
func (Implementation) Dtrmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float64, lda int, x []float64, incX int) {
	// declared at cblas.h:214:6 void cblas_dtrmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dtrmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX))
}

// Dtbmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// where A is an n×n triangular banded matrix with k diagonals, and x is a vector.
func (Implementation) Dtbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float64, lda int, x []float64, incX int) {
	// declared at cblas.h:218:6 void cblas_dtbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_dtbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX))
}

// Dtpmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans or blas.ConjTrans
// where A is an n×n unit triangular matrix in packed format, and x is a vector.
func (Implementation) Dtpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []float64, incX int) {
	// declared at cblas.h:222:6 void cblas_dtpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dtpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.double)(&ap[0]), (*C.double)(&x[0]), C.int(incX))
}

// Dtrsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans or blas.ConjTrans
// A is an n×n triangular matrix and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Dtrsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float64, lda int, x []float64, incX int) {
	// declared at cblas.h:225:6 void cblas_dtrsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dtrsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX))
}

// Dtbsv solves
//  A * x = b
// where A is an n×n triangular banded matrix with k diagonals in packed format,
// and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Dtbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float64, lda int, x []float64, incX int) {
	// declared at cblas.h:229:6 void cblas_dtbsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_dtbsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX))
}

// Dtpsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans or blas.ConjTrans
// where A is an n×n triangular matrix in packed format and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func (Implementation) Dtpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []float64, incX int) {
	// declared at cblas.h:233:6 void cblas_dtpsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dtpsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), (*C.double)(&ap[0]), (*C.double)(&x[0]), C.int(incX))
}

func (Implementation) Cgemv(tA blas.Transpose, m, n int, alpha complex64, a []complex64, lda int, x []complex64, incX int, beta complex64, y []complex64, incY int) {
	// declared at cblas.h:237:6 void cblas_cgemv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_cgemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Cgbmv(tA blas.Transpose, m, n, kL, kU int, alpha complex64, a []complex64, lda int, x []complex64, incX int, beta complex64, y []complex64, incY int) {
	// declared at cblas.h:242:6 void cblas_cgbmv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if kL < 0 {
		panic("blas: kL < 0")
	}
	if kU < 0 {
		panic("blas: kU < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+kL+kU+1 > len(a) || lda < kL+kU+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_cgbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.int(kL), C.int(kU), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Ctrmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex64, lda int, x []complex64, incX int) {
	// declared at cblas.h:247:6 void cblas_ctrmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ctrmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ctbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex64, lda int, x []complex64, incX int) {
	// declared at cblas.h:251:6 void cblas_ctbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_ctbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ctpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []complex64, incX int) {
	// declared at cblas.h:255:6 void cblas_ctpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_ctpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ctrsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex64, lda int, x []complex64, incX int) {
	// declared at cblas.h:258:6 void cblas_ctrsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ctrsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ctbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex64, lda int, x []complex64, incX int) {
	// declared at cblas.h:262:6 void cblas_ctbsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_ctbsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ctpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []complex64, incX int) {
	// declared at cblas.h:266:6 void cblas_ctpsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_ctpsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Zgemv(tA blas.Transpose, m, n int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	// declared at cblas.h:270:6 void cblas_zgemv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zgemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zgbmv(tA blas.Transpose, m, n, kL, kU int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	// declared at cblas.h:275:6 void cblas_zgbmv ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if kL < 0 {
		panic("blas: kL < 0")
	}
	if kU < 0 {
		panic("blas: kU < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	var lenX, lenY int
	if tA == blas.NoTrans {
		lenX, lenY = n, m
	} else {
		lenX, lenY = m, n
	}
	if (incX > 0 && (lenX-1)*incX >= len(x)) || (incX < 0 && (1-lenX)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (lenY-1)*incY >= len(y)) || (incY < 0 && (1-lenY)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+kL+kU+1 > len(a) || lda < kL+kU+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_zgbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.int(m), C.int(n), C.int(kL), C.int(kU), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Ztrmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex128, lda int, x []complex128, incX int) {
	// declared at cblas.h:280:6 void cblas_ztrmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ztrmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ztbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex128, lda int, x []complex128, incX int) {
	// declared at cblas.h:284:6 void cblas_ztbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_ztbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ztpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []complex128, incX int) {
	// declared at cblas.h:288:6 void cblas_ztpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_ztpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ztrsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex128, lda int, x []complex128, incX int) {
	// declared at cblas.h:291:6 void cblas_ztrsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ztrsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ztbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex128, lda int, x []complex128, incX int) {
	// declared at cblas.h:295:6 void cblas_ztbsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_ztbsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), C.int(k), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX))
}

func (Implementation) Ztpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap, x []complex128, incX int) {
	// declared at cblas.h:299:6 void cblas_ztpsv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_ztpsv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(n), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX))
}

// Ssymv computes
//    y = alpha * A * x + beta * y,
// where a is an n×n symmetric matrix, x and y are vectors, and alpha and
// beta are scalars.
func (Implementation) Ssymv(ul blas.Uplo, n int, alpha float32, a []float32, lda int, x []float32, incX int, beta float32, y []float32, incY int) {
	// declared at cblas.h:307:6 void cblas_ssymv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ssymv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX), C.float(beta), (*C.float)(&y[0]), C.int(incY))
}

// Ssbmv performs
//  y = alpha * A * x + beta * y
// where A is an n×n symmetric banded matrix, x and y are vectors, and alpha
// and beta are scalars.
func (Implementation) Ssbmv(ul blas.Uplo, n, k int, alpha float32, a []float32, lda int, x []float32, incX int, beta float32, y []float32, incY int) {
	// declared at cblas.h:311:6 void cblas_ssbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_ssbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.int(k), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&x[0]), C.int(incX), C.float(beta), (*C.float)(&y[0]), C.int(incY))
}

// Sspmv performs
//    y = alpha * A * x + beta * y,
// where A is an n×n symmetric matrix in packed format, x and y are vectors
// and alpha and beta are scalars.
func (Implementation) Sspmv(ul blas.Uplo, n int, alpha float32, ap, x []float32, incX int, beta float32, y []float32, incY int) {
	// declared at cblas.h:315:6 void cblas_sspmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_sspmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&ap[0]), (*C.float)(&x[0]), C.int(incX), C.float(beta), (*C.float)(&y[0]), C.int(incY))
}

// Sger performs the rank-one operation
//  A += alpha * x * y^T
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Sger(m, n int, alpha float32, x []float32, incX int, y []float32, incY int, a []float32, lda int) {
	// declared at cblas.h:319:6 void cblas_sger ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_sger(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY), (*C.float)(&a[0]), C.int(lda))
}

// Ssyr performs the rank-one update
//  a += alpha * x * x^T
// where a is an n×n symmetric matrix, and x is a vector.
func (Implementation) Ssyr(ul blas.Uplo, n int, alpha float32, x []float32, incX int, a []float32, lda int) {
	// declared at cblas.h:322:6 void cblas_ssyr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ssyr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&a[0]), C.int(lda))
}

// Sspr computes the rank-one operation
//  a += alpha * x * x^T
// where a is an n×n symmetric matrix in packed format, x is a vector, and
// alpha is a scalar.
func (Implementation) Sspr(ul blas.Uplo, n int, alpha float32, x []float32, incX int, ap []float32) {
	// declared at cblas.h:325:6 void cblas_sspr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_sspr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&ap[0]))
}

// Ssyr2 performs the symmetric rank-two update
//  A += alpha * x * y^T + alpha * y * x^T
// where A is a symmetric n×n matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Ssyr2(ul blas.Uplo, n int, alpha float32, x []float32, incX int, y []float32, incY int, a []float32, lda int) {
	// declared at cblas.h:328:6 void cblas_ssyr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_ssyr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY), (*C.float)(&a[0]), C.int(lda))
}

// Sspr2 performs the symmetric rank-2 update
//  A += alpha * x * y^T + alpha * y * x^T,
// where A is an n×n symmetric matrix in packed format, x and y are vectors,
// and alpha is a scalar.
func (Implementation) Sspr2(ul blas.Uplo, n int, alpha float32, x []float32, incX int, y []float32, incY int, ap []float32) {
	// declared at cblas.h:332:6 void cblas_sspr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_sspr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), (*C.float)(&x[0]), C.int(incX), (*C.float)(&y[0]), C.int(incY), (*C.float)(&ap[0]))
}

// Dsymv computes
//    y = alpha * A * x + beta * y,
// where a is an n×n symmetric matrix, x and y are vectors, and alpha and
// beta are scalars.
func (Implementation) Dsymv(ul blas.Uplo, n int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	// declared at cblas.h:336:6 void cblas_dsymv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dsymv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX), C.double(beta), (*C.double)(&y[0]), C.int(incY))
}

// Dsbmv performs
//  y = alpha * A * x + beta * y
// where A is an n×n symmetric banded matrix, x and y are vectors, and alpha
// and beta are scalars.
func (Implementation) Dsbmv(ul blas.Uplo, n, k int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	// declared at cblas.h:340:6 void cblas_dsbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_dsbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.int(k), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&x[0]), C.int(incX), C.double(beta), (*C.double)(&y[0]), C.int(incY))
}

// Dspmv performs
//    y = alpha * A * x + beta * y,
// where A is an n×n symmetric matrix in packed format, x and y are vectors
// and alpha and beta are scalars.
func (Implementation) Dspmv(ul blas.Uplo, n int, alpha float64, ap, x []float64, incX int, beta float64, y []float64, incY int) {
	// declared at cblas.h:344:6 void cblas_dspmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dspmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&ap[0]), (*C.double)(&x[0]), C.int(incX), C.double(beta), (*C.double)(&y[0]), C.int(incY))
}

// Dger performs the rank-one operation
//  A += alpha * x * y^T
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Dger(m, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64, lda int) {
	// declared at cblas.h:348:6 void cblas_dger ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dger(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY), (*C.double)(&a[0]), C.int(lda))
}

// Dsyr performs the rank-one update
//  a += alpha * x * x^T
// where a is an n×n symmetric matrix, and x is a vector.
func (Implementation) Dsyr(ul blas.Uplo, n int, alpha float64, x []float64, incX int, a []float64, lda int) {
	// declared at cblas.h:351:6 void cblas_dsyr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dsyr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&a[0]), C.int(lda))
}

// Dspr computes the rank-one operation
//  a += alpha * x * x^T
// where a is an n×n symmetric matrix in packed format, x is a vector, and
// alpha is a scalar.
func (Implementation) Dspr(ul blas.Uplo, n int, alpha float64, x []float64, incX int, ap []float64) {
	// declared at cblas.h:354:6 void cblas_dspr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dspr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&ap[0]))
}

// Dsyr2 performs the symmetric rank-two update
//  A += alpha * x * y^T + alpha * y * x^T
// where A is a symmetric n×n matrix, x and y are vectors, and alpha is a scalar.
func (Implementation) Dsyr2(ul blas.Uplo, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64, lda int) {
	// declared at cblas.h:357:6 void cblas_dsyr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_dsyr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY), (*C.double)(&a[0]), C.int(lda))
}

// Dspr2 performs the symmetric rank-2 update
//  A += alpha * x * y^T + alpha * y * x^T,
// where A is an n×n symmetric matrix in packed format, x and y are vectors,
// and alpha is a scalar.
func (Implementation) Dspr2(ul blas.Uplo, n int, alpha float64, x []float64, incX int, y []float64, incY int, ap []float64) {
	// declared at cblas.h:361:6 void cblas_dspr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_dspr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), (*C.double)(&x[0]), C.int(incX), (*C.double)(&y[0]), C.int(incY), (*C.double)(&ap[0]))
}

func (Implementation) Chemv(ul blas.Uplo, n int, alpha complex64, a []complex64, lda int, x []complex64, incX int, beta complex64, y []complex64, incY int) {
	// declared at cblas.h:369:6 void cblas_chemv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_chemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Chbmv(ul blas.Uplo, n, k int, alpha complex64, a []complex64, lda int, x []complex64, incX int, beta complex64, y []complex64, incY int) {
	// declared at cblas.h:373:6 void cblas_chbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_chbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Chpmv(ul blas.Uplo, n int, alpha complex64, ap, x []complex64, incX int, beta complex64, y []complex64, incY int) {
	// declared at cblas.h:377:6 void cblas_chpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_chpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Cgeru(m, n int, alpha complex64, x []complex64, incX int, y []complex64, incY int, a []complex64, lda int) {
	// declared at cblas.h:381:6 void cblas_cgeru ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_cgeru(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Cgerc(m, n int, alpha complex64, x []complex64, incX int, y []complex64, incY int, a []complex64, lda int) {
	// declared at cblas.h:384:6 void cblas_cgerc ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_cgerc(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Cher(ul blas.Uplo, n int, alpha float32, x []complex64, incX int, a []complex64, lda int) {
	// declared at cblas.h:387:6 void cblas_cher ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_cher(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Chpr(ul blas.Uplo, n int, alpha float32, x []complex64, incX int, ap []complex64) {
	// declared at cblas.h:390:6 void cblas_chpr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_chpr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.float(alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&ap[0]))
}

func (Implementation) Cher2(ul blas.Uplo, n int, alpha complex64, x []complex64, incX int, y []complex64, incY int, a []complex64, lda int) {
	// declared at cblas.h:393:6 void cblas_cher2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_cher2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Chpr2(ul blas.Uplo, n int, alpha complex64, x []complex64, incX int, y []complex64, incY int, ap []complex64) {
	// declared at cblas.h:396:6 void cblas_chpr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_chpr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&ap[0]))
}

func (Implementation) Zhemv(ul blas.Uplo, n int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	// declared at cblas.h:400:6 void cblas_zhemv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zhemv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zhbmv(ul blas.Uplo, n, k int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	// declared at cblas.h:404:6 void cblas_zhbmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+k+1 > len(a) || lda < k+1 {
		panic("blas: index of a out of range")
	}
	C.cblas_zhbmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zhpmv(ul blas.Uplo, n int, alpha complex128, ap, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	// declared at cblas.h:408:6 void cblas_zhpmv ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zhpmv(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&ap[0]), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&beta), unsafe.Pointer(&y[0]), C.int(incY))
}

func (Implementation) Zgeru(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	// declared at cblas.h:412:6 void cblas_zgeru ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zgeru(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Zgerc(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	// declared at cblas.h:415:6 void cblas_zgerc ...

	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (m-1)*incX >= len(x)) || (incX < 0 && (1-m)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(m-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zgerc(C.enum_CBLAS_ORDER(rowMajor), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Zher(ul blas.Uplo, n int, alpha float64, x []complex128, incX int, a []complex128, lda int) {
	// declared at cblas.h:418:6 void cblas_zher ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zher(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Zhpr(ul blas.Uplo, n int, alpha float64, x []complex128, incX int, ap []complex128) {
	// declared at cblas.h:421:6 void cblas_zhpr ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zhpr(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), C.double(alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&ap[0]))
}

func (Implementation) Zher2(ul blas.Uplo, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	// declared at cblas.h:424:6 void cblas_zher2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if lda*(n-1)+n > len(a) || lda < max(1, n) {
		panic("blas: index of a out of range")
	}
	C.cblas_zher2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&a[0]), C.int(lda))
}

func (Implementation) Zhpr2(ul blas.Uplo, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, ap []complex128) {
	// declared at cblas.h:427:6 void cblas_zhpr2 ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if incX == 0 {
		panic("blas: zero x index increment")
	}
	if incY == 0 {
		panic("blas: zero y index increment")
	}
	if n*(n+1)/2 > len(ap) {
		panic("blas: index of ap out of range")
	}
	if (incX > 0 && (n-1)*incX >= len(x)) || (incX < 0 && (1-n)*incX >= len(x)) {
		panic("blas: x index out of range")
	}
	if (incY > 0 && (n-1)*incY >= len(y)) || (incY < 0 && (1-n)*incY >= len(y)) {
		panic("blas: y index out of range")
	}
	if n == 0 {
		return
	}
	C.cblas_zhpr2(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&x[0]), C.int(incX), unsafe.Pointer(&y[0]), C.int(incY), unsafe.Pointer(&ap[0]))
}

// Sgemm computes
//  C = beta * C + alpha * A * B,
// where A, B, and C are dense matrices, and alpha and beta are scalars.
// tA and tB specify whether A or B are transposed.
func (Implementation) Sgemm(tA, tB blas.Transpose, m, n, k int, alpha float32, a []float32, lda int, b []float32, ldb int, beta float32, c []float32, ldc int) {
	// declared at cblas.h:440:6 void cblas_sgemm ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if tB != blas.NoTrans && tB != blas.Trans && tB != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var rowA, colA, rowB, colB int
	if tA == blas.NoTrans {
		rowA, colA = m, k
	} else {
		rowA, colA = k, m
	}
	if tB == blas.NoTrans {
		rowB, colB = k, n
	} else {
		rowB, colB = n, k
	}
	if lda*(rowA-1)+colA > len(a) || lda < max(1, colA) {
		panic("blas: index of a out of range")
	}
	if ldb*(rowB-1)+colB > len(b) || ldb < max(1, colB) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_sgemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_TRANSPOSE(tB), C.int(m), C.int(n), C.int(k), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb), C.float(beta), (*C.float)(&c[0]), C.int(ldc))
}

// Ssymm performs one of
//  C = alpha * A * B + beta * C, if side == blas.Left,
//  C = alpha * B * A + beta * C, if side == blas.Right,
// where A is an n×n or m×m symmetric matrix, B and C are m×n matrices, and alpha
// is a scalar.
func (Implementation) Ssymm(s blas.Side, ul blas.Uplo, m, n int, alpha float32, a []float32, lda int, b []float32, ldb int, beta float32, c []float32, ldc int) {
	// declared at cblas.h:445:6 void cblas_ssymm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_ssymm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb), C.float(beta), (*C.float)(&c[0]), C.int(ldc))
}

// Ssyrk performs the symmetric rank-k operation
//  C = alpha * A * A^T + beta*C
// C is an n×n symmetric matrix. A is an n×k matrix if tA == blas.NoTrans, and
// a k×n matrix otherwise. alpha and beta are scalars.
func (Implementation) Ssyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float32, a []float32, lda int, beta float32, c []float32, ldc int) {
	// declared at cblas.h:450:6 void cblas_ssyrk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_ssyrk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.float(alpha), (*C.float)(&a[0]), C.int(lda), C.float(beta), (*C.float)(&c[0]), C.int(ldc))
}

// Ssyr2k performs the symmetric rank 2k operation
//  C = alpha * A * B^T + alpha * B * A^T + beta * C
// where C is an n×n symmetric matrix. A and B are n×k matrices if
// tA == NoTrans and k×n otherwise. alpha and beta are scalars.
func (Implementation) Ssyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha float32, a []float32, lda int, b []float32, ldb int, beta float32, c []float32, ldc int) {
	// declared at cblas.h:454:6 void cblas_ssyr2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_ssyr2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb), C.float(beta), (*C.float)(&c[0]), C.int(ldc))
}

// Strmm performs
//  B = alpha * A * B,   if tA == blas.NoTrans and side == blas.Left,
//  B = alpha * A^T * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Left,
//  B = alpha * B * A,   if tA == blas.NoTrans and side == blas.Right,
//  B = alpha * B * A^T, if tA == blas.Trans or blas.ConjTrans, and side == blas.Right,
// where A is an n×n or m×m triangular matrix, and B is an m×n matrix.
func (Implementation) Strmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float32, a []float32, lda int, b []float32, ldb int) {
	// declared at cblas.h:459:6 void cblas_strmm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_strmm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb))
}

// Strsm solves
//  A * X = alpha * B,   if tA == blas.NoTrans side == blas.Left,
//  A^T * X = alpha * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Left,
//  X * A = alpha * B,   if tA == blas.NoTrans side == blas.Right,
//  X * A^T = alpha * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Right,
// where A is an n×n or m×m triangular matrix, X is an m×n matrix, and alpha is a
// scalar.
//
// At entry to the function, X contains the values of B, and the result is
// stored in place into X.
//
// No check is made that A is invertible.
func (Implementation) Strsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float32, a []float32, lda int, b []float32, ldb int) {
	// declared at cblas.h:464:6 void cblas_strsm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_strsm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), C.float(alpha), (*C.float)(&a[0]), C.int(lda), (*C.float)(&b[0]), C.int(ldb))
}

// Dgemm computes
//  C = beta * C + alpha * A * B,
// where A, B, and C are dense matrices, and alpha and beta are scalars.
// tA and tB specify whether A or B are transposed.
func (Implementation) Dgemm(tA, tB blas.Transpose, m, n, k int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	// declared at cblas.h:470:6 void cblas_dgemm ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if tB != blas.NoTrans && tB != blas.Trans && tB != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var rowA, colA, rowB, colB int
	if tA == blas.NoTrans {
		rowA, colA = m, k
	} else {
		rowA, colA = k, m
	}
	if tB == blas.NoTrans {
		rowB, colB = k, n
	} else {
		rowB, colB = n, k
	}
	if lda*(rowA-1)+colA > len(a) || lda < max(1, colA) {
		panic("blas: index of a out of range")
	}
	if ldb*(rowB-1)+colB > len(b) || ldb < max(1, colB) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_dgemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_TRANSPOSE(tB), C.int(m), C.int(n), C.int(k), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&b[0]), C.int(ldb), C.double(beta), (*C.double)(&c[0]), C.int(ldc))
}

// Dsymm performs one of
//  C = alpha * A * B + beta * C, if side == blas.Left,
//  C = alpha * B * A + beta * C, if side == blas.Right,
// where A is an n×n or m×m symmetric matrix, B and C are m×n matrices, and alpha
// is a scalar.
func (Implementation) Dsymm(s blas.Side, ul blas.Uplo, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	// declared at cblas.h:475:6 void cblas_dsymm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_dsymm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&b[0]), C.int(ldb), C.double(beta), (*C.double)(&c[0]), C.int(ldc))
}

// Dsyrk performs the symmetric rank-k operation
//  C = alpha * A * A^T + beta*C
// C is an n×n symmetric matrix. A is an n×k matrix if tA == blas.NoTrans, and
// a k×n matrix otherwise. alpha and beta are scalars.
func (Implementation) Dsyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []float64, lda int, beta float64, c []float64, ldc int) {
	// declared at cblas.h:480:6 void cblas_dsyrk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_dsyrk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.double(alpha), (*C.double)(&a[0]), C.int(lda), C.double(beta), (*C.double)(&c[0]), C.int(ldc))
}

// Dsyr2k performs the symmetric rank 2k operation
//  C = alpha * A * B^T + alpha * B * A^T + beta * C
// where C is an n×n symmetric matrix. A and B are n×k matrices if
// tA == NoTrans and k×n otherwise. alpha and beta are scalars.
func (Implementation) Dsyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	// declared at cblas.h:484:6 void cblas_dsyr2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_dsyr2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&b[0]), C.int(ldb), C.double(beta), (*C.double)(&c[0]), C.int(ldc))
}

// Dtrmm performs
//  B = alpha * A * B,   if tA == blas.NoTrans and side == blas.Left,
//  B = alpha * A^T * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Left,
//  B = alpha * B * A,   if tA == blas.NoTrans and side == blas.Right,
//  B = alpha * B * A^T, if tA == blas.Trans or blas.ConjTrans, and side == blas.Right,
// where A is an n×n or m×m triangular matrix, and B is an m×n matrix.
func (Implementation) Dtrmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int) {
	// declared at cblas.h:489:6 void cblas_dtrmm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_dtrmm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&b[0]), C.int(ldb))
}

// Dtrsm solves
//  A * X = alpha * B,   if tA == blas.NoTrans side == blas.Left,
//  A^T * X = alpha * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Left,
//  X * A = alpha * B,   if tA == blas.NoTrans side == blas.Right,
//  X * A^T = alpha * B, if tA == blas.Trans or blas.ConjTrans, and side == blas.Right,
// where A is an n×n or m×m triangular matrix, X is an m×n matrix, and alpha is a
// scalar.
//
// At entry to the function, X contains the values of B, and the result is
// stored in place into X.
//
// No check is made that A is invertible.
func (Implementation) Dtrsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int) {
	// declared at cblas.h:494:6 void cblas_dtrsm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_dtrsm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), C.double(alpha), (*C.double)(&a[0]), C.int(lda), (*C.double)(&b[0]), C.int(ldb))
}

func (Implementation) Cgemm(tA, tB blas.Transpose, m, n, k int, alpha complex64, a []complex64, lda int, b []complex64, ldb int, beta complex64, c []complex64, ldc int) {
	// declared at cblas.h:500:6 void cblas_cgemm ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if tB != blas.NoTrans && tB != blas.Trans && tB != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var rowA, colA, rowB, colB int
	if tA == blas.NoTrans {
		rowA, colA = m, k
	} else {
		rowA, colA = k, m
	}
	if tB == blas.NoTrans {
		rowB, colB = k, n
	} else {
		rowB, colB = n, k
	}
	if lda*(rowA-1)+colA > len(a) || lda < max(1, colA) {
		panic("blas: index of a out of range")
	}
	if ldb*(rowB-1)+colB > len(b) || ldb < max(1, colB) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_cgemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_TRANSPOSE(tB), C.int(m), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Csymm(s blas.Side, ul blas.Uplo, m, n int, alpha complex64, a []complex64, lda int, b []complex64, ldb int, beta complex64, c []complex64, ldc int) {
	// declared at cblas.h:505:6 void cblas_csymm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_csymm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Csyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex64, a []complex64, lda int, beta complex64, c []complex64, ldc int) {
	// declared at cblas.h:510:6 void cblas_csyrk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_csyrk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Csyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex64, a []complex64, lda int, b []complex64, ldb int, beta complex64, c []complex64, ldc int) {
	// declared at cblas.h:514:6 void cblas_csyr2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_csyr2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Ctrmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex64, a []complex64, lda int, b []complex64, ldb int) {
	// declared at cblas.h:519:6 void cblas_ctrmm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_ctrmm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb))
}

func (Implementation) Ctrsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex64, a []complex64, lda int, b []complex64, ldb int) {
	// declared at cblas.h:524:6 void cblas_ctrsm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_ctrsm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb))
}

func (Implementation) Zgemm(tA, tB blas.Transpose, m, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
	// declared at cblas.h:530:6 void cblas_zgemm ...

	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if tB != blas.NoTrans && tB != blas.Trans && tB != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var rowA, colA, rowB, colB int
	if tA == blas.NoTrans {
		rowA, colA = m, k
	} else {
		rowA, colA = k, m
	}
	if tB == blas.NoTrans {
		rowB, colB = k, n
	} else {
		rowB, colB = n, k
	}
	if lda*(rowA-1)+colA > len(a) || lda < max(1, colA) {
		panic("blas: index of a out of range")
	}
	if ldb*(rowB-1)+colB > len(b) || ldb < max(1, colB) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zgemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_TRANSPOSE(tB), C.int(m), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zsymm(s blas.Side, ul blas.Uplo, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
	// declared at cblas.h:535:6 void cblas_zsymm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zsymm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zsyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, beta complex128, c []complex128, ldc int) {
	// declared at cblas.h:540:6 void cblas_zsyrk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zsyrk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zsyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
	// declared at cblas.h:544:6 void cblas_zsyr2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.Trans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zsyr2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Ztrmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int) {
	// declared at cblas.h:549:6 void cblas_ztrmm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_ztrmm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb))
}

func (Implementation) Ztrsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int) {
	// declared at cblas.h:554:6 void cblas_ztrsm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if tA != blas.NoTrans && tA != blas.Trans && tA != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if d != blas.NonUnit && d != blas.Unit {
		panic("blas: illegal diagonal")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	C.cblas_ztrsm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(tA), C.enum_CBLAS_DIAG(d), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb))
}

func (Implementation) Chemm(s blas.Side, ul blas.Uplo, m, n int, alpha complex64, a []complex64, lda int, b []complex64, ldb int, beta complex64, c []complex64, ldc int) {
	// declared at cblas.h:564:6 void cblas_chemm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_chemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Cherk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float32, a []complex64, lda int, beta float32, c []complex64, ldc int) {
	// declared at cblas.h:569:6 void cblas_cherk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_cherk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.float(alpha), unsafe.Pointer(&a[0]), C.int(lda), C.float(beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Cher2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex64, a []complex64, lda int, b []complex64, ldb int, beta float32, c []complex64, ldc int) {
	// declared at cblas.h:573:6 void cblas_cher2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_cher2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), C.float(beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zhemm(s blas.Side, ul blas.Uplo, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
	// declared at cblas.h:578:6 void cblas_zhemm ...

	if s != blas.Left && s != blas.Right {
		panic("blas: illegal side")
	}
	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if m < 0 {
		panic("blas: m < 0")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	var k int
	if s == blas.Left {
		k = m
	} else {
		k = n
	}
	if lda*(k-1)+k > len(a) || lda < max(1, k) {
		panic("blas: index of a out of range")
	}
	if ldb*(m-1)+n > len(b) || ldb < max(1, n) {
		panic("blas: index of b out of range")
	}
	if ldc*(m-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zhemm(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_SIDE(s), C.enum_CBLAS_UPLO(ul), C.int(m), C.int(n), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), unsafe.Pointer(&beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zherk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []complex128, lda int, beta float64, c []complex128, ldc int) {
	// declared at cblas.h:583:6 void cblas_zherk ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zherk(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), C.double(alpha), unsafe.Pointer(&a[0]), C.int(lda), C.double(beta), unsafe.Pointer(&c[0]), C.int(ldc))
}

func (Implementation) Zher2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta float64, c []complex128, ldc int) {
	// declared at cblas.h:587:6 void cblas_zher2k ...

	if ul != blas.Upper && ul != blas.Lower {
		panic("blas: illegal triangle")
	}
	if t != blas.NoTrans && t != blas.ConjTrans {
		panic("blas: illegal transpose")
	}
	if n < 0 {
		panic("blas: n < 0")
	}
	if k < 0 {
		panic("blas: k < 0")
	}
	var row, col int
	if t == blas.NoTrans {
		row, col = n, k
	} else {
		row, col = k, n
	}
	if lda*(row-1)+col > len(a) || lda < max(1, col) {
		panic("blas: index of a out of range")
	}
	if ldb*(row-1)+col > len(b) || ldb < max(1, col) {
		panic("blas: index of b out of range")
	}
	if ldc*(n-1)+n > len(c) || ldc < max(1, n) {
		panic("blas: index of c out of range")
	}
	C.cblas_zher2k(C.enum_CBLAS_ORDER(rowMajor), C.enum_CBLAS_UPLO(ul), C.enum_CBLAS_TRANSPOSE(t), C.int(n), C.int(k), unsafe.Pointer(&alpha), unsafe.Pointer(&a[0]), C.int(lda), unsafe.Pointer(&b[0]), C.int(ldb), C.double(beta), unsafe.Pointer(&c[0]), C.int(ldc))
}
