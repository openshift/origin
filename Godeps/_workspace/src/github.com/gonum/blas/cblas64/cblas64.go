// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cblas64 provides a simple interface to the complex64 BLAS API.
package cblas64

import (
	"github.com/gonum/blas"
	"github.com/gonum/blas/cgo"
)

// TODO(kortschak): Change this and the comment below to native.Implementation
// when blas/native covers the complex BLAS API.
var cblas64 blas.Complex64 = cgo.Implementation{}

// Use sets the BLAS complex64 implementation to be used by subsequent BLAS calls.
// The default implementation is cgo.Implementation.
func Use(b blas.Complex64) {
	cblas64 = b
}

// Implementation returns the current BLAS complex64 implementation.
//
// Implementation allows direct calls to the current the BLAS complex64 implementation
// giving finer control of parameters.
func Implementation() blas.Complex64 {
	return cblas64
}

// Vector represents a vector with an associated element increment.
type Vector struct {
	Inc  int
	Data []complex64
}

// General represents a matrix using the conventional storage scheme.
type General struct {
	Rows, Cols int
	Stride     int
	Data       []complex64
}

// Band represents a band matrix using the band storage scheme.
type Band struct {
	Rows, Cols int
	KL, KU     int
	Stride     int
	Data       []complex64
}

// Triangular represents a triangular matrix using the conventional storage scheme.
type Triangular struct {
	N      int
	Stride int
	Data   []complex64
	Uplo   blas.Uplo
	Diag   blas.Diag
}

// TriangularBand represents a triangular matrix using the band storage scheme.
type TriangularBand struct {
	N, K   int
	Stride int
	Data   []complex64
	Uplo   blas.Uplo
	Diag   blas.Diag
}

// TriangularPacked represents a triangular matrix using the packed storage scheme.
type TriangularPacked struct {
	N    int
	Data []complex64
	Uplo blas.Uplo
	Diag blas.Diag
}

// Symmetric represents a symmetric matrix using the conventional storage scheme.
type Symmetric struct {
	N      int
	Stride int
	Data   []complex64
	Uplo   blas.Uplo
}

// SymmetricBand represents a symmetric matrix using the band storage scheme.
type SymmetricBand struct {
	N, K   int
	Stride int
	Data   []complex64
	Uplo   blas.Uplo
}

// SymmetricPacked represents a symmetric matrix using the packed storage scheme.
type SymmetricPacked struct {
	N    int
	Data []complex64
	Uplo blas.Uplo
}

// Hermitian represents an Hermitian matrix using the conventional storage scheme.
type Hermitian Symmetric

// HermitianBand represents an Hermitian matrix using the band storage scheme.
type HermitianBand SymmetricBand

// HermitianPacked represents an Hermitian matrix using the packed storage scheme.
type HermitianPacked SymmetricPacked

// Level 1

const negInc = "cblas64: negative vector increment"

// Dotu computes the dot product of the two vectors without
// complex conjugation
//  x^T * y
func Dotu(n int, x, y Vector) complex64 {
	return cblas64.Cdotu(n, x.Data, x.Inc, y.Data, y.Inc)
}

// Dotu computes the dot product of the two vectors with
// complex conjugation
//  x^H * y
func Dotc(n int, x, y Vector) complex64 {
	return cblas64.Cdotc(n, x.Data, x.Inc, y.Data, y.Inc)
}

// Nrm2 computes the Euclidean norm of a vector,
//  sqrt(\sum_i x[i] * x[i]).
//
// Nrm2 will panic if the vector increment is negative.
func Nrm2(n int, x Vector) float32 {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Scnrm2(n, x.Data, x.Inc)
}

// Asum computes the sum of the absolute values of the elements of x.
//  \sum_i |x[i]|
//
// Asum will panic if the vector increment is negative.
func Asum(n int, x Vector) float32 {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Scasum(n, x.Data, x.Inc)
}

// Iamax returns the index of the largest element of x. If there are multiple
// such indices the earliest is returned. Iamax returns -1 if n == 0.
//
// Iamax will panic if the vector increment is negative.
func Iamax(n int, x Vector) int {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Icamax(n, x.Data, x.Inc)
}

// Swap exchanges the elements of two vectors.
//  x[i], y[i] = y[i], x[i] for all i
func Swap(n int, x, y Vector) {
	cblas64.Cswap(n, x.Data, x.Inc, y.Data, y.Inc)
}

// Copy copies the elements of x into the elements of y.
//  y[i] = x[i] for all i
func Copy(n int, x, y Vector) {
	cblas64.Ccopy(n, x.Data, x.Inc, y.Data, y.Inc)
}

// Axpy adds alpha times x to y
//  y[i] += alpha * x[i] for all i
func Axpy(n int, alpha complex64, x, y Vector) {
	cblas64.Caxpy(n, alpha, x.Data, x.Inc, y.Data, y.Inc)
}

// Scal scales x by alpha.
//  x[i] *= alpha
//
// Scal will panic if the vector increment is negative
func Scal(n int, alpha complex64, x Vector) {
	if x.Inc < 0 {
		panic(negInc)
	}
	cblas64.Cscal(n, alpha, x.Data, x.Inc)
}

// Dscal scales x by alpha.
//  x[i] *= alpha
//
// Dscal will panic if the vector increment is negative
func Dscal(n int, alpha float32, x Vector) {
	if x.Inc < 0 {
		panic(negInc)
	}
	cblas64.Csscal(n, alpha, x.Data, x.Inc)
}

// Level 2

// Gemv computes
//  y = alpha * a * x + beta * y if tA = blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA = blas.Trans
//  y = alpha * A^H * x + beta * y if tA = blas.ConjTrans
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func Gemv(tA blas.Transpose, alpha complex64, a General, x Vector, beta complex64, y Vector) {
	cblas64.Cgemv(tA, a.Rows, a.Cols, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Gbmv computes
//  y = alpha * A * x + beta * y if tA == blas.NoTrans
//  y = alpha * A^T * x + beta * y if tA = blas.Trans
//  y = alpha * A^H * x + beta * y if tA = blas.ConjTrans
// where a is an m×n band matrix kL subdiagonals and kU super-diagonals, and
// m and n refer to the size of the full dense matrix it represents.
// x and y are vectors, and alpha and beta are scalars.
func Gbmv(tA blas.Transpose, alpha complex64, a Band, x Vector, beta complex64, y Vector) {
	cblas64.Cgbmv(tA, a.Rows, a.Cols, a.KL, a.KU, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Trmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans
//  x = A^H * x if tA == blas.ConjTrans
// A is an n×n Triangular matrix and x is a vector.
func Trmv(tA blas.Transpose, a Triangular, x Vector) {
	cblas64.Ctrmv(a.Uplo, tA, a.Diag, a.N, a.Data, a.Stride, x.Data, x.Inc)
}

// Tbmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans
//  x = A^H * x if tA == blas.ConjTrans
// where A is an n×n triangular banded matrix with k diagonals, and x is a vector.
func Tbmv(tA blas.Transpose, a TriangularBand, x Vector) {
	cblas64.Ctbmv(a.Uplo, tA, a.Diag, a.N, a.K, a.Data, a.Stride, x.Data, x.Inc)
}

// Tpmv computes
//  x = A * x if tA == blas.NoTrans
//  x = A^T * x if tA == blas.Trans
//  x = A^H * x if tA == blas.ConjTrans
// where A is an n×n unit triangular matrix in packed format, and x is a vector.
func Tpmv(tA blas.Transpose, a TriangularPacked, x Vector) {
	cblas64.Ctpmv(a.Uplo, tA, a.Diag, a.N, a.Data, x.Data, x.Inc)
}

// Trsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans
//  A^H * x = b if tA == blas.ConjTrans
// A is an n×n triangular matrix and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Trsv(tA blas.Transpose, a Triangular, x Vector) {
	cblas64.Ctrsv(a.Uplo, tA, a.Diag, a.N, a.Data, a.Stride, x.Data, x.Inc)
}

// Tbsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans
//  A^H * x = b if tA == blas.ConjTrans
// where A is an n×n triangular banded matrix with k diagonals in packed format,
// and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Tbsv(tA blas.Transpose, a TriangularBand, x Vector) {
	cblas64.Ctbsv(a.Uplo, tA, a.Diag, a.N, a.K, a.Data, a.Stride, x.Data, x.Inc)
}

// Tpsv solves
//  A * x = b if tA == blas.NoTrans
//  A^T * x = b if tA == blas.Trans
//  A^H * x = b if tA == blas.ConjTrans
// where A is an n×n triangular matrix in packed format and x is a vector.
// At entry to the function, x contains the values of b, and the result is
// stored in place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Tpsv(tA blas.Transpose, a TriangularPacked, x Vector) {
	cblas64.Ctpsv(a.Uplo, tA, a.Diag, a.N, a.Data, x.Data, x.Inc)
}

// Hemv computes
//  y = alpha * A * x + beta * y,
// where a is an n×n Hermitian matrix, x and y are vectors, and alpha and
// beta are scalars.
func Hemv(alpha complex64, a Hermitian, x Vector, beta complex64, y Vector) {
	cblas64.Chemv(a.Uplo, a.N, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Hbmv performs
//  y = alpha * A * x + beta * y
// where A is an n×n Hermitian banded matrix, x and y are vectors, and alpha
// and beta are scalars.
func Hbmv(alpha complex64, a HermitianBand, x Vector, beta complex64, y Vector) {
	cblas64.Chbmv(a.Uplo, a.N, a.K, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Hpmv performs
//  y = alpha * A * x + beta * y,
// where A is an n×n Hermitian matrix in packed format, x and y are vectors
// and alpha and beta are scalars.
func Hpmv(alpha complex64, a HermitianPacked, x Vector, beta complex64, y Vector) {
	cblas64.Chpmv(a.Uplo, a.N, alpha, a.Data, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Geru performs the rank-one operation
//  A += alpha * x * y^T
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func Geru(alpha complex64, x, y Vector, a General) {
	cblas64.Cgeru(a.Rows, a.Cols, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Gerc performs the rank-one operation
//  A += alpha * x * y^H
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func Gerc(alpha complex64, x, y Vector, a General) {
	cblas64.Cgerc(a.Rows, a.Cols, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Ger performs the rank-one operation
//  A += alpha * x * y^H
// where A is an m×n Hermitian matrix, x and y are vectors, and alpha is a scalar.
func Her(alpha float32, x Vector, a Hermitian) {
	cblas64.Cher(a.Uplo, a.N, alpha, x.Data, x.Inc, a.Data, a.Stride)
}

// Hpr computes the rank-one operation
//  a += alpha * x * x^H
// where a is an n×n Hermitian matrix in packed format, x is a vector, and
// alpha is a scalar.
func Hpr(alpha float32, x Vector, a HermitianPacked) {
	cblas64.Chpr(a.Uplo, a.N, alpha, x.Data, x.Inc, a.Data)
}

// Her2 performs the symmetric rank-two update
//  A += alpha * x * y^H + alpha * y * x^H
// where A is a symmetric n×n matrix, x and y are vectors, and alpha is a scalar.
func Her2(alpha complex64, x, y Vector, a Hermitian) {
	cblas64.Cher2(a.Uplo, a.N, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Hpr2 performs the symmetric rank-2 update
//  a += alpha * x * y^H + alpha * y * x^H
// where a is an n×n symmetric matirx in packed format and x and y are vectors.
func Hpr2(alpha complex64, x, y Vector, a HermitianPacked) {
	cblas64.Chpr2(a.Uplo, a.N, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data)
}

// Level 3

// Gemm computes
//  C = beta * C + alpha * A * B.
// tA and tB specify whether A or B are transposed or conjugated. A, B, and C are m×n dense
// matrices.
func Gemm(tA, tB blas.Transpose, alpha complex64, a, b General, beta complex64, c General) {
	var m, n, k int
	if tA == blas.NoTrans {
		m, k = a.Rows, a.Cols
	} else {
		m, k = a.Cols, a.Rows
	}
	if tB == blas.NoTrans {
		n = b.Cols
	} else {
		n = b.Rows
	}
	cblas64.Cgemm(tA, tB, m, n, k, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Symm performs one of
//  C = alpha * A * B + beta * C if side == blas.Left
//  C = alpha * B * A + beta * C if side == blas.Right
// where A is an n×n symmetric matrix, B and C are m×n matrices, and alpha
// is a scalar.
func Symm(s blas.Side, alpha complex64, a Symmetric, b General, beta complex64, c General) {
	var m, n int
	if s == blas.Left {
		m, n = a.N, b.Cols
	} else {
		m, n = b.Rows, a.N
	}
	cblas64.Csymm(s, a.Uplo, m, n, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Syrk performs the symmetric rank-k operation
//  C = alpha * A * A^T + beta*C
// C is an n×n symmetric matrix. A is an n×k matrix if tA == blas.NoTrans, and
// a k×n matrix otherwise. alpha and beta are scalars.
func Syrk(t blas.Transpose, alpha complex64, a General, beta complex64, c Symmetric) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Csyrk(c.Uplo, t, n, k, alpha, a.Data, a.Stride, beta, c.Data, c.Stride)
}

// Syr2k performs the symmetric rank 2k operation
//  C = alpha * A * B^T + alpha * B * A^T + beta * C
// where C is an n×n symmetric matrix. A and B are n×k matrices if
// tA == NoTrans and k×n otherwise. alpha and beta are scalars.
func Syr2k(t blas.Transpose, alpha complex64, a, b General, beta complex64, c Symmetric) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Csyr2k(c.Uplo, t, n, k, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Trmm performs
//  B = alpha * A * B if tA == blas.NoTrans and side == blas.Left
//  B = alpha * A^T * B if tA == blas.Trans and side == blas.Left
//  B = alpha * A^H * B if tA == blas.ConjTrans and side == blas.Left
//  B = alpha * B * A if tA == blas.NoTrans and side == blas.Right
//  B = alpha * B * A^T if tA == blas.Trans and side == blas.Right
//  B = alpha * B * A^H if tA == blas.ConjTrans and side == blas.Right
// where A is an n×n triangular matrix, and B is an m×n matrix.
func Trmm(s blas.Side, tA blas.Transpose, alpha complex64, a Triangular, b General) {
	cblas64.Ctrmm(s, a.Uplo, tA, a.Diag, b.Rows, b.Cols, alpha, a.Data, a.Stride, b.Data, b.Stride)
}

// Trsm solves
//  A * X = alpha * B if tA == blas.NoTrans and side == blas.Left
//  A^T * X = alpha * B if tA == blas.Trans and side == blas.Left
//  A^H * X = alpha * B if tA == blas.ConjTrans and side == blas.Left
//  X * A = alpha * B if tA == blas.NoTrans and side == blas.Right
//  X * A^T = alpha * B if tA == blas.Trans and side == blas.Right
//  X * A^H = alpha * B if tA ==  blas.ConjTrans and side == blas.Right
// where A is an n×n triangular matrix, x is an m×n matrix, and alpha is a
// scalar.
//
// At entry to the function, X contains the values of B, and the result is
// stored in place into X.
//
// No check is made that A is invertible.
func Trsm(s blas.Side, tA blas.Transpose, alpha complex64, a Triangular, b General) {
	cblas64.Ctrsm(s, a.Uplo, tA, a.Diag, b.Rows, b.Cols, alpha, a.Data, a.Stride, b.Data, b.Stride)
}

// Hemm performs
//  B = alpha * A * B if tA == blas.NoTrans and side == blas.Left
//  B = alpha * A^H * B if tA == blas.ConjTrans and side == blas.Left
//  B = alpha * B * A if tA == blas.NoTrans and side == blas.Right
//  B = alpha * B * A^H if tA == blas.ConjTrans and side == blas.Right
// where A is an n×n Hermitia matrix, and B is an m×n matrix.
func Hemm(s blas.Side, alpha complex64, a Hermitian, b General, beta complex64, c General) {
	var m, n int
	if s == blas.Left {
		m, n = a.N, b.Cols
	} else {
		m, n = b.Rows, a.N
	}
	cblas64.Chemm(s, a.Uplo, m, n, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Herk performs the symmetric rank-k operation
//  C = alpha * A * A^H + beta*C
// C is an n×n Hermitian matrix. A is an n×k matrix if tA == blas.NoTrans, and
// a k×n matrix otherwise. alpha and beta are scalars.
func Herk(t blas.Transpose, alpha float32, a General, beta float32, c Hermitian) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Cherk(c.Uplo, t, n, k, alpha, a.Data, a.Stride, beta, c.Data, c.Stride)
}

// Her2k performs the symmetric rank 2k operation
//  C = alpha * A * B^H + alpha * B * A^H + beta * C
// where C is an n×n Hermitian matrix. A and B are n×k matrices if
// tA == NoTrans and k×n otherwise. alpha and beta are scalars.
func Her2k(t blas.Transpose, alpha complex64, a, b General, beta float32, c Hermitian) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Cher2k(c.Uplo, t, n, k, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}
