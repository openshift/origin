// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

// TODO: Need to add tests where one is overwritten.
func TestMulTypes(t *testing.T) {
	for _, test := range []struct {
		ar     int
		ac     int
		br     int
		bc     int
		Panics bool
	}{
		{
			ar:     5,
			ac:     5,
			br:     5,
			bc:     5,
			Panics: false,
		},
		{
			ar:     10,
			ac:     5,
			br:     5,
			bc:     3,
			Panics: false,
		},
		{
			ar:     10,
			ac:     5,
			br:     5,
			bc:     8,
			Panics: false,
		},
		{
			ar:     8,
			ac:     10,
			br:     10,
			bc:     3,
			Panics: false,
		},
		{
			ar:     8,
			ac:     3,
			br:     3,
			bc:     10,
			Panics: false,
		},
		{
			ar:     5,
			ac:     8,
			br:     8,
			bc:     10,
			Panics: false,
		},
		{
			ar:     5,
			ac:     12,
			br:     12,
			bc:     8,
			Panics: false,
		},
		{
			ar:     5,
			ac:     7,
			br:     8,
			bc:     10,
			Panics: true,
		},
	} {
		ar := test.ar
		ac := test.ac
		br := test.br
		bc := test.bc

		// Generate random matrices
		avec := make([]float64, ar*ac)
		randomSlice(avec)
		a := NewDense(ar, ac, avec)

		bvec := make([]float64, br*bc)
		randomSlice(bvec)

		b := NewDense(br, bc, bvec)

		// Check that it panics if it is supposed to
		if test.Panics {
			c := &Dense{}
			fn := func() {
				c.Mul(a, b)
			}
			pan, _ := panics(fn)
			if !pan {
				t.Errorf("Mul did not panic with dimension mismatch")
			}
			continue
		}

		cvec := make([]float64, ar*bc)

		// Get correct matrix multiply answer from blas64.Gemm
		blas64.Gemm(blas.NoTrans, blas.NoTrans,
			1, a.mat, b.mat,
			0, blas64.General{Rows: ar, Cols: bc, Stride: bc, Data: cvec},
		)

		avecCopy := append([]float64{}, avec...)
		bvecCopy := append([]float64{}, bvec...)
		cvecCopy := append([]float64{}, cvec...)

		acomp := matComp{r: ar, c: ac, data: avecCopy}
		bcomp := matComp{r: br, c: bc, data: bvecCopy}
		ccomp := matComp{r: ar, c: bc, data: cvecCopy}

		// Do normal multiply with empty dense
		d := &Dense{}

		testMul(t, a, b, d, acomp, bcomp, ccomp, false, "zero receiver")

		// Normal multiply with existing receiver
		c := NewDense(ar, bc, cvec)
		randomSlice(cvec)
		testMul(t, a, b, c, acomp, bcomp, ccomp, false, "existing receiver")

		// Cast a as a basic matrix
		am := (*basicMatrix)(a)
		bm := (*basicMatrix)(b)
		d.Reset()
		testMul(t, am, b, d, acomp, bcomp, ccomp, true, "a is basic, receiver is zero")
		d.Reset()
		testMul(t, a, bm, d, acomp, bcomp, ccomp, true, "b is basic, receiver is zero")
		d.Reset()
		testMul(t, am, bm, d, acomp, bcomp, ccomp, true, "both basic, receiver is zero")
		randomSlice(cvec)
		testMul(t, am, b, d, acomp, bcomp, ccomp, true, "a is basic, receiver is full")
		randomSlice(cvec)
		testMul(t, a, bm, d, acomp, bcomp, ccomp, true, "b is basic, receiver is full")
		randomSlice(cvec)
		testMul(t, am, bm, d, acomp, bcomp, ccomp, true, "both basic, receiver is full")
	}
}

func randomSlice(s []float64) {
	for i := range s {
		s[i] = rand.NormFloat64()
	}
}

type matComp struct {
	r, c int
	data []float64
}

func testMul(t *testing.T, a, b Matrix, c *Dense, acomp, bcomp, ccomp matComp, cvecApprox bool, name string) {
	c.Mul(a, b)
	var aDense *Dense
	switch t := a.(type) {
	case *Dense:
		aDense = t
	case *basicMatrix:
		aDense = (*Dense)(t)
	}

	var bDense *Dense
	switch t := b.(type) {
	case *Dense:
		bDense = t
	case *basicMatrix:
		bDense = (*Dense)(t)
	}

	if !denseEqual(aDense, acomp) {
		t.Errorf("a changed unexpectedly for %v", name)
	}
	if !denseEqual(bDense, bcomp) {
		t.Errorf("b changed unexpectedly for %v", name)
	}
	if cvecApprox {
		if !denseEqualApprox(c, ccomp, 1e-14) {
			t.Errorf("mul answer not within tol for %v", name)
		}
		return
	}

	if !denseEqual(c, ccomp) {
		t.Errorf("mul answer not equal for %v", name)
	}
}

type basicMatrix Dense

var _ Matrix = &basicMatrix{}

func (m *basicMatrix) At(r, c int) float64 {
	return (*Dense)(m).At(r, c)
}

func (m *basicMatrix) Dims() (r, c int) {
	return (*Dense)(m).Dims()
}

func (m *basicMatrix) T() Matrix {
	return Transpose{m}
}

type basicBanded BandDense

var _ Banded = &basicBanded{}

func (m *basicBanded) At(r, c int) float64 {
	return (*BandDense)(m).At(r, c)
}

func (m *basicBanded) Dims() (r, c int) {
	return (*BandDense)(m).Dims()
}

func (m *basicBanded) Bandwidth() (kl, ku int) {
	return (*BandDense)(m).Bandwidth()
}

func (m *basicBanded) T() Matrix {
	return Transpose{m}
}

func (m *basicBanded) TBand() Banded {
	return TransposeBand{m}
}

type basicSymmetric SymDense

var _ Symmetric = &basicSymmetric{}

func (m *basicSymmetric) At(r, c int) float64 {
	return (*SymDense)(m).At(r, c)
}

func (m *basicSymmetric) Dims() (r, c int) {
	return (*SymDense)(m).Dims()
}

func (m *basicSymmetric) T() Matrix {
	return m
}

func (m *basicSymmetric) Symmetric() int {
	return (*SymDense)(m).Symmetric()
}

type basicTriangular TriDense

var _ Triangular = &basicTriangular{}

func (m *basicTriangular) At(r, c int) float64 {
	return (*TriDense)(m).At(r, c)
}

func (m *basicTriangular) Dims() (r, c int) {
	return (*TriDense)(m).Dims()
}

func (m *basicTriangular) T() Matrix {
	return Transpose{m}
}

func (m *basicTriangular) Triangle() (int, TriKind) {
	return (*TriDense)(m).Triangle()
}

func (m *basicTriangular) TTri() Triangular {
	return TransposeTri{m}
}

type basicSymBanded SymBandDense

var _ SymBanded = &basicSymBanded{}

func (m *basicSymBanded) At(r, c int) float64 {
	return (*SymBandDense)(m).At(r, c)
}

func (m *basicSymBanded) Dims() (r, c int) {
	return (*SymBandDense)(m).Dims()
}

func (m *basicSymBanded) T() Matrix {
	return m
}

func (m *basicSymBanded) TBand() Banded {
	return m
}

func (m *basicSymBanded) Symmetric() int {
	return (*SymBandDense)(m).Symmetric()
}

func (m *basicSymBanded) SymBand() (n, k int) {
	return (*SymBandDense)(m).SymBand()
}

func (m *basicSymBanded) Bandwidth() (kl, ku int) {
	return (*SymBandDense)(m).Bandwidth()
}

type basicTriBanded TriBandDense

var _ TriBanded = &basicTriBanded{}

func (m *basicTriBanded) At(r, c int) float64 {
	return (*TriBandDense)(m).At(r, c)
}

func (m *basicTriBanded) Dims() (r, c int) {
	return (*TriBandDense)(m).Dims()
}

func (m *basicTriBanded) T() Matrix {
	return Transpose{m}
}

func (m *basicTriBanded) TTri() Triangular {
	return TransposeTri{m}
}

func (m *basicTriBanded) TBand() Banded {
	return TransposeBand{m}
}

func (m *basicTriBanded) TTriBand() TriBanded {
	return TransposeTriBand{m}
}

func (m *basicTriBanded) Bandwidth() (kl, ku int) {
	return (*TriBandDense)(m).Bandwidth()
}

func (m *basicTriBanded) Triangle() (int, TriKind) {
	return (*TriBandDense)(m).Triangle()
}

func (m *basicTriBanded) TriBand() (n, k int, kind TriKind) {
	return (*TriBandDense)(m).TriBand()
}

type basicDiagonal DiagDense

var _ Diagonal = &basicDiagonal{}

func (m *basicDiagonal) At(r, c int) float64 {
	return (*DiagDense)(m).At(r, c)
}

func (m *basicDiagonal) Dims() (r, c int) {
	return (*DiagDense)(m).Dims()
}

func (m *basicDiagonal) Diag() int {
	return (*DiagDense)(m).Diag()
}

func (m *basicDiagonal) T() Matrix {
	return Transpose{m}
}

func (m *basicDiagonal) TTri() Triangular {
	return TransposeTri{m}
}

func (m *basicDiagonal) TBand() Banded {
	return TransposeBand{m}
}

func (m *basicDiagonal) TTriBand() TriBanded {
	return TransposeTriBand{m}
}

func (m *basicDiagonal) Bandwidth() (kl, ku int) {
	return (*DiagDense)(m).Bandwidth()
}

func (m *basicDiagonal) Symmetric() int {
	return (*DiagDense)(m).Symmetric()
}

func (m *basicDiagonal) SymBand() (n, k int) {
	return (*DiagDense)(m).SymBand()
}

func (m *basicDiagonal) Triangle() (int, TriKind) {
	return (*DiagDense)(m).Triangle()
}

func (m *basicDiagonal) TriBand() (n, k int, kind TriKind) {
	return (*DiagDense)(m).TriBand()
}

func denseEqual(a *Dense, acomp matComp) bool {
	ar2, ac2 := a.Dims()
	if ar2 != acomp.r {
		return false
	}
	if ac2 != acomp.c {
		return false
	}
	if !floats.Equal(a.mat.Data, acomp.data) {
		return false
	}
	return true
}

func denseEqualApprox(a *Dense, acomp matComp, tol float64) bool {
	ar2, ac2 := a.Dims()
	if ar2 != acomp.r {
		return false
	}
	if ac2 != acomp.c {
		return false
	}
	if !floats.EqualApprox(a.mat.Data, acomp.data, tol) {
		return false
	}
	return true
}
