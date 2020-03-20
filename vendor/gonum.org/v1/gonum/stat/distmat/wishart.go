// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package distmat

import (
	"math"
	"sync"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/mathext"
	"gonum.org/v1/gonum/stat/distuv"
)

// Wishart is a distribution over d×d positive symmetric definite matrices. It
// is parametrized by a scalar degrees of freedom parameter ν and a d×d positive
// definite matrix V.
//
// The Wishart PDF is given by
//  p(X) = [|X|^((ν-d-1)/2) * exp(-tr(V^-1 * X)/2)] / [2^(ν*d/2) * |V|^(ν/2) * Γ_d(ν/2)]
// where X is a d×d PSD matrix, ν > d-1, |·| denotes the determinant, tr is the
// trace and Γ_d is the multivariate gamma function.
//
// See https://en.wikipedia.org/wiki/Wishart_distribution for more information.
type Wishart struct {
	nu  float64
	src rand.Source

	dim     int
	cholv   mat.Cholesky
	logdetv float64
	upper   mat.TriDense

	once sync.Once
	v    *mat.SymDense // only stored if needed
}

// NewWishart returns a new Wishart distribution with the given shape matrix and
// degrees of freedom parameter. NewWishart returns whether the creation was
// successful.
//
// NewWishart panics if nu <= d - 1 where d is the order of v.
func NewWishart(v mat.Symmetric, nu float64, src rand.Source) (*Wishart, bool) {
	dim := v.Symmetric()
	if nu <= float64(dim-1) {
		panic("wishart: nu must be greater than dim-1")
	}
	var chol mat.Cholesky
	ok := chol.Factorize(v)
	if !ok {
		return nil, false
	}

	var u mat.TriDense
	chol.UTo(&u)

	w := &Wishart{
		nu:  nu,
		src: src,

		dim:     dim,
		cholv:   chol,
		logdetv: chol.LogDet(),
		upper:   u,
	}
	return w, true
}

// MeanSymTo calculates the mean matrix of the distribution in and stores it in dst.
// If dst is empty, it is resized to be an d×d symmetric matrix where d is the order
// of the receiver. When dst is non-empty, MeanSymTo panics if dst is not d×d.
func (w *Wishart) MeanSymTo(dst *mat.SymDense) {
	if dst.IsEmpty() {
		dst.ReuseAsSym(w.dim)
	} else if dst.Symmetric() != w.dim {
		panic(badDim)
	}
	w.setV()
	dst.CopySym(w.v)
	dst.ScaleSym(w.nu, dst)
}

// ProbSym returns the probability of the symmetric matrix x. If x is not positive
// definite (the Cholesky decomposition fails), it has 0 probability.
func (w *Wishart) ProbSym(x mat.Symmetric) float64 {
	return math.Exp(w.LogProbSym(x))
}

// LogProbSym returns the log of the probability of the input symmetric matrix.
//
// LogProbSym returns -∞ if the input matrix is not positive definite (the Cholesky
// decomposition fails).
func (w *Wishart) LogProbSym(x mat.Symmetric) float64 {
	dim := x.Symmetric()
	if dim != w.dim {
		panic(badDim)
	}
	var chol mat.Cholesky
	ok := chol.Factorize(x)
	if !ok {
		return math.Inf(-1)
	}
	return w.logProbSymChol(&chol)
}

// LogProbSymChol returns the log of the probability of the input symmetric matrix
// given its Cholesky decomposition.
func (w *Wishart) LogProbSymChol(cholX *mat.Cholesky) float64 {
	dim := cholX.Symmetric()
	if dim != w.dim {
		panic(badDim)
	}
	return w.logProbSymChol(cholX)
}

func (w *Wishart) logProbSymChol(cholX *mat.Cholesky) float64 {
	// The PDF is
	//  p(X) = [|X|^((ν-d-1)/2) * exp(-tr(V^-1 * X)/2)] / [2^(ν*d/2) * |V|^(ν/2) * Γ_d(ν/2)]
	// The LogPDF is thus
	//  (ν-d-1)/2 * log(|X|) - tr(V^-1 * X)/2  - (ν*d/2)*log(2) - ν/2 * log(|V|) - log(Γ_d(ν/2))
	logdetx := cholX.LogDet()

	// Compute tr(V^-1 * X), using the fact that X = Uᵀ * U.
	var u mat.TriDense
	cholX.UTo(&u)

	var vinvx mat.Dense
	err := w.cholv.SolveTo(&vinvx, u.T())
	if err != nil {
		return math.Inf(-1)
	}
	vinvx.Mul(&vinvx, &u)
	tr := mat.Trace(&vinvx)

	fnu := float64(w.nu)
	fdim := float64(w.dim)

	return 0.5*((fnu-fdim-1)*logdetx-tr-fnu*fdim*math.Ln2-fnu*w.logdetv) - mathext.MvLgamma(0.5*fnu, w.dim)
}

// RandSymTo generates a random symmetric matrix from the distribution.
// If dst is empty, it is resized to be an d×d symmetric matrix where d is the order
// of the receiver. When dst is non-empty, RandSymTo panics if dst is not d×d.
func (w *Wishart) RandSymTo(dst *mat.SymDense) {
	var c mat.Cholesky
	w.RandCholTo(&c)
	c.ToSym(dst)
}

// RandCholTo generates the Cholesky decomposition of a random matrix from the distribution.
// If dst is empty, it is resized to be an d×d symmetric matrix where d is the order
// of the receiver. When dst is non-empty, RandCholTo panics if dst is not d×d.
func (w *Wishart) RandCholTo(dst *mat.Cholesky) {
	// TODO(btracey): Modify the code if the underlying data from dst is exposed
	// to avoid the dim^2 allocation here.

	// Use the Bartlett Decomposition, which says that
	//  X ~ L A Aᵀ Lᵀ
	// Where A is a lower triangular matrix in which the diagonal of A is
	// generated from the square roots of χ^2 random variables, and the
	// off-diagonals are generated from standard normal variables.
	// The above gives the cholesky decomposition of X, where L_x = L A.
	//
	// mat works with the upper triagular decomposition, so we would like to do
	// the same. We can instead say that
	//  U_x = L_xᵀ = (L * A)ᵀ = Aᵀ * Lᵀ = Aᵀ * U
	// Instead, generate Aᵀ, by using the procedure above, except as an upper
	// triangular matrix.
	norm := distuv.Normal{
		Mu:    0,
		Sigma: 1,
		Src:   w.src,
	}

	t := mat.NewTriDense(w.dim, mat.Upper, nil)
	for i := 0; i < w.dim; i++ {
		v := distuv.ChiSquared{
			K:   w.nu - float64(i),
			Src: w.src,
		}.Rand()
		t.SetTri(i, i, math.Sqrt(v))
	}
	for i := 0; i < w.dim; i++ {
		for j := i + 1; j < w.dim; j++ {
			t.SetTri(i, j, norm.Rand())
		}
	}

	t.MulTri(t, &w.upper)
	dst.SetFromU(t)
}

// setV computes and stores the covariance matrix of the distribution.
func (w *Wishart) setV() {
	w.once.Do(func() {
		w.v = mat.NewSymDense(w.dim, nil)
		w.cholv.ToSym(w.v)
	})
}
