// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cblas64

import (
	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/gonum"
)

var cblas64 blas.Complex64 = gonum.Implementation{}

// Use sets the BLAS complex64 implementation to be used by subsequent BLAS calls.
// The default implementation is
// gonum.org/v1/gonum/blas/gonum.Implementation.
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
	N    int
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

const (
	negInc    = "cblas64: negative vector increment"
	badLength = "cblas64: vector length mismatch"
)

// Dotu computes the dot product of the two vectors without
// complex conjugation:
//  xᵀ * y
// Dotu will panic if the lengths of x and y do not match.
func Dotu(x, y Vector) complex64 {
	if x.N != y.N {
		panic(badLength)
	}
	return cblas64.Cdotu(x.N, x.Data, x.Inc, y.Data, y.Inc)
}

// Dotc computes the dot product of the two vectors with
// complex conjugation:
//  xᴴ * y.
// Dotc will panic if the lengths of x and y do not match.
func Dotc(x, y Vector) complex64 {
	if x.N != y.N {
		panic(badLength)
	}
	return cblas64.Cdotc(x.N, x.Data, x.Inc, y.Data, y.Inc)
}

// Nrm2 computes the Euclidean norm of the vector x:
//  sqrt(\sum_i x[i] * x[i]).
//
// Nrm2 will panic if the vector increment is negative.
func Nrm2(x Vector) float32 {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Scnrm2(x.N, x.Data, x.Inc)
}

// Asum computes the sum of magnitudes of the real and imaginary parts of
// elements of the vector x:
//  \sum_i (|Re x[i]| + |Im x[i]|).
//
// Asum will panic if the vector increment is negative.
func Asum(x Vector) float32 {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Scasum(x.N, x.Data, x.Inc)
}

// Iamax returns the index of an element of x with the largest sum of
// magnitudes of the real and imaginary parts (|Re x[i]|+|Im x[i]|).
// If there are multiple such indices, the earliest is returned.
//
// Iamax returns -1 if n == 0.
//
// Iamax will panic if the vector increment is negative.
func Iamax(x Vector) int {
	if x.Inc < 0 {
		panic(negInc)
	}
	return cblas64.Icamax(x.N, x.Data, x.Inc)
}

// Swap exchanges the elements of two vectors:
//  x[i], y[i] = y[i], x[i] for all i.
// Swap will panic if the lengths of x and y do not match.
func Swap(x, y Vector) {
	if x.N != y.N {
		panic(badLength)
	}
	cblas64.Cswap(x.N, x.Data, x.Inc, y.Data, y.Inc)
}

// Copy copies the elements of x into the elements of y:
//  y[i] = x[i] for all i.
// Copy will panic if the lengths of x and y do not match.
func Copy(x, y Vector) {
	if x.N != y.N {
		panic(badLength)
	}
	cblas64.Ccopy(x.N, x.Data, x.Inc, y.Data, y.Inc)
}

// Axpy computes
//  y = alpha * x + y,
// where x and y are vectors, and alpha is a scalar.
// Axpy will panic if the lengths of x and y do not match.
func Axpy(alpha complex64, x, y Vector) {
	if x.N != y.N {
		panic(badLength)
	}
	cblas64.Caxpy(x.N, alpha, x.Data, x.Inc, y.Data, y.Inc)
}

// Scal computes
//  x = alpha * x,
// where x is a vector, and alpha is a scalar.
//
// Scal will panic if the vector increment is negative.
func Scal(alpha complex64, x Vector) {
	if x.Inc < 0 {
		panic(negInc)
	}
	cblas64.Cscal(x.N, alpha, x.Data, x.Inc)
}

// Dscal computes
//  x = alpha * x,
// where x is a vector, and alpha is a real scalar.
//
// Dscal will panic if the vector increment is negative.
func Dscal(alpha float32, x Vector) {
	if x.Inc < 0 {
		panic(negInc)
	}
	cblas64.Csscal(x.N, alpha, x.Data, x.Inc)
}

// Level 2

// Gemv computes
//  y = alpha * A * x + beta * y   if t == blas.NoTrans,
//  y = alpha * Aᵀ * x + beta * y  if t == blas.Trans,
//  y = alpha * Aᴴ * x + beta * y  if t == blas.ConjTrans,
// where A is an m×n dense matrix, x and y are vectors, and alpha and beta are
// scalars.
func Gemv(t blas.Transpose, alpha complex64, a General, x Vector, beta complex64, y Vector) {
	cblas64.Cgemv(t, a.Rows, a.Cols, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Gbmv computes
//  y = alpha * A * x + beta * y   if t == blas.NoTrans,
//  y = alpha * Aᵀ * x + beta * y  if t == blas.Trans,
//  y = alpha * Aᴴ * x + beta * y  if t == blas.ConjTrans,
// where A is an m×n band matrix, x and y are vectors, and alpha and beta are
// scalars.
func Gbmv(t blas.Transpose, alpha complex64, a Band, x Vector, beta complex64, y Vector) {
	cblas64.Cgbmv(t, a.Rows, a.Cols, a.KL, a.KU, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Trmv computes
//  x = A * x   if t == blas.NoTrans,
//  x = Aᵀ * x  if t == blas.Trans,
//  x = Aᴴ * x  if t == blas.ConjTrans,
// where A is an n×n triangular matrix, and x is a vector.
func Trmv(t blas.Transpose, a Triangular, x Vector) {
	cblas64.Ctrmv(a.Uplo, t, a.Diag, a.N, a.Data, a.Stride, x.Data, x.Inc)
}

// Tbmv computes
//  x = A * x   if t == blas.NoTrans,
//  x = Aᵀ * x  if t == blas.Trans,
//  x = Aᴴ * x  if t == blas.ConjTrans,
// where A is an n×n triangular band matrix, and x is a vector.
func Tbmv(t blas.Transpose, a TriangularBand, x Vector) {
	cblas64.Ctbmv(a.Uplo, t, a.Diag, a.N, a.K, a.Data, a.Stride, x.Data, x.Inc)
}

// Tpmv computes
//  x = A * x   if t == blas.NoTrans,
//  x = Aᵀ * x  if t == blas.Trans,
//  x = Aᴴ * x  if t == blas.ConjTrans,
// where A is an n×n triangular matrix in packed format, and x is a vector.
func Tpmv(t blas.Transpose, a TriangularPacked, x Vector) {
	cblas64.Ctpmv(a.Uplo, t, a.Diag, a.N, a.Data, x.Data, x.Inc)
}

// Trsv solves
//  A * x = b   if t == blas.NoTrans,
//  Aᵀ * x = b  if t == blas.Trans,
//  Aᴴ * x = b  if t == blas.ConjTrans,
// where A is an n×n triangular matrix and x is a vector.
//
// At entry to the function, x contains the values of b, and the result is
// stored in-place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Trsv(t blas.Transpose, a Triangular, x Vector) {
	cblas64.Ctrsv(a.Uplo, t, a.Diag, a.N, a.Data, a.Stride, x.Data, x.Inc)
}

// Tbsv solves
//  A * x = b   if t == blas.NoTrans,
//  Aᵀ * x = b  if t == blas.Trans,
//  Aᴴ * x = b  if t == blas.ConjTrans,
// where A is an n×n triangular band matrix, and x is a vector.
//
// At entry to the function, x contains the values of b, and the result is
// stored in-place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Tbsv(t blas.Transpose, a TriangularBand, x Vector) {
	cblas64.Ctbsv(a.Uplo, t, a.Diag, a.N, a.K, a.Data, a.Stride, x.Data, x.Inc)
}

// Tpsv solves
//  A * x = b   if t == blas.NoTrans,
//  Aᵀ * x = b  if t == blas.Trans,
//  Aᴴ * x = b  if t == blas.ConjTrans,
// where A is an n×n triangular matrix in packed format and x is a vector.
//
// At entry to the function, x contains the values of b, and the result is
// stored in-place into x.
//
// No test for singularity or near-singularity is included in this
// routine. Such tests must be performed before calling this routine.
func Tpsv(t blas.Transpose, a TriangularPacked, x Vector) {
	cblas64.Ctpsv(a.Uplo, t, a.Diag, a.N, a.Data, x.Data, x.Inc)
}

// Hemv computes
//  y = alpha * A * x + beta * y,
// where A is an n×n Hermitian matrix, x and y are vectors, and alpha and
// beta are scalars.
func Hemv(alpha complex64, a Hermitian, x Vector, beta complex64, y Vector) {
	cblas64.Chemv(a.Uplo, a.N, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Hbmv performs
//  y = alpha * A * x + beta * y,
// where A is an n×n Hermitian band matrix, x and y are vectors, and alpha
// and beta are scalars.
func Hbmv(alpha complex64, a HermitianBand, x Vector, beta complex64, y Vector) {
	cblas64.Chbmv(a.Uplo, a.N, a.K, alpha, a.Data, a.Stride, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Hpmv performs
//  y = alpha * A * x + beta * y,
// where A is an n×n Hermitian matrix in packed format, x and y are vectors,
// and alpha and beta are scalars.
func Hpmv(alpha complex64, a HermitianPacked, x Vector, beta complex64, y Vector) {
	cblas64.Chpmv(a.Uplo, a.N, alpha, a.Data, x.Data, x.Inc, beta, y.Data, y.Inc)
}

// Geru performs a rank-1 update
//  A += alpha * x * yᵀ,
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func Geru(alpha complex64, x, y Vector, a General) {
	cblas64.Cgeru(a.Rows, a.Cols, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Gerc performs a rank-1 update
//  A += alpha * x * yᴴ,
// where A is an m×n dense matrix, x and y are vectors, and alpha is a scalar.
func Gerc(alpha complex64, x, y Vector, a General) {
	cblas64.Cgerc(a.Rows, a.Cols, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Her performs a rank-1 update
//  A += alpha * x * yᵀ,
// where A is an m×n Hermitian matrix, x and y are vectors, and alpha is a scalar.
func Her(alpha float32, x Vector, a Hermitian) {
	cblas64.Cher(a.Uplo, a.N, alpha, x.Data, x.Inc, a.Data, a.Stride)
}

// Hpr performs a rank-1 update
//  A += alpha * x * xᴴ,
// where A is an n×n Hermitian matrix in packed format, x is a vector, and
// alpha is a scalar.
func Hpr(alpha float32, x Vector, a HermitianPacked) {
	cblas64.Chpr(a.Uplo, a.N, alpha, x.Data, x.Inc, a.Data)
}

// Her2 performs a rank-2 update
//  A += alpha * x * yᴴ + conj(alpha) * y * xᴴ,
// where A is an n×n Hermitian matrix, x and y are vectors, and alpha is a scalar.
func Her2(alpha complex64, x, y Vector, a Hermitian) {
	cblas64.Cher2(a.Uplo, a.N, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data, a.Stride)
}

// Hpr2 performs a rank-2 update
//  A += alpha * x * yᴴ + conj(alpha) * y * xᴴ,
// where A is an n×n Hermitian matrix in packed format, x and y are vectors,
// and alpha is a scalar.
func Hpr2(alpha complex64, x, y Vector, a HermitianPacked) {
	cblas64.Chpr2(a.Uplo, a.N, alpha, x.Data, x.Inc, y.Data, y.Inc, a.Data)
}

// Level 3

// Gemm computes
//  C = alpha * A * B + beta * C,
// where A, B, and C are dense matrices, and alpha and beta are scalars.
// tA and tB specify whether A or B are transposed or conjugated.
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

// Symm performs
//  C = alpha * A * B + beta * C  if s == blas.Left,
//  C = alpha * B * A + beta * C  if s == blas.Right,
// where A is an n×n or m×m symmetric matrix, B and C are m×n matrices, and
// alpha and beta are scalars.
func Symm(s blas.Side, alpha complex64, a Symmetric, b General, beta complex64, c General) {
	var m, n int
	if s == blas.Left {
		m, n = a.N, b.Cols
	} else {
		m, n = b.Rows, a.N
	}
	cblas64.Csymm(s, a.Uplo, m, n, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Syrk performs a symmetric rank-k update
//  C = alpha * A * Aᵀ + beta * C  if t == blas.NoTrans,
//  C = alpha * Aᵀ * A + beta * C  if t == blas.Trans,
// where C is an n×n symmetric matrix, A is an n×k matrix if t == blas.NoTrans
// and a k×n matrix otherwise, and alpha and beta are scalars.
func Syrk(t blas.Transpose, alpha complex64, a General, beta complex64, c Symmetric) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Csyrk(c.Uplo, t, n, k, alpha, a.Data, a.Stride, beta, c.Data, c.Stride)
}

// Syr2k performs a symmetric rank-2k update
//  C = alpha * A * Bᵀ + alpha * B * Aᵀ + beta * C  if t == blas.NoTrans,
//  C = alpha * Aᵀ * B + alpha * Bᵀ * A + beta * C  if t == blas.Trans,
// where C is an n×n symmetric matrix, A and B are n×k matrices if
// t == blas.NoTrans and k×n otherwise, and alpha and beta are scalars.
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
//  B = alpha * A * B   if tA == blas.NoTrans and s == blas.Left,
//  B = alpha * Aᵀ * B  if tA == blas.Trans and s == blas.Left,
//  B = alpha * Aᴴ * B  if tA == blas.ConjTrans and s == blas.Left,
//  B = alpha * B * A   if tA == blas.NoTrans and s == blas.Right,
//  B = alpha * B * Aᵀ  if tA == blas.Trans and s == blas.Right,
//  B = alpha * B * Aᴴ  if tA == blas.ConjTrans and s == blas.Right,
// where A is an n×n or m×m triangular matrix, B is an m×n matrix, and alpha is
// a scalar.
func Trmm(s blas.Side, tA blas.Transpose, alpha complex64, a Triangular, b General) {
	cblas64.Ctrmm(s, a.Uplo, tA, a.Diag, b.Rows, b.Cols, alpha, a.Data, a.Stride, b.Data, b.Stride)
}

// Trsm solves
//  A * X = alpha * B   if tA == blas.NoTrans and s == blas.Left,
//  Aᵀ * X = alpha * B  if tA == blas.Trans and s == blas.Left,
//  Aᴴ * X = alpha * B  if tA == blas.ConjTrans and s == blas.Left,
//  X * A = alpha * B   if tA == blas.NoTrans and s == blas.Right,
//  X * Aᵀ = alpha * B  if tA == blas.Trans and s == blas.Right,
//  X * Aᴴ = alpha * B  if tA == blas.ConjTrans and s == blas.Right,
// where A is an n×n or m×m triangular matrix, X and B are m×n matrices, and
// alpha is a scalar.
//
// At entry to the function, b contains the values of B, and the result is
// stored in-place into b.
//
// No check is made that A is invertible.
func Trsm(s blas.Side, tA blas.Transpose, alpha complex64, a Triangular, b General) {
	cblas64.Ctrsm(s, a.Uplo, tA, a.Diag, b.Rows, b.Cols, alpha, a.Data, a.Stride, b.Data, b.Stride)
}

// Hemm performs
//  C = alpha * A * B + beta * C  if s == blas.Left,
//  C = alpha * B * A + beta * C  if s == blas.Right,
// where A is an n×n or m×m Hermitian matrix, B and C are m×n matrices, and
// alpha and beta are scalars.
func Hemm(s blas.Side, alpha complex64, a Hermitian, b General, beta complex64, c General) {
	var m, n int
	if s == blas.Left {
		m, n = a.N, b.Cols
	} else {
		m, n = b.Rows, a.N
	}
	cblas64.Chemm(s, a.Uplo, m, n, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}

// Herk performs the Hermitian rank-k update
//  C = alpha * A * Aᴴ + beta*C  if t == blas.NoTrans,
//  C = alpha * Aᴴ * A + beta*C  if t == blas.ConjTrans,
// where C is an n×n Hermitian matrix, A is an n×k matrix if t == blas.NoTrans
// and a k×n matrix otherwise, and alpha and beta are scalars.
func Herk(t blas.Transpose, alpha float32, a General, beta float32, c Hermitian) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Cherk(c.Uplo, t, n, k, alpha, a.Data, a.Stride, beta, c.Data, c.Stride)
}

// Her2k performs the Hermitian rank-2k update
//  C = alpha * A * Bᴴ + conj(alpha) * B * Aᴴ + beta * C  if t == blas.NoTrans,
//  C = alpha * Aᴴ * B + conj(alpha) * Bᴴ * A + beta * C  if t == blas.ConjTrans,
// where C is an n×n Hermitian matrix, A and B are n×k matrices if t == NoTrans
// and k×n matrices otherwise, and alpha and beta are scalars.
func Her2k(t blas.Transpose, alpha complex64, a, b General, beta float32, c Hermitian) {
	var n, k int
	if t == blas.NoTrans {
		n, k = a.Rows, a.Cols
	} else {
		n, k = a.Cols, a.Rows
	}
	cblas64.Cher2k(c.Uplo, t, n, k, alpha, a.Data, a.Stride, b.Data, b.Stride, beta, c.Data, c.Stride)
}
