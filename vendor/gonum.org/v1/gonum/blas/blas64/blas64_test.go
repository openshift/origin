// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package blas64

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/testblas"
)

var impl = b64{}

func TestDasum(t *testing.T)  { testblas.DasumTest(t, impl) }
func TestDaxpy(t *testing.T)  { testblas.DaxpyTest(t, impl) }
func TestDdot(t *testing.T)   { testblas.DdotTest(t, impl) }
func TestDnrm2(t *testing.T)  { testblas.Dnrm2Test(t, impl) }
func TestIdamax(t *testing.T) { testblas.IdamaxTest(t, impl) }
func TestDswap(t *testing.T)  { testblas.DswapTest(t, impl) }
func TestDcopy(t *testing.T)  { testblas.DcopyTest(t, impl) }
func TestDrotg(t *testing.T)  { testblas.DrotgTest(t, impl) }
func TestDrotmg(t *testing.T) { testblas.DrotmgTest(t, impl) }
func TestDrot(t *testing.T)   { testblas.DrotTest(t, impl) }
func TestDrotm(t *testing.T)  { testblas.DrotmTest(t, impl) }
func TestDscal(t *testing.T)  { testblas.DscalTest(t, impl) }
func TestDgemv(t *testing.T)  { testblas.DgemvTest(t, impl) }
func TestDger(t *testing.T)   { testblas.DgerTest(t, impl) }
func TestDtxmv(t *testing.T)  { testblas.DtxmvTest(t, impl) }
func TestDgbmv(t *testing.T)  { testblas.DgbmvTest(t, impl) }
func TestDtbsv(t *testing.T)  { testblas.DtbsvTest(t, impl) }
func TestDsbmv(t *testing.T)  { testblas.DsbmvTest(t, impl) }
func TestDtbmv(t *testing.T)  { testblas.DtbmvTest(t, impl) }
func TestDtrsv(t *testing.T)  { testblas.DtrsvTest(t, impl) }
func TestDtrmv(t *testing.T)  { testblas.DtrmvTest(t, impl) }
func TestDsymv(t *testing.T)  { testblas.DsymvTest(t, impl) }
func TestDsyr(t *testing.T)   { testblas.DsyrTest(t, impl) }
func TestDsyr2(t *testing.T)  { testblas.Dsyr2Test(t, impl) }
func TestDspr2(t *testing.T)  { testblas.Dspr2Test(t, impl) }
func TestDspr(t *testing.T)   { testblas.DsprTest(t, impl) }
func TestDspmv(t *testing.T)  { testblas.DspmvTest(t, impl) }
func TestDtpsv(t *testing.T)  { testblas.DtpsvTest(t, impl) }
func TestDtpmv(t *testing.T)  { testblas.DtpmvTest(t, impl) }
func TestDgemm(t *testing.T)  { testblas.TestDgemm(t, impl) }
func TestDsymm(t *testing.T)  { testblas.DsymmTest(t, impl) }
func TestDtrsm(t *testing.T)  { testblas.DtrsmTest(t, impl) }
func TestDsyrk(t *testing.T)  { testblas.DsyrkTest(t, impl) }
func TestDsyr2k(t *testing.T) { testblas.Dsyr2kTest(t, impl) }
func TestDtrmm(t *testing.T)  { testblas.DtrmmTest(t, impl) }

type b64 struct{}

var _ blas.Float64 = b64{}

func (b64) Ddot(n int, x []float64, incX int, y []float64, incY int) float64 {
	return Dot(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (b64) Dnrm2(n int, x []float64, incX int) float64 {
	if incX < 0 {
		return 0
	}
	return Nrm2(Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dasum(n int, x []float64, incX int) float64 {
	if incX < 0 {
		return 0
	}
	return Asum(Vector{N: n, Inc: incX, Data: x})
}
func (b64) Idamax(n int, x []float64, incX int) int {
	if incX < 0 {
		return -1
	}
	return Iamax(Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dswap(n int, x []float64, incX int, y []float64, incY int) {
	Swap(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (b64) Dcopy(n int, x []float64, incX int, y []float64, incY int) {
	Copy(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (b64) Daxpy(n int, alpha float64, x []float64, incX int, y []float64, incY int) {
	Axpy(alpha, Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (b64) Drotg(a, b float64) (c, s, r, z float64) {
	return Rotg(a, b)
}
func (b64) Drotmg(d1, d2, b1, b2 float64) (p blas.DrotmParams, rd1, rd2, rb1 float64) {
	return Rotmg(d1, d2, b1, b2)
}
func (b64) Drot(n int, x []float64, incX int, y []float64, incY int, c float64, s float64) {
	Rot(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y}, c, s)
}
func (b64) Drotm(n int, x []float64, incX int, y []float64, incY int, p blas.DrotmParams) {
	Rotm(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y}, p)
}
func (b64) Dscal(n int, alpha float64, x []float64, incX int) {
	if incX < 0 {
		return
	}
	Scal(alpha, Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dgemv(tA blas.Transpose, m, n int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	lenX := m
	lenY := n
	if tA == blas.NoTrans {
		lenX = n
		lenY = m
	}
	Gemv(tA, alpha,
		General{Rows: m, Cols: n, Data: a, Stride: lda},
		Vector{N: lenX, Inc: incX, Data: x},
		beta,
		Vector{N: lenY, Inc: incY, Data: y})
}
func (b64) Dgbmv(tA blas.Transpose, m, n, kL, kU int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	lenX := m
	lenY := n
	if tA == blas.NoTrans {
		lenX = n
		lenY = m
	}
	Gbmv(tA, alpha,
		Band{Rows: m, Cols: n, KL: kL, KU: kU, Data: a, Stride: lda},
		Vector{N: lenX, Inc: incX, Data: x},
		beta,
		Vector{N: lenY, Inc: incY, Data: y})
}
func (b64) Dtrmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float64, lda int, x []float64, incX int) {
	Trmv(tA,
		Triangular{Uplo: ul, Diag: d, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dtbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float64, lda int, x []float64, incX int) {
	Tbmv(tA,
		TriangularBand{Uplo: ul, Diag: d, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dtpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap []float64, x []float64, incX int) {
	Tpmv(tA,
		TriangularPacked{Uplo: ul, Diag: d, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dtrsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []float64, lda int, x []float64, incX int) {
	Trsv(tA,
		Triangular{Uplo: ul, Diag: d, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dtbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []float64, lda int, x []float64, incX int) {
	Tbsv(tA,
		TriangularBand{Uplo: ul, Diag: d, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dtpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap []float64, x []float64, incX int) {
	Tpsv(tA,
		TriangularPacked{Uplo: ul, Diag: d, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x})
}
func (b64) Dsymv(ul blas.Uplo, n int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	Symv(alpha,
		Symmetric{Uplo: ul, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (b64) Dsbmv(ul blas.Uplo, n, k int, alpha float64, a []float64, lda int, x []float64, incX int, beta float64, y []float64, incY int) {
	Sbmv(alpha,
		SymmetricBand{Uplo: ul, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (b64) Dspmv(ul blas.Uplo, n int, alpha float64, ap []float64, x []float64, incX int, beta float64, y []float64, incY int) {
	Spmv(alpha,
		SymmetricPacked{Uplo: ul, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (b64) Dger(m, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64, lda int) {
	Ger(alpha,
		Vector{N: m, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		General{Rows: m, Cols: n, Data: a, Stride: lda})
}
func (b64) Dsyr(ul blas.Uplo, n int, alpha float64, x []float64, incX int, a []float64, lda int) {
	Syr(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Symmetric{Uplo: ul, N: n, Data: a, Stride: lda})
}
func (b64) Dspr(ul blas.Uplo, n int, alpha float64, x []float64, incX int, ap []float64) {
	Spr(alpha,
		Vector{N: n, Inc: incX, Data: x},
		SymmetricPacked{Uplo: ul, N: n, Data: ap})
}
func (b64) Dsyr2(ul blas.Uplo, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64, lda int) {
	Syr2(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		Symmetric{Uplo: ul, N: n, Data: a, Stride: lda})
}
func (b64) Dspr2(ul blas.Uplo, n int, alpha float64, x []float64, incX int, y []float64, incY int, a []float64) {
	Spr2(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		SymmetricPacked{Uplo: ul, N: n, Data: a})
}
func (b64) Dgemm(tA, tB blas.Transpose, m, n, k int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	am, an := m, k
	if tA != blas.NoTrans {
		am, an = an, am
	}
	bm, bn := k, n
	if tB != blas.NoTrans {
		bm, bn = bn, bm
	}
	Gemm(tA, tB, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		General{Rows: bm, Cols: bn, Data: b, Stride: ldb},
		beta,
		General{Rows: m, Cols: n, Data: c, Stride: ldc})
}
func (b64) Dsymm(s blas.Side, ul blas.Uplo, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	var an int
	switch s {
	case blas.Left:
		an = m
	case blas.Right:
		an = n
	default:
		panic(fmt.Sprintf("blas64: bad test: invalid side: %q", s))
	}
	Symm(s, alpha,
		Symmetric{Uplo: ul, N: an, Data: a, Stride: lda},
		General{Rows: m, Cols: n, Data: b, Stride: ldb},
		beta,
		General{Rows: m, Cols: n, Data: c, Stride: ldc})
}
func (b64) Dsyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []float64, lda int, beta float64, c []float64, ldc int) {
	am, an := n, k
	if t != blas.NoTrans {
		am, an = an, am
	}
	Syrk(t, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		beta,
		Symmetric{Uplo: ul, N: n, Data: c, Stride: ldc})
}
func (b64) Dsyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []float64, lda int, b []float64, ldb int, beta float64, c []float64, ldc int) {
	am, an := n, k
	if t != blas.NoTrans {
		am, an = an, am
	}
	Syr2k(t, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		General{Rows: am, Cols: an, Data: b, Stride: ldb},
		beta,
		Symmetric{Uplo: ul, N: n, Data: c, Stride: ldc})
}
func (b64) Dtrmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int) {
	var k int
	switch s {
	case blas.Left:
		k = m
	case blas.Right:
		k = n
	default:
		panic(fmt.Sprintf("blas64: bad test: invalid side: %q", s))
	}
	Trmm(s, tA, alpha,
		Triangular{Uplo: ul, Diag: d, N: k, Data: a, Stride: lda},
		General{Rows: m, Cols: n, Data: b, Stride: ldb})
}
func (b64) Dtrsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha float64, a []float64, lda int, b []float64, ldb int) {
	var k int
	switch s {
	case blas.Left:
		k = m
	case blas.Right:
		k = n
	default:
		panic(fmt.Sprintf("blas64: bad test: invalid side: %q", s))
	}
	Trsm(s, tA, alpha,
		Triangular{Uplo: ul, Diag: d, N: k, Data: a, Stride: lda},
		General{Rows: m, Cols: n, Data: b, Stride: ldb})
}
