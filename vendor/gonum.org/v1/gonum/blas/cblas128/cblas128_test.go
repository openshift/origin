// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cblas128

import (
	"fmt"
	"testing"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/blas/testblas"
)

var impl = c128{}

func TestDzasum(t *testing.T) { testblas.DzasumTest(t, impl) }
func TestDznrm2(t *testing.T) { testblas.Dznrm2Test(t, impl) }
func TestIzamax(t *testing.T) { testblas.IzamaxTest(t, impl) }
func TestZaxpy(t *testing.T)  { testblas.ZaxpyTest(t, impl) }
func TestZcopy(t *testing.T)  { testblas.ZcopyTest(t, impl) }
func TestZdotc(t *testing.T)  { testblas.ZdotcTest(t, impl) }
func TestZdotu(t *testing.T)  { testblas.ZdotuTest(t, impl) }
func TestZdscal(t *testing.T) { testblas.ZdscalTest(t, impl) }
func TestZscal(t *testing.T)  { testblas.ZscalTest(t, impl) }
func TestZswap(t *testing.T)  { testblas.ZswapTest(t, impl) }
func TestZgbmv(t *testing.T)  { testblas.ZgbmvTest(t, impl) }
func TestZgemv(t *testing.T)  { testblas.ZgemvTest(t, impl) }
func TestZgerc(t *testing.T)  { testblas.ZgercTest(t, impl) }
func TestZgeru(t *testing.T)  { testblas.ZgeruTest(t, impl) }
func TestZhbmv(t *testing.T)  { testblas.ZhbmvTest(t, impl) }
func TestZhemv(t *testing.T)  { testblas.ZhemvTest(t, impl) }
func TestZher(t *testing.T)   { testblas.ZherTest(t, impl) }
func TestZher2(t *testing.T)  { testblas.Zher2Test(t, impl) }
func TestZhpmv(t *testing.T)  { testblas.ZhpmvTest(t, impl) }
func TestZhpr(t *testing.T)   { testblas.ZhprTest(t, impl) }
func TestZhpr2(t *testing.T)  { testblas.Zhpr2Test(t, impl) }
func TestZtbmv(t *testing.T)  { testblas.ZtbmvTest(t, impl) }
func TestZtbsv(t *testing.T)  { testblas.ZtbsvTest(t, impl) }
func TestZtpmv(t *testing.T)  { testblas.ZtpmvTest(t, impl) }
func TestZtpsv(t *testing.T)  { testblas.ZtpsvTest(t, impl) }
func TestZtrmv(t *testing.T)  { testblas.ZtrmvTest(t, impl) }
func TestZtrsv(t *testing.T)  { testblas.ZtrsvTest(t, impl) }
func TestZgemm(t *testing.T)  { testblas.ZgemmTest(t, impl) }
func TestZhemm(t *testing.T)  { testblas.ZhemmTest(t, impl) }
func TestZherk(t *testing.T)  { testblas.ZherkTest(t, impl) }
func TestZher2k(t *testing.T) { testblas.Zher2kTest(t, impl) }
func TestZsymm(t *testing.T)  { testblas.ZsymmTest(t, impl) }
func TestZsyrk(t *testing.T)  { testblas.ZsyrkTest(t, impl) }
func TestZsyr2k(t *testing.T) { testblas.Zsyr2kTest(t, impl) }
func TestZtrmm(t *testing.T)  { testblas.ZtrmmTest(t, impl) }
func TestZtrsm(t *testing.T)  { testblas.ZtrsmTest(t, impl) }

type c128 struct{}

var _ blas.Complex128 = c128{}

func (c128) Zdotu(n int, x []complex128, incX int, y []complex128, incY int) complex128 {
	return Dotu(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zdotc(n int, x []complex128, incX int, y []complex128, incY int) complex128 {
	return Dotc(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (c128) Dznrm2(n int, x []complex128, incX int) float64 {
	if incX < 0 {
		return 0
	}
	return Nrm2(Vector{N: n, Inc: incX, Data: x})
}
func (c128) Dnrm2(n int, x []float64, incX int) float64 {
	return blas64.Nrm2(blas64.Vector{N: n, Inc: incX, Data: x})
}
func (c128) Dzasum(n int, x []complex128, incX int) float64 {
	if incX < 0 {
		return 0
	}
	return Asum(Vector{N: n, Inc: incX, Data: x})
}
func (c128) Izamax(n int, x []complex128, incX int) int {
	if incX < 0 {
		return -1
	}
	return Iamax(Vector{N: n, Inc: incX, Data: x})
}
func (c128) Zswap(n int, x []complex128, incX int, y []complex128, incY int) {
	Swap(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zcopy(n int, x []complex128, incX int, y []complex128, incY int) {
	Copy(Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zaxpy(n int, alpha complex128, x []complex128, incX int, y []complex128, incY int) {
	Axpy(alpha, Vector{N: n, Inc: incX, Data: x}, Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zscal(n int, alpha complex128, x []complex128, incX int) {
	if incX < 0 {
		return
	}
	Scal(alpha, Vector{N: n, Inc: incX, Data: x})
}
func (c128) Zdscal(n int, alpha float64, x []complex128, incX int) {
	if incX < 0 {
		return
	}
	Dscal(alpha, Vector{N: n, Inc: incX, Data: x})
}
func (c128) Zgemv(tA blas.Transpose, m, n int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
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
func (c128) Zgbmv(tA blas.Transpose, m, n, kL, kU int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
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
func (c128) Ztrmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex128, lda int, x []complex128, incX int) {
	Trmv(tA,
		Triangular{Uplo: ul, Diag: d, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Ztbmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex128, lda int, x []complex128, incX int) {
	Tbmv(tA,
		TriangularBand{Uplo: ul, Diag: d, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Ztpmv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap []complex128, x []complex128, incX int) {
	Tpmv(tA,
		TriangularPacked{Uplo: ul, Diag: d, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Ztrsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, a []complex128, lda int, x []complex128, incX int) {
	Trsv(tA,
		Triangular{Uplo: ul, Diag: d, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Ztbsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n, k int, a []complex128, lda int, x []complex128, incX int) {
	Tbsv(tA,
		TriangularBand{Uplo: ul, Diag: d, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Ztpsv(ul blas.Uplo, tA blas.Transpose, d blas.Diag, n int, ap []complex128, x []complex128, incX int) {
	Tpsv(tA,
		TriangularPacked{Uplo: ul, Diag: d, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x})
}
func (c128) Zhemv(ul blas.Uplo, n int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	Hemv(alpha,
		Hermitian{Uplo: ul, N: n, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zhbmv(ul blas.Uplo, n, k int, alpha complex128, a []complex128, lda int, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	Hbmv(alpha,
		HermitianBand{Uplo: ul, N: n, K: k, Data: a, Stride: lda},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zhpmv(ul blas.Uplo, n int, alpha complex128, ap []complex128, x []complex128, incX int, beta complex128, y []complex128, incY int) {
	Hpmv(alpha,
		HermitianPacked{Uplo: ul, N: n, Data: ap},
		Vector{N: n, Inc: incX, Data: x},
		beta,
		Vector{N: n, Inc: incY, Data: y})
}
func (c128) Zgeru(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	Geru(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		General{Rows: m, Cols: n, Data: a, Stride: lda})
}
func (c128) Zgerc(m, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	Gerc(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		General{Rows: m, Cols: n, Data: a, Stride: lda})
}
func (c128) Zher(ul blas.Uplo, n int, alpha float64, x []complex128, incX int, a []complex128, lda int) {
	Her(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Hermitian{Uplo: ul, N: n, Data: a, Stride: lda})
}
func (c128) Zhpr(ul blas.Uplo, n int, alpha float64, x []complex128, incX int, ap []complex128) {
	Hpr(alpha,
		Vector{N: n, Inc: incX, Data: x},
		HermitianPacked{Uplo: ul, N: n, Data: ap})
}
func (c128) Zher2(ul blas.Uplo, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128, lda int) {
	Her2(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		Hermitian{Uplo: ul, N: n, Data: a, Stride: lda})
}
func (c128) Zhpr2(ul blas.Uplo, n int, alpha complex128, x []complex128, incX int, y []complex128, incY int, a []complex128) {
	Hpr2(alpha,
		Vector{N: n, Inc: incX, Data: x},
		Vector{N: n, Inc: incY, Data: y},
		HermitianPacked{Uplo: ul, N: n, Data: a})
}
func (c128) Zgemm(tA, tB blas.Transpose, m, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
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
func (c128) Zsymm(s blas.Side, ul blas.Uplo, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
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
func (c128) Zsyrk(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, beta complex128, c []complex128, ldc int) {
	am, an := n, k
	if t != blas.NoTrans {
		am, an = an, am
	}
	Syrk(t, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		beta,
		Symmetric{Uplo: ul, N: n, Data: c, Stride: ldc})
}
func (c128) Zsyr2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
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
func (c128) Zhemm(s blas.Side, ul blas.Uplo, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta complex128, c []complex128, ldc int) {
	var an int
	switch s {
	case blas.Left:
		an = m
	case blas.Right:
		an = n
	default:
		panic(fmt.Sprintf("blas64: bad test: invalid side: %q", s))
	}
	Hemm(s, alpha,
		Hermitian{Uplo: ul, N: an, Data: a, Stride: lda},
		General{Rows: m, Cols: n, Data: b, Stride: ldb},
		beta,
		General{Rows: m, Cols: n, Data: c, Stride: ldc})
}
func (c128) Zherk(ul blas.Uplo, t blas.Transpose, n, k int, alpha float64, a []complex128, lda int, beta float64, c []complex128, ldc int) {
	am, an := n, k
	if t != blas.NoTrans {
		am, an = an, am
	}
	Herk(t, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		beta,
		Hermitian{Uplo: ul, N: n, Data: c, Stride: ldc})
}
func (c128) Zher2k(ul blas.Uplo, t blas.Transpose, n, k int, alpha complex128, a []complex128, lda int, b []complex128, ldb int, beta float64, c []complex128, ldc int) {
	am, an := n, k
	if t != blas.NoTrans {
		am, an = an, am
	}
	Her2k(t, alpha,
		General{Rows: am, Cols: an, Data: a, Stride: lda},
		General{Rows: am, Cols: an, Data: b, Stride: ldb},
		beta,
		Hermitian{Uplo: ul, N: n, Data: c, Stride: ldc})
}
func (c128) Ztrmm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int) {
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
func (c128) Ztrsm(s blas.Side, ul blas.Uplo, tA blas.Transpose, d blas.Diag, m, n int, alpha complex128, a []complex128, lda int, b []complex128, ldb int) {
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
