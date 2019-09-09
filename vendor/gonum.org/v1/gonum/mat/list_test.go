// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas"
	"gonum.org/v1/gonum/blas/blas64"
	"gonum.org/v1/gonum/floats"
)

// legalSizeSameRectangular returns whether the two matrices have the same rectangular shape.
func legalSizeSameRectangular(ar, ac, br, bc int) bool {
	if ar != br {
		return false
	}
	if ac != bc {
		return false
	}
	return true
}

// legalSizeSameSquare returns whether the two matrices have the same square shape.
func legalSizeSameSquare(ar, ac, br, bc int) bool {
	if ar != br {
		return false
	}
	if ac != bc {
		return false
	}
	if ar != ac {
		return false
	}
	return true
}

// legalSizeSameHeight returns whether the two matrices have the same number of rows.
func legalSizeSameHeight(ar, _, br, _ int) bool {
	return ar == br
}

// legalSizeSameWidth returns whether the two matrices have the same number of columns.
func legalSizeSameWidth(_, ac, _, bc int) bool {
	return ac == bc
}

// legalSizeSolve returns whether the two matrices can be used in a linear solve.
func legalSizeSolve(ar, ac, br, bc int) bool {
	return ar == br
}

// legalSizeSameVec returns whether the two matrices are column vectors.
func legalSizeVector(_, ac, _, bc int) bool {
	return ac == 1 && bc == 1
}

// legalSizeSameVec returns whether the two matrices are column vectors of the
// same dimension.
func legalSizeSameVec(ar, ac, br, bc int) bool {
	return ac == 1 && bc == 1 && ar == br
}

// isAnySize returns true for all matrix sizes.
func isAnySize(ar, ac int) bool {
	return true
}

// isAnySize2 returns true for all matrix sizes.
func isAnySize2(ar, ac, br, bc int) bool {
	return true
}

// isAnyColumnVector returns true for any column vector sizes.
func isAnyColumnVector(ar, ac int) bool {
	return ac == 1
}

// isSquare returns whether the input matrix is square.
func isSquare(r, c int) bool {
	return r == c
}

// sameAnswerFloat returns whether the two inputs are both NaN or are equal.
func sameAnswerFloat(a, b interface{}) bool {
	if math.IsNaN(a.(float64)) {
		return math.IsNaN(b.(float64))
	}
	return a.(float64) == b.(float64)
}

// sameAnswerFloatApproxTol returns a function that determines whether its two
// inputs are both NaN or within tol of each other.
func sameAnswerFloatApproxTol(tol float64) func(a, b interface{}) bool {
	return func(a, b interface{}) bool {
		if math.IsNaN(a.(float64)) {
			return math.IsNaN(b.(float64))
		}
		return floats.EqualWithinAbsOrRel(a.(float64), b.(float64), tol, tol)
	}
}

func sameAnswerF64SliceOfSlice(a, b interface{}) bool {
	for i, v := range a.([][]float64) {
		if same := floats.Same(v, b.([][]float64)[i]); !same {
			return false
		}
	}
	return true
}

// sameAnswerBool returns whether the two inputs have the same value.
func sameAnswerBool(a, b interface{}) bool {
	return a.(bool) == b.(bool)
}

// isAnyType returns true for all Matrix types.
func isAnyType(Matrix) bool {
	return true
}

// legalTypesAll returns true for all Matrix types.
func legalTypesAll(a, b Matrix) bool {
	return true
}

// legalTypeSym returns whether a is a Symmetric.
func legalTypeSym(a Matrix) bool {
	_, ok := a.(Symmetric)
	return ok
}

// legalTypeTri returns whether a is a Triangular.
func legalTypeTri(a Matrix) bool {
	_, ok := a.(Triangular)
	return ok
}

// legalTypeTriLower returns whether a is a Triangular with kind == Lower.
func legalTypeTriLower(a Matrix) bool {
	t, ok := a.(Triangular)
	if !ok {
		return false
	}
	_, kind := t.Triangle()
	return kind == Lower
}

// legalTypeTriUpper returns whether a is a Triangular with kind == Upper.
func legalTypeTriUpper(a Matrix) bool {
	t, ok := a.(Triangular)
	if !ok {
		return false
	}
	_, kind := t.Triangle()
	return kind == Upper
}

// legalTypesSym returns whether both input arguments are Symmetric.
func legalTypesSym(a, b Matrix) bool {
	if _, ok := a.(Symmetric); !ok {
		return false
	}
	if _, ok := b.(Symmetric); !ok {
		return false
	}
	return true
}

// legalTypeVector returns whether v is a Vector.
func legalTypeVector(v Matrix) bool {
	_, ok := v.(Vector)
	return ok
}

// legalTypeVec returns whether v is a *VecDense.
func legalTypeVecDense(v Matrix) bool {
	_, ok := v.(*VecDense)
	return ok
}

// legalTypesVectorVector returns whether both inputs are Vector
func legalTypesVectorVector(a, b Matrix) bool {
	if _, ok := a.(Vector); !ok {
		return false
	}
	if _, ok := b.(Vector); !ok {
		return false
	}
	return true
}

// legalTypesVecDenseVecDense returns whether both inputs are *VecDense.
func legalTypesVecDenseVecDense(a, b Matrix) bool {
	if _, ok := a.(*VecDense); !ok {
		return false
	}
	if _, ok := b.(*VecDense); !ok {
		return false
	}
	return true
}

// legalTypesMatrixVector returns whether the first input is an arbitrary Matrix
// and the second input is a Vector.
func legalTypesMatrixVector(a, b Matrix) bool {
	_, ok := b.(Vector)
	return ok
}

// legalTypesMatrixVecDense returns whether the first input is an arbitrary Matrix
// and the second input is a *VecDense.
func legalTypesMatrixVecDense(a, b Matrix) bool {
	_, ok := b.(*VecDense)
	return ok
}

// legalDims returns whether {m,n} is a valid dimension of the given matrix type.
func legalDims(a Matrix, m, n int) bool {
	switch t := a.(type) {
	default:
		panic("legal dims type not coded")
	case Untransposer:
		return legalDims(t.Untranspose(), n, m)
	case *Dense, *basicMatrix, *BandDense, *basicBanded:
		if m < 0 || n < 0 {
			return false
		}
		return true
	case *SymDense, *TriDense, *basicSymmetric, *basicTriangular,
		*SymBandDense, *basicSymBanded, *TriBandDense, *basicTriBanded,
		*basicDiagonal, *DiagDense:
		if m < 0 || n < 0 || m != n {
			return false
		}
		return true
	case *VecDense, *basicVector:
		if m < 0 || n < 0 {
			return false
		}
		return n == 1
	}
}

// returnAs returns the matrix a with the type of t. Used for making a concrete
// type and changing to the basic form.
func returnAs(a, t Matrix) Matrix {
	switch mat := a.(type) {
	default:
		panic("unknown type for a")
	case *Dense:
		switch t.(type) {
		default:
			panic("bad type")
		case *Dense:
			return mat
		case *basicMatrix:
			return asBasicMatrix(mat)
		}
	case *SymDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *SymDense:
			return mat
		case *basicSymmetric:
			return asBasicSymmetric(mat)
		}
	case *TriDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *TriDense:
			return mat
		case *basicTriangular:
			return asBasicTriangular(mat)
		}
	case *BandDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *BandDense:
			return mat
		case *basicBanded:
			return asBasicBanded(mat)
		}
	case *SymBandDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *SymBandDense:
			return mat
		case *basicSymBanded:
			return asBasicSymBanded(mat)
		}
	case *TriBandDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *TriBandDense:
			return mat
		case *basicTriBanded:
			return asBasicTriBanded(mat)
		}
	case *DiagDense:
		switch t.(type) {
		default:
			panic("bad type")
		case *DiagDense:
			return mat
		case *basicDiagonal:
			return asBasicDiagonal(mat)
		}
	}
}

// retranspose returns the matrix m inside an Untransposer of the type
// of a.
func retranspose(a, m Matrix) Matrix {
	switch a.(type) {
	case TransposeTriBand:
		return TransposeTriBand{m.(TriBanded)}
	case TransposeBand:
		return TransposeBand{m.(Banded)}
	case TransposeTri:
		return TransposeTri{m.(Triangular)}
	case Transpose:
		return Transpose{m}
	case Untransposer:
		panic("unknown transposer type")
	default:
		panic("a is not an untransposer")
	}
}

// makeRandOf returns a new randomly filled m×n matrix of the underlying matrix type.
func makeRandOf(a Matrix, m, n int) Matrix {
	var rMatrix Matrix
	switch t := a.(type) {
	default:
		panic("unknown type for make rand of")
	case Untransposer:
		rMatrix = retranspose(a, makeRandOf(t.Untranspose(), n, m))
	case *Dense, *basicMatrix:
		var mat = &Dense{}
		if m != 0 && n != 0 {
			mat = NewDense(m, n, nil)
		}
		for i := 0; i < m; i++ {
			for j := 0; j < n; j++ {
				mat.Set(i, j, rand.NormFloat64())
			}
		}
		rMatrix = returnAs(mat, t)
	case *VecDense:
		if m == 0 && n == 0 {
			return &VecDense{}
		}
		if n != 1 {
			panic(fmt.Sprintf("bad vector size: m = %v, n = %v", m, n))
		}
		length := m
		inc := 1
		if t.mat.Inc != 0 {
			inc = t.mat.Inc
		}
		mat := &VecDense{
			mat: blas64.Vector{
				N:    length,
				Inc:  inc,
				Data: make([]float64, inc*(length-1)+1),
			},
		}
		for i := 0; i < length; i++ {
			mat.SetVec(i, rand.NormFloat64())
		}
		return mat
	case *basicVector:
		if m == 0 && n == 0 {
			return &basicVector{}
		}
		if n != 1 {
			panic(fmt.Sprintf("bad vector size: m = %v, n = %v", m, n))
		}
		mat := &basicVector{
			m: make([]float64, m),
		}
		for i := 0; i < m; i++ {
			mat.m[i] = rand.NormFloat64()
		}
		return mat
	case *SymDense, *basicSymmetric:
		if m != n {
			panic("bad size")
		}
		mat := &SymDense{}
		if n != 0 {
			mat = NewSymDense(n, nil)
		}
		for i := 0; i < m; i++ {
			for j := i; j < n; j++ {
				mat.SetSym(i, j, rand.NormFloat64())
			}
		}
		rMatrix = returnAs(mat, t)
	case *TriDense, *basicTriangular:
		if m != n {
			panic("bad size")
		}

		// This is necessary because we are making
		// a triangle from the zero value, which
		// always returns upper as true.
		var triKind TriKind
		switch t := t.(type) {
		case *TriDense:
			triKind = t.triKind()
		case *basicTriangular:
			triKind = (*TriDense)(t).triKind()
		}

		if n == 0 {
			uplo := blas.Upper
			if triKind == Lower {
				uplo = blas.Lower
			}
			return returnAs(&TriDense{mat: blas64.Triangular{Uplo: uplo}}, t)
		}

		mat := NewTriDense(n, triKind, nil)
		if triKind == Upper {
			for i := 0; i < m; i++ {
				for j := i; j < n; j++ {
					mat.SetTri(i, j, rand.NormFloat64())
				}
			}
		} else {
			for i := 0; i < m; i++ {
				for j := 0; j <= i; j++ {
					mat.SetTri(i, j, rand.NormFloat64())
				}
			}
		}
		rMatrix = returnAs(mat, t)
	case *BandDense, *basicBanded:
		var kl, ku int
		switch t := t.(type) {
		case *BandDense:
			kl = t.mat.KL
			ku = t.mat.KU
		case *basicBanded:
			ku = (*BandDense)(t).mat.KU
			kl = (*BandDense)(t).mat.KL
		}
		ku = min(ku, n-1)
		kl = min(kl, m-1)
		data := make([]float64, min(m, n+kl)*(kl+ku+1))
		for i := range data {
			data[i] = rand.NormFloat64()
		}
		mat := NewBandDense(m, n, kl, ku, data)
		rMatrix = returnAs(mat, t)
	case *SymBandDense, *basicSymBanded:
		if m != n {
			panic("bad size")
		}
		var k int
		switch t := t.(type) {
		case *SymBandDense:
			k = t.mat.K
		case *basicSymBanded:
			k = (*SymBandDense)(t).mat.K
		}
		k = min(k, m-1) // Special case for small sizes.
		data := make([]float64, m*(k+1))
		for i := range data {
			data[i] = rand.NormFloat64()
		}
		mat := NewSymBandDense(n, k, data)
		rMatrix = returnAs(mat, t)
	case *TriBandDense, *basicTriBanded:
		if m != n {
			panic("bad size")
		}
		var k int
		var triKind TriKind
		switch t := t.(type) {
		case *TriBandDense:
			k = t.mat.K
			triKind = t.triKind()
		case *basicTriBanded:
			k = (*TriBandDense)(t).mat.K
			triKind = (*TriBandDense)(t).triKind()
		}
		k = min(k, m-1) // Special case for small sizes.
		data := make([]float64, m*(k+1))
		for i := range data {
			data[i] = rand.NormFloat64()
		}
		mat := NewTriBandDense(n, k, triKind, data)
		rMatrix = returnAs(mat, t)
	case *DiagDense, *basicDiagonal:
		if m != n {
			panic("bad size")
		}
		var inc int
		switch t := t.(type) {
		case *DiagDense:
			inc = t.mat.Inc
		case *basicDiagonal:
			inc = (*DiagDense)(t).mat.Inc
		}
		if inc == 0 {
			inc = 1
		}
		mat := &DiagDense{
			mat: blas64.Vector{
				N:    n,
				Inc:  inc,
				Data: make([]float64, inc*(n-1)+1),
			},
		}
		for i := 0; i < n; i++ {
			mat.SetDiag(i, rand.Float64())
		}
		rMatrix = returnAs(mat, t)
	}
	if mr, mc := rMatrix.Dims(); mr != m || mc != n {
		panic(fmt.Sprintf("makeRandOf for %T returns wrong size: %d×%d != %d×%d", a, m, n, mr, mc))
	}
	return rMatrix
}

// makeCopyOf returns a copy of the matrix.
func makeCopyOf(a Matrix) Matrix {
	switch t := a.(type) {
	default:
		panic("unknown type in makeCopyOf")
	case Untransposer:
		return retranspose(a, makeCopyOf(t.Untranspose()))
	case *Dense, *basicMatrix:
		var m Dense
		m.Clone(a)
		return returnAs(&m, t)
	case *SymDense, *basicSymmetric:
		n := t.(Symmetric).Symmetric()
		m := NewSymDense(n, nil)
		m.CopySym(t.(Symmetric))
		return returnAs(m, t)
	case *TriDense, *basicTriangular:
		n, upper := t.(Triangular).Triangle()
		m := NewTriDense(n, upper, nil)
		if upper {
			for i := 0; i < n; i++ {
				for j := i; j < n; j++ {
					m.SetTri(i, j, t.At(i, j))
				}
			}
		} else {
			for i := 0; i < n; i++ {
				for j := 0; j <= i; j++ {
					m.SetTri(i, j, t.At(i, j))
				}
			}
		}
		return returnAs(m, t)
	case *BandDense, *basicBanded:
		var band *BandDense
		switch s := t.(type) {
		case *BandDense:
			band = s
		case *basicBanded:
			band = (*BandDense)(s)
		}
		m := &BandDense{
			mat: blas64.Band{
				Rows:   band.mat.Rows,
				Cols:   band.mat.Cols,
				KL:     band.mat.KL,
				KU:     band.mat.KU,
				Data:   make([]float64, len(band.mat.Data)),
				Stride: band.mat.Stride,
			},
		}
		copy(m.mat.Data, band.mat.Data)
		return returnAs(m, t)
	case *SymBandDense, *basicSymBanded:
		var sym *SymBandDense
		switch s := t.(type) {
		case *SymBandDense:
			sym = s
		case *basicSymBanded:
			sym = (*SymBandDense)(s)
		}
		m := &SymBandDense{
			mat: blas64.SymmetricBand{
				Uplo:   blas.Upper,
				N:      sym.mat.N,
				K:      sym.mat.K,
				Data:   make([]float64, len(sym.mat.Data)),
				Stride: sym.mat.Stride,
			},
		}
		copy(m.mat.Data, sym.mat.Data)
		return returnAs(m, t)
	case *TriBandDense, *basicTriBanded:
		var tri *TriBandDense
		switch s := t.(type) {
		case *TriBandDense:
			tri = s
		case *basicTriBanded:
			tri = (*TriBandDense)(s)
		}
		m := &TriBandDense{
			mat: blas64.TriangularBand{
				Uplo:   tri.mat.Uplo,
				Diag:   tri.mat.Diag,
				N:      tri.mat.N,
				K:      tri.mat.K,
				Data:   make([]float64, len(tri.mat.Data)),
				Stride: tri.mat.Stride,
			},
		}
		copy(m.mat.Data, tri.mat.Data)
		return returnAs(m, t)
	case *VecDense:
		m := &VecDense{
			mat: blas64.Vector{
				N:    t.mat.N,
				Inc:  t.mat.Inc,
				Data: make([]float64, t.mat.Inc*(t.mat.N-1)+1),
			},
		}
		copy(m.mat.Data, t.mat.Data)
		return m
	case *basicVector:
		m := &basicVector{
			m: make([]float64, t.Len()),
		}
		copy(m.m, t.m)
		return m
	case *DiagDense, *basicDiagonal:
		var diag *DiagDense
		switch s := t.(type) {
		case *DiagDense:
			diag = s
		case *basicDiagonal:
			diag = (*DiagDense)(s)
		}
		d := &DiagDense{
			mat: blas64.Vector{N: diag.mat.N, Inc: diag.mat.Inc, Data: make([]float64, len(diag.mat.Data))},
		}
		copy(d.mat.Data, diag.mat.Data)
		return returnAs(d, t)
	}
}

// sameType returns true if a and b have the same underlying type.
func sameType(a, b Matrix) bool {
	return reflect.ValueOf(a).Type() == reflect.ValueOf(b).Type()
}

// maybeSame returns true if the two matrices could be represented by the same
// pointer.
func maybeSame(receiver, a Matrix) bool {
	rr, rc := receiver.Dims()
	u, trans := a.(Untransposer)
	if trans {
		a = u.Untranspose()
	}
	if !sameType(receiver, a) {
		return false
	}
	ar, ac := a.Dims()
	if rr != ar || rc != ac {
		return false
	}
	if _, ok := a.(Triangular); ok {
		// They are both triangular types. The TriType needs to match
		_, aKind := a.(Triangular).Triangle()
		_, rKind := receiver.(Triangular).Triangle()
		if aKind != rKind {
			return false
		}
	}
	return true
}

// equalApprox returns whether the elements of a and b are the same to within
// the tolerance. If ignoreNaN is true the test is relaxed such that NaN == NaN.
func equalApprox(a, b Matrix, tol float64, ignoreNaN bool) bool {
	ar, ac := a.Dims()
	br, bc := b.Dims()
	if ar != br {
		return false
	}
	if ac != bc {
		return false
	}
	for i := 0; i < ar; i++ {
		for j := 0; j < ac; j++ {
			if !floats.EqualWithinAbsOrRel(a.At(i, j), b.At(i, j), tol, tol) {
				if ignoreNaN && math.IsNaN(a.At(i, j)) && math.IsNaN(b.At(i, j)) {
					continue
				}
				return false
			}
		}
	}
	return true
}

// equal returns true if the matrices have equal entries.
func equal(a, b Matrix) bool {
	ar, ac := a.Dims()
	br, bc := b.Dims()
	if ar != br {
		return false
	}
	if ac != bc {
		return false
	}
	for i := 0; i < ar; i++ {
		for j := 0; j < ac; j++ {
			if a.At(i, j) != b.At(i, j) {
				return false
			}
		}
	}
	return true
}

// isDiagonal returns whether a is a diagonal matrix.
func isDiagonal(a Matrix) bool {
	r, c := a.Dims()
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			if a.At(i, j) != 0 && i != j {
				return false
			}
		}
	}
	return true
}

// equalDiagonal returns whether a and b are equal on the diagonal.
func equalDiagonal(a, b Matrix) bool {
	ar, ac := a.Dims()
	br, bc := a.Dims()
	if min(ar, ac) != min(br, bc) {
		return false
	}
	for i := 0; i < min(ar, ac); i++ {
		if a.At(i, i) != b.At(i, i) {
			return false
		}
	}
	return true
}

// underlyingData extracts the underlying data of the matrix a.
func underlyingData(a Matrix) []float64 {
	switch t := a.(type) {
	default:
		panic("matrix type not implemented for extracting underlying data")
	case Untransposer:
		return underlyingData(t.Untranspose())
	case *Dense:
		return t.mat.Data
	case *SymDense:
		return t.mat.Data
	case *TriDense:
		return t.mat.Data
	case *VecDense:
		return t.mat.Data
	}
}

// testMatrices is a list of matrix types to test.
// This test relies on the fact that the implementations of Triangle do not
// corrupt the value of Uplo when they are zero-valued. This test will fail
// if that changes (and some mechanism will need to be used to force the
// correct TriKind to be read).
var testMatrices = []Matrix{
	&Dense{},
	&basicMatrix{},
	Transpose{&Dense{}},

	&VecDense{mat: blas64.Vector{Inc: 1}},
	&VecDense{mat: blas64.Vector{Inc: 10}},
	&basicVector{},
	Transpose{&VecDense{mat: blas64.Vector{Inc: 1}}},
	Transpose{&VecDense{mat: blas64.Vector{Inc: 10}}},
	Transpose{&basicVector{}},

	&BandDense{mat: blas64.Band{KL: 2, KU: 1}},
	&BandDense{mat: blas64.Band{KL: 1, KU: 2}},
	Transpose{&BandDense{mat: blas64.Band{KL: 2, KU: 1}}},
	Transpose{&BandDense{mat: blas64.Band{KL: 1, KU: 2}}},
	TransposeBand{&BandDense{mat: blas64.Band{KL: 2, KU: 1}}},
	TransposeBand{&BandDense{mat: blas64.Band{KL: 1, KU: 2}}},

	&SymDense{},
	&basicSymmetric{},
	Transpose{&basicSymmetric{}},

	&TriDense{mat: blas64.Triangular{Uplo: blas.Upper}},
	&TriDense{mat: blas64.Triangular{Uplo: blas.Lower}},
	&basicTriangular{mat: blas64.Triangular{Uplo: blas.Upper}},
	&basicTriangular{mat: blas64.Triangular{Uplo: blas.Lower}},
	Transpose{&TriDense{mat: blas64.Triangular{Uplo: blas.Upper}}},
	Transpose{&TriDense{mat: blas64.Triangular{Uplo: blas.Lower}}},
	TransposeTri{&TriDense{mat: blas64.Triangular{Uplo: blas.Upper}}},
	TransposeTri{&TriDense{mat: blas64.Triangular{Uplo: blas.Lower}}},
	Transpose{&basicTriangular{mat: blas64.Triangular{Uplo: blas.Upper}}},
	Transpose{&basicTriangular{mat: blas64.Triangular{Uplo: blas.Lower}}},
	TransposeTri{&basicTriangular{mat: blas64.Triangular{Uplo: blas.Upper}}},
	TransposeTri{&basicTriangular{mat: blas64.Triangular{Uplo: blas.Lower}}},

	&SymBandDense{},
	&basicSymBanded{},
	Transpose{&basicSymBanded{}},

	&SymBandDense{mat: blas64.SymmetricBand{K: 2}},
	&basicSymBanded{mat: blas64.SymmetricBand{K: 2}},
	Transpose{&basicSymBanded{mat: blas64.SymmetricBand{K: 2}}},
	TransposeBand{&basicSymBanded{mat: blas64.SymmetricBand{K: 2}}},

	&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}},
	&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}},
	&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}},
	&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}},
	Transpose{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	Transpose{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	Transpose{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	Transpose{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeTri{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeTri{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeTri{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeTri{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeBand{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeBand{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeBand{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeBand{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeTriBand{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeTriBand{&TriBandDense{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},
	TransposeTriBand{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Upper}}},
	TransposeTriBand{&basicTriBanded{mat: blas64.TriangularBand{K: 2, Uplo: blas.Lower}}},

	&DiagDense{},
	&DiagDense{mat: blas64.Vector{Inc: 10}},
	Transpose{&DiagDense{}},
	Transpose{&DiagDense{mat: blas64.Vector{Inc: 10}}},
	TransposeTri{&DiagDense{}},
	TransposeTri{&DiagDense{mat: blas64.Vector{Inc: 10}}},
	TransposeBand{&DiagDense{}},
	TransposeBand{&DiagDense{mat: blas64.Vector{Inc: 10}}},
	TransposeTriBand{&DiagDense{}},
	TransposeTriBand{&DiagDense{mat: blas64.Vector{Inc: 10}}},
	&basicDiagonal{},
	Transpose{&basicDiagonal{}},
	TransposeTri{&basicDiagonal{}},
	TransposeBand{&basicDiagonal{}},
	TransposeTriBand{&basicDiagonal{}},
}

var sizes = []struct {
	ar, ac int
}{
	{1, 1},
	{1, 3},
	{3, 1},

	{6, 6},
	{6, 11},
	{11, 6},
}

func testOneInputFunc(t *testing.T,
	// name is the name of the function being tested.
	name string,

	// f is the function being tested.
	f func(a Matrix) interface{},

	// denseComparison performs the same operation, but using Dense matrices for
	// comparison.
	denseComparison func(a *Dense) interface{},

	// sameAnswer compares the result from two different evaluations of the function
	// and returns true if they are the same. The specific function being tested
	// determines the definition of "same". It may mean identical or it may mean
	// approximately equal.
	sameAnswer func(a, b interface{}) bool,

	// legalType returns true if the type of the input is a legal type for the
	// input of the function.
	legalType func(a Matrix) bool,

	// legalSize returns true if the size is valid for the function.
	legalSize func(r, c int) bool,
) {
	for _, aMat := range testMatrices {
		for _, test := range sizes {
			// Skip the test if the argument would not be assignable to the
			// method's corresponding input parameter or it is not possible
			// to construct an argument of the requested size.
			if !legalType(aMat) {
				continue
			}
			if !legalDims(aMat, test.ar, test.ac) {
				continue
			}
			a := makeRandOf(aMat, test.ar, test.ac)

			// Compute the true answer if the sizes are legal.
			dimsOK := legalSize(test.ar, test.ac)
			var want interface{}
			if dimsOK {
				var aDense Dense
				aDense.Clone(a)
				want = denseComparison(&aDense)
			}
			aCopy := makeCopyOf(a)
			// Test the method for a zero-value of the receiver.
			aType, aTrans := untranspose(a)
			errStr := fmt.Sprintf("%v(%T), size: %#v, atrans %t", name, aType, test, aTrans)
			var got interface{}
			panicked, err := panics(func() { got = f(a) })
			if !dimsOK && !panicked {
				t.Errorf("Did not panic with illegal size: %s", errStr)
				continue
			}
			if dimsOK && panicked {
				t.Errorf("Panicked with legal size: %s: %v", errStr, err)
				continue
			}
			if !equal(a, aCopy) {
				t.Errorf("First input argument changed in call: %s", errStr)
			}
			if !dimsOK {
				continue
			}
			if !sameAnswer(want, got) {
				t.Errorf("Answer mismatch: %s", errStr)
			}
		}
	}
}

var sizePairs = []struct {
	ar, ac, br, bc int
}{
	{1, 1, 1, 1},
	{6, 6, 6, 6},
	{7, 7, 7, 7},

	{1, 1, 1, 5},
	{1, 1, 5, 1},
	{1, 5, 1, 1},
	{5, 1, 1, 1},

	{5, 5, 5, 1},
	{5, 5, 1, 5},
	{5, 1, 5, 5},
	{1, 5, 5, 5},

	{6, 6, 6, 11},
	{6, 6, 11, 6},
	{6, 11, 6, 6},
	{11, 6, 6, 6},
	{11, 11, 11, 6},
	{11, 11, 6, 11},
	{11, 6, 11, 11},
	{6, 11, 11, 11},

	{1, 1, 5, 5},
	{1, 5, 1, 5},
	{1, 5, 5, 1},
	{5, 1, 1, 5},
	{5, 1, 5, 1},
	{5, 5, 1, 1},
	{6, 6, 11, 11},
	{6, 11, 6, 11},
	{6, 11, 11, 6},
	{11, 6, 6, 11},
	{11, 6, 11, 6},
	{11, 11, 6, 6},

	{1, 1, 17, 11},
	{1, 1, 11, 17},
	{1, 11, 1, 17},
	{1, 17, 1, 11},
	{1, 11, 17, 1},
	{1, 17, 11, 1},
	{11, 1, 1, 17},
	{17, 1, 1, 11},
	{11, 1, 17, 1},
	{17, 1, 11, 1},
	{11, 17, 1, 1},
	{17, 11, 1, 1},

	{6, 6, 1, 11},
	{6, 6, 11, 1},
	{6, 11, 6, 1},
	{6, 1, 6, 11},
	{6, 11, 1, 6},
	{6, 1, 11, 6},
	{11, 6, 6, 1},
	{1, 6, 6, 11},
	{11, 6, 1, 6},
	{1, 6, 11, 6},
	{11, 1, 6, 6},
	{1, 11, 6, 6},

	{6, 6, 17, 1},
	{6, 6, 1, 17},
	{6, 1, 6, 17},
	{6, 17, 6, 1},
	{6, 1, 17, 6},
	{6, 17, 1, 6},
	{1, 6, 6, 17},
	{17, 6, 6, 1},
	{1, 6, 17, 6},
	{17, 6, 1, 6},
	{1, 17, 6, 6},
	{17, 1, 6, 6},

	{6, 6, 17, 11},
	{6, 6, 11, 17},
	{6, 11, 6, 17},
	{6, 17, 6, 11},
	{6, 11, 17, 6},
	{6, 17, 11, 6},
	{11, 6, 6, 17},
	{17, 6, 6, 11},
	{11, 6, 17, 6},
	{17, 6, 11, 6},
	{11, 17, 6, 6},
	{17, 11, 6, 6},
}

func testTwoInputFunc(t *testing.T,
	// name is the name of the function being tested.
	name string,

	// f is the function being tested.
	f func(a, b Matrix) interface{},

	// denseComparison performs the same operation, but using Dense matrices for
	// comparison.
	denseComparison func(a, b *Dense) interface{},

	// sameAnswer compares the result from two different evaluations of the function
	// and returns true if they are the same. The specific function being tested
	// determines the definition of "same". It may mean identical or it may mean
	// approximately equal.
	sameAnswer func(a, b interface{}) bool,

	// legalType returns true if the types of the inputs are legal for the
	// input of the function.
	legalType func(a, b Matrix) bool,

	// legalSize returns true if the sizes are valid for the function.
	legalSize func(ar, ac, br, bc int) bool,
) {
	for _, aMat := range testMatrices {
		for _, bMat := range testMatrices {
			// Loop over all of the size combinations (bigger, smaller, etc.).
			for _, test := range sizePairs {
				// Skip the test if the argument would not be assignable to the
				// method's corresponding input parameter or it is not possible
				// to construct an argument of the requested size.
				if !legalType(aMat, bMat) {
					continue
				}
				if !legalDims(aMat, test.ar, test.ac) {
					continue
				}
				if !legalDims(bMat, test.br, test.bc) {
					continue
				}
				a := makeRandOf(aMat, test.ar, test.ac)
				b := makeRandOf(bMat, test.br, test.bc)

				// Compute the true answer if the sizes are legal.
				dimsOK := legalSize(test.ar, test.ac, test.br, test.bc)
				var want interface{}
				if dimsOK {
					var aDense, bDense Dense
					aDense.Clone(a)
					bDense.Clone(b)
					want = denseComparison(&aDense, &bDense)
				}
				aCopy := makeCopyOf(a)
				bCopy := makeCopyOf(b)
				// Test the method for a zero-value of the receiver.
				aType, aTrans := untranspose(a)
				bType, bTrans := untranspose(b)
				errStr := fmt.Sprintf("%v(%T, %T), size: %#v, atrans %t, btrans %t", name, aType, bType, test, aTrans, bTrans)
				var got interface{}
				panicked, err := panics(func() { got = f(a, b) })
				if !dimsOK && !panicked {
					t.Errorf("Did not panic with illegal size: %s", errStr)
					continue
				}
				if dimsOK && panicked {
					t.Errorf("Panicked with legal size: %s: %v", errStr, err)
					continue
				}
				if !equal(a, aCopy) {
					t.Errorf("First input argument changed in call: %s", errStr)
				}
				if !equal(b, bCopy) {
					t.Errorf("First input argument changed in call: %s", errStr)
				}
				if !dimsOK {
					continue
				}
				if !sameAnswer(want, got) {
					t.Errorf("Answer mismatch: %s", errStr)
				}
			}
		}
	}
}

// testOneInput tests a method that has one matrix input argument
func testOneInput(t *testing.T,
	// name is the name of the method being tested.
	name string,

	// receiver is a value of the receiver type.
	receiver Matrix,

	// method is the generalized receiver.Method(a).
	method func(receiver, a Matrix),

	// denseComparison performs the same operation as method, but with dense
	// matrices for comparison with the result.
	denseComparison func(receiver, a *Dense),

	// legalTypes returns whether the concrete types in Matrix are valid for
	// the method.
	legalType func(a Matrix) bool,

	// legalSize returns whether the matrix sizes are valid for the method.
	legalSize func(ar, ac int) bool,

	// tol is the tolerance for equality when comparing method results.
	tol float64,
) {
	for _, aMat := range testMatrices {
		for _, test := range sizes {
			// Skip the test if the argument would not be assignable to the
			// method's corresponding input parameter or it is not possible
			// to construct an argument of the requested size.
			if !legalType(aMat) {
				continue
			}
			if !legalDims(aMat, test.ar, test.ac) {
				continue
			}
			a := makeRandOf(aMat, test.ar, test.ac)

			// Compute the true answer if the sizes are legal.
			dimsOK := legalSize(test.ar, test.ac)
			var want Dense
			if dimsOK {
				var aDense Dense
				aDense.Clone(a)
				denseComparison(&want, &aDense)
			}
			aCopy := makeCopyOf(a)

			// Test the method for a zero-value of the receiver.
			aType, aTrans := untranspose(a)
			errStr := fmt.Sprintf("%T.%s(%T), size: %#v, atrans %v", receiver, name, aType, test, aTrans)
			zero := makeRandOf(receiver, 0, 0)
			panicked, err := panics(func() { method(zero, a) })
			if !dimsOK && !panicked {
				t.Errorf("Did not panic with illegal size: %s", errStr)
				continue
			}
			if dimsOK && panicked {
				t.Errorf("Panicked with legal size: %s: %v", errStr, err)
				continue
			}
			if !equal(a, aCopy) {
				t.Errorf("First input argument changed in call: %s", errStr)
			}
			if !dimsOK {
				continue
			}
			if !equalApprox(zero, &want, tol, false) {
				t.Errorf("Answer mismatch with zero receiver: %s.\nGot:\n% v\nWant:\n% v\n", errStr, Formatted(zero), Formatted(&want))
				continue
			}

			// Test the method with a non-zero-value of the receiver.
			// The receiver has been overwritten in place so use its size
			// to construct a new random matrix.
			rr, rc := zero.Dims()
			neverZero := makeRandOf(receiver, rr, rc)
			panicked, _ = panics(func() { method(neverZero, a) })
			if panicked {
				t.Errorf("Panicked with non-zero receiver: %s", errStr)
			}
			if !equalApprox(neverZero, &want, tol, false) {
				t.Errorf("Answer mismatch non-zero receiver: %s", errStr)
			}

			// Test with an incorrectly sized matrix.
			switch receiver.(type) {
			default:
				panic("matrix type not coded for incorrect receiver size")
			case *Dense:
				wrongSize := makeRandOf(receiver, rr+1, rc)
				panicked, _ = panics(func() { method(wrongSize, a) })
				if !panicked {
					t.Errorf("Did not panic with wrong number of rows: %s", errStr)
				}
				wrongSize = makeRandOf(receiver, rr, rc+1)
				panicked, _ = panics(func() { method(wrongSize, a) })
				if !panicked {
					t.Errorf("Did not panic with wrong number of columns: %s", errStr)
				}
			case *TriDense, *SymDense:
				// Add to the square size.
				wrongSize := makeRandOf(receiver, rr+1, rc+1)
				panicked, _ = panics(func() { method(wrongSize, a) })
				if !panicked {
					t.Errorf("Did not panic with wrong size: %s", errStr)
				}
			case *VecDense:
				// Add to the column length.
				wrongSize := makeRandOf(receiver, rr+1, rc)
				panicked, _ = panics(func() { method(wrongSize, a) })
				if !panicked {
					t.Errorf("Did not panic with wrong number of rows: %s", errStr)
				}
			}

			// The receiver and the input may share a matrix pointer
			// if the type and size of the receiver and one of the
			// arguments match. Test the method works properly
			// when this is the case.
			aMaybeSame := maybeSame(neverZero, a)
			if aMaybeSame {
				aSame := makeCopyOf(a)
				receiver = aSame
				u, ok := aSame.(Untransposer)
				if ok {
					receiver = u.Untranspose()
				}
				preData := underlyingData(receiver)
				panicked, err = panics(func() { method(receiver, aSame) })
				if panicked {
					t.Errorf("Panics when a maybeSame: %s: %v", errStr, err)
				} else {
					if !equalApprox(receiver, &want, tol, false) {
						t.Errorf("Wrong answer when a maybeSame: %s", errStr)
					}
					postData := underlyingData(receiver)
					if !floats.Equal(preData, postData) {
						t.Errorf("Original data slice not modified when a maybeSame: %s", errStr)
					}
				}
			}
		}
	}
}

// testTwoInput tests a method that has two input arguments.
func testTwoInput(t *testing.T,
	// name is the name of the method being tested.
	name string,

	// receiver is a value of the receiver type.
	receiver Matrix,

	// method is the generalized receiver.Method(a, b).
	method func(receiver, a, b Matrix),

	// denseComparison performs the same operation as method, but with dense
	// matrices for comparison with the result.
	denseComparison func(receiver, a, b *Dense),

	// legalTypes returns whether the concrete types in Matrix are valid for
	// the method.
	legalTypes func(a, b Matrix) bool,

	// legalSize returns whether the matrix sizes are valid for the method.
	legalSize func(ar, ac, br, bc int) bool,

	// tol is the tolerance for equality when comparing method results.
	tol float64,
) {
	for _, aMat := range testMatrices {
		for _, bMat := range testMatrices {
			// Loop over all of the size combinations (bigger, smaller, etc.).
			for _, test := range sizePairs {
				// Skip the test if any argument would not be assignable to the
				// method's corresponding input parameter or it is not possible
				// to construct an argument of the requested size.
				if !legalTypes(aMat, bMat) {
					continue
				}
				if !legalDims(aMat, test.ar, test.ac) {
					continue
				}
				if !legalDims(bMat, test.br, test.bc) {
					continue
				}
				a := makeRandOf(aMat, test.ar, test.ac)
				b := makeRandOf(bMat, test.br, test.bc)

				// Compute the true answer if the sizes are legal.
				dimsOK := legalSize(test.ar, test.ac, test.br, test.bc)
				var want Dense
				if dimsOK {
					var aDense, bDense Dense
					aDense.Clone(a)
					bDense.Clone(b)
					denseComparison(&want, &aDense, &bDense)
				}
				aCopy := makeCopyOf(a)
				bCopy := makeCopyOf(b)

				// Test the method for a zero-value of the receiver.
				aType, aTrans := untranspose(a)
				bType, bTrans := untranspose(b)
				errStr := fmt.Sprintf("%T.%s(%T, %T), sizes: %#v, atrans %v, btrans %v", receiver, name, aType, bType, test, aTrans, bTrans)
				zero := makeRandOf(receiver, 0, 0)
				panicked, err := panics(func() { method(zero, a, b) })
				if !dimsOK && !panicked {
					t.Errorf("Did not panic with illegal size: %s", errStr)
					continue
				}
				if dimsOK && panicked {
					t.Errorf("Panicked with legal size: %s: %v", errStr, err)
					continue
				}
				if !equal(a, aCopy) {
					t.Errorf("First input argument changed in call: %s", errStr)
				}
				if !equal(b, bCopy) {
					t.Errorf("Second input argument changed in call: %s", errStr)
				}
				if !dimsOK {
					continue
				}
				wasZero, zero := zero, nil // Nil-out zero so we detect illegal use.
				// NaN equality is allowed because of 0/0 in DivElem test.
				if !equalApprox(wasZero, &want, tol, true) {
					t.Errorf("Answer mismatch with zero receiver: %s", errStr)
					continue
				}

				// Test the method with a non-zero-value of the receiver.
				// The receiver has been overwritten in place so use its size
				// to construct a new random matrix.
				rr, rc := wasZero.Dims()
				neverZero := makeRandOf(receiver, rr, rc)
				panicked, message := panics(func() { method(neverZero, a, b) })
				if panicked {
					t.Errorf("Panicked with non-zero receiver: %s: %s", errStr, message)
				}
				// NaN equality is allowed because of 0/0 in DivElem test.
				if !equalApprox(neverZero, &want, tol, true) {
					t.Errorf("Answer mismatch non-zero receiver: %s", errStr)
				}

				// Test with an incorrectly sized matrix.
				switch receiver.(type) {
				default:
					panic("matrix type not coded for incorrect receiver size")
				case *Dense:
					wrongSize := makeRandOf(receiver, rr+1, rc)
					panicked, _ = panics(func() { method(wrongSize, a, b) })
					if !panicked {
						t.Errorf("Did not panic with wrong number of rows: %s", errStr)
					}
					wrongSize = makeRandOf(receiver, rr, rc+1)
					panicked, _ = panics(func() { method(wrongSize, a, b) })
					if !panicked {
						t.Errorf("Did not panic with wrong number of columns: %s", errStr)
					}
				case *TriDense, *SymDense:
					// Add to the square size.
					wrongSize := makeRandOf(receiver, rr+1, rc+1)
					panicked, _ = panics(func() { method(wrongSize, a, b) })
					if !panicked {
						t.Errorf("Did not panic with wrong size: %s", errStr)
					}
				case *VecDense:
					// Add to the column length.
					wrongSize := makeRandOf(receiver, rr+1, rc)
					panicked, _ = panics(func() { method(wrongSize, a, b) })
					if !panicked {
						t.Errorf("Did not panic with wrong number of rows: %s", errStr)
					}
				}

				// The receiver and an input may share a matrix pointer
				// if the type and size of the receiver and one of the
				// arguments match. Test the method works properly
				// when this is the case.
				aMaybeSame := maybeSame(neverZero, a)
				bMaybeSame := maybeSame(neverZero, b)
				if aMaybeSame {
					aSame := makeCopyOf(a)
					receiver = aSame
					u, ok := aSame.(Untransposer)
					if ok {
						receiver = u.Untranspose()
					}
					preData := underlyingData(receiver)
					panicked, err = panics(func() { method(receiver, aSame, b) })
					if panicked {
						t.Errorf("Panics when a maybeSame: %s: %v", errStr, err)
					} else {
						if !equalApprox(receiver, &want, tol, false) {
							t.Errorf("Wrong answer when a maybeSame: %s", errStr)
						}
						postData := underlyingData(receiver)
						if !floats.Equal(preData, postData) {
							t.Errorf("Original data slice not modified when a maybeSame: %s", errStr)
						}
					}
				}
				if bMaybeSame {
					bSame := makeCopyOf(b)
					receiver = bSame
					u, ok := bSame.(Untransposer)
					if ok {
						receiver = u.Untranspose()
					}
					preData := underlyingData(receiver)
					panicked, err = panics(func() { method(receiver, a, bSame) })
					if panicked {
						t.Errorf("Panics when b maybeSame: %s: %v", errStr, err)
					} else {
						if !equalApprox(receiver, &want, tol, false) {
							t.Errorf("Wrong answer when b maybeSame: %s", errStr)
						}
						postData := underlyingData(receiver)
						if !floats.Equal(preData, postData) {
							t.Errorf("Original data slice not modified when b maybeSame: %s", errStr)
						}
					}
				}
				if aMaybeSame && bMaybeSame {
					aSame := makeCopyOf(a)
					receiver = aSame
					u, ok := aSame.(Untransposer)
					if ok {
						receiver = u.Untranspose()
					}
					// Ensure that b is the correct transpose type if applicable.
					// The receiver is always a concrete type so use it.
					bSame := receiver
					u, ok = b.(Untransposer)
					if ok {
						bSame = retranspose(b, receiver)
					}
					// Compute the real answer for this case. It is different
					// from the initial answer since now a and b have the
					// same data.
					zero = makeRandOf(wasZero, 0, 0)
					method(zero, aSame, bSame)
					wasZero, zero = zero, nil // Nil-out zero so we detect illegal use.
					preData := underlyingData(receiver)
					panicked, err = panics(func() { method(receiver, aSame, bSame) })
					if panicked {
						t.Errorf("Panics when both maybeSame: %s: %v", errStr, err)
					} else {
						if !equalApprox(receiver, wasZero, tol, false) {
							t.Errorf("Wrong answer when both maybeSame: %s", errStr)
						}
						postData := underlyingData(receiver)
						if !floats.Equal(preData, postData) {
							t.Errorf("Original data slice not modified when both maybeSame: %s", errStr)
						}
					}
				}
			}
		}
	}
}
