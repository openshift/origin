// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package cgo provides an interface to bindings for a C LAPACK library.
package cgo

import (
	"github.com/gonum/blas"
	"github.com/gonum/lapack"
	"github.com/gonum/lapack/cgo/clapack"
)

// Copied from lapack/native. Keep in sync.
const (
	badDirect     = "lapack: bad direct"
	badLdA        = "lapack: index of a out of range"
	badSide       = "lapack: bad side"
	badStore      = "lapack: bad store"
	badTau        = "lapack: tau has insufficient length"
	badTrans      = "lapack: bad trans"
	badUplo       = "lapack: illegal triangle"
	badWork       = "lapack: insufficient working memory"
	badWorkStride = "lapack: insufficient working array stride"
	negDimension  = "lapack: negative matrix dimension"
	nLT0          = "lapack: n < 0"
	shortWork     = "lapack: working array shorter than declared"
)

// Implementation is the cgo-based C implementation of LAPACK routines.
type Implementation struct{}

var _ lapack.Float64 = Implementation{}

// Dpotrf computes the cholesky decomposition of the symmetric positive definite
// matrix a. If ul == blas.Upper, then a is stored as an upper-triangular matrix,
// and a = U U^T is stored in place into a. If ul == blas.Lower, then a = L L^T
// is computed and stored in-place into a. If a is not positive definite, false
// is returned. This is the blocked version of the algorithm.
func (impl Implementation) Dpotrf(ul blas.Uplo, n int, a []float64, lda int) (ok bool) {
	// ul is checked in clapack.Dpotrf.
	if n < 0 {
		panic(nLT0)
	}
	if lda < n {
		panic(badLdA)
	}
	if n == 0 {
		return true
	}
	return clapack.Dpotrf(ul, n, a, lda)
}
