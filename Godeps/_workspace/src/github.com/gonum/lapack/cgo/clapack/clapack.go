// Do not manually edit this file. It was created by the genLapack.pl script from lapacke.h.

// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package clapack provides bindings to a C LAPACK library.
//
// Links are provided to the NETLIB fortran implementation/dependencies for each function.
package clapack

/*
#cgo CFLAGS: -g -O2
#include "lapacke.h"
*/
import "C"

import (
	"github.com/gonum/blas"
	"github.com/gonum/lapack"
	"unsafe"
)

// Type order is used to specify the matrix storage format. We still interact with
// an API that allows client calls to specify order, so this is here to document that fact.
type order int

const (
	rowMajor order = 101 + iota
	colMajor
)

func isZero(ret C.int) bool { return ret == 0 }

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sbdsdc.f.
func Sbdsdc(ul blas.Uplo, compq lapack.CompSV, n int, d []float32, e []float32, u []float32, ldu int, vt []float32, ldvt int, q []float32, iq []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sbdsdc((C.int)(rowMajor), (C.char)(ul), (C.char)(compq), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&vt[0]), (C.lapack_int)(ldvt), (*C.float)(&q[0]), (*C.lapack_int)(&iq[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dbdsdc.f.
func Dbdsdc(ul blas.Uplo, compq lapack.CompSV, n int, d []float64, e []float64, u []float64, ldu int, vt []float64, ldvt int, q []float64, iq []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dbdsdc((C.int)(rowMajor), (C.char)(ul), (C.char)(compq), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&vt[0]), (C.lapack_int)(ldvt), (*C.double)(&q[0]), (*C.lapack_int)(&iq[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sbdsqr.f.
func Sbdsqr(ul blas.Uplo, n int, ncvt int, nru int, ncc int, d []float32, e []float32, vt []float32, ldvt int, u []float32, ldu int, c []float32, ldc int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sbdsqr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ncvt), (C.lapack_int)(nru), (C.lapack_int)(ncc), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&vt[0]), (C.lapack_int)(ldvt), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dbdsqr.f.
func Dbdsqr(ul blas.Uplo, n int, ncvt int, nru int, ncc int, d []float64, e []float64, vt []float64, ldvt int, u []float64, ldu int, c []float64, ldc int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dbdsqr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ncvt), (C.lapack_int)(nru), (C.lapack_int)(ncc), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&vt[0]), (C.lapack_int)(ldvt), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cbdsqr.f.
func Cbdsqr(ul blas.Uplo, n int, ncvt int, nru int, ncc int, d []float32, e []float32, vt []complex64, ldvt int, u []complex64, ldu int, c []complex64, ldc int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cbdsqr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ncvt), (C.lapack_int)(nru), (C.lapack_int)(ncc), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&vt[0]), (C.lapack_int)(ldvt), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zbdsqr.f.
func Zbdsqr(ul blas.Uplo, n int, ncvt int, nru int, ncc int, d []float64, e []float64, vt []complex128, ldvt int, u []complex128, ldu int, c []complex128, ldc int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zbdsqr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ncvt), (C.lapack_int)(nru), (C.lapack_int)(ncc), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&vt[0]), (C.lapack_int)(ldvt), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sdisna.f.
func Sdisna(job lapack.Job, m int, n int, d []float32, sep []float32) bool {
	return isZero(C.LAPACKE_sdisna((C.char)(job), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&sep[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ddisna.f.
func Ddisna(job lapack.Job, m int, n int, d []float64, sep []float64) bool {
	return isZero(C.LAPACKE_ddisna((C.char)(job), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&sep[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbbrd.f.
func Sgbbrd(vect byte, m int, n int, ncc int, kl int, ku int, ab []float32, ldab int, d []float32, e []float32, q []float32, ldq int, pt []float32, ldpt int, c []float32, ldc int) bool {
	return isZero(C.LAPACKE_sgbbrd((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ncc), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.float)(&pt[0]), (C.lapack_int)(ldpt), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbbrd.f.
func Dgbbrd(vect byte, m int, n int, ncc int, kl int, ku int, ab []float64, ldab int, d []float64, e []float64, q []float64, ldq int, pt []float64, ldpt int, c []float64, ldc int) bool {
	return isZero(C.LAPACKE_dgbbrd((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ncc), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.double)(&pt[0]), (C.lapack_int)(ldpt), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbbrd.f.
func Cgbbrd(vect byte, m int, n int, ncc int, kl int, ku int, ab []complex64, ldab int, d []float32, e []float32, q []complex64, ldq int, pt []complex64, ldpt int, c []complex64, ldc int) bool {
	return isZero(C.LAPACKE_cgbbrd((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ncc), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_float)(&pt[0]), (C.lapack_int)(ldpt), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbbrd.f.
func Zgbbrd(vect byte, m int, n int, ncc int, kl int, ku int, ab []complex128, ldab int, d []float64, e []float64, q []complex128, ldq int, pt []complex128, ldpt int, c []complex128, ldc int) bool {
	return isZero(C.LAPACKE_zgbbrd((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ncc), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_double)(&pt[0]), (C.lapack_int)(ldpt), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbcon.f.
func Sgbcon(norm byte, n int, kl int, ku int, ab []float32, ldab int, ipiv []int32, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_sgbcon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbcon.f.
func Dgbcon(norm byte, n int, kl int, ku int, ab []float64, ldab int, ipiv []int32, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_dgbcon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbcon.f.
func Cgbcon(norm byte, n int, kl int, ku int, ab []complex64, ldab int, ipiv []int32, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_cgbcon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbcon.f.
func Zgbcon(norm byte, n int, kl int, ku int, ab []complex128, ldab int, ipiv []int32, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_zgbcon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbequ.f.
func Sgbequ(m int, n int, kl int, ku int, ab []float32, ldab int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_sgbequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbequ.f.
func Dgbequ(m int, n int, kl int, ku int, ab []float64, ldab int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dgbequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbequ.f.
func Cgbequ(m int, n int, kl int, ku int, ab []complex64, ldab int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cgbequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbequ.f.
func Zgbequ(m int, n int, kl int, ku int, ab []complex128, ldab int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zgbequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbequb.f.
func Sgbequb(m int, n int, kl int, ku int, ab []float32, ldab int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_sgbequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbequb.f.
func Dgbequb(m int, n int, kl int, ku int, ab []float64, ldab int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dgbequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbequb.f.
func Cgbequb(m int, n int, kl int, ku int, ab []complex64, ldab int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cgbequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbequb.f.
func Zgbequb(m int, n int, kl int, ku int, ab []complex128, ldab int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zgbequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbrfs.f.
func Sgbrfs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float32, ldab int, afb []float32, ldafb int, ipiv []int32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgbrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbrfs.f.
func Dgbrfs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float64, ldab int, afb []float64, ldafb int, ipiv []int32, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgbrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbrfs.f.
func Cgbrfs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex64, ldab int, afb []complex64, ldafb int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgbrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbrfs.f.
func Zgbrfs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex128, ldab int, afb []complex128, ldafb int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgbrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbsv.f.
func Sgbsv(n int, kl int, ku int, nrhs int, ab []float32, ldab int, ipiv []int32, b []float32, ldb int) bool {
	return isZero(C.LAPACKE_sgbsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbsv.f.
func Dgbsv(n int, kl int, ku int, nrhs int, ab []float64, ldab int, ipiv []int32, b []float64, ldb int) bool {
	return isZero(C.LAPACKE_dgbsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbsv.f.
func Cgbsv(n int, kl int, ku int, nrhs int, ab []complex64, ldab int, ipiv []int32, b []complex64, ldb int) bool {
	return isZero(C.LAPACKE_cgbsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbsv.f.
func Zgbsv(n int, kl int, ku int, nrhs int, ab []complex128, ldab int, ipiv []int32, b []complex128, ldb int) bool {
	return isZero(C.LAPACKE_zgbsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbsvx.f.
func Sgbsvx(fact byte, trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float32, ldab int, afb []float32, ldafb int, ipiv []int32, equed []byte, r []float32, c []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32, rpivot []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0]), (*C.float)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbsvx.f.
func Dgbsvx(fact byte, trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float64, ldab int, afb []float64, ldafb int, ipiv []int32, equed []byte, r []float64, c []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64, rpivot []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0]), (*C.double)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbsvx.f.
func Cgbsvx(fact byte, trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex64, ldab int, afb []complex64, ldafb int, ipiv []int32, equed []byte, r []float32, c []float32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32, rpivot []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0]), (*C.float)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbsvx.f.
func Zgbsvx(fact byte, trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex128, ldab int, afb []complex128, ldafb int, ipiv []int32, equed []byte, r []float64, c []float64, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64, rpivot []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0]), (*C.double)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbtrf.f.
func Sgbtrf(m int, n int, kl int, ku int, ab []float32, ldab int, ipiv []int32) bool {
	return isZero(C.LAPACKE_sgbtrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbtrf.f.
func Dgbtrf(m int, n int, kl int, ku int, ab []float64, ldab int, ipiv []int32) bool {
	return isZero(C.LAPACKE_dgbtrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbtrf.f.
func Cgbtrf(m int, n int, kl int, ku int, ab []complex64, ldab int, ipiv []int32) bool {
	return isZero(C.LAPACKE_cgbtrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbtrf.f.
func Zgbtrf(m int, n int, kl int, ku int, ab []complex128, ldab int, ipiv []int32) bool {
	return isZero(C.LAPACKE_zgbtrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgbtrs.f.
func Sgbtrs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float32, ldab int, ipiv []int32, b []float32, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgbtrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgbtrs.f.
func Dgbtrs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []float64, ldab int, ipiv []int32, b []float64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgbtrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgbtrs.f.
func Cgbtrs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex64, ldab int, ipiv []int32, b []complex64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgbtrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgbtrs.f.
func Zgbtrs(trans blas.Transpose, n int, kl int, ku int, nrhs int, ab []complex128, ldab int, ipiv []int32, b []complex128, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgbtrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(kl), (C.lapack_int)(ku), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgebak.f.
func Sgebak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, scale []float32, m int, v []float32, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_sgebak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&scale[0]), (C.lapack_int)(m), (*C.float)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgebak.f.
func Dgebak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, scale []float64, m int, v []float64, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_dgebak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&scale[0]), (C.lapack_int)(m), (*C.double)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgebak.f.
func Cgebak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, scale []float32, m int, v []complex64, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_cgebak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&scale[0]), (C.lapack_int)(m), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgebak.f.
func Zgebak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, scale []float64, m int, v []complex128, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_zgebak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&scale[0]), (C.lapack_int)(m), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgebal.f.
func Sgebal(job lapack.Job, n int, a []float32, lda int, ilo []int32, ihi []int32, scale []float32) bool {
	return isZero(C.LAPACKE_sgebal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgebal.f.
func Dgebal(job lapack.Job, n int, a []float64, lda int, ilo []int32, ihi []int32, scale []float64) bool {
	return isZero(C.LAPACKE_dgebal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgebal.f.
func Cgebal(job lapack.Job, n int, a []complex64, lda int, ilo []int32, ihi []int32, scale []float32) bool {
	return isZero(C.LAPACKE_cgebal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgebal.f.
func Zgebal(job lapack.Job, n int, a []complex128, lda int, ilo []int32, ihi []int32, scale []float64) bool {
	return isZero(C.LAPACKE_zgebal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgebrd.f.
func Sgebrd(m int, n int, a []float32, lda int, d []float32, e []float32, tauq []float32, taup []float32) bool {
	return isZero(C.LAPACKE_sgebrd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&tauq[0]), (*C.float)(&taup[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgebrd.f.
func Dgebrd(m int, n int, a []float64, lda int, d []float64, e []float64, tauq []float64, taup []float64) bool {
	return isZero(C.LAPACKE_dgebrd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&tauq[0]), (*C.double)(&taup[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgebrd.f.
func Cgebrd(m int, n int, a []complex64, lda int, d []float32, e []float32, tauq []complex64, taup []complex64) bool {
	return isZero(C.LAPACKE_cgebrd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&tauq[0]), (*C.lapack_complex_float)(&taup[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgebrd.f.
func Zgebrd(m int, n int, a []complex128, lda int, d []float64, e []float64, tauq []complex128, taup []complex128) bool {
	return isZero(C.LAPACKE_zgebrd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&tauq[0]), (*C.lapack_complex_double)(&taup[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgecon.f.
func Sgecon(norm byte, n int, a []float32, lda int, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_sgecon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgecon.f.
func Dgecon(norm byte, n int, a []float64, lda int, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_dgecon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgecon.f.
func Cgecon(norm byte, n int, a []complex64, lda int, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_cgecon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgecon.f.
func Zgecon(norm byte, n int, a []complex128, lda int, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_zgecon((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeequ.f.
func Sgeequ(m int, n int, a []float32, lda int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_sgeequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeequ.f.
func Dgeequ(m int, n int, a []float64, lda int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dgeequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeequ.f.
func Cgeequ(m int, n int, a []complex64, lda int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cgeequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeequ.f.
func Zgeequ(m int, n int, a []complex128, lda int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zgeequ((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeequb.f.
func Sgeequb(m int, n int, a []float32, lda int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_sgeequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeequb.f.
func Dgeequb(m int, n int, a []float64, lda int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dgeequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeequb.f.
func Cgeequb(m int, n int, a []complex64, lda int, r []float32, c []float32, rowcnd []float32, colcnd []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cgeequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&rowcnd[0]), (*C.float)(&colcnd[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeequb.f.
func Zgeequb(m int, n int, a []complex128, lda int, r []float64, c []float64, rowcnd []float64, colcnd []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zgeequb((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&rowcnd[0]), (*C.double)(&colcnd[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeev.f.
func Sgeev(jobvl lapack.Job, jobvr lapack.Job, n int, a []float32, lda int, wr []float32, wi []float32, vl []float32, ldvl int, vr []float32, ldvr int) bool {
	return isZero(C.LAPACKE_sgeev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&wr[0]), (*C.float)(&wi[0]), (*C.float)(&vl[0]), (C.lapack_int)(ldvl), (*C.float)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeev.f.
func Dgeev(jobvl lapack.Job, jobvr lapack.Job, n int, a []float64, lda int, wr []float64, wi []float64, vl []float64, ldvl int, vr []float64, ldvr int) bool {
	return isZero(C.LAPACKE_dgeev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&wr[0]), (*C.double)(&wi[0]), (*C.double)(&vl[0]), (C.lapack_int)(ldvl), (*C.double)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeev.f.
func Cgeev(jobvl lapack.Job, jobvr lapack.Job, n int, a []complex64, lda int, w []complex64, vl []complex64, ldvl int, vr []complex64, ldvr int) bool {
	return isZero(C.LAPACKE_cgeev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&w[0]), (*C.lapack_complex_float)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_float)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeev.f.
func Zgeev(jobvl lapack.Job, jobvr lapack.Job, n int, a []complex128, lda int, w []complex128, vl []complex128, ldvl int, vr []complex128, ldvr int) bool {
	return isZero(C.LAPACKE_zgeev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&w[0]), (*C.lapack_complex_double)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_double)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeevx.f.
func Sgeevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []float32, lda int, wr []float32, wi []float32, vl []float32, ldvl int, vr []float32, ldvr int, ilo []int32, ihi []int32, scale []float32, abnrm []float32, rconde []float32, rcondv []float32) bool {
	return isZero(C.LAPACKE_sgeevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&wr[0]), (*C.float)(&wi[0]), (*C.float)(&vl[0]), (C.lapack_int)(ldvl), (*C.float)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&scale[0]), (*C.float)(&abnrm[0]), (*C.float)(&rconde[0]), (*C.float)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeevx.f.
func Dgeevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []float64, lda int, wr []float64, wi []float64, vl []float64, ldvl int, vr []float64, ldvr int, ilo []int32, ihi []int32, scale []float64, abnrm []float64, rconde []float64, rcondv []float64) bool {
	return isZero(C.LAPACKE_dgeevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&wr[0]), (*C.double)(&wi[0]), (*C.double)(&vl[0]), (C.lapack_int)(ldvl), (*C.double)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&scale[0]), (*C.double)(&abnrm[0]), (*C.double)(&rconde[0]), (*C.double)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeevx.f.
func Cgeevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []complex64, lda int, w []complex64, vl []complex64, ldvl int, vr []complex64, ldvr int, ilo []int32, ihi []int32, scale []float32, abnrm []float32, rconde []float32, rcondv []float32) bool {
	return isZero(C.LAPACKE_cgeevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&w[0]), (*C.lapack_complex_float)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_float)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&scale[0]), (*C.float)(&abnrm[0]), (*C.float)(&rconde[0]), (*C.float)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeevx.f.
func Zgeevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []complex128, lda int, w []complex128, vl []complex128, ldvl int, vr []complex128, ldvr int, ilo []int32, ihi []int32, scale []float64, abnrm []float64, rconde []float64, rcondv []float64) bool {
	return isZero(C.LAPACKE_zgeevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&w[0]), (*C.lapack_complex_double)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_double)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&scale[0]), (*C.double)(&abnrm[0]), (*C.double)(&rconde[0]), (*C.double)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgehrd.f.
func Sgehrd(n int, ilo int, ihi int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgehrd((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgehrd.f.
func Dgehrd(n int, ilo int, ihi int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgehrd((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgehrd.f.
func Cgehrd(n int, ilo int, ihi int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgehrd((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgehrd.f.
func Zgehrd(n int, ilo int, ihi int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgehrd((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgejsv.f.
func Sgejsv(joba lapack.Job, jobu lapack.Job, jobv lapack.Job, jobr lapack.Job, jobt lapack.Job, jobp lapack.Job, m int, n int, a []float32, lda int, sva []float32, u []float32, ldu int, v []float32, ldv int, stat []float32, istat []int32) bool {
	return isZero(C.LAPACKE_sgejsv((C.int)(rowMajor), (C.char)(joba), (C.char)(jobu), (C.char)(jobv), (C.char)(jobr), (C.char)(jobt), (C.char)(jobp), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&sva[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&stat[0]), (*C.lapack_int)(&istat[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgejsv.f.
func Dgejsv(joba lapack.Job, jobu lapack.Job, jobv lapack.Job, jobr lapack.Job, jobt lapack.Job, jobp lapack.Job, m int, n int, a []float64, lda int, sva []float64, u []float64, ldu int, v []float64, ldv int, stat []float64, istat []int32) bool {
	return isZero(C.LAPACKE_dgejsv((C.int)(rowMajor), (C.char)(joba), (C.char)(jobu), (C.char)(jobv), (C.char)(jobr), (C.char)(jobt), (C.char)(jobp), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&sva[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&stat[0]), (*C.lapack_int)(&istat[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgelq2.f.
func Sgelq2(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgelq2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgelq2.f.
func Dgelq2(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgelq2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgelq2.f.
func Cgelq2(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgelq2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgelq2.f.
func Zgelq2(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgelq2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgelqf.f.
func Sgelqf(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgelqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgelqf.f.
func Dgelqf(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgelqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgelqf.f.
func Cgelqf(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgelqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgelqf.f.
func Zgelqf(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgelqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgels.f.
func Sgels(trans blas.Transpose, m int, n int, nrhs int, a []float32, lda int, b []float32, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgels((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgels.f.
func Dgels(trans blas.Transpose, m int, n int, nrhs int, a []float64, lda int, b []float64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgels((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgels.f.
func Cgels(trans blas.Transpose, m int, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgels((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgels.f.
func Zgels(trans blas.Transpose, m int, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgels((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgelsd.f.
func Sgelsd(m int, n int, nrhs int, a []float32, lda int, b []float32, ldb int, s []float32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_sgelsd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&s[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgelsd.f.
func Dgelsd(m int, n int, nrhs int, a []float64, lda int, b []float64, ldb int, s []float64, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_dgelsd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&s[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgelsd.f.
func Cgelsd(m int, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int, s []float32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_cgelsd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&s[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgelsd.f.
func Zgelsd(m int, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int, s []float64, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_zgelsd((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&s[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgelss.f.
func Sgelss(m int, n int, nrhs int, a []float32, lda int, b []float32, ldb int, s []float32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_sgelss((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&s[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgelss.f.
func Dgelss(m int, n int, nrhs int, a []float64, lda int, b []float64, ldb int, s []float64, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_dgelss((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&s[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgelss.f.
func Cgelss(m int, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int, s []float32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_cgelss((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&s[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgelss.f.
func Zgelss(m int, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int, s []float64, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_zgelss((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&s[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgelsy.f.
func Sgelsy(m int, n int, nrhs int, a []float32, lda int, b []float32, ldb int, jpvt []int32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_sgelsy((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&jpvt[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgelsy.f.
func Dgelsy(m int, n int, nrhs int, a []float64, lda int, b []float64, ldb int, jpvt []int32, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_dgelsy((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&jpvt[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgelsy.f.
func Cgelsy(m int, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int, jpvt []int32, rcond float32, rank []int32) bool {
	return isZero(C.LAPACKE_cgelsy((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&jpvt[0]), (C.float)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgelsy.f.
func Zgelsy(m int, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int, jpvt []int32, rcond float64, rank []int32) bool {
	return isZero(C.LAPACKE_zgelsy((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&jpvt[0]), (C.double)(rcond), (*C.lapack_int)(&rank[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqlf.f.
func Sgeqlf(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqlf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqlf.f.
func Dgeqlf(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqlf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqlf.f.
func Cgeqlf(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqlf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqlf.f.
func Zgeqlf(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqlf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqp3.f.
func Sgeqp3(m int, n int, a []float32, lda int, jpvt []int32, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqp3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqp3.f.
func Dgeqp3(m int, n int, a []float64, lda int, jpvt []int32, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqp3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqp3.f.
func Cgeqp3(m int, n int, a []complex64, lda int, jpvt []int32, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqp3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqp3.f.
func Zgeqp3(m int, n int, a []complex128, lda int, jpvt []int32, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqp3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqpf.f.
func Sgeqpf(m int, n int, a []float32, lda int, jpvt []int32, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqpf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqpf.f.
func Dgeqpf(m int, n int, a []float64, lda int, jpvt []int32, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqpf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqpf.f.
func Cgeqpf(m int, n int, a []complex64, lda int, jpvt []int32, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqpf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqpf.f.
func Zgeqpf(m int, n int, a []complex128, lda int, jpvt []int32, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqpf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&jpvt[0]), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqr2.f.
func Sgeqr2(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqr2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqr2.f.
func Dgeqr2(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqr2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqr2.f.
func Cgeqr2(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqr2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqr2.f.
func Zgeqr2(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqr2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqrf.f.
func Sgeqrf(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqrf.f.
func Dgeqrf(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqrf.f.
func Cgeqrf(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqrf.f.
func Zgeqrf(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqrfp.f.
func Sgeqrfp(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgeqrfp((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqrfp.f.
func Dgeqrfp(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgeqrfp((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqrfp.f.
func Cgeqrfp(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgeqrfp((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqrfp.f.
func Zgeqrfp(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgeqrfp((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgerfs.f.
func Sgerfs(trans blas.Transpose, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, ipiv []int32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgerfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgerfs.f.
func Dgerfs(trans blas.Transpose, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, ipiv []int32, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgerfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgerfs.f.
func Cgerfs(trans blas.Transpose, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgerfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgerfs.f.
func Zgerfs(trans blas.Transpose, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgerfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgerqf.f.
func Sgerqf(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sgerqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgerqf.f.
func Dgerqf(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dgerqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgerqf.f.
func Cgerqf(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cgerqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgerqf.f.
func Zgerqf(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zgerqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgesdd.f.
func Sgesdd(jobz lapack.Job, m int, n int, a []float32, lda int, s []float32, u []float32, ldu int, vt []float32, ldvt int) bool {
	return isZero(C.LAPACKE_sgesdd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&vt[0]), (C.lapack_int)(ldvt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgesdd.f.
func Dgesdd(jobz lapack.Job, m int, n int, a []float64, lda int, s []float64, u []float64, ldu int, vt []float64, ldvt int) bool {
	return isZero(C.LAPACKE_dgesdd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&vt[0]), (C.lapack_int)(ldvt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgesdd.f.
func Cgesdd(jobz lapack.Job, m int, n int, a []complex64, lda int, s []float32, u []complex64, ldu int, vt []complex64, ldvt int) bool {
	return isZero(C.LAPACKE_cgesdd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&vt[0]), (C.lapack_int)(ldvt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgesdd.f.
func Zgesdd(jobz lapack.Job, m int, n int, a []complex128, lda int, s []float64, u []complex128, ldu int, vt []complex128, ldvt int) bool {
	return isZero(C.LAPACKE_zgesdd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&vt[0]), (C.lapack_int)(ldvt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgesv.f.
func Sgesv(n int, nrhs int, a []float32, lda int, ipiv []int32, b []float32, ldb int) bool {
	return isZero(C.LAPACKE_sgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgesv.f.
func Dgesv(n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int) bool {
	return isZero(C.LAPACKE_dgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgesv.f.
func Cgesv(n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	return isZero(C.LAPACKE_cgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgesv.f.
func Zgesv(n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	return isZero(C.LAPACKE_zgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsgesv.f.
func Dsgesv(n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int, x []float64, ldx int, iter []int32) bool {
	return isZero(C.LAPACKE_dsgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&iter[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zcgesv.f.
func Zcgesv(n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, iter []int32) bool {
	return isZero(C.LAPACKE_zcgesv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&iter[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgesvd.f.
func Sgesvd(jobu lapack.Job, jobvt lapack.Job, m int, n int, a []float32, lda int, s []float32, u []float32, ldu int, vt []float32, ldvt int, superb []float32) bool {
	return isZero(C.LAPACKE_sgesvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobvt), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&vt[0]), (C.lapack_int)(ldvt), (*C.float)(&superb[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgesvd.f.
func Dgesvd(jobu lapack.Job, jobvt lapack.Job, m int, n int, a []float64, lda int, s []float64, u []float64, ldu int, vt []float64, ldvt int, superb []float64) bool {
	return isZero(C.LAPACKE_dgesvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobvt), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&vt[0]), (C.lapack_int)(ldvt), (*C.double)(&superb[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgesvd.f.
func Cgesvd(jobu lapack.Job, jobvt lapack.Job, m int, n int, a []complex64, lda int, s []float32, u []complex64, ldu int, vt []complex64, ldvt int, superb []float32) bool {
	return isZero(C.LAPACKE_cgesvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobvt), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&vt[0]), (C.lapack_int)(ldvt), (*C.float)(&superb[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgesvd.f.
func Zgesvd(jobu lapack.Job, jobvt lapack.Job, m int, n int, a []complex128, lda int, s []float64, u []complex128, ldu int, vt []complex128, ldvt int, superb []float64) bool {
	return isZero(C.LAPACKE_zgesvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobvt), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&vt[0]), (C.lapack_int)(ldvt), (*C.double)(&superb[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgesvj.f.
func Sgesvj(joba lapack.Job, jobu lapack.Job, jobv lapack.Job, m int, n int, a []float32, lda int, sva []float32, mv int, v []float32, ldv int, stat []float32) bool {
	return isZero(C.LAPACKE_sgesvj((C.int)(rowMajor), (C.char)(joba), (C.char)(jobu), (C.char)(jobv), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&sva[0]), (C.lapack_int)(mv), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&stat[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgesvj.f.
func Dgesvj(joba lapack.Job, jobu lapack.Job, jobv lapack.Job, m int, n int, a []float64, lda int, sva []float64, mv int, v []float64, ldv int, stat []float64) bool {
	return isZero(C.LAPACKE_dgesvj((C.int)(rowMajor), (C.char)(joba), (C.char)(jobu), (C.char)(jobv), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&sva[0]), (C.lapack_int)(mv), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&stat[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgesvx.f.
func Sgesvx(fact byte, trans blas.Transpose, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, ipiv []int32, equed []byte, r []float32, c []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32, rpivot []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0]), (*C.float)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgesvx.f.
func Dgesvx(fact byte, trans blas.Transpose, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, ipiv []int32, equed []byte, r []float64, c []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64, rpivot []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0]), (*C.double)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgesvx.f.
func Cgesvx(fact byte, trans blas.Transpose, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, equed []byte, r []float32, c []float32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32, rpivot []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&r[0]), (*C.float)(&c[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0]), (*C.float)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgesvx.f.
func Zgesvx(fact byte, trans blas.Transpose, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, equed []byte, r []float64, c []float64, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64, rpivot []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&r[0]), (*C.double)(&c[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0]), (*C.double)(&rpivot[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgetf2.f.
func Sgetf2(m int, n int, a []float32, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_sgetf2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgetf2.f.
func Dgetf2(m int, n int, a []float64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_dgetf2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgetf2.f.
func Cgetf2(m int, n int, a []complex64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_cgetf2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgetf2.f.
func Zgetf2(m int, n int, a []complex128, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_zgetf2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgetrf.f.
func Sgetrf(m int, n int, a []float32, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_sgetrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgetrf.f.
func Dgetrf(m int, n int, a []float64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_dgetrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgetrf.f.
func Cgetrf(m int, n int, a []complex64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_cgetrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgetrf.f.
func Zgetrf(m int, n int, a []complex128, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_zgetrf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgetri.f.
func Sgetri(n int, a []float32, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_sgetri((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgetri.f.
func Dgetri(n int, a []float64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_dgetri((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgetri.f.
func Cgetri(n int, a []complex64, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_cgetri((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgetri.f.
func Zgetri(n int, a []complex128, lda int, ipiv []int32) bool {
	return isZero(C.LAPACKE_zgetri((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgetrs.f.
func Sgetrs(trans blas.Transpose, n int, nrhs int, a []float32, lda int, ipiv []int32, b []float32, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgetrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgetrs.f.
func Dgetrs(trans blas.Transpose, n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgetrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgetrs.f.
func Cgetrs(trans blas.Transpose, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgetrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgetrs.f.
func Zgetrs(trans blas.Transpose, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgetrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggbak.f.
func Sggbak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, lscale []float32, rscale []float32, m int, v []float32, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_sggbak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&lscale[0]), (*C.float)(&rscale[0]), (C.lapack_int)(m), (*C.float)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggbak.f.
func Dggbak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, lscale []float64, rscale []float64, m int, v []float64, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_dggbak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&lscale[0]), (*C.double)(&rscale[0]), (C.lapack_int)(m), (*C.double)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggbak.f.
func Cggbak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, lscale []float32, rscale []float32, m int, v []complex64, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_cggbak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&lscale[0]), (*C.float)(&rscale[0]), (C.lapack_int)(m), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggbak.f.
func Zggbak(job lapack.Job, s blas.Side, n int, ilo int, ihi int, lscale []float64, rscale []float64, m int, v []complex128, ldv int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_zggbak((C.int)(rowMajor), (C.char)(job), (C.char)(s), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&lscale[0]), (*C.double)(&rscale[0]), (C.lapack_int)(m), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggbal.f.
func Sggbal(job lapack.Job, n int, a []float32, lda int, b []float32, ldb int, ilo []int32, ihi []int32, lscale []float32, rscale []float32) bool {
	return isZero(C.LAPACKE_sggbal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&lscale[0]), (*C.float)(&rscale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggbal.f.
func Dggbal(job lapack.Job, n int, a []float64, lda int, b []float64, ldb int, ilo []int32, ihi []int32, lscale []float64, rscale []float64) bool {
	return isZero(C.LAPACKE_dggbal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&lscale[0]), (*C.double)(&rscale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggbal.f.
func Cggbal(job lapack.Job, n int, a []complex64, lda int, b []complex64, ldb int, ilo []int32, ihi []int32, lscale []float32, rscale []float32) bool {
	return isZero(C.LAPACKE_cggbal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&lscale[0]), (*C.float)(&rscale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggbal.f.
func Zggbal(job lapack.Job, n int, a []complex128, lda int, b []complex128, ldb int, ilo []int32, ihi []int32, lscale []float64, rscale []float64) bool {
	return isZero(C.LAPACKE_zggbal((C.int)(rowMajor), (C.char)(job), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&lscale[0]), (*C.double)(&rscale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggev.f.
func Sggev(jobvl lapack.Job, jobvr lapack.Job, n int, a []float32, lda int, b []float32, ldb int, alphar []float32, alphai []float32, beta []float32, vl []float32, ldvl int, vr []float32, ldvr int) bool {
	return isZero(C.LAPACKE_sggev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&alphar[0]), (*C.float)(&alphai[0]), (*C.float)(&beta[0]), (*C.float)(&vl[0]), (C.lapack_int)(ldvl), (*C.float)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggev.f.
func Dggev(jobvl lapack.Job, jobvr lapack.Job, n int, a []float64, lda int, b []float64, ldb int, alphar []float64, alphai []float64, beta []float64, vl []float64, ldvl int, vr []float64, ldvr int) bool {
	return isZero(C.LAPACKE_dggev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&alphar[0]), (*C.double)(&alphai[0]), (*C.double)(&beta[0]), (*C.double)(&vl[0]), (C.lapack_int)(ldvl), (*C.double)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggev.f.
func Cggev(jobvl lapack.Job, jobvr lapack.Job, n int, a []complex64, lda int, b []complex64, ldb int, alpha []complex64, beta []complex64, vl []complex64, ldvl int, vr []complex64, ldvr int) bool {
	return isZero(C.LAPACKE_cggev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&alpha[0]), (*C.lapack_complex_float)(&beta[0]), (*C.lapack_complex_float)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_float)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggev.f.
func Zggev(jobvl lapack.Job, jobvr lapack.Job, n int, a []complex128, lda int, b []complex128, ldb int, alpha []complex128, beta []complex128, vl []complex128, ldvl int, vr []complex128, ldvr int) bool {
	return isZero(C.LAPACKE_zggev((C.int)(rowMajor), (C.char)(jobvl), (C.char)(jobvr), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&alpha[0]), (*C.lapack_complex_double)(&beta[0]), (*C.lapack_complex_double)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_double)(&vr[0]), (C.lapack_int)(ldvr)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggevx.f.
func Sggevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []float32, lda int, b []float32, ldb int, alphar []float32, alphai []float32, beta []float32, vl []float32, ldvl int, vr []float32, ldvr int, ilo []int32, ihi []int32, lscale []float32, rscale []float32, abnrm []float32, bbnrm []float32, rconde []float32, rcondv []float32) bool {
	return isZero(C.LAPACKE_sggevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&alphar[0]), (*C.float)(&alphai[0]), (*C.float)(&beta[0]), (*C.float)(&vl[0]), (C.lapack_int)(ldvl), (*C.float)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&lscale[0]), (*C.float)(&rscale[0]), (*C.float)(&abnrm[0]), (*C.float)(&bbnrm[0]), (*C.float)(&rconde[0]), (*C.float)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggevx.f.
func Dggevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []float64, lda int, b []float64, ldb int, alphar []float64, alphai []float64, beta []float64, vl []float64, ldvl int, vr []float64, ldvr int, ilo []int32, ihi []int32, lscale []float64, rscale []float64, abnrm []float64, bbnrm []float64, rconde []float64, rcondv []float64) bool {
	return isZero(C.LAPACKE_dggevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&alphar[0]), (*C.double)(&alphai[0]), (*C.double)(&beta[0]), (*C.double)(&vl[0]), (C.lapack_int)(ldvl), (*C.double)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&lscale[0]), (*C.double)(&rscale[0]), (*C.double)(&abnrm[0]), (*C.double)(&bbnrm[0]), (*C.double)(&rconde[0]), (*C.double)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggevx.f.
func Cggevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []complex64, lda int, b []complex64, ldb int, alpha []complex64, beta []complex64, vl []complex64, ldvl int, vr []complex64, ldvr int, ilo []int32, ihi []int32, lscale []float32, rscale []float32, abnrm []float32, bbnrm []float32, rconde []float32, rcondv []float32) bool {
	return isZero(C.LAPACKE_cggevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&alpha[0]), (*C.lapack_complex_float)(&beta[0]), (*C.lapack_complex_float)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_float)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.float)(&lscale[0]), (*C.float)(&rscale[0]), (*C.float)(&abnrm[0]), (*C.float)(&bbnrm[0]), (*C.float)(&rconde[0]), (*C.float)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggevx.f.
func Zggevx(balanc byte, jobvl lapack.Job, jobvr lapack.Job, sense byte, n int, a []complex128, lda int, b []complex128, ldb int, alpha []complex128, beta []complex128, vl []complex128, ldvl int, vr []complex128, ldvr int, ilo []int32, ihi []int32, lscale []float64, rscale []float64, abnrm []float64, bbnrm []float64, rconde []float64, rcondv []float64) bool {
	return isZero(C.LAPACKE_zggevx((C.int)(rowMajor), (C.char)(balanc), (C.char)(jobvl), (C.char)(jobvr), (C.char)(sense), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&alpha[0]), (*C.lapack_complex_double)(&beta[0]), (*C.lapack_complex_double)(&vl[0]), (C.lapack_int)(ldvl), (*C.lapack_complex_double)(&vr[0]), (C.lapack_int)(ldvr), (*C.lapack_int)(&ilo[0]), (*C.lapack_int)(&ihi[0]), (*C.double)(&lscale[0]), (*C.double)(&rscale[0]), (*C.double)(&abnrm[0]), (*C.double)(&bbnrm[0]), (*C.double)(&rconde[0]), (*C.double)(&rcondv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggglm.f.
func Sggglm(n int, m int, p int, a []float32, lda int, b []float32, ldb int, d []float32, x []float32, y []float32) bool {
	return isZero(C.LAPACKE_sggglm((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&d[0]), (*C.float)(&x[0]), (*C.float)(&y[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggglm.f.
func Dggglm(n int, m int, p int, a []float64, lda int, b []float64, ldb int, d []float64, x []float64, y []float64) bool {
	return isZero(C.LAPACKE_dggglm((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&d[0]), (*C.double)(&x[0]), (*C.double)(&y[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggglm.f.
func Cggglm(n int, m int, p int, a []complex64, lda int, b []complex64, ldb int, d []complex64, x []complex64, y []complex64) bool {
	return isZero(C.LAPACKE_cggglm((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&x[0]), (*C.lapack_complex_float)(&y[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggglm.f.
func Zggglm(n int, m int, p int, a []complex128, lda int, b []complex128, ldb int, d []complex128, x []complex128, y []complex128) bool {
	return isZero(C.LAPACKE_zggglm((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&x[0]), (*C.lapack_complex_double)(&y[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgghrd.f.
func Sgghrd(compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, a []float32, lda int, b []float32, ldb int, q []float32, ldq int, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_sgghrd((C.int)(rowMajor), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgghrd.f.
func Dgghrd(compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, a []float64, lda int, b []float64, ldb int, q []float64, ldq int, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dgghrd((C.int)(rowMajor), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgghrd.f.
func Cgghrd(compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, a []complex64, lda int, b []complex64, ldb int, q []complex64, ldq int, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_cgghrd((C.int)(rowMajor), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgghrd.f.
func Zgghrd(compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, a []complex128, lda int, b []complex128, ldb int, q []complex128, ldq int, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zgghrd((C.int)(rowMajor), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgglse.f.
func Sgglse(m int, n int, p int, a []float32, lda int, b []float32, ldb int, c []float32, d []float32, x []float32) bool {
	return isZero(C.LAPACKE_sgglse((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&c[0]), (*C.float)(&d[0]), (*C.float)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgglse.f.
func Dgglse(m int, n int, p int, a []float64, lda int, b []float64, ldb int, c []float64, d []float64, x []float64) bool {
	return isZero(C.LAPACKE_dgglse((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&c[0]), (*C.double)(&d[0]), (*C.double)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgglse.f.
func Cgglse(m int, n int, p int, a []complex64, lda int, b []complex64, ldb int, c []complex64, d []complex64, x []complex64) bool {
	return isZero(C.LAPACKE_cgglse((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&c[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgglse.f.
func Zgglse(m int, n int, p int, a []complex128, lda int, b []complex128, ldb int, c []complex128, d []complex128, x []complex128) bool {
	return isZero(C.LAPACKE_zgglse((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&c[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggqrf.f.
func Sggqrf(n int, m int, p int, a []float32, lda int, taua []float32, b []float32, ldb int, taub []float32) bool {
	return isZero(C.LAPACKE_sggqrf((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&taua[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggqrf.f.
func Dggqrf(n int, m int, p int, a []float64, lda int, taua []float64, b []float64, ldb int, taub []float64) bool {
	return isZero(C.LAPACKE_dggqrf((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&taua[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggqrf.f.
func Cggqrf(n int, m int, p int, a []complex64, lda int, taua []complex64, b []complex64, ldb int, taub []complex64) bool {
	return isZero(C.LAPACKE_cggqrf((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&taua[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggqrf.f.
func Zggqrf(n int, m int, p int, a []complex128, lda int, taua []complex128, b []complex128, ldb int, taub []complex128) bool {
	return isZero(C.LAPACKE_zggqrf((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(m), (C.lapack_int)(p), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&taua[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggrqf.f.
func Sggrqf(m int, p int, n int, a []float32, lda int, taua []float32, b []float32, ldb int, taub []float32) bool {
	return isZero(C.LAPACKE_sggrqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&taua[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggrqf.f.
func Dggrqf(m int, p int, n int, a []float64, lda int, taua []float64, b []float64, ldb int, taub []float64) bool {
	return isZero(C.LAPACKE_dggrqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&taua[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggrqf.f.
func Cggrqf(m int, p int, n int, a []complex64, lda int, taua []complex64, b []complex64, ldb int, taub []complex64) bool {
	return isZero(C.LAPACKE_cggrqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&taua[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggrqf.f.
func Zggrqf(m int, p int, n int, a []complex128, lda int, taua []complex128, b []complex128, ldb int, taub []complex128) bool {
	return isZero(C.LAPACKE_zggrqf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&taua[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&taub[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggsvd.f.
func Sggsvd(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, n int, p int, k []int32, l []int32, a []float32, lda int, b []float32, ldb int, alpha []float32, beta []float32, u []float32, ldu int, v []float32, ldv int, q []float32, ldq int, iwork []int32) bool {
	return isZero(C.LAPACKE_sggsvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&alpha[0]), (*C.float)(&beta[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&iwork[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggsvd.f.
func Dggsvd(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, n int, p int, k []int32, l []int32, a []float64, lda int, b []float64, ldb int, alpha []float64, beta []float64, u []float64, ldu int, v []float64, ldv int, q []float64, ldq int, iwork []int32) bool {
	return isZero(C.LAPACKE_dggsvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&alpha[0]), (*C.double)(&beta[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&iwork[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggsvd.f.
func Cggsvd(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, n int, p int, k []int32, l []int32, a []complex64, lda int, b []complex64, ldb int, alpha []float32, beta []float32, u []complex64, ldu int, v []complex64, ldv int, q []complex64, ldq int, iwork []int32) bool {
	return isZero(C.LAPACKE_cggsvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&alpha[0]), (*C.float)(&beta[0]), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&iwork[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggsvd.f.
func Zggsvd(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, n int, p int, k []int32, l []int32, a []complex128, lda int, b []complex128, ldb int, alpha []float64, beta []float64, u []complex128, ldu int, v []complex128, ldv int, q []complex128, ldq int, iwork []int32) bool {
	return isZero(C.LAPACKE_zggsvd((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(p), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&alpha[0]), (*C.double)(&beta[0]), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&iwork[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sggsvp.f.
func Sggsvp(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, a []float32, lda int, b []float32, ldb int, tola float32, tolb float32, k []int32, l []int32, u []float32, ldu int, v []float32, ldv int, q []float32, ldq int) bool {
	return isZero(C.LAPACKE_sggsvp((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (C.float)(tola), (C.float)(tolb), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dggsvp.f.
func Dggsvp(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, a []float64, lda int, b []float64, ldb int, tola float64, tolb float64, k []int32, l []int32, u []float64, ldu int, v []float64, ldv int, q []float64, ldq int) bool {
	return isZero(C.LAPACKE_dggsvp((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (C.double)(tola), (C.double)(tolb), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cggsvp.f.
func Cggsvp(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, a []complex64, lda int, b []complex64, ldb int, tola float32, tolb float32, k []int32, l []int32, u []complex64, ldu int, v []complex64, ldv int, q []complex64, ldq int) bool {
	return isZero(C.LAPACKE_cggsvp((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (C.float)(tola), (C.float)(tolb), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zggsvp.f.
func Zggsvp(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, a []complex128, lda int, b []complex128, ldb int, tola float64, tolb float64, k []int32, l []int32, u []complex128, ldu int, v []complex128, ldv int, q []complex128, ldq int) bool {
	return isZero(C.LAPACKE_zggsvp((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (C.double)(tola), (C.double)(tolb), (*C.lapack_int)(&k[0]), (*C.lapack_int)(&l[0]), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgtcon.f.
func Sgtcon(norm byte, n int, dl []float32, d []float32, du []float32, du2 []float32, ipiv []int32, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_sgtcon((C.char)(norm), (C.lapack_int)(n), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgtcon.f.
func Dgtcon(norm byte, n int, dl []float64, d []float64, du []float64, du2 []float64, ipiv []int32, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_dgtcon((C.char)(norm), (C.lapack_int)(n), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgtcon.f.
func Cgtcon(norm byte, n int, dl []complex64, d []complex64, du []complex64, du2 []complex64, ipiv []int32, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_cgtcon((C.char)(norm), (C.lapack_int)(n), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgtcon.f.
func Zgtcon(norm byte, n int, dl []complex128, d []complex128, du []complex128, du2 []complex128, ipiv []int32, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_zgtcon((C.char)(norm), (C.lapack_int)(n), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgtrfs.f.
func Sgtrfs(trans blas.Transpose, n int, nrhs int, dl []float32, d []float32, du []float32, dlf []float32, df []float32, duf []float32, du2 []float32, ipiv []int32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgtrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&dlf[0]), (*C.float)(&df[0]), (*C.float)(&duf[0]), (*C.float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgtrfs.f.
func Dgtrfs(trans blas.Transpose, n int, nrhs int, dl []float64, d []float64, du []float64, dlf []float64, df []float64, duf []float64, du2 []float64, ipiv []int32, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgtrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&dlf[0]), (*C.double)(&df[0]), (*C.double)(&duf[0]), (*C.double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgtrfs.f.
func Cgtrfs(trans blas.Transpose, n int, nrhs int, dl []complex64, d []complex64, du []complex64, dlf []complex64, df []complex64, duf []complex64, du2 []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgtrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&dlf[0]), (*C.lapack_complex_float)(&df[0]), (*C.lapack_complex_float)(&duf[0]), (*C.lapack_complex_float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgtrfs.f.
func Zgtrfs(trans blas.Transpose, n int, nrhs int, dl []complex128, d []complex128, du []complex128, dlf []complex128, df []complex128, duf []complex128, du2 []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgtrfs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&dlf[0]), (*C.lapack_complex_double)(&df[0]), (*C.lapack_complex_double)(&duf[0]), (*C.lapack_complex_double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgtsv.f.
func Sgtsv(n int, nrhs int, dl []float32, d []float32, du []float32, b []float32, ldb int) bool {
	return isZero(C.LAPACKE_sgtsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgtsv.f.
func Dgtsv(n int, nrhs int, dl []float64, d []float64, du []float64, b []float64, ldb int) bool {
	return isZero(C.LAPACKE_dgtsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgtsv.f.
func Cgtsv(n int, nrhs int, dl []complex64, d []complex64, du []complex64, b []complex64, ldb int) bool {
	return isZero(C.LAPACKE_cgtsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgtsv.f.
func Zgtsv(n int, nrhs int, dl []complex128, d []complex128, du []complex128, b []complex128, ldb int) bool {
	return isZero(C.LAPACKE_zgtsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgtsvx.f.
func Sgtsvx(fact byte, trans blas.Transpose, n int, nrhs int, dl []float32, d []float32, du []float32, dlf []float32, df []float32, duf []float32, du2 []float32, ipiv []int32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgtsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&dlf[0]), (*C.float)(&df[0]), (*C.float)(&duf[0]), (*C.float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgtsvx.f.
func Dgtsvx(fact byte, trans blas.Transpose, n int, nrhs int, dl []float64, d []float64, du []float64, dlf []float64, df []float64, duf []float64, du2 []float64, ipiv []int32, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgtsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&dlf[0]), (*C.double)(&df[0]), (*C.double)(&duf[0]), (*C.double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgtsvx.f.
func Cgtsvx(fact byte, trans blas.Transpose, n int, nrhs int, dl []complex64, d []complex64, du []complex64, dlf []complex64, df []complex64, duf []complex64, du2 []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgtsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&dlf[0]), (*C.lapack_complex_float)(&df[0]), (*C.lapack_complex_float)(&duf[0]), (*C.lapack_complex_float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgtsvx.f.
func Zgtsvx(fact byte, trans blas.Transpose, n int, nrhs int, dl []complex128, d []complex128, du []complex128, dlf []complex128, df []complex128, duf []complex128, du2 []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgtsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&dlf[0]), (*C.lapack_complex_double)(&df[0]), (*C.lapack_complex_double)(&duf[0]), (*C.lapack_complex_double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgttrf.f.
func Sgttrf(n int, dl []float32, d []float32, du []float32, du2 []float32, ipiv []int32) bool {
	return isZero(C.LAPACKE_sgttrf((C.lapack_int)(n), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&du2[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgttrf.f.
func Dgttrf(n int, dl []float64, d []float64, du []float64, du2 []float64, ipiv []int32) bool {
	return isZero(C.LAPACKE_dgttrf((C.lapack_int)(n), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&du2[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgttrf.f.
func Cgttrf(n int, dl []complex64, d []complex64, du []complex64, du2 []complex64, ipiv []int32) bool {
	return isZero(C.LAPACKE_cgttrf((C.lapack_int)(n), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&du2[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgttrf.f.
func Zgttrf(n int, dl []complex128, d []complex128, du []complex128, du2 []complex128, ipiv []int32) bool {
	return isZero(C.LAPACKE_zgttrf((C.lapack_int)(n), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&du2[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgttrs.f.
func Sgttrs(trans blas.Transpose, n int, nrhs int, dl []float32, d []float32, du []float32, du2 []float32, ipiv []int32, b []float32, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgttrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&dl[0]), (*C.float)(&d[0]), (*C.float)(&du[0]), (*C.float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgttrs.f.
func Dgttrs(trans blas.Transpose, n int, nrhs int, dl []float64, d []float64, du []float64, du2 []float64, ipiv []int32, b []float64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgttrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&dl[0]), (*C.double)(&d[0]), (*C.double)(&du[0]), (*C.double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgttrs.f.
func Cgttrs(trans blas.Transpose, n int, nrhs int, dl []complex64, d []complex64, du []complex64, du2 []complex64, ipiv []int32, b []complex64, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgttrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&dl[0]), (*C.lapack_complex_float)(&d[0]), (*C.lapack_complex_float)(&du[0]), (*C.lapack_complex_float)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgttrs.f.
func Zgttrs(trans blas.Transpose, n int, nrhs int, dl []complex128, d []complex128, du []complex128, du2 []complex128, ipiv []int32, b []complex128, ldb int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgttrs((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&dl[0]), (*C.lapack_complex_double)(&d[0]), (*C.lapack_complex_double)(&du[0]), (*C.lapack_complex_double)(&du2[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbev.f.
func Chbev(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []complex64, ldab int, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbev.f.
func Zhbev(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []complex128, ldab int, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbevd.f.
func Chbevd(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []complex64, ldab int, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbevd.f.
func Zhbevd(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []complex128, ldab int, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbevx.f.
func Chbevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, kd int, ab []complex64, ldab int, q []complex64, ldq int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbevx.f.
func Zhbevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, kd int, ab []complex128, ldab int, q []complex128, ldq int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbgst.f.
func Chbgst(vect byte, ul blas.Uplo, n int, ka int, kb int, ab []complex64, ldab int, bb []complex64, ldbb int, x []complex64, ldx int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbgst((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&bb[0]), (C.lapack_int)(ldbb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbgst.f.
func Zhbgst(vect byte, ul blas.Uplo, n int, ka int, kb int, ab []complex128, ldab int, bb []complex128, ldbb int, x []complex128, ldx int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbgst((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&bb[0]), (C.lapack_int)(ldbb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbgv.f.
func Chbgv(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []complex64, ldab int, bb []complex64, ldbb int, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbgv((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbgv.f.
func Zhbgv(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []complex128, ldab int, bb []complex128, ldbb int, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbgv((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbgvd.f.
func Chbgvd(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []complex64, ldab int, bb []complex64, ldbb int, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbgvd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbgvd.f.
func Zhbgvd(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []complex128, ldab int, bb []complex128, ldbb int, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbgvd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbgvx.f.
func Chbgvx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ka int, kb int, ab []complex64, ldab int, bb []complex64, ldbb int, q []complex64, ldq int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbgvx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&bb[0]), (C.lapack_int)(ldbb), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbgvx.f.
func Zhbgvx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ka int, kb int, ab []complex128, ldab int, bb []complex128, ldbb int, q []complex128, ldq int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbgvx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&bb[0]), (C.lapack_int)(ldbb), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chbtrd.f.
func Chbtrd(vect byte, ul blas.Uplo, n int, kd int, ab []complex64, ldab int, d []float32, e []float32, q []complex64, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chbtrd((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhbtrd.f.
func Zhbtrd(vect byte, ul blas.Uplo, n int, kd int, ab []complex128, ldab int, d []float64, e []float64, q []complex128, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhbtrd((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/checon.f.
func Checon(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_checon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhecon.f.
func Zhecon(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhecon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheequb.f.
func Cheequb(ul blas.Uplo, n int, a []complex64, lda int, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheequb.f.
func Zheequb(ul blas.Uplo, n int, a []complex128, lda int, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheev.f.
func Cheev(jobz lapack.Job, ul blas.Uplo, n int, a []complex64, lda int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheev.f.
func Zheev(jobz lapack.Job, ul blas.Uplo, n int, a []complex128, lda int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheevd.f.
func Cheevd(jobz lapack.Job, ul blas.Uplo, n int, a []complex64, lda int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheevd.f.
func Zheevd(jobz lapack.Job, ul blas.Uplo, n int, a []complex128, lda int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheevr.f.
func Cheevr(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex64, lda int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, isuppz []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheevr.f.
func Zheevr(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex128, lda int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, isuppz []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheevx.f.
func Cheevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex64, lda int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheevx.f.
func Zheevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex128, lda int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chegst.f.
func Chegst(itype int, ul blas.Uplo, n int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chegst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhegst.f.
func Zhegst(itype int, ul blas.Uplo, n int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhegst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chegv.f.
func Chegv(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []complex64, lda int, b []complex64, ldb int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chegv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhegv.f.
func Zhegv(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []complex128, lda int, b []complex128, ldb int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhegv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chegvd.f.
func Chegvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []complex64, lda int, b []complex64, ldb int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chegvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhegvd.f.
func Zhegvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []complex128, lda int, b []complex128, ldb int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhegvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chegvx.f.
func Chegvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex64, lda int, b []complex64, ldb int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chegvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhegvx.f.
func Zhegvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []complex128, lda int, b []complex128, ldb int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhegvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cherfs.f.
func Cherfs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cherfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zherfs.f.
func Zherfs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zherfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chesv.f.
func Chesv(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chesv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhesv.f.
func Zhesv(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhesv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chesvx.f.
func Chesvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhesvx.f.
func Zhesvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhesvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetrd.f.
func Chetrd(ul blas.Uplo, n int, a []complex64, lda int, d []float32, e []float32, tau []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetrd.f.
func Zhetrd(ul blas.Uplo, n int, a []complex128, lda int, d []float64, e []float64, tau []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetrf.f.
func Chetrf(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetrf.f.
func Zhetrf(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetri.f.
func Chetri(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetri.f.
func Zhetri(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetrs.f.
func Chetrs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetrs.f.
func Zhetrs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chfrk.f.
func Chfrk(transr blas.Transpose, ul blas.Uplo, trans blas.Transpose, n int, k int, alpha float32, a []complex64, lda int, beta float32, c []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_chfrk((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(k), (C.float)(alpha), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.float)(beta), (*C.lapack_complex_float)(&c[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhfrk.f.
func Zhfrk(transr blas.Transpose, ul blas.Uplo, trans blas.Transpose, n int, k int, alpha float64, a []complex128, lda int, beta float64, c []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zhfrk((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(k), (C.double)(alpha), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.double)(beta), (*C.lapack_complex_double)(&c[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/shgeqz.f.
func Shgeqz(job lapack.Job, compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, h []float32, ldh int, t []float32, ldt int, alphar []float32, alphai []float32, beta []float32, q []float32, ldq int, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_shgeqz((C.int)(rowMajor), (C.char)(job), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&h[0]), (C.lapack_int)(ldh), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&alphar[0]), (*C.float)(&alphai[0]), (*C.float)(&beta[0]), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dhgeqz.f.
func Dhgeqz(job lapack.Job, compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, h []float64, ldh int, t []float64, ldt int, alphar []float64, alphai []float64, beta []float64, q []float64, ldq int, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dhgeqz((C.int)(rowMajor), (C.char)(job), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&h[0]), (C.lapack_int)(ldh), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&alphar[0]), (*C.double)(&alphai[0]), (*C.double)(&beta[0]), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chgeqz.f.
func Chgeqz(job lapack.Job, compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, h []complex64, ldh int, t []complex64, ldt int, alpha []complex64, beta []complex64, q []complex64, ldq int, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_chgeqz((C.int)(rowMajor), (C.char)(job), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&h[0]), (C.lapack_int)(ldh), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&alpha[0]), (*C.lapack_complex_float)(&beta[0]), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhgeqz.f.
func Zhgeqz(job lapack.Job, compq lapack.CompSV, compz lapack.CompSV, n int, ilo int, ihi int, h []complex128, ldh int, t []complex128, ldt int, alpha []complex128, beta []complex128, q []complex128, ldq int, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zhgeqz((C.int)(rowMajor), (C.char)(job), (C.char)(compq), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&h[0]), (C.lapack_int)(ldh), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&alpha[0]), (*C.lapack_complex_double)(&beta[0]), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpcon.f.
func Chpcon(ul blas.Uplo, n int, ap []complex64, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpcon.f.
func Zhpcon(ul blas.Uplo, n int, ap []complex128, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpev.f.
func Chpev(jobz lapack.Job, ul blas.Uplo, n int, ap []complex64, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpev.f.
func Zhpev(jobz lapack.Job, ul blas.Uplo, n int, ap []complex128, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpevd.f.
func Chpevd(jobz lapack.Job, ul blas.Uplo, n int, ap []complex64, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpevd.f.
func Zhpevd(jobz lapack.Job, ul blas.Uplo, n int, ap []complex128, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpevx.f.
func Chpevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []complex64, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpevx.f.
func Zhpevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []complex128, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpgst.f.
func Chpgst(itype int, ul blas.Uplo, n int, ap []complex64, bp []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpgst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&bp[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpgst.f.
func Zhpgst(itype int, ul blas.Uplo, n int, ap []complex128, bp []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpgst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&bp[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpgv.f.
func Chpgv(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []complex64, bp []complex64, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpgv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&bp[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpgv.f.
func Zhpgv(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []complex128, bp []complex128, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpgv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&bp[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpgvd.f.
func Chpgvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []complex64, bp []complex64, w []float32, z []complex64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpgvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&bp[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpgvd.f.
func Zhpgvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []complex128, bp []complex128, w []float64, z []complex128, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpgvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&bp[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpgvx.f.
func Chpgvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []complex64, bp []complex64, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpgvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&bp[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpgvx.f.
func Zhpgvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []complex128, bp []complex128, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpgvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&bp[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chprfs.f.
func Chprfs(ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhprfs.f.
func Zhprfs(ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpsv.f.
func Chpsv(ul blas.Uplo, n int, nrhs int, ap []complex64, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpsv.f.
func Zhpsv(ul blas.Uplo, n int, nrhs int, ap []complex128, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chpsvx.f.
func Chpsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chpsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhpsvx.f.
func Zhpsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhpsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chptrd.f.
func Chptrd(ul blas.Uplo, n int, ap []complex64, d []float32, e []float32, tau []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chptrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhptrd.f.
func Zhptrd(ul blas.Uplo, n int, ap []complex128, d []float64, e []float64, tau []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhptrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chptrf.f.
func Chptrf(ul blas.Uplo, n int, ap []complex64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhptrf.f.
func Zhptrf(ul blas.Uplo, n int, ap []complex128, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chptri.f.
func Chptri(ul blas.Uplo, n int, ap []complex64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhptri.f.
func Zhptri(ul blas.Uplo, n int, ap []complex128, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chptrs.f.
func Chptrs(ul blas.Uplo, n int, nrhs int, ap []complex64, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhptrs.f.
func Zhptrs(ul blas.Uplo, n int, nrhs int, ap []complex128, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/shseqr.f.
func Shseqr(job lapack.Job, compz lapack.CompSV, n int, ilo int, ihi int, h []float32, ldh int, wr []float32, wi []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_shseqr((C.int)(rowMajor), (C.char)(job), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&h[0]), (C.lapack_int)(ldh), (*C.float)(&wr[0]), (*C.float)(&wi[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dhseqr.f.
func Dhseqr(job lapack.Job, compz lapack.CompSV, n int, ilo int, ihi int, h []float64, ldh int, wr []float64, wi []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dhseqr((C.int)(rowMajor), (C.char)(job), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&h[0]), (C.lapack_int)(ldh), (*C.double)(&wr[0]), (*C.double)(&wi[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chseqr.f.
func Chseqr(job lapack.Job, compz lapack.CompSV, n int, ilo int, ihi int, h []complex64, ldh int, w []complex64, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_chseqr((C.int)(rowMajor), (C.char)(job), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&h[0]), (C.lapack_int)(ldh), (*C.lapack_complex_float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhseqr.f.
func Zhseqr(job lapack.Job, compz lapack.CompSV, n int, ilo int, ihi int, h []complex128, ldh int, w []complex128, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zhseqr((C.int)(rowMajor), (C.char)(job), (C.char)(compz), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&h[0]), (C.lapack_int)(ldh), (*C.lapack_complex_double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clacgv.f.
func Clacgv(n int, x []complex64, incx int) bool {
	return isZero(C.LAPACKE_clacgv((C.lapack_int)(n), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlacgv.f.
func Zlacgv(n int, x []complex128, incx int) bool {
	return isZero(C.LAPACKE_zlacgv((C.lapack_int)(n), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slacpy.f.
func Slacpy(ul blas.Uplo, m int, n int, a []float32, lda int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_slacpy((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlacpy.f.
func Dlacpy(ul blas.Uplo, m int, n int, a []float64, lda int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dlacpy((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clacpy.f.
func Clacpy(ul blas.Uplo, m int, n int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_clacpy((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlacpy.f.
func Zlacpy(ul blas.Uplo, m int, n int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zlacpy((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slamch.f.
func Slamch(cmach byte) float32 {
	return float32(C.LAPACKE_slamch((C.char)(cmach)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlamch.f.
func Dlamch(cmach byte) float64 {
	return float64(C.LAPACKE_dlamch((C.char)(cmach)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slange.f.
func Slange(norm byte, m int, n int, a []float32, lda int) float32 {
	return float32(C.LAPACKE_slange((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlange.f.
func Dlange(norm byte, m int, n int, a []float64, lda int) float64 {
	return float64(C.LAPACKE_dlange((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clange.f.
func Clange(norm byte, m int, n int, a []complex64, lda int) float32 {
	return float32(C.LAPACKE_clange((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlange.f.
func Zlange(norm byte, m int, n int, a []complex128, lda int) float64 {
	return float64(C.LAPACKE_zlange((C.int)(rowMajor), (C.char)(norm), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clanhe.f.
func Clanhe(norm byte, ul blas.Uplo, n int, a []complex64, lda int) float32 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float32(C.LAPACKE_clanhe((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlanhe.f.
func Zlanhe(norm byte, ul blas.Uplo, n int, a []complex128, lda int) float64 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float64(C.LAPACKE_zlanhe((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slansy.f.
func Slansy(norm byte, ul blas.Uplo, n int, a []float32, lda int) float32 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float32(C.LAPACKE_slansy((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlansy.f.
func Dlansy(norm byte, ul blas.Uplo, n int, a []float64, lda int) float64 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float64(C.LAPACKE_dlansy((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clansy.f.
func Clansy(norm byte, ul blas.Uplo, n int, a []complex64, lda int) float32 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float32(C.LAPACKE_clansy((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlansy.f.
func Zlansy(norm byte, ul blas.Uplo, n int, a []complex128, lda int) float64 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return float64(C.LAPACKE_zlansy((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slantr.f.
func Slantr(norm byte, ul blas.Uplo, d blas.Diag, m int, n int, a []float32, lda int) float32 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return float32(C.LAPACKE_slantr((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlantr.f.
func Dlantr(norm byte, ul blas.Uplo, d blas.Diag, m int, n int, a []float64, lda int) float64 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return float64(C.LAPACKE_dlantr((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clantr.f.
func Clantr(norm byte, ul blas.Uplo, d blas.Diag, m int, n int, a []complex64, lda int) float32 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return float32(C.LAPACKE_clantr((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlantr.f.
func Zlantr(norm byte, ul blas.Uplo, d blas.Diag, m int, n int, a []complex128, lda int) float64 {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return float64(C.LAPACKE_zlantr((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slarfb.f.
func Slarfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, v []float32, ldv int, t []float32, ldt int, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_slarfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlarfb.f.
func Dlarfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, v []float64, ldv int, t []float64, ldt int, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dlarfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clarfb.f.
func Clarfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, v []complex64, ldv int, t []complex64, ldt int, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_clarfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlarfb.f.
func Zlarfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, v []complex128, ldv int, t []complex128, ldt int, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zlarfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slarfg.f.
func Slarfg(n int, alpha []float32, x []float32, incx int, tau []float32) bool {
	return isZero(C.LAPACKE_slarfg((C.lapack_int)(n), (*C.float)(&alpha[0]), (*C.float)(&x[0]), (C.lapack_int)(incx), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlarfg.f.
func Dlarfg(n int, alpha []float64, x []float64, incx int, tau []float64) bool {
	return isZero(C.LAPACKE_dlarfg((C.lapack_int)(n), (*C.double)(&alpha[0]), (*C.double)(&x[0]), (C.lapack_int)(incx), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clarfg.f.
func Clarfg(n int, alpha []complex64, x []complex64, incx int, tau []complex64) bool {
	return isZero(C.LAPACKE_clarfg((C.lapack_int)(n), (*C.lapack_complex_float)(&alpha[0]), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(incx), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlarfg.f.
func Zlarfg(n int, alpha []complex128, x []complex128, incx int, tau []complex128) bool {
	return isZero(C.LAPACKE_zlarfg((C.lapack_int)(n), (*C.lapack_complex_double)(&alpha[0]), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(incx), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slarft.f.
func Slarft(direct byte, storev byte, n int, k int, v []float32, ldv int, tau []float32, t []float32, ldt int) bool {
	return isZero(C.LAPACKE_slarft((C.int)(rowMajor), (C.char)(direct), (C.char)(storev), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&tau[0]), (*C.float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlarft.f.
func Dlarft(direct byte, storev byte, n int, k int, v []float64, ldv int, tau []float64, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dlarft((C.int)(rowMajor), (C.char)(direct), (C.char)(storev), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&tau[0]), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clarft.f.
func Clarft(direct byte, storev byte, n int, k int, v []complex64, ldv int, tau []complex64, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_clarft((C.int)(rowMajor), (C.char)(direct), (C.char)(storev), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlarft.f.
func Zlarft(direct byte, storev byte, n int, k int, v []complex128, ldv int, tau []complex128, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_zlarft((C.int)(rowMajor), (C.char)(direct), (C.char)(storev), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slarfx.f.
func Slarfx(s blas.Side, m int, n int, v []float32, tau float32, c []float32, ldc int, work []float32) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_slarfx((C.int)(rowMajor), (C.char)(s), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&v[0]), (C.float)(tau), (*C.float)(&c[0]), (C.lapack_int)(ldc), (*C.float)(&work[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlarfx.f.
func Dlarfx(s blas.Side, m int, n int, v []float64, tau float64, c []float64, ldc int, work []float64) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_dlarfx((C.int)(rowMajor), (C.char)(s), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&v[0]), (C.double)(tau), (*C.double)(&c[0]), (C.lapack_int)(ldc), (*C.double)(&work[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clarfx.f.
func Clarfx(s blas.Side, m int, n int, v []complex64, tau complex64, c []complex64, ldc int, work []complex64) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_clarfx((C.int)(rowMajor), (C.char)(s), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&v[0]), (C.lapack_complex_float)(tau), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc), (*C.lapack_complex_float)(&work[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlarfx.f.
func Zlarfx(s blas.Side, m int, n int, v []complex128, tau complex128, c []complex128, ldc int, work []complex128) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	return isZero(C.LAPACKE_zlarfx((C.int)(rowMajor), (C.char)(s), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&v[0]), (C.lapack_complex_double)(tau), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc), (*C.lapack_complex_double)(&work[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slarnv.f.
func Slarnv(idist int, iseed []int32, n int, x []float32) bool {
	return isZero(C.LAPACKE_slarnv((C.lapack_int)(idist), (*C.lapack_int)(&iseed[0]), (C.lapack_int)(n), (*C.float)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlarnv.f.
func Dlarnv(idist int, iseed []int32, n int, x []float64) bool {
	return isZero(C.LAPACKE_dlarnv((C.lapack_int)(idist), (*C.lapack_int)(&iseed[0]), (C.lapack_int)(n), (*C.double)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clarnv.f.
func Clarnv(idist int, iseed []int32, n int, x []complex64) bool {
	return isZero(C.LAPACKE_clarnv((C.lapack_int)(idist), (*C.lapack_int)(&iseed[0]), (C.lapack_int)(n), (*C.lapack_complex_float)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlarnv.f.
func Zlarnv(idist int, iseed []int32, n int, x []complex128) bool {
	return isZero(C.LAPACKE_zlarnv((C.lapack_int)(idist), (*C.lapack_int)(&iseed[0]), (C.lapack_int)(n), (*C.lapack_complex_double)(&x[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slaset.f.
func Slaset(ul blas.Uplo, m int, n int, alpha float32, beta float32, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_slaset((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (C.float)(alpha), (C.float)(beta), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlaset.f.
func Dlaset(ul blas.Uplo, m int, n int, alpha float64, beta float64, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dlaset((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (C.double)(alpha), (C.double)(beta), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/claset.f.
func Claset(ul blas.Uplo, m int, n int, alpha complex64, beta complex64, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_claset((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_complex_float)(alpha), (C.lapack_complex_float)(beta), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlaset.f.
func Zlaset(ul blas.Uplo, m int, n int, alpha complex128, beta complex128, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zlaset((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_complex_double)(alpha), (C.lapack_complex_double)(beta), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slasrt.f.
func Slasrt(id byte, n int, d []float32) bool {
	return isZero(C.LAPACKE_slasrt((C.char)(id), (C.lapack_int)(n), (*C.float)(&d[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlasrt.f.
func Dlasrt(id byte, n int, d []float64) bool {
	return isZero(C.LAPACKE_dlasrt((C.char)(id), (C.lapack_int)(n), (*C.double)(&d[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slaswp.f.
func Slaswp(n int, a []float32, lda int, k1 int, k2 int, ipiv []int32, incx int) bool {
	return isZero(C.LAPACKE_slaswp((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.lapack_int)(k1), (C.lapack_int)(k2), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlaswp.f.
func Dlaswp(n int, a []float64, lda int, k1 int, k2 int, ipiv []int32, incx int) bool {
	return isZero(C.LAPACKE_dlaswp((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.lapack_int)(k1), (C.lapack_int)(k2), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/claswp.f.
func Claswp(n int, a []complex64, lda int, k1 int, k2 int, ipiv []int32, incx int) bool {
	return isZero(C.LAPACKE_claswp((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.lapack_int)(k1), (C.lapack_int)(k2), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlaswp.f.
func Zlaswp(n int, a []complex128, lda int, k1 int, k2 int, ipiv []int32, incx int) bool {
	return isZero(C.LAPACKE_zlaswp((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.lapack_int)(k1), (C.lapack_int)(k2), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(incx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slauum.f.
func Slauum(ul blas.Uplo, n int, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_slauum((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlauum.f.
func Dlauum(ul blas.Uplo, n int, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dlauum((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clauum.f.
func Clauum(ul blas.Uplo, n int, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_clauum((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlauum.f.
func Zlauum(ul blas.Uplo, n int, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zlauum((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sopgtr.f.
func Sopgtr(ul blas.Uplo, n int, ap []float32, tau []float32, q []float32, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sopgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&tau[0]), (*C.float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dopgtr.f.
func Dopgtr(ul blas.Uplo, n int, ap []float64, tau []float64, q []float64, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dopgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&tau[0]), (*C.double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sopmtr.f.
func Sopmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, ap []float32, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sopmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dopmtr.f.
func Dopmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, ap []float64, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dopmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorgbr.f.
func Sorgbr(vect byte, m int, n int, k int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorgbr((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorgbr.f.
func Dorgbr(vect byte, m int, n int, k int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorgbr((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorghr.f.
func Sorghr(n int, ilo int, ihi int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorghr((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorghr.f.
func Dorghr(n int, ilo int, ihi int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorghr((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorglq.f.
func Sorglq(m int, n int, k int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorglq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorglq.f.
func Dorglq(m int, n int, k int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorglq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorgql.f.
func Sorgql(m int, n int, k int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorgql((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorgql.f.
func Dorgql(m int, n int, k int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorgql((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorgqr.f.
func Sorgqr(m int, n int, k int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorgqr((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorgqr.f.
func Dorgqr(m int, n int, k int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorgqr((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorgrq.f.
func Sorgrq(m int, n int, k int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_sorgrq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorgrq.f.
func Dorgrq(m int, n int, k int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dorgrq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorgtr.f.
func Sorgtr(ul blas.Uplo, n int, a []float32, lda int, tau []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sorgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorgtr.f.
func Dorgtr(ul blas.Uplo, n int, a []float64, lda int, tau []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dorgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormbr.f.
func Sormbr(vect byte, s blas.Side, trans blas.Transpose, m int, n int, k int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormbr((C.int)(rowMajor), (C.char)(vect), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormbr.f.
func Dormbr(vect byte, s blas.Side, trans blas.Transpose, m int, n int, k int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormbr((C.int)(rowMajor), (C.char)(vect), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormhr.f.
func Sormhr(s blas.Side, trans blas.Transpose, m int, n int, ilo int, ihi int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormhr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormhr.f.
func Dormhr(s blas.Side, trans blas.Transpose, m int, n int, ilo int, ihi int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormhr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormlq.f.
func Sormlq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormlq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormlq.f.
func Dormlq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormlq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormql.f.
func Sormql(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormql((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormql.f.
func Dormql(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormql((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormqr.f.
func Sormqr(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormqr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormqr.f.
func Dormqr(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormqr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormrq.f.
func Sormrq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormrq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormrq.f.
func Dormrq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormrq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormrz.f.
func Sormrz(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormrz((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormrz.f.
func Dormrz(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormrz((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sormtr.f.
func Sormtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, a []float32, lda int, tau []float32, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sormtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0]), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dormtr.f.
func Dormtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, a []float64, lda int, tau []float64, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dormtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0]), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbcon.f.
func Spbcon(ul blas.Uplo, n int, kd int, ab []float32, ldab int, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbcon.f.
func Dpbcon(ul blas.Uplo, n int, kd int, ab []float64, ldab int, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbcon.f.
func Cpbcon(ul blas.Uplo, n int, kd int, ab []complex64, ldab int, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbcon.f.
func Zpbcon(ul blas.Uplo, n int, kd int, ab []complex128, ldab int, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbequ.f.
func Spbequ(ul blas.Uplo, n int, kd int, ab []float32, ldab int, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbequ.f.
func Dpbequ(ul blas.Uplo, n int, kd int, ab []float64, ldab int, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbequ.f.
func Cpbequ(ul blas.Uplo, n int, kd int, ab []complex64, ldab int, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbequ.f.
func Zpbequ(ul blas.Uplo, n int, kd int, ab []complex128, ldab int, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbrfs.f.
func Spbrfs(ul blas.Uplo, n int, kd int, nrhs int, ab []float32, ldab int, afb []float32, ldafb int, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&afb[0]), (C.lapack_int)(ldafb), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbrfs.f.
func Dpbrfs(ul blas.Uplo, n int, kd int, nrhs int, ab []float64, ldab int, afb []float64, ldafb int, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&afb[0]), (C.lapack_int)(ldafb), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbrfs.f.
func Cpbrfs(ul blas.Uplo, n int, kd int, nrhs int, ab []complex64, ldab int, afb []complex64, ldafb int, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbrfs.f.
func Zpbrfs(ul blas.Uplo, n int, kd int, nrhs int, ab []complex128, ldab int, afb []complex128, ldafb int, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&afb[0]), (C.lapack_int)(ldafb), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbstf.f.
func Spbstf(ul blas.Uplo, n int, kb int, bb []float32, ldbb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbstf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kb), (*C.float)(&bb[0]), (C.lapack_int)(ldbb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbstf.f.
func Dpbstf(ul blas.Uplo, n int, kb int, bb []float64, ldbb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbstf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kb), (*C.double)(&bb[0]), (C.lapack_int)(ldbb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbstf.f.
func Cpbstf(ul blas.Uplo, n int, kb int, bb []complex64, ldbb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbstf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kb), (*C.lapack_complex_float)(&bb[0]), (C.lapack_int)(ldbb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbstf.f.
func Zpbstf(ul blas.Uplo, n int, kb int, bb []complex128, ldbb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbstf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kb), (*C.lapack_complex_double)(&bb[0]), (C.lapack_int)(ldbb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbsv.f.
func Spbsv(ul blas.Uplo, n int, kd int, nrhs int, ab []float32, ldab int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbsv.f.
func Dpbsv(ul blas.Uplo, n int, kd int, nrhs int, ab []float64, ldab int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbsv.f.
func Cpbsv(ul blas.Uplo, n int, kd int, nrhs int, ab []complex64, ldab int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbsv.f.
func Zpbsv(ul blas.Uplo, n int, kd int, nrhs int, ab []complex128, ldab int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbsvx.f.
func Spbsvx(fact byte, ul blas.Uplo, n int, kd int, nrhs int, ab []float32, ldab int, afb []float32, ldafb int, equed []byte, s []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&afb[0]), (C.lapack_int)(ldafb), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbsvx.f.
func Dpbsvx(fact byte, ul blas.Uplo, n int, kd int, nrhs int, ab []float64, ldab int, afb []float64, ldafb int, equed []byte, s []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&afb[0]), (C.lapack_int)(ldafb), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbsvx.f.
func Cpbsvx(fact byte, ul blas.Uplo, n int, kd int, nrhs int, ab []complex64, ldab int, afb []complex64, ldafb int, equed []byte, s []float32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&afb[0]), (C.lapack_int)(ldafb), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbsvx.f.
func Zpbsvx(fact byte, ul blas.Uplo, n int, kd int, nrhs int, ab []complex128, ldab int, afb []complex128, ldafb int, equed []byte, s []float64, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&afb[0]), (C.lapack_int)(ldafb), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbtrf.f.
func Spbtrf(ul blas.Uplo, n int, kd int, ab []float32, ldab int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbtrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbtrf.f.
func Dpbtrf(ul blas.Uplo, n int, kd int, ab []float64, ldab int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbtrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbtrf.f.
func Cpbtrf(ul blas.Uplo, n int, kd int, ab []complex64, ldab int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbtrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbtrf.f.
func Zpbtrf(ul blas.Uplo, n int, kd int, ab []complex128, ldab int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbtrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spbtrs.f.
func Spbtrs(ul blas.Uplo, n int, kd int, nrhs int, ab []float32, ldab int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spbtrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpbtrs.f.
func Dpbtrs(ul blas.Uplo, n int, kd int, nrhs int, ab []float64, ldab int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpbtrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpbtrs.f.
func Cpbtrs(ul blas.Uplo, n int, kd int, nrhs int, ab []complex64, ldab int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpbtrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpbtrs.f.
func Zpbtrs(ul blas.Uplo, n int, kd int, nrhs int, ab []complex128, ldab int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpbtrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spftrf.f.
func Spftrf(transr blas.Transpose, ul blas.Uplo, n int, a []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spftrf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpftrf.f.
func Dpftrf(transr blas.Transpose, ul blas.Uplo, n int, a []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpftrf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpftrf.f.
func Cpftrf(transr blas.Transpose, ul blas.Uplo, n int, a []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpftrf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpftrf.f.
func Zpftrf(transr blas.Transpose, ul blas.Uplo, n int, a []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpftrf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spftri.f.
func Spftri(transr blas.Transpose, ul blas.Uplo, n int, a []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpftri.f.
func Dpftri(transr blas.Transpose, ul blas.Uplo, n int, a []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpftri.f.
func Cpftri(transr blas.Transpose, ul blas.Uplo, n int, a []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpftri.f.
func Zpftri(transr blas.Transpose, ul blas.Uplo, n int, a []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spftrs.f.
func Spftrs(transr blas.Transpose, ul blas.Uplo, n int, nrhs int, a []float32, b []float32, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spftrs((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpftrs.f.
func Dpftrs(transr blas.Transpose, ul blas.Uplo, n int, nrhs int, a []float64, b []float64, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpftrs((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpftrs.f.
func Cpftrs(transr blas.Transpose, ul blas.Uplo, n int, nrhs int, a []complex64, b []complex64, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpftrs((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpftrs.f.
func Zpftrs(transr blas.Transpose, ul blas.Uplo, n int, nrhs int, a []complex128, b []complex128, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpftrs((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spocon.f.
func Spocon(ul blas.Uplo, n int, a []float32, lda int, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spocon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpocon.f.
func Dpocon(ul blas.Uplo, n int, a []float64, lda int, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpocon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpocon.f.
func Cpocon(ul blas.Uplo, n int, a []complex64, lda int, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpocon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpocon.f.
func Zpocon(ul blas.Uplo, n int, a []complex128, lda int, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpocon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spoequ.f.
func Spoequ(n int, a []float32, lda int, s []float32, scond []float32, amax []float32) bool {
	return isZero(C.LAPACKE_spoequ((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpoequ.f.
func Dpoequ(n int, a []float64, lda int, s []float64, scond []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dpoequ((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpoequ.f.
func Cpoequ(n int, a []complex64, lda int, s []float32, scond []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cpoequ((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpoequ.f.
func Zpoequ(n int, a []complex128, lda int, s []float64, scond []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zpoequ((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spoequb.f.
func Spoequb(n int, a []float32, lda int, s []float32, scond []float32, amax []float32) bool {
	return isZero(C.LAPACKE_spoequb((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpoequb.f.
func Dpoequb(n int, a []float64, lda int, s []float64, scond []float64, amax []float64) bool {
	return isZero(C.LAPACKE_dpoequb((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpoequb.f.
func Cpoequb(n int, a []complex64, lda int, s []float32, scond []float32, amax []float32) bool {
	return isZero(C.LAPACKE_cpoequb((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpoequb.f.
func Zpoequb(n int, a []complex128, lda int, s []float64, scond []float64, amax []float64) bool {
	return isZero(C.LAPACKE_zpoequb((C.int)(rowMajor), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sporfs.f.
func Sporfs(ul blas.Uplo, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sporfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dporfs.f.
func Dporfs(ul blas.Uplo, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dporfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cporfs.f.
func Cporfs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cporfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zporfs.f.
func Zporfs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zporfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sposv.f.
func Sposv(ul blas.Uplo, n int, nrhs int, a []float32, lda int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dposv.f.
func Dposv(ul blas.Uplo, n int, nrhs int, a []float64, lda int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cposv.f.
func Cposv(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zposv.f.
func Zposv(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsposv.f.
func Dsposv(ul blas.Uplo, n int, nrhs int, a []float64, lda int, b []float64, ldb int, x []float64, ldx int, iter []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&iter[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zcposv.f.
func Zcposv(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int, x []complex128, ldx int, iter []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zcposv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&iter[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sposvx.f.
func Sposvx(fact byte, ul blas.Uplo, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, equed []byte, s []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sposvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dposvx.f.
func Dposvx(fact byte, ul blas.Uplo, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, equed []byte, s []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dposvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cposvx.f.
func Cposvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, equed []byte, s []float32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cposvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zposvx.f.
func Zposvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, equed []byte, s []float64, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zposvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spotrf.f.
func Spotrf(ul blas.Uplo, n int, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spotrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpotrf.f.
func Dpotrf(ul blas.Uplo, n int, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpotrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpotrf.f.
func Cpotrf(ul blas.Uplo, n int, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpotrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpotrf.f.
func Zpotrf(ul blas.Uplo, n int, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpotrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spotri.f.
func Spotri(ul blas.Uplo, n int, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spotri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpotri.f.
func Dpotri(ul blas.Uplo, n int, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpotri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpotri.f.
func Cpotri(ul blas.Uplo, n int, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpotri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpotri.f.
func Zpotri(ul blas.Uplo, n int, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpotri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spotrs.f.
func Spotrs(ul blas.Uplo, n int, nrhs int, a []float32, lda int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spotrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpotrs.f.
func Dpotrs(ul blas.Uplo, n int, nrhs int, a []float64, lda int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpotrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpotrs.f.
func Cpotrs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpotrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpotrs.f.
func Zpotrs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpotrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sppcon.f.
func Sppcon(ul blas.Uplo, n int, ap []float32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sppcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dppcon.f.
func Dppcon(ul blas.Uplo, n int, ap []float64, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dppcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cppcon.f.
func Cppcon(ul blas.Uplo, n int, ap []complex64, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cppcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zppcon.f.
func Zppcon(ul blas.Uplo, n int, ap []complex128, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zppcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sppequ.f.
func Sppequ(ul blas.Uplo, n int, ap []float32, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sppequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dppequ.f.
func Dppequ(ul blas.Uplo, n int, ap []float64, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dppequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cppequ.f.
func Cppequ(ul blas.Uplo, n int, ap []complex64, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cppequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zppequ.f.
func Zppequ(ul blas.Uplo, n int, ap []complex128, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zppequ((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spprfs.f.
func Spprfs(ul blas.Uplo, n int, nrhs int, ap []float32, afp []float32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&afp[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpprfs.f.
func Dpprfs(ul blas.Uplo, n int, nrhs int, ap []float64, afp []float64, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&afp[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpprfs.f.
func Cpprfs(ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpprfs.f.
func Zpprfs(ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sppsv.f.
func Sppsv(ul blas.Uplo, n int, nrhs int, ap []float32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sppsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dppsv.f.
func Dppsv(ul blas.Uplo, n int, nrhs int, ap []float64, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dppsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cppsv.f.
func Cppsv(ul blas.Uplo, n int, nrhs int, ap []complex64, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cppsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zppsv.f.
func Zppsv(ul blas.Uplo, n int, nrhs int, ap []complex128, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zppsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sppsvx.f.
func Sppsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []float32, afp []float32, equed []byte, s []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sppsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&afp[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dppsvx.f.
func Dppsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []float64, afp []float64, equed []byte, s []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dppsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&afp[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cppsvx.f.
func Cppsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, equed []byte, s []float32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cppsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.float)(&s[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zppsvx.f.
func Zppsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, equed []byte, s []float64, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zppsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.char)(unsafe.Pointer(&equed[0])), (*C.double)(&s[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spptrf.f.
func Spptrf(ul blas.Uplo, n int, ap []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpptrf.f.
func Dpptrf(ul blas.Uplo, n int, ap []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpptrf.f.
func Cpptrf(ul blas.Uplo, n int, ap []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpptrf.f.
func Zpptrf(ul blas.Uplo, n int, ap []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spptri.f.
func Spptri(ul blas.Uplo, n int, ap []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpptri.f.
func Dpptri(ul blas.Uplo, n int, ap []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpptri.f.
func Cpptri(ul blas.Uplo, n int, ap []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpptri.f.
func Zpptri(ul blas.Uplo, n int, ap []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spptrs.f.
func Spptrs(ul blas.Uplo, n int, nrhs int, ap []float32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpptrs.f.
func Dpptrs(ul blas.Uplo, n int, nrhs int, ap []float64, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpptrs.f.
func Cpptrs(ul blas.Uplo, n int, nrhs int, ap []complex64, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpptrs.f.
func Zpptrs(ul blas.Uplo, n int, nrhs int, ap []complex128, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spstrf.f.
func Spstrf(ul blas.Uplo, n int, a []float32, lda int, piv []int32, rank []int32, tol float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_spstrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&piv[0]), (*C.lapack_int)(&rank[0]), (C.float)(tol)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpstrf.f.
func Dpstrf(ul blas.Uplo, n int, a []float64, lda int, piv []int32, rank []int32, tol float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dpstrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&piv[0]), (*C.lapack_int)(&rank[0]), (C.double)(tol)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpstrf.f.
func Cpstrf(ul blas.Uplo, n int, a []complex64, lda int, piv []int32, rank []int32, tol float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpstrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&piv[0]), (*C.lapack_int)(&rank[0]), (C.float)(tol)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpstrf.f.
func Zpstrf(ul blas.Uplo, n int, a []complex128, lda int, piv []int32, rank []int32, tol float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpstrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&piv[0]), (*C.lapack_int)(&rank[0]), (C.double)(tol)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sptcon.f.
func Sptcon(n int, d []float32, e []float32, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_sptcon((C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dptcon.f.
func Dptcon(n int, d []float64, e []float64, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_dptcon((C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cptcon.f.
func Cptcon(n int, d []float32, e []complex64, anorm float32, rcond []float32) bool {
	return isZero(C.LAPACKE_cptcon((C.lapack_int)(n), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zptcon.f.
func Zptcon(n int, d []float64, e []complex128, anorm float64, rcond []float64) bool {
	return isZero(C.LAPACKE_zptcon((C.lapack_int)(n), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spteqr.f.
func Spteqr(compz lapack.CompSV, n int, d []float32, e []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_spteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpteqr.f.
func Dpteqr(compz lapack.CompSV, n int, d []float64, e []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dpteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpteqr.f.
func Cpteqr(compz lapack.CompSV, n int, d []float32, e []float32, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_cpteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpteqr.f.
func Zpteqr(compz lapack.CompSV, n int, d []float64, e []float64, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zpteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sptrfs.f.
func Sptrfs(n int, nrhs int, d []float32, e []float32, df []float32, ef []float32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	return isZero(C.LAPACKE_sptrfs((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&df[0]), (*C.float)(&ef[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dptrfs.f.
func Dptrfs(n int, nrhs int, d []float64, e []float64, df []float64, ef []float64, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	return isZero(C.LAPACKE_dptrfs((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&df[0]), (*C.double)(&ef[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cptrfs.f.
func Cptrfs(ul blas.Uplo, n int, nrhs int, d []float32, e []complex64, df []float32, ef []complex64, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cptrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0]), (*C.float)(&df[0]), (*C.lapack_complex_float)(&ef[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zptrfs.f.
func Zptrfs(ul blas.Uplo, n int, nrhs int, d []float64, e []complex128, df []float64, ef []complex128, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zptrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0]), (*C.double)(&df[0]), (*C.lapack_complex_double)(&ef[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sptsv.f.
func Sptsv(n int, nrhs int, d []float32, e []float32, b []float32, ldb int) bool {
	return isZero(C.LAPACKE_sptsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dptsv.f.
func Dptsv(n int, nrhs int, d []float64, e []float64, b []float64, ldb int) bool {
	return isZero(C.LAPACKE_dptsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cptsv.f.
func Cptsv(n int, nrhs int, d []float32, e []complex64, b []complex64, ldb int) bool {
	return isZero(C.LAPACKE_cptsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zptsv.f.
func Zptsv(n int, nrhs int, d []float64, e []complex128, b []complex128, ldb int) bool {
	return isZero(C.LAPACKE_zptsv((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sptsvx.f.
func Sptsvx(fact byte, n int, nrhs int, d []float32, e []float32, df []float32, ef []float32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	return isZero(C.LAPACKE_sptsvx((C.int)(rowMajor), (C.char)(fact), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&df[0]), (*C.float)(&ef[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dptsvx.f.
func Dptsvx(fact byte, n int, nrhs int, d []float64, e []float64, df []float64, ef []float64, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	return isZero(C.LAPACKE_dptsvx((C.int)(rowMajor), (C.char)(fact), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&df[0]), (*C.double)(&ef[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cptsvx.f.
func Cptsvx(fact byte, n int, nrhs int, d []float32, e []complex64, df []float32, ef []complex64, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	return isZero(C.LAPACKE_cptsvx((C.int)(rowMajor), (C.char)(fact), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0]), (*C.float)(&df[0]), (*C.lapack_complex_float)(&ef[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zptsvx.f.
func Zptsvx(fact byte, n int, nrhs int, d []float64, e []complex128, df []float64, ef []complex128, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	return isZero(C.LAPACKE_zptsvx((C.int)(rowMajor), (C.char)(fact), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0]), (*C.double)(&df[0]), (*C.lapack_complex_double)(&ef[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spttrf.f.
func Spttrf(n int, d []float32, e []float32) bool {
	return isZero(C.LAPACKE_spttrf((C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpttrf.f.
func Dpttrf(n int, d []float64, e []float64) bool {
	return isZero(C.LAPACKE_dpttrf((C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpttrf.f.
func Cpttrf(n int, d []float32, e []complex64) bool {
	return isZero(C.LAPACKE_cpttrf((C.lapack_int)(n), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpttrf.f.
func Zpttrf(n int, d []float64, e []complex128) bool {
	return isZero(C.LAPACKE_zpttrf((C.lapack_int)(n), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/spttrs.f.
func Spttrs(n int, nrhs int, d []float32, e []float32, b []float32, ldb int) bool {
	return isZero(C.LAPACKE_spttrs((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dpttrs.f.
func Dpttrs(n int, nrhs int, d []float64, e []float64, b []float64, ldb int) bool {
	return isZero(C.LAPACKE_dpttrs((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cpttrs.f.
func Cpttrs(ul blas.Uplo, n int, nrhs int, d []float32, e []complex64, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cpttrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&d[0]), (*C.lapack_complex_float)(&e[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zpttrs.f.
func Zpttrs(ul blas.Uplo, n int, nrhs int, d []float64, e []complex128, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zpttrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&d[0]), (*C.lapack_complex_double)(&e[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbev.f.
func Ssbev(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []float32, ldab int, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbev.f.
func Dsbev(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []float64, ldab int, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbevd.f.
func Ssbevd(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []float32, ldab int, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbevd.f.
func Dsbevd(jobz lapack.Job, ul blas.Uplo, n int, kd int, ab []float64, ldab int, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbevx.f.
func Ssbevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, kd int, ab []float32, ldab int, q []float32, ldq int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&q[0]), (C.lapack_int)(ldq), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbevx.f.
func Dsbevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, kd int, ab []float64, ldab int, q []float64, ldq int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&q[0]), (C.lapack_int)(ldq), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbgst.f.
func Ssbgst(vect byte, ul blas.Uplo, n int, ka int, kb int, ab []float32, ldab int, bb []float32, ldbb int, x []float32, ldx int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbgst((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&x[0]), (C.lapack_int)(ldx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbgst.f.
func Dsbgst(vect byte, ul blas.Uplo, n int, ka int, kb int, ab []float64, ldab int, bb []float64, ldbb int, x []float64, ldx int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbgst((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&x[0]), (C.lapack_int)(ldx)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbgv.f.
func Ssbgv(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []float32, ldab int, bb []float32, ldbb int, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbgv((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbgv.f.
func Dsbgv(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []float64, ldab int, bb []float64, ldbb int, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbgv((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbgvd.f.
func Ssbgvd(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []float32, ldab int, bb []float32, ldbb int, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbgvd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbgvd.f.
func Dsbgvd(jobz lapack.Job, ul blas.Uplo, n int, ka int, kb int, ab []float64, ldab int, bb []float64, ldbb int, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbgvd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbgvx.f.
func Ssbgvx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ka int, kb int, ab []float32, ldab int, bb []float32, ldbb int, q []float32, ldq int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbgvx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&bb[0]), (C.lapack_int)(ldbb), (*C.float)(&q[0]), (C.lapack_int)(ldq), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbgvx.f.
func Dsbgvx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ka int, kb int, ab []float64, ldab int, bb []float64, ldbb int, q []float64, ldq int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbgvx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(ka), (C.lapack_int)(kb), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&bb[0]), (C.lapack_int)(ldbb), (*C.double)(&q[0]), (C.lapack_int)(ldq), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssbtrd.f.
func Ssbtrd(vect byte, ul blas.Uplo, n int, kd int, ab []float32, ldab int, d []float32, e []float32, q []float32, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssbtrd((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsbtrd.f.
func Dsbtrd(vect byte, ul blas.Uplo, n int, kd int, ab []float64, ldab int, d []float64, e []float64, q []float64, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsbtrd((C.int)(rowMajor), (C.char)(vect), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssfrk.f.
func Ssfrk(transr blas.Transpose, ul blas.Uplo, trans blas.Transpose, n int, k int, alpha float32, a []float32, lda int, beta float32, c []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ssfrk((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(k), (C.float)(alpha), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.float)(beta), (*C.float)(&c[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsfrk.f.
func Dsfrk(transr blas.Transpose, ul blas.Uplo, trans blas.Transpose, n int, k int, alpha float64, a []float64, lda int, beta float64, c []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dsfrk((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(trans), (C.lapack_int)(n), (C.lapack_int)(k), (C.double)(alpha), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.double)(beta), (*C.double)(&c[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspcon.f.
func Sspcon(ul blas.Uplo, n int, ap []float32, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspcon.f.
func Dspcon(ul blas.Uplo, n int, ap []float64, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cspcon.f.
func Cspcon(ul blas.Uplo, n int, ap []complex64, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cspcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zspcon.f.
func Zspcon(ul blas.Uplo, n int, ap []complex128, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zspcon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspev.f.
func Sspev(jobz lapack.Job, ul blas.Uplo, n int, ap []float32, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspev.f.
func Dspev(jobz lapack.Job, ul blas.Uplo, n int, ap []float64, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspevd.f.
func Sspevd(jobz lapack.Job, ul blas.Uplo, n int, ap []float32, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspevd.f.
func Dspevd(jobz lapack.Job, ul blas.Uplo, n int, ap []float64, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspevx.f.
func Sspevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspevx.f.
func Dspevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspgst.f.
func Sspgst(itype int, ul blas.Uplo, n int, ap []float32, bp []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspgst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&bp[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspgst.f.
func Dspgst(itype int, ul blas.Uplo, n int, ap []float64, bp []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspgst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&bp[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspgv.f.
func Sspgv(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []float32, bp []float32, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspgv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&bp[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspgv.f.
func Dspgv(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []float64, bp []float64, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspgv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&bp[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspgvd.f.
func Sspgvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []float32, bp []float32, w []float32, z []float32, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspgvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&bp[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspgvd.f.
func Dspgvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, ap []float64, bp []float64, w []float64, z []float64, ldz int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspgvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&bp[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspgvx.f.
func Sspgvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []float32, bp []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspgvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&bp[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspgvx.f.
func Dspgvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, ap []float64, bp []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspgvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&bp[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssprfs.f.
func Ssprfs(ul blas.Uplo, n int, nrhs int, ap []float32, afp []float32, ipiv []int32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsprfs.f.
func Dsprfs(ul blas.Uplo, n int, nrhs int, ap []float64, afp []float64, ipiv []int32, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csprfs.f.
func Csprfs(ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsprfs.f.
func Zsprfs(ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsprfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspsv.f.
func Sspsv(ul blas.Uplo, n int, nrhs int, ap []float32, ipiv []int32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspsv.f.
func Dspsv(ul blas.Uplo, n int, nrhs int, ap []float64, ipiv []int32, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cspsv.f.
func Cspsv(ul blas.Uplo, n int, nrhs int, ap []complex64, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cspsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zspsv.f.
func Zspsv(ul blas.Uplo, n int, nrhs int, ap []complex128, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zspsv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sspsvx.f.
func Sspsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []float32, afp []float32, ipiv []int32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_sspsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dspsvx.f.
func Dspsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []float64, afp []float64, ipiv []int32, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dspsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cspsvx.f.
func Cspsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex64, afp []complex64, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cspsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zspsvx.f.
func Zspsvx(fact byte, ul blas.Uplo, n int, nrhs int, ap []complex128, afp []complex128, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zspsvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&afp[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssptrd.f.
func Ssptrd(ul blas.Uplo, n int, ap []float32, d []float32, e []float32, tau []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssptrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsptrd.f.
func Dsptrd(ul blas.Uplo, n int, ap []float64, d []float64, e []float64, tau []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsptrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssptrf.f.
func Ssptrf(ul blas.Uplo, n int, ap []float32, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsptrf.f.
func Dsptrf(ul blas.Uplo, n int, ap []float64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csptrf.f.
func Csptrf(ul blas.Uplo, n int, ap []complex64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsptrf.f.
func Zsptrf(ul blas.Uplo, n int, ap []complex128, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsptrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssptri.f.
func Ssptri(ul blas.Uplo, n int, ap []float32, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsptri.f.
func Dsptri(ul blas.Uplo, n int, ap []float64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csptri.f.
func Csptri(ul blas.Uplo, n int, ap []complex64, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsptri.f.
func Zsptri(ul blas.Uplo, n int, ap []complex128, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsptri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssptrs.f.
func Ssptrs(ul blas.Uplo, n int, nrhs int, ap []float32, ipiv []int32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsptrs.f.
func Dsptrs(ul blas.Uplo, n int, nrhs int, ap []float64, ipiv []int32, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csptrs.f.
func Csptrs(ul blas.Uplo, n int, nrhs int, ap []complex64, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsptrs.f.
func Zsptrs(ul blas.Uplo, n int, nrhs int, ap []complex128, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsptrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstebz.f.
func Sstebz(rng byte, order byte, n int, vl float32, vu float32, il int, iu int, abstol float32, d []float32, e []float32, m []int32, nsplit []int32, w []float32, iblock []int32, isplit []int32) bool {
	return isZero(C.LAPACKE_sstebz((C.char)(rng), (C.char)(order), (C.lapack_int)(n), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_int)(&m[0]), (*C.lapack_int)(&nsplit[0]), (*C.float)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstebz.f.
func Dstebz(rng byte, order byte, n int, vl float64, vu float64, il int, iu int, abstol float64, d []float64, e []float64, m []int32, nsplit []int32, w []float64, iblock []int32, isplit []int32) bool {
	return isZero(C.LAPACKE_dstebz((C.char)(rng), (C.char)(order), (C.lapack_int)(n), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_int)(&m[0]), (*C.lapack_int)(&nsplit[0]), (*C.double)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstedc.f.
func Sstedc(compz lapack.CompSV, n int, d []float32, e []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_sstedc((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstedc.f.
func Dstedc(compz lapack.CompSV, n int, d []float64, e []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dstedc((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cstedc.f.
func Cstedc(compz lapack.CompSV, n int, d []float32, e []float32, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_cstedc((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zstedc.f.
func Zstedc(compz lapack.CompSV, n int, d []float64, e []float64, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zstedc((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstegr.f.
func Sstegr(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_sstegr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstegr.f.
func Dstegr(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_dstegr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cstegr.f.
func Cstegr(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []complex64, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_cstegr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zstegr.f.
func Zstegr(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []complex128, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_zstegr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstein.f.
func Sstein(n int, d []float32, e []float32, m int, w []float32, iblock []int32, isplit []int32, z []float32, ldz int, ifailv []int32) bool {
	return isZero(C.LAPACKE_sstein((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.lapack_int)(m), (*C.float)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifailv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstein.f.
func Dstein(n int, d []float64, e []float64, m int, w []float64, iblock []int32, isplit []int32, z []float64, ldz int, ifailv []int32) bool {
	return isZero(C.LAPACKE_dstein((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.lapack_int)(m), (*C.double)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifailv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cstein.f.
func Cstein(n int, d []float32, e []float32, m int, w []float32, iblock []int32, isplit []int32, z []complex64, ldz int, ifailv []int32) bool {
	return isZero(C.LAPACKE_cstein((C.int)(rowMajor), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.lapack_int)(m), (*C.float)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifailv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zstein.f.
func Zstein(n int, d []float64, e []float64, m int, w []float64, iblock []int32, isplit []int32, z []complex128, ldz int, ifailv []int32) bool {
	return isZero(C.LAPACKE_zstein((C.int)(rowMajor), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.lapack_int)(m), (*C.double)(&w[0]), (*C.lapack_int)(&iblock[0]), (*C.lapack_int)(&isplit[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifailv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstemr.f.
func Sstemr(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, m []int32, w []float32, z []float32, ldz int, nzc int, isuppz []int32, tryrac []int32) bool {
	return isZero(C.LAPACKE_sstemr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(nzc), (*C.lapack_int)(&isuppz[0]), (*C.lapack_logical)(&tryrac[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstemr.f.
func Dstemr(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, m []int32, w []float64, z []float64, ldz int, nzc int, isuppz []int32, tryrac []int32) bool {
	return isZero(C.LAPACKE_dstemr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(nzc), (*C.lapack_int)(&isuppz[0]), (*C.lapack_logical)(&tryrac[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cstemr.f.
func Cstemr(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, m []int32, w []float32, z []complex64, ldz int, nzc int, isuppz []int32, tryrac []int32) bool {
	return isZero(C.LAPACKE_cstemr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(nzc), (*C.lapack_int)(&isuppz[0]), (*C.lapack_logical)(&tryrac[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zstemr.f.
func Zstemr(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, m []int32, w []float64, z []complex128, ldz int, nzc int, isuppz []int32, tryrac []int32) bool {
	return isZero(C.LAPACKE_zstemr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(nzc), (*C.lapack_int)(&isuppz[0]), (*C.lapack_logical)(&tryrac[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssteqr.f.
func Ssteqr(compz lapack.CompSV, n int, d []float32, e []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_ssteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsteqr.f.
func Dsteqr(compz lapack.CompSV, n int, d []float64, e []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dsteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csteqr.f.
func Csteqr(compz lapack.CompSV, n int, d []float32, e []float32, z []complex64, ldz int) bool {
	return isZero(C.LAPACKE_csteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsteqr.f.
func Zsteqr(compz lapack.CompSV, n int, d []float64, e []float64, z []complex128, ldz int) bool {
	return isZero(C.LAPACKE_zsteqr((C.int)(rowMajor), (C.char)(compz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssterf.f.
func Ssterf(n int, d []float32, e []float32) bool {
	return isZero(C.LAPACKE_ssterf((C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsterf.f.
func Dsterf(n int, d []float64, e []float64) bool {
	return isZero(C.LAPACKE_dsterf((C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstev.f.
func Sstev(jobz lapack.Job, n int, d []float32, e []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_sstev((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstev.f.
func Dstev(jobz lapack.Job, n int, d []float64, e []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dstev((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstevd.f.
func Sstevd(jobz lapack.Job, n int, d []float32, e []float32, z []float32, ldz int) bool {
	return isZero(C.LAPACKE_sstevd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstevd.f.
func Dstevd(jobz lapack.Job, n int, d []float64, e []float64, z []float64, ldz int) bool {
	return isZero(C.LAPACKE_dstevd((C.int)(rowMajor), (C.char)(jobz), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstevr.f.
func Sstevr(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_sstevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstevr.f.
func Dstevr(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, isuppz []int32) bool {
	return isZero(C.LAPACKE_dstevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sstevx.f.
func Sstevx(jobz lapack.Job, rng byte, n int, d []float32, e []float32, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	return isZero(C.LAPACKE_sstevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.float)(&d[0]), (*C.float)(&e[0]), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dstevx.f.
func Dstevx(jobz lapack.Job, rng byte, n int, d []float64, e []float64, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	return isZero(C.LAPACKE_dstevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.lapack_int)(n), (*C.double)(&d[0]), (*C.double)(&e[0]), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssycon.f.
func Ssycon(ul blas.Uplo, n int, a []float32, lda int, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssycon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsycon.f.
func Dsycon(ul blas.Uplo, n int, a []float64, lda int, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsycon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csycon.f.
func Csycon(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32, anorm float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csycon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.float)(anorm), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsycon.f.
func Zsycon(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32, anorm float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsycon((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.double)(anorm), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyequb.f.
func Ssyequb(ul blas.Uplo, n int, a []float32, lda int, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyequb.f.
func Dsyequb(ul blas.Uplo, n int, a []float64, lda int, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csyequb.f.
func Csyequb(ul blas.Uplo, n int, a []complex64, lda int, s []float32, scond []float32, amax []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csyequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&s[0]), (*C.float)(&scond[0]), (*C.float)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsyequb.f.
func Zsyequb(ul blas.Uplo, n int, a []complex128, lda int, s []float64, scond []float64, amax []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsyequb((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&s[0]), (*C.double)(&scond[0]), (*C.double)(&amax[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyev.f.
func Ssyev(jobz lapack.Job, ul blas.Uplo, n int, a []float32, lda int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyev.f.
func Dsyev(jobz lapack.Job, ul blas.Uplo, n int, a []float64, lda int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyev((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyevd.f.
func Ssyevd(jobz lapack.Job, ul blas.Uplo, n int, a []float32, lda int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyevd.f.
func Dsyevd(jobz lapack.Job, ul blas.Uplo, n int, a []float64, lda int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyevd((C.int)(rowMajor), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyevr.f.
func Ssyevr(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float32, lda int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, isuppz []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyevr.f.
func Dsyevr(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float64, lda int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, isuppz []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyevr((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&isuppz[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyevx.f.
func Ssyevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float32, lda int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyevx.f.
func Dsyevx(jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float64, lda int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyevx((C.int)(rowMajor), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssygst.f.
func Ssygst(itype int, ul blas.Uplo, n int, a []float32, lda int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssygst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsygst.f.
func Dsygst(itype int, ul blas.Uplo, n int, a []float64, lda int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsygst((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssygv.f.
func Ssygv(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []float32, lda int, b []float32, ldb int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssygv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsygv.f.
func Dsygv(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []float64, lda int, b []float64, ldb int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsygv((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssygvd.f.
func Ssygvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []float32, lda int, b []float32, ldb int, w []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssygvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsygvd.f.
func Dsygvd(itype int, jobz lapack.Job, ul blas.Uplo, n int, a []float64, lda int, b []float64, ldb int, w []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsygvd((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&w[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssygvx.f.
func Ssygvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float32, lda int, b []float32, ldb int, vl float32, vu float32, il int, iu int, abstol float32, m []int32, w []float32, z []float32, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssygvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (C.float)(vl), (C.float)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.float)(abstol), (*C.lapack_int)(&m[0]), (*C.float)(&w[0]), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsygvx.f.
func Dsygvx(itype int, jobz lapack.Job, rng byte, ul blas.Uplo, n int, a []float64, lda int, b []float64, ldb int, vl float64, vu float64, il int, iu int, abstol float64, m []int32, w []float64, z []float64, ldz int, ifail []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsygvx((C.int)(rowMajor), (C.lapack_int)(itype), (C.char)(jobz), (C.char)(rng), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (C.double)(vl), (C.double)(vu), (C.lapack_int)(il), (C.lapack_int)(iu), (C.double)(abstol), (*C.lapack_int)(&m[0]), (*C.double)(&w[0]), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifail[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyrfs.f.
func Ssyrfs(ul blas.Uplo, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, ipiv []int32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyrfs.f.
func Dsyrfs(ul blas.Uplo, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, ipiv []int32, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csyrfs.f.
func Csyrfs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csyrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsyrfs.f.
func Zsyrfs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsyrfs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssysv.f.
func Ssysv(ul blas.Uplo, n int, nrhs int, a []float32, lda int, ipiv []int32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssysv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsysv.f.
func Dsysv(ul blas.Uplo, n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsysv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csysv.f.
func Csysv(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csysv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsysv.f.
func Zsysv(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsysv((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssysvx.f.
func Ssysvx(fact byte, ul blas.Uplo, n int, nrhs int, a []float32, lda int, af []float32, ldaf int, ipiv []int32, b []float32, ldb int, x []float32, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssysvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsysvx.f.
func Dsysvx(fact byte, ul blas.Uplo, n int, nrhs int, a []float64, lda int, af []float64, ldaf int, ipiv []int32, b []float64, ldb int, x []float64, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsysvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csysvx.f.
func Csysvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex64, lda int, af []complex64, ldaf int, ipiv []int32, b []complex64, ldb int, x []complex64, ldx int, rcond []float32, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csysvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&rcond[0]), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsysvx.f.
func Zsysvx(fact byte, ul blas.Uplo, n int, nrhs int, a []complex128, lda int, af []complex128, ldaf int, ipiv []int32, b []complex128, ldb int, x []complex128, ldx int, rcond []float64, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsysvx((C.int)(rowMajor), (C.char)(fact), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&af[0]), (C.lapack_int)(ldaf), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&rcond[0]), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytrd.f.
func Ssytrd(ul blas.Uplo, n int, a []float32, lda int, d []float32, e []float32, tau []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&d[0]), (*C.float)(&e[0]), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytrd.f.
func Dsytrd(ul blas.Uplo, n int, a []float64, lda int, d []float64, e []float64, tau []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytrd((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&d[0]), (*C.double)(&e[0]), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytrf.f.
func Ssytrf(ul blas.Uplo, n int, a []float32, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytrf.f.
func Dsytrf(ul blas.Uplo, n int, a []float64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytrf.f.
func Csytrf(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytrf.f.
func Zsytrf(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytrf((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytri.f.
func Ssytri(ul blas.Uplo, n int, a []float32, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytri.f.
func Dsytri(ul blas.Uplo, n int, a []float64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytri.f.
func Csytri(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytri.f.
func Zsytri(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytri((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytrs.f.
func Ssytrs(ul blas.Uplo, n int, nrhs int, a []float32, lda int, ipiv []int32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytrs.f.
func Dsytrs(ul blas.Uplo, n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytrs.f.
func Csytrs(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytrs.f.
func Zsytrs(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytrs((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stbcon.f.
func Stbcon(norm byte, ul blas.Uplo, d blas.Diag, n int, kd int, ab []float32, ldab int, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stbcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtbcon.f.
func Dtbcon(norm byte, ul blas.Uplo, d blas.Diag, n int, kd int, ab []float64, ldab int, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtbcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctbcon.f.
func Ctbcon(norm byte, ul blas.Uplo, d blas.Diag, n int, kd int, ab []complex64, ldab int, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctbcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztbcon.f.
func Ztbcon(norm byte, ul blas.Uplo, d blas.Diag, n int, kd int, ab []complex128, ldab int, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztbcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stbrfs.f.
func Stbrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []float32, ldab int, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stbrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtbrfs.f.
func Dtbrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []float64, ldab int, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtbrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctbrfs.f.
func Ctbrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []complex64, ldab int, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctbrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztbrfs.f.
func Ztbrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []complex128, ldab int, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztbrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stbtrs.f.
func Stbtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []float32, ldab int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stbtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.float)(&ab[0]), (C.lapack_int)(ldab), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtbtrs.f.
func Dtbtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []float64, ldab int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtbtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.double)(&ab[0]), (C.lapack_int)(ldab), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctbtrs.f.
func Ctbtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []complex64, ldab int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctbtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztbtrs.f.
func Ztbtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, kd int, nrhs int, ab []complex128, ldab int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztbtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(kd), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ab[0]), (C.lapack_int)(ldab), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stfsm.f.
func Stfsm(transr blas.Transpose, s blas.Side, ul blas.Uplo, trans blas.Transpose, d blas.Diag, m int, n int, alpha float32, a []float32, b []float32, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stfsm((C.int)(rowMajor), (C.char)(transr), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (C.float)(alpha), (*C.float)(&a[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtfsm.f.
func Dtfsm(transr blas.Transpose, s blas.Side, ul blas.Uplo, trans blas.Transpose, d blas.Diag, m int, n int, alpha float64, a []float64, b []float64, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtfsm((C.int)(rowMajor), (C.char)(transr), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (C.double)(alpha), (*C.double)(&a[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctfsm.f.
func Ctfsm(transr blas.Transpose, s blas.Side, ul blas.Uplo, trans blas.Transpose, d blas.Diag, m int, n int, alpha complex64, a []complex64, b []complex64, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctfsm((C.int)(rowMajor), (C.char)(transr), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_complex_float)(alpha), (*C.lapack_complex_float)(&a[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztfsm.f.
func Ztfsm(transr blas.Transpose, s blas.Side, ul blas.Uplo, trans blas.Transpose, d blas.Diag, m int, n int, alpha complex128, a []complex128, b []complex128, ldb int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztfsm((C.int)(rowMajor), (C.char)(transr), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_complex_double)(alpha), (*C.lapack_complex_double)(&a[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stftri.f.
func Stftri(transr blas.Transpose, ul blas.Uplo, d blas.Diag, n int, a []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtftri.f.
func Dtftri(transr blas.Transpose, ul blas.Uplo, d blas.Diag, n int, a []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctftri.f.
func Ctftri(transr blas.Transpose, ul blas.Uplo, d blas.Diag, n int, a []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztftri.f.
func Ztftri(transr blas.Transpose, ul blas.Uplo, d blas.Diag, n int, a []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztftri((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stfttp.f.
func Stfttp(transr blas.Transpose, ul blas.Uplo, n int, arf []float32, ap []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_stfttp((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&arf[0]), (*C.float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtfttp.f.
func Dtfttp(transr blas.Transpose, ul blas.Uplo, n int, arf []float64, ap []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtfttp((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&arf[0]), (*C.double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctfttp.f.
func Ctfttp(transr blas.Transpose, ul blas.Uplo, n int, arf []complex64, ap []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctfttp((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&arf[0]), (*C.lapack_complex_float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztfttp.f.
func Ztfttp(transr blas.Transpose, ul blas.Uplo, n int, arf []complex128, ap []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztfttp((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&arf[0]), (*C.lapack_complex_double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stfttr.f.
func Stfttr(transr blas.Transpose, ul blas.Uplo, n int, arf []float32, a []float32, lda int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_stfttr((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&arf[0]), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtfttr.f.
func Dtfttr(transr blas.Transpose, ul blas.Uplo, n int, arf []float64, a []float64, lda int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtfttr((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&arf[0]), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctfttr.f.
func Ctfttr(transr blas.Transpose, ul blas.Uplo, n int, arf []complex64, a []complex64, lda int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctfttr((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&arf[0]), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztfttr.f.
func Ztfttr(transr blas.Transpose, ul blas.Uplo, n int, arf []complex128, a []complex128, lda int) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztfttr((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&arf[0]), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stgexc.f.
func Stgexc(wantq int32, wantz int32, n int, a []float32, lda int, b []float32, ldb int, q []float32, ldq int, z []float32, ldz int, ifst []int32, ilst []int32) bool {
	return isZero(C.LAPACKE_stgexc((C.int)(rowMajor), (C.lapack_logical)(wantq), (C.lapack_logical)(wantz), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.float)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifst[0]), (*C.lapack_int)(&ilst[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtgexc.f.
func Dtgexc(wantq int32, wantz int32, n int, a []float64, lda int, b []float64, ldb int, q []float64, ldq int, z []float64, ldz int, ifst []int32, ilst []int32) bool {
	return isZero(C.LAPACKE_dtgexc((C.int)(rowMajor), (C.lapack_logical)(wantq), (C.lapack_logical)(wantz), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.double)(&z[0]), (C.lapack_int)(ldz), (*C.lapack_int)(&ifst[0]), (*C.lapack_int)(&ilst[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctgexc.f.
func Ctgexc(wantq int32, wantz int32, n int, a []complex64, lda int, b []complex64, ldb int, q []complex64, ldq int, z []complex64, ldz int, ifst int, ilst int) bool {
	return isZero(C.LAPACKE_ctgexc((C.int)(rowMajor), (C.lapack_logical)(wantq), (C.lapack_logical)(wantz), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_float)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(ifst), (C.lapack_int)(ilst)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztgexc.f.
func Ztgexc(wantq int32, wantz int32, n int, a []complex128, lda int, b []complex128, ldb int, q []complex128, ldq int, z []complex128, ldz int, ifst int, ilst int) bool {
	return isZero(C.LAPACKE_ztgexc((C.int)(rowMajor), (C.lapack_logical)(wantq), (C.lapack_logical)(wantz), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_complex_double)(&z[0]), (C.lapack_int)(ldz), (C.lapack_int)(ifst), (C.lapack_int)(ilst)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stgsja.f.
func Stgsja(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, k int, l int, a []float32, lda int, b []float32, ldb int, tola float32, tolb float32, alpha []float32, beta []float32, u []float32, ldu int, v []float32, ldv int, q []float32, ldq int, ncycle []int32) bool {
	return isZero(C.LAPACKE_stgsja((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (C.float)(tola), (C.float)(tolb), (*C.float)(&alpha[0]), (*C.float)(&beta[0]), (*C.float)(&u[0]), (C.lapack_int)(ldu), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ncycle[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtgsja.f.
func Dtgsja(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, k int, l int, a []float64, lda int, b []float64, ldb int, tola float64, tolb float64, alpha []float64, beta []float64, u []float64, ldu int, v []float64, ldv int, q []float64, ldq int, ncycle []int32) bool {
	return isZero(C.LAPACKE_dtgsja((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (C.double)(tola), (C.double)(tolb), (*C.double)(&alpha[0]), (*C.double)(&beta[0]), (*C.double)(&u[0]), (C.lapack_int)(ldu), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ncycle[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctgsja.f.
func Ctgsja(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, k int, l int, a []complex64, lda int, b []complex64, ldb int, tola float32, tolb float32, alpha []float32, beta []float32, u []complex64, ldu int, v []complex64, ldv int, q []complex64, ldq int, ncycle []int32) bool {
	return isZero(C.LAPACKE_ctgsja((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (C.float)(tola), (C.float)(tolb), (*C.float)(&alpha[0]), (*C.float)(&beta[0]), (*C.lapack_complex_float)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ncycle[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztgsja.f.
func Ztgsja(jobu lapack.Job, jobv lapack.Job, jobq lapack.Job, m int, p int, n int, k int, l int, a []complex128, lda int, b []complex128, ldb int, tola float64, tolb float64, alpha []float64, beta []float64, u []complex128, ldu int, v []complex128, ldv int, q []complex128, ldq int, ncycle []int32) bool {
	return isZero(C.LAPACKE_ztgsja((C.int)(rowMajor), (C.char)(jobu), (C.char)(jobv), (C.char)(jobq), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (C.double)(tola), (C.double)(tolb), (*C.double)(&alpha[0]), (*C.double)(&beta[0]), (*C.lapack_complex_double)(&u[0]), (C.lapack_int)(ldu), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ncycle[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stgsyl.f.
func Stgsyl(trans blas.Transpose, ijob lapack.Job, m int, n int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int, d []float32, ldd int, e []float32, lde int, f []float32, ldf int, scale []float32, dif []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_stgsyl((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(ijob), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&c[0]), (C.lapack_int)(ldc), (*C.float)(&d[0]), (C.lapack_int)(ldd), (*C.float)(&e[0]), (C.lapack_int)(lde), (*C.float)(&f[0]), (C.lapack_int)(ldf), (*C.float)(&scale[0]), (*C.float)(&dif[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtgsyl.f.
func Dtgsyl(trans blas.Transpose, ijob lapack.Job, m int, n int, a []float64, lda int, b []float64, ldb int, c []float64, ldc int, d []float64, ldd int, e []float64, lde int, f []float64, ldf int, scale []float64, dif []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dtgsyl((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(ijob), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&c[0]), (C.lapack_int)(ldc), (*C.double)(&d[0]), (C.lapack_int)(ldd), (*C.double)(&e[0]), (C.lapack_int)(lde), (*C.double)(&f[0]), (C.lapack_int)(ldf), (*C.double)(&scale[0]), (*C.double)(&dif[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctgsyl.f.
func Ctgsyl(trans blas.Transpose, ijob lapack.Job, m int, n int, a []complex64, lda int, b []complex64, ldb int, c []complex64, ldc int, d []complex64, ldd int, e []complex64, lde int, f []complex64, ldf int, scale []float32, dif []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ctgsyl((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(ijob), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc), (*C.lapack_complex_float)(&d[0]), (C.lapack_int)(ldd), (*C.lapack_complex_float)(&e[0]), (C.lapack_int)(lde), (*C.lapack_complex_float)(&f[0]), (C.lapack_int)(ldf), (*C.float)(&scale[0]), (*C.float)(&dif[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztgsyl.f.
func Ztgsyl(trans blas.Transpose, ijob lapack.Job, m int, n int, a []complex128, lda int, b []complex128, ldb int, c []complex128, ldc int, d []complex128, ldd int, e []complex128, lde int, f []complex128, ldf int, scale []float64, dif []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ztgsyl((C.int)(rowMajor), (C.char)(trans), (C.lapack_int)(ijob), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc), (*C.lapack_complex_double)(&d[0]), (C.lapack_int)(ldd), (*C.lapack_complex_double)(&e[0]), (C.lapack_int)(lde), (*C.lapack_complex_double)(&f[0]), (C.lapack_int)(ldf), (*C.double)(&scale[0]), (*C.double)(&dif[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stpcon.f.
func Stpcon(norm byte, ul blas.Uplo, d blas.Diag, n int, ap []float32, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stpcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpcon.f.
func Dtpcon(norm byte, ul blas.Uplo, d blas.Diag, n int, ap []float64, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtpcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpcon.f.
func Ctpcon(norm byte, ul blas.Uplo, d blas.Diag, n int, ap []complex64, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctpcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpcon.f.
func Ztpcon(norm byte, ul blas.Uplo, d blas.Diag, n int, ap []complex128, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztpcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stprfs.f.
func Stprfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []float32, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stprfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtprfs.f.
func Dtprfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []float64, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtprfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctprfs.f.
func Ctprfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []complex64, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctprfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztprfs.f.
func Ztprfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []complex128, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztprfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stptri.f.
func Stptri(ul blas.Uplo, d blas.Diag, n int, ap []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stptri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtptri.f.
func Dtptri(ul blas.Uplo, d blas.Diag, n int, ap []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtptri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctptri.f.
func Ctptri(ul blas.Uplo, d blas.Diag, n int, ap []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctptri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztptri.f.
func Ztptri(ul blas.Uplo, d blas.Diag, n int, ap []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztptri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stptrs.f.
func Stptrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []float32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_stptrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&ap[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtptrs.f.
func Dtptrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []float64, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtptrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&ap[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctptrs.f.
func Ctptrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []complex64, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctptrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztptrs.f.
func Ztptrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, ap []complex128, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztptrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stpttf.f.
func Stpttf(transr blas.Transpose, ul blas.Uplo, n int, ap []float32, arf []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_stpttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpttf.f.
func Dtpttf(transr blas.Transpose, ul blas.Uplo, n int, ap []float64, arf []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtpttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpttf.f.
func Ctpttf(transr blas.Transpose, ul blas.Uplo, n int, ap []complex64, arf []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctpttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpttf.f.
func Ztpttf(transr blas.Transpose, ul blas.Uplo, n int, ap []complex128, arf []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztpttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stpttr.f.
func Stpttr(ul blas.Uplo, n int, ap []float32, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_stpttr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&ap[0]), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpttr.f.
func Dtpttr(ul blas.Uplo, n int, ap []float64, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtpttr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&ap[0]), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpttr.f.
func Ctpttr(ul blas.Uplo, n int, ap []complex64, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctpttr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpttr.f.
func Ztpttr(ul blas.Uplo, n int, ap []complex128, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztpttr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strcon.f.
func Strcon(norm byte, ul blas.Uplo, d blas.Diag, n int, a []float32, lda int, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_strcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrcon.f.
func Dtrcon(norm byte, ul blas.Uplo, d blas.Diag, n int, a []float64, lda int, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtrcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrcon.f.
func Ctrcon(norm byte, ul blas.Uplo, d blas.Diag, n int, a []complex64, lda int, rcond []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctrcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrcon.f.
func Ztrcon(norm byte, ul blas.Uplo, d blas.Diag, n int, a []complex128, lda int, rcond []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztrcon((C.int)(rowMajor), (C.char)(norm), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&rcond[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strexc.f.
func Strexc(compq lapack.CompSV, n int, t []float32, ldt int, q []float32, ldq int, ifst []int32, ilst []int32) bool {
	return isZero(C.LAPACKE_strexc((C.int)(rowMajor), (C.char)(compq), (C.lapack_int)(n), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ifst[0]), (*C.lapack_int)(&ilst[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrexc.f.
func Dtrexc(compq lapack.CompSV, n int, t []float64, ldt int, q []float64, ldq int, ifst []int32, ilst []int32) bool {
	return isZero(C.LAPACKE_dtrexc((C.int)(rowMajor), (C.char)(compq), (C.lapack_int)(n), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&q[0]), (C.lapack_int)(ldq), (*C.lapack_int)(&ifst[0]), (*C.lapack_int)(&ilst[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrexc.f.
func Ctrexc(compq lapack.CompSV, n int, t []complex64, ldt int, q []complex64, ldq int, ifst int, ilst int) bool {
	return isZero(C.LAPACKE_ctrexc((C.int)(rowMajor), (C.char)(compq), (C.lapack_int)(n), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq), (C.lapack_int)(ifst), (C.lapack_int)(ilst)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrexc.f.
func Ztrexc(compq lapack.CompSV, n int, t []complex128, ldt int, q []complex128, ldq int, ifst int, ilst int) bool {
	return isZero(C.LAPACKE_ztrexc((C.int)(rowMajor), (C.char)(compq), (C.lapack_int)(n), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq), (C.lapack_int)(ifst), (C.lapack_int)(ilst)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strrfs.f.
func Strrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []float32, lda int, b []float32, ldb int, x []float32, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_strrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrrfs.f.
func Dtrrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []float64, lda int, b []float64, ldb int, x []float64, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtrrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrrfs.f.
func Ctrrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int, x []complex64, ldx int, ferr []float32, berr []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctrrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.float)(&ferr[0]), (*C.float)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrrfs.f.
func Ztrrfs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int, x []complex128, ldx int, ferr []float64, berr []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztrrfs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.double)(&ferr[0]), (*C.double)(&berr[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strsyl.f.
func Strsyl(trana byte, tranb byte, isgn int, m int, n int, a []float32, lda int, b []float32, ldb int, c []float32, ldc int, scale []float32) bool {
	return isZero(C.LAPACKE_strsyl((C.int)(rowMajor), (C.char)(trana), (C.char)(tranb), (C.lapack_int)(isgn), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&c[0]), (C.lapack_int)(ldc), (*C.float)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrsyl.f.
func Dtrsyl(trana byte, tranb byte, isgn int, m int, n int, a []float64, lda int, b []float64, ldb int, c []float64, ldc int, scale []float64) bool {
	return isZero(C.LAPACKE_dtrsyl((C.int)(rowMajor), (C.char)(trana), (C.char)(tranb), (C.lapack_int)(isgn), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&c[0]), (C.lapack_int)(ldc), (*C.double)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrsyl.f.
func Ctrsyl(trana byte, tranb byte, isgn int, m int, n int, a []complex64, lda int, b []complex64, ldb int, c []complex64, ldc int, scale []float32) bool {
	return isZero(C.LAPACKE_ctrsyl((C.int)(rowMajor), (C.char)(trana), (C.char)(tranb), (C.lapack_int)(isgn), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc), (*C.float)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrsyl.f.
func Ztrsyl(trana byte, tranb byte, isgn int, m int, n int, a []complex128, lda int, b []complex128, ldb int, c []complex128, ldc int, scale []float64) bool {
	return isZero(C.LAPACKE_ztrsyl((C.int)(rowMajor), (C.char)(trana), (C.char)(tranb), (C.lapack_int)(isgn), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc), (*C.double)(&scale[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strtri.f.
func Strtri(ul blas.Uplo, d blas.Diag, n int, a []float32, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_strtri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrtri.f.
func Dtrtri(ul blas.Uplo, d blas.Diag, n int, a []float64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtrtri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrtri.f.
func Ctrtri(ul blas.Uplo, d blas.Diag, n int, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctrtri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrtri.f.
func Ztrtri(ul blas.Uplo, d blas.Diag, n int, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztrtri((C.int)(rowMajor), (C.char)(ul), (C.char)(d), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strtrs.f.
func Strtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []float32, lda int, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_strtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrtrs.f.
func Dtrtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []float64, lda int, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_dtrtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrtrs.f.
func Ctrtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ctrtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrtrs.f.
func Ztrtrs(ul blas.Uplo, trans blas.Transpose, d blas.Diag, n int, nrhs int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
	return isZero(C.LAPACKE_ztrtrs((C.int)(rowMajor), (C.char)(ul), (C.char)(trans), (C.char)(d), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strttf.f.
func Strttf(transr blas.Transpose, ul blas.Uplo, n int, a []float32, lda int, arf []float32) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_strttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrttf.f.
func Dtrttf(transr blas.Transpose, ul blas.Uplo, n int, a []float64, lda int, arf []float64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtrttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrttf.f.
func Ctrttf(transr blas.Transpose, ul blas.Uplo, n int, a []complex64, lda int, arf []complex64) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctrttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrttf.f.
func Ztrttf(transr blas.Transpose, ul blas.Uplo, n int, a []complex128, lda int, arf []complex128) bool {
	switch transr {
	case blas.NoTrans:
		transr = 'N'
	case blas.Trans:
		transr = 'T'
	case blas.ConjTrans:
		transr = 'C'
	default:
		panic("lapack: bad trans")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztrttf((C.int)(rowMajor), (C.char)(transr), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&arf[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/strttp.f.
func Strttp(ul blas.Uplo, n int, a []float32, lda int, ap []float32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_strttp((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtrttp.f.
func Dtrttp(ul blas.Uplo, n int, a []float64, lda int, ap []float64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dtrttp((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctrttp.f.
func Ctrttp(ul blas.Uplo, n int, a []complex64, lda int, ap []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ctrttp((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztrttp.f.
func Ztrttp(ul blas.Uplo, n int, a []complex128, lda int, ap []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ztrttp((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&ap[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stzrzf.f.
func Stzrzf(m int, n int, a []float32, lda int, tau []float32) bool {
	return isZero(C.LAPACKE_stzrzf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtzrzf.f.
func Dtzrzf(m int, n int, a []float64, lda int, tau []float64) bool {
	return isZero(C.LAPACKE_dtzrzf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctzrzf.f.
func Ctzrzf(m int, n int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_ctzrzf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztzrzf.f.
func Ztzrzf(m int, n int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_ztzrzf((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cungbr.f.
func Cungbr(vect byte, m int, n int, k int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cungbr((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zungbr.f.
func Zungbr(vect byte, m int, n int, k int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zungbr((C.int)(rowMajor), (C.char)(vect), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunghr.f.
func Cunghr(n int, ilo int, ihi int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cunghr((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunghr.f.
func Zunghr(n int, ilo int, ihi int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zunghr((C.int)(rowMajor), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunglq.f.
func Cunglq(m int, n int, k int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cunglq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunglq.f.
func Zunglq(m int, n int, k int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zunglq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cungql.f.
func Cungql(m int, n int, k int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cungql((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zungql.f.
func Zungql(m int, n int, k int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zungql((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cungqr.f.
func Cungqr(m int, n int, k int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cungqr((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zungqr.f.
func Zungqr(m int, n int, k int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zungqr((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cungrq.f.
func Cungrq(m int, n int, k int, a []complex64, lda int, tau []complex64) bool {
	return isZero(C.LAPACKE_cungrq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zungrq.f.
func Zungrq(m int, n int, k int, a []complex128, lda int, tau []complex128) bool {
	return isZero(C.LAPACKE_zungrq((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cungtr.f.
func Cungtr(ul blas.Uplo, n int, a []complex64, lda int, tau []complex64) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cungtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zungtr.f.
func Zungtr(ul blas.Uplo, n int, a []complex128, lda int, tau []complex128) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zungtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmbr.f.
func Cunmbr(vect byte, s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmbr((C.int)(rowMajor), (C.char)(vect), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmbr.f.
func Zunmbr(vect byte, s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmbr((C.int)(rowMajor), (C.char)(vect), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmhr.f.
func Cunmhr(s blas.Side, trans blas.Transpose, m int, n int, ilo int, ihi int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmhr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmhr.f.
func Zunmhr(s blas.Side, trans blas.Transpose, m int, n int, ilo int, ihi int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmhr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(ilo), (C.lapack_int)(ihi), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmlq.f.
func Cunmlq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmlq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmlq.f.
func Zunmlq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmlq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmql.f.
func Cunmql(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmql((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmql.f.
func Zunmql(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmql((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmqr.f.
func Cunmqr(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmqr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmqr.f.
func Zunmqr(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmqr((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmrq.f.
func Cunmrq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmrq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmrq.f.
func Zunmrq(s blas.Side, trans blas.Transpose, m int, n int, k int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmrq((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmrz.f.
func Cunmrz(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmrz((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmrz.f.
func Zunmrz(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmrz((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunmtr.f.
func Cunmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, a []complex64, lda int, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunmtr.f.
func Zunmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, a []complex128, lda int, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cupgtr.f.
func Cupgtr(ul blas.Uplo, n int, ap []complex64, tau []complex64, q []complex64, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cupgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zupgtr.f.
func Zupgtr(ul blas.Uplo, n int, ap []complex128, tau []complex128, q []complex128, ldq int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zupgtr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&q[0]), (C.lapack_int)(ldq)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cupmtr.f.
func Cupmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, ap []complex64, tau []complex64, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cupmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&ap[0]), (*C.lapack_complex_float)(&tau[0]), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zupmtr.f.
func Zupmtr(s blas.Side, ul blas.Uplo, trans blas.Transpose, m int, n int, ap []complex128, tau []complex128, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zupmtr((C.int)(rowMajor), (C.char)(s), (C.char)(ul), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&ap[0]), (*C.lapack_complex_double)(&tau[0]), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slapmr.f.
func SlapmrWork(forwrd int32, m int, n int, x []float32, ldx int, k []int32) bool {
	return isZero(C.LAPACKE_slapmr((C.int)(rowMajor), (C.lapack_logical)(forwrd), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&k[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlapmr.f.
func DlapmrWork(forwrd int32, m int, n int, x []float64, ldx int, k []int32) bool {
	return isZero(C.LAPACKE_dlapmr((C.int)(rowMajor), (C.lapack_logical)(forwrd), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&k[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/clapmr.f.
func ClapmrWork(forwrd int32, m int, n int, x []complex64, ldx int, k []int32) bool {
	return isZero(C.LAPACKE_clapmr((C.int)(rowMajor), (C.lapack_logical)(forwrd), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&k[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zlapmr.f.
func ZlapmrWork(forwrd int32, m int, n int, x []complex128, ldx int, k []int32) bool {
	return isZero(C.LAPACKE_zlapmr((C.int)(rowMajor), (C.lapack_logical)(forwrd), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(ldx), (*C.lapack_int)(&k[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slartgp.f.
func SlartgpWork(f float32, g float32, cs []float32, sn []float32, r []float32) bool {
	return isZero(C.LAPACKE_slartgp((C.float)(f), (C.float)(g), (*C.float)(&cs[0]), (*C.float)(&sn[0]), (*C.float)(&r[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlartgp.f.
func DlartgpWork(f float64, g float64, cs []float64, sn []float64, r []float64) bool {
	return isZero(C.LAPACKE_dlartgp((C.double)(f), (C.double)(g), (*C.double)(&cs[0]), (*C.double)(&sn[0]), (*C.double)(&r[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slartgs.f.
func SlartgsWork(x float32, y float32, sigma float32, cs []float32, sn []float32) bool {
	return isZero(C.LAPACKE_slartgs((C.float)(x), (C.float)(y), (C.float)(sigma), (*C.float)(&cs[0]), (*C.float)(&sn[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlartgs.f.
func DlartgsWork(x float64, y float64, sigma float64, cs []float64, sn []float64) bool {
	return isZero(C.LAPACKE_dlartgs((C.double)(x), (C.double)(y), (C.double)(sigma), (*C.double)(&cs[0]), (*C.double)(&sn[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slapy2.f.
func Slapy2Work(x float32, y float32) float32 {
	return float32(C.LAPACKE_slapy2((C.float)(x), (C.float)(y)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlapy2.f.
func Dlapy2Work(x float64, y float64) float64 {
	return float64(C.LAPACKE_dlapy2((C.double)(x), (C.double)(y)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/slapy3.f.
func Slapy3Work(x float32, y float32, z float32) float32 {
	return float32(C.LAPACKE_slapy3((C.float)(x), (C.float)(y), (C.float)(z)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dlapy3.f.
func Dlapy3Work(x float64, y float64, z float64) float64 {
	return float64(C.LAPACKE_dlapy3((C.double)(x), (C.double)(y), (C.double)(z)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cbbcsd.f.
func Cbbcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, m int, p int, q int, theta []float32, phi []float32, u1 []complex64, ldu1 int, u2 []complex64, ldu2 int, v1t []complex64, ldv1t int, v2t []complex64, ldv2t int, b11d []float32, b11e []float32, b12d []float32, b12e []float32, b21d []float32, b21e []float32, b22d []float32, b22e []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cbbcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.float)(&theta[0]), (*C.float)(&phi[0]), (*C.lapack_complex_float)(&u1[0]), (C.lapack_int)(ldu1), (*C.lapack_complex_float)(&u2[0]), (C.lapack_int)(ldu2), (*C.lapack_complex_float)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.lapack_complex_float)(&v2t[0]), (C.lapack_int)(ldv2t), (*C.float)(&b11d[0]), (*C.float)(&b11e[0]), (*C.float)(&b12d[0]), (*C.float)(&b12e[0]), (*C.float)(&b21d[0]), (*C.float)(&b21e[0]), (*C.float)(&b22d[0]), (*C.float)(&b22e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cheswapr.f.
func Cheswapr(ul blas.Uplo, n int, a []complex64, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_cheswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetri2.f.
func Chetri2(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetri2x.f.
func Chetri2x(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/chetrs2.f.
func Chetrs2(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_chetrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csyconv.f.
func Csyconv(ul blas.Uplo, way byte, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csyconv((C.int)(rowMajor), (C.char)(ul), (C.char)(way), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csyswapr.f.
func Csyswapr(ul blas.Uplo, n int, a []complex64, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csyswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytri2.f.
func Csytri2(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytri2x.f.
func Csytri2x(ul blas.Uplo, n int, a []complex64, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csytrs2.f.
func Csytrs2(ul blas.Uplo, n int, nrhs int, a []complex64, lda int, ipiv []int32, b []complex64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csytrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cunbdb.f.
func Cunbdb(trans blas.Transpose, signs byte, m int, p int, q int, x11 []complex64, ldx11 int, x12 []complex64, ldx12 int, x21 []complex64, ldx21 int, x22 []complex64, ldx22 int, theta []float32, phi []float32, taup1 []complex64, taup2 []complex64, tauq1 []complex64, tauq2 []complex64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cunbdb((C.int)(rowMajor), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.lapack_complex_float)(&x11[0]), (C.lapack_int)(ldx11), (*C.lapack_complex_float)(&x12[0]), (C.lapack_int)(ldx12), (*C.lapack_complex_float)(&x21[0]), (C.lapack_int)(ldx21), (*C.lapack_complex_float)(&x22[0]), (C.lapack_int)(ldx22), (*C.float)(&theta[0]), (*C.float)(&phi[0]), (*C.lapack_complex_float)(&taup1[0]), (*C.lapack_complex_float)(&taup2[0]), (*C.lapack_complex_float)(&tauq1[0]), (*C.lapack_complex_float)(&tauq2[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cuncsd.f.
func Cuncsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, signs byte, m int, p int, q int, x11 []complex64, ldx11 int, x12 []complex64, ldx12 int, x21 []complex64, ldx21 int, x22 []complex64, ldx22 int, theta []float32, u1 []complex64, ldu1 int, u2 []complex64, ldu2 int, v1t []complex64, ldv1t int, v2t []complex64, ldv2t int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cuncsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.lapack_complex_float)(&x11[0]), (C.lapack_int)(ldx11), (*C.lapack_complex_float)(&x12[0]), (C.lapack_int)(ldx12), (*C.lapack_complex_float)(&x21[0]), (C.lapack_int)(ldx21), (*C.lapack_complex_float)(&x22[0]), (C.lapack_int)(ldx22), (*C.float)(&theta[0]), (*C.lapack_complex_float)(&u1[0]), (C.lapack_int)(ldu1), (*C.lapack_complex_float)(&u2[0]), (C.lapack_int)(ldu2), (*C.lapack_complex_float)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.lapack_complex_float)(&v2t[0]), (C.lapack_int)(ldv2t)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dbbcsd.f.
func Dbbcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, m int, p int, q int, theta []float64, phi []float64, u1 []float64, ldu1 int, u2 []float64, ldu2 int, v1t []float64, ldv1t int, v2t []float64, ldv2t int, b11d []float64, b11e []float64, b12d []float64, b12e []float64, b21d []float64, b21e []float64, b22d []float64, b22e []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dbbcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.double)(&theta[0]), (*C.double)(&phi[0]), (*C.double)(&u1[0]), (C.lapack_int)(ldu1), (*C.double)(&u2[0]), (C.lapack_int)(ldu2), (*C.double)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.double)(&v2t[0]), (C.lapack_int)(ldv2t), (*C.double)(&b11d[0]), (*C.double)(&b11e[0]), (*C.double)(&b12d[0]), (*C.double)(&b12e[0]), (*C.double)(&b21d[0]), (*C.double)(&b21e[0]), (*C.double)(&b22d[0]), (*C.double)(&b22e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorbdb.f.
func Dorbdb(trans blas.Transpose, signs byte, m int, p int, q int, x11 []float64, ldx11 int, x12 []float64, ldx12 int, x21 []float64, ldx21 int, x22 []float64, ldx22 int, theta []float64, phi []float64, taup1 []float64, taup2 []float64, tauq1 []float64, tauq2 []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dorbdb((C.int)(rowMajor), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.double)(&x11[0]), (C.lapack_int)(ldx11), (*C.double)(&x12[0]), (C.lapack_int)(ldx12), (*C.double)(&x21[0]), (C.lapack_int)(ldx21), (*C.double)(&x22[0]), (C.lapack_int)(ldx22), (*C.double)(&theta[0]), (*C.double)(&phi[0]), (*C.double)(&taup1[0]), (*C.double)(&taup2[0]), (*C.double)(&tauq1[0]), (*C.double)(&tauq2[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dorcsd.f.
func Dorcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, signs byte, m int, p int, q int, x11 []float64, ldx11 int, x12 []float64, ldx12 int, x21 []float64, ldx21 int, x22 []float64, ldx22 int, theta []float64, u1 []float64, ldu1 int, u2 []float64, ldu2 int, v1t []float64, ldv1t int, v2t []float64, ldv2t int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dorcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.double)(&x11[0]), (C.lapack_int)(ldx11), (*C.double)(&x12[0]), (C.lapack_int)(ldx12), (*C.double)(&x21[0]), (C.lapack_int)(ldx21), (*C.double)(&x22[0]), (C.lapack_int)(ldx22), (*C.double)(&theta[0]), (*C.double)(&u1[0]), (C.lapack_int)(ldu1), (*C.double)(&u2[0]), (C.lapack_int)(ldu2), (*C.double)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.double)(&v2t[0]), (C.lapack_int)(ldv2t)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyconv.f.
func Dsyconv(ul blas.Uplo, way byte, n int, a []float64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyconv((C.int)(rowMajor), (C.char)(ul), (C.char)(way), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsyswapr.f.
func Dsyswapr(ul blas.Uplo, n int, a []float64, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsyswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytri2.f.
func Dsytri2(ul blas.Uplo, n int, a []float64, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytri2x.f.
func Dsytri2x(ul blas.Uplo, n int, a []float64, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dsytrs2.f.
func Dsytrs2(ul blas.Uplo, n int, nrhs int, a []float64, lda int, ipiv []int32, b []float64, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_dsytrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sbbcsd.f.
func Sbbcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, m int, p int, q int, theta []float32, phi []float32, u1 []float32, ldu1 int, u2 []float32, ldu2 int, v1t []float32, ldv1t int, v2t []float32, ldv2t int, b11d []float32, b11e []float32, b12d []float32, b12e []float32, b21d []float32, b21e []float32, b22d []float32, b22e []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sbbcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.float)(&theta[0]), (*C.float)(&phi[0]), (*C.float)(&u1[0]), (C.lapack_int)(ldu1), (*C.float)(&u2[0]), (C.lapack_int)(ldu2), (*C.float)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.float)(&v2t[0]), (C.lapack_int)(ldv2t), (*C.float)(&b11d[0]), (*C.float)(&b11e[0]), (*C.float)(&b12d[0]), (*C.float)(&b12e[0]), (*C.float)(&b21d[0]), (*C.float)(&b21e[0]), (*C.float)(&b22d[0]), (*C.float)(&b22e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorbdb.f.
func Sorbdb(trans blas.Transpose, signs byte, m int, p int, q int, x11 []float32, ldx11 int, x12 []float32, ldx12 int, x21 []float32, ldx21 int, x22 []float32, ldx22 int, theta []float32, phi []float32, taup1 []float32, taup2 []float32, tauq1 []float32, tauq2 []float32) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sorbdb((C.int)(rowMajor), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.float)(&x11[0]), (C.lapack_int)(ldx11), (*C.float)(&x12[0]), (C.lapack_int)(ldx12), (*C.float)(&x21[0]), (C.lapack_int)(ldx21), (*C.float)(&x22[0]), (C.lapack_int)(ldx22), (*C.float)(&theta[0]), (*C.float)(&phi[0]), (*C.float)(&taup1[0]), (*C.float)(&taup2[0]), (*C.float)(&tauq1[0]), (*C.float)(&tauq2[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sorcsd.f.
func Sorcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, signs byte, m int, p int, q int, x11 []float32, ldx11 int, x12 []float32, ldx12 int, x21 []float32, ldx21 int, x22 []float32, ldx22 int, theta []float32, u1 []float32, ldu1 int, u2 []float32, ldu2 int, v1t []float32, ldv1t int, v2t []float32, ldv2t int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sorcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.float)(&x11[0]), (C.lapack_int)(ldx11), (*C.float)(&x12[0]), (C.lapack_int)(ldx12), (*C.float)(&x21[0]), (C.lapack_int)(ldx21), (*C.float)(&x22[0]), (C.lapack_int)(ldx22), (*C.float)(&theta[0]), (*C.float)(&u1[0]), (C.lapack_int)(ldu1), (*C.float)(&u2[0]), (C.lapack_int)(ldu2), (*C.float)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.float)(&v2t[0]), (C.lapack_int)(ldv2t)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyconv.f.
func Ssyconv(ul blas.Uplo, way byte, n int, a []float32, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyconv((C.int)(rowMajor), (C.char)(ul), (C.char)(way), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssyswapr.f.
func Ssyswapr(ul blas.Uplo, n int, a []float32, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssyswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytri2.f.
func Ssytri2(ul blas.Uplo, n int, a []float32, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytri2x.f.
func Ssytri2x(ul blas.Uplo, n int, a []float32, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ssytrs2.f.
func Ssytrs2(ul blas.Uplo, n int, nrhs int, a []float32, lda int, ipiv []int32, b []float32, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_ssytrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zbbcsd.f.
func Zbbcsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, m int, p int, q int, theta []float64, phi []float64, u1 []complex128, ldu1 int, u2 []complex128, ldu2 int, v1t []complex128, ldv1t int, v2t []complex128, ldv2t int, b11d []float64, b11e []float64, b12d []float64, b12e []float64, b21d []float64, b21e []float64, b22d []float64, b22e []float64) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zbbcsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.double)(&theta[0]), (*C.double)(&phi[0]), (*C.lapack_complex_double)(&u1[0]), (C.lapack_int)(ldu1), (*C.lapack_complex_double)(&u2[0]), (C.lapack_int)(ldu2), (*C.lapack_complex_double)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.lapack_complex_double)(&v2t[0]), (C.lapack_int)(ldv2t), (*C.double)(&b11d[0]), (*C.double)(&b11e[0]), (*C.double)(&b12d[0]), (*C.double)(&b12e[0]), (*C.double)(&b21d[0]), (*C.double)(&b21e[0]), (*C.double)(&b22d[0]), (*C.double)(&b22e[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zheswapr.f.
func Zheswapr(ul blas.Uplo, n int, a []complex128, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zheswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetri2.f.
func Zhetri2(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetri2x.f.
func Zhetri2x(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zhetrs2.f.
func Zhetrs2(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zhetrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsyconv.f.
func Zsyconv(ul blas.Uplo, way byte, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsyconv((C.int)(rowMajor), (C.char)(ul), (C.char)(way), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsyswapr.f.
func Zsyswapr(ul blas.Uplo, n int, a []complex128, i1 int, i2 int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsyswapr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(i1), (C.lapack_int)(i2)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytri2.f.
func Zsytri2(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytri2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytri2x.f.
func Zsytri2x(ul blas.Uplo, n int, a []complex128, lda int, ipiv []int32, nb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytri2x((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (C.lapack_int)(nb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsytrs2.f.
func Zsytrs2(ul blas.Uplo, n int, nrhs int, a []complex128, lda int, ipiv []int32, b []complex128, ldb int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsytrs2((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_int)(nrhs), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_int)(&ipiv[0]), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zunbdb.f.
func Zunbdb(trans blas.Transpose, signs byte, m int, p int, q int, x11 []complex128, ldx11 int, x12 []complex128, ldx12 int, x21 []complex128, ldx21 int, x22 []complex128, ldx22 int, theta []float64, phi []float64, taup1 []complex128, taup2 []complex128, tauq1 []complex128, tauq2 []complex128) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zunbdb((C.int)(rowMajor), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.lapack_complex_double)(&x11[0]), (C.lapack_int)(ldx11), (*C.lapack_complex_double)(&x12[0]), (C.lapack_int)(ldx12), (*C.lapack_complex_double)(&x21[0]), (C.lapack_int)(ldx21), (*C.lapack_complex_double)(&x22[0]), (C.lapack_int)(ldx22), (*C.double)(&theta[0]), (*C.double)(&phi[0]), (*C.lapack_complex_double)(&taup1[0]), (*C.lapack_complex_double)(&taup2[0]), (*C.lapack_complex_double)(&tauq1[0]), (*C.lapack_complex_double)(&tauq2[0])))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zuncsd.f.
func Zuncsd(jobu1 lapack.Job, jobu2 lapack.Job, jobv1t lapack.Job, jobv2t lapack.Job, trans blas.Transpose, signs byte, m int, p int, q int, x11 []complex128, ldx11 int, x12 []complex128, ldx12 int, x21 []complex128, ldx21 int, x22 []complex128, ldx22 int, theta []float64, u1 []complex128, ldu1 int, u2 []complex128, ldu2 int, v1t []complex128, ldv1t int, v2t []complex128, ldv2t int) bool {
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zuncsd((C.int)(rowMajor), (C.char)(jobu1), (C.char)(jobu2), (C.char)(jobv1t), (C.char)(jobv2t), (C.char)(trans), (C.char)(signs), (C.lapack_int)(m), (C.lapack_int)(p), (C.lapack_int)(q), (*C.lapack_complex_double)(&x11[0]), (C.lapack_int)(ldx11), (*C.lapack_complex_double)(&x12[0]), (C.lapack_int)(ldx12), (*C.lapack_complex_double)(&x21[0]), (C.lapack_int)(ldx21), (*C.lapack_complex_double)(&x22[0]), (C.lapack_int)(ldx22), (*C.double)(&theta[0]), (*C.lapack_complex_double)(&u1[0]), (C.lapack_int)(ldu1), (*C.lapack_complex_double)(&u2[0]), (C.lapack_int)(ldu2), (*C.lapack_complex_double)(&v1t[0]), (C.lapack_int)(ldv1t), (*C.lapack_complex_double)(&v2t[0]), (C.lapack_int)(ldv2t)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgemqrt.f.
func Sgemqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, nb int, v []float32, ldv int, t []float32, ldt int, c []float32, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_sgemqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(nb), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgemqrt.f.
func Dgemqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, nb int, v []float64, ldv int, t []float64, ldt int, c []float64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dgemqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(nb), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgemqrt.f.
func Cgemqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, nb int, v []complex64, ldv int, t []complex64, ldt int, c []complex64, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_cgemqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(nb), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgemqrt.f.
func Zgemqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, nb int, v []complex128, ldv int, t []complex128, ldt int, c []complex128, ldc int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_zgemqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(nb), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&c[0]), (C.lapack_int)(ldc)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqrt.f.
func Sgeqrt(m int, n int, nb int, a []float32, lda int, t []float32, ldt int) bool {
	return isZero(C.LAPACKE_sgeqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nb), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqrt.f.
func Dgeqrt(m int, n int, nb int, a []float64, lda int, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dgeqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nb), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqrt.f.
func Cgeqrt(m int, n int, nb int, a []complex64, lda int, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_cgeqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nb), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqrt.f.
func Zgeqrt(m int, n int, nb int, a []complex128, lda int, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_zgeqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(nb), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqrt2.f.
func Sgeqrt2(m int, n int, a []float32, lda int, t []float32, ldt int) bool {
	return isZero(C.LAPACKE_sgeqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqrt2.f.
func Dgeqrt2(m int, n int, a []float64, lda int, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dgeqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqrt2.f.
func Cgeqrt2(m int, n int, a []complex64, lda int, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_cgeqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqrt2.f.
func Zgeqrt2(m int, n int, a []complex128, lda int, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_zgeqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/sgeqrt3.f.
func Sgeqrt3(m int, n int, a []float32, lda int, t []float32, ldt int) bool {
	return isZero(C.LAPACKE_sgeqrt3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dgeqrt3.f.
func Dgeqrt3(m int, n int, a []float64, lda int, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dgeqrt3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/cgeqrt3.f.
func Cgeqrt3(m int, n int, a []complex64, lda int, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_cgeqrt3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zgeqrt3.f.
func Zgeqrt3(m int, n int, a []complex128, lda int, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_zgeqrt3((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stpmqrt.f.
func Stpmqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, nb int, v []float32, ldv int, t []float32, ldt int, a []float32, lda int, b []float32, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_stpmqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpmqrt.f.
func Dtpmqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, nb int, v []float64, ldv int, t []float64, ldt int, a []float64, lda int, b []float64, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dtpmqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpmqrt.f.
func Ctpmqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, nb int, v []complex64, ldv int, t []complex64, ldt int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ctpmqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpmqrt.f.
func Ztpmqrt(s blas.Side, trans blas.Transpose, m int, n int, k int, l int, nb int, v []complex128, ldv int, t []complex128, ldt int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ztpmqrt((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpqrt.f.
func Dtpqrt(m int, n int, l int, nb int, a []float64, lda int, b []float64, ldb int, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dtpqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpqrt.f.
func Ctpqrt(m int, n int, l int, nb int, a []complex64, lda int, b []complex64, ldb int, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_ctpqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpqrt.f.
func Ztpqrt(m int, n int, l int, nb int, a []complex128, lda int, b []complex128, ldb int, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_ztpqrt((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (C.lapack_int)(nb), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stpqrt2.f.
func Stpqrt2(m int, n int, l int, a []float32, lda int, b []float32, ldb int, t []float32, ldt int) bool {
	return isZero(C.LAPACKE_stpqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb), (*C.float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtpqrt2.f.
func Dtpqrt2(m int, n int, l int, a []float64, lda int, b []float64, ldb int, t []float64, ldt int) bool {
	return isZero(C.LAPACKE_dtpqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb), (*C.double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctpqrt2.f.
func Ctpqrt2(m int, n int, l int, a []complex64, lda int, b []complex64, ldb int, t []complex64, ldt int) bool {
	return isZero(C.LAPACKE_ctpqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztpqrt2.f.
func Ztpqrt2(m int, n int, l int, a []complex128, lda int, b []complex128, ldb int, t []complex128, ldt int) bool {
	return isZero(C.LAPACKE_ztpqrt2((C.int)(rowMajor), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(l), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/stprfb.f.
func Stprfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, l int, v []float32, ldv int, t []float32, ldt int, a []float32, lda int, b []float32, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_stprfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.float)(&v[0]), (C.lapack_int)(ldv), (*C.float)(&t[0]), (C.lapack_int)(ldt), (*C.float)(&a[0]), (C.lapack_int)(lda), (*C.float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/dtprfb.f.
func Dtprfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, l int, v []float64, ldv int, t []float64, ldt int, a []float64, lda int, b []float64, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_dtprfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.double)(&v[0]), (C.lapack_int)(ldv), (*C.double)(&t[0]), (C.lapack_int)(ldt), (*C.double)(&a[0]), (C.lapack_int)(lda), (*C.double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ctprfb.f.
func Ctprfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, l int, v []complex64, ldv int, t []complex64, ldt int, a []complex64, lda int, b []complex64, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ctprfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_float)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_float)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_float)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/ztprfb.f.
func Ztprfb(s blas.Side, trans blas.Transpose, direct byte, storev byte, m int, n int, k int, l int, v []complex128, ldv int, t []complex128, ldt int, a []complex128, lda int, b []complex128, ldb int) bool {
	switch s {
	case blas.Left:
		s = 'L'
	case blas.Right:
		s = 'R'
	default:
		panic("lapack: bad side")
	}
	switch trans {
	case blas.NoTrans:
		trans = 'N'
	case blas.Trans:
		trans = 'T'
	case blas.ConjTrans:
		trans = 'C'
	default:
		panic("lapack: bad trans")
	}
	return isZero(C.LAPACKE_ztprfb((C.int)(rowMajor), (C.char)(s), (C.char)(trans), (C.char)(direct), (C.char)(storev), (C.lapack_int)(m), (C.lapack_int)(n), (C.lapack_int)(k), (C.lapack_int)(l), (*C.lapack_complex_double)(&v[0]), (C.lapack_int)(ldv), (*C.lapack_complex_double)(&t[0]), (C.lapack_int)(ldt), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda), (*C.lapack_complex_double)(&b[0]), (C.lapack_int)(ldb)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/csyr.f.
func Csyr(ul blas.Uplo, n int, alpha complex64, x []complex64, incx int, a []complex64, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_csyr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_complex_float)(alpha), (*C.lapack_complex_float)(&x[0]), (C.lapack_int)(incx), (*C.lapack_complex_float)(&a[0]), (C.lapack_int)(lda)))
}

// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/zsyr.f.
func Zsyr(ul blas.Uplo, n int, alpha complex128, x []complex128, incx int, a []complex128, lda int) bool {
	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
	return isZero(C.LAPACKE_zsyr((C.int)(rowMajor), (C.char)(ul), (C.lapack_int)(n), (C.lapack_complex_double)(alpha), (*C.lapack_complex_double)(&x[0]), (C.lapack_int)(incx), (*C.lapack_complex_double)(&a[0]), (C.lapack_int)(lda)))
}
