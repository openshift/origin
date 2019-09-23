// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

func TestEigen(t *testing.T) {
	for i, test := range []struct {
		a *Dense

		values []complex128
		left   *CDense
		right  *CDense
	}{
		{
			a: NewDense(3, 3, []float64{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			}),
			values: []complex128{1, 1, 1},
			left: NewCDense(3, 3, []complex128{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			}),
			right: NewCDense(3, 3, []complex128{
				1, 0, 0,
				0, 1, 0,
				0, 0, 1,
			}),
		},
		{
			// Values compared with numpy.
			a: NewDense(4, 4, []float64{
				0.9025, 0.025, 0.475, 0.0475,
				0.0475, 0.475, 0.475, 0.0025,
				0.0475, 0.025, 0.025, 0.9025,
				0.0025, 0.475, 0.025, 0.0475,
			}),
			values: []complex128{1, 0.7300317046114154, -0.1400158523057075 + 0.452854925738716i, -0.1400158523057075 - 0.452854925738716i},
			left: NewCDense(4, 4, []complex128{
				0.5, -0.3135167160788314, 0.0205812178013689 - 0.0045809393001271i, 0.0205812178013689 + 0.0045809393001271i,
				0.5, 0.7842199280224774, -0.3755102695419336 + 0.2924634904103882i, -0.3755102695419336 - 0.2924634904103882i,
				0.5, 0.3320220078078358, -0.1605261632278496 - 0.3881393645202528i, -0.1605261632278496 + 0.3881393645202528i,
				0.5, 0.4200806584012395, 0.7723935249234153, 0.7723935249234153,
			}),
			right: NewCDense(4, 4, []complex128{
				0.9476399565969628, -0.8637347682162745, -0.2688989440320280 - 0.1282234938321029i, -0.2688989440320280 + 0.1282234938321029i,
				0.2394935907064427, 0.3457075153704627, -0.3621360383713332 - 0.2583198964498771i, -0.3621360383713332 + 0.2583198964498771i,
				0.1692743801716332, 0.2706851011641580, 0.7426369401030960, 0.7426369401030960,
				0.1263626404003607, 0.2473421516816520, -0.1116019576997347 + 0.3865433902819795i, -0.1116019576997347 - 0.3865433902819795i,
			}),
		},
	} {
		var e1, e2, e3, e4 Eigen
		ok := e1.Factorize(test.a, EigenBoth)
		if !ok {
			panic("bad factorization")
		}
		e2.Factorize(test.a, EigenRight)
		e3.Factorize(test.a, EigenLeft)
		e4.Factorize(test.a, EigenNone)

		v1 := e1.Values(nil)
		if !cmplxEqualTol(v1, test.values, 1e-14) {
			t.Errorf("eigenvalue mismatch. Case %v", i)
		}
		if !CEqualApprox(e1.LeftVectorsTo(nil), test.left, 1e-14) {
			t.Errorf("left eigenvector mismatch. Case %v", i)
		}
		if !CEqualApprox(e1.VectorsTo(nil), test.right, 1e-14) {
			t.Errorf("right eigenvector mismatch. Case %v", i)
		}

		// Check that the eigenvectors and values are the same in all combinations.
		if !cmplxEqual(v1, e2.Values(nil)) {
			t.Errorf("eigenvector mismatch. Case %v", i)
		}
		if !cmplxEqual(v1, e3.Values(nil)) {
			t.Errorf("eigenvector mismatch. Case %v", i)
		}
		if !cmplxEqual(v1, e4.Values(nil)) {
			t.Errorf("eigenvector mismatch. Case %v", i)
		}
		if !CEqual(e1.VectorsTo(nil), e2.VectorsTo(nil)) {
			t.Errorf("right eigenvector mismatch. Case %v", i)
		}
		if !CEqual(e1.LeftVectorsTo(nil), e3.LeftVectorsTo(nil)) {
			t.Errorf("left eigenvector mismatch. Case %v", i)
		}

		// TODO(btracey): Also add in a test for correctness when #308 is
		// resolved and we have a CMat.Mul().
	}
}

func cmplxEqual(v1, v2 []complex128) bool {
	for i, v := range v1 {
		if v != v2[i] {
			return false
		}
	}
	return true
}

func cmplxEqualTol(v1, v2 []complex128, tol float64) bool {
	for i, v := range v1 {
		if !cEqualWithinAbsOrRel(v, v2[i], tol, tol) {
			return false
		}
	}
	return true
}

func TestSymEigen(t *testing.T) {
	// Hand coded tests with results from lapack.
	for _, test := range []struct {
		mat *SymDense

		values  []float64
		vectors *Dense
	}{
		{
			mat:    NewSymDense(3, []float64{8, 2, 4, 2, 6, 10, 4, 10, 5}),
			values: []float64{-4.707679201365891, 6.294580208480216, 17.413098992885672},
			vectors: NewDense(3, 3, []float64{
				-0.127343483135656, -0.902414161226903, -0.411621572466779,
				-0.664177720955769, 0.385801900032553, -0.640331827193739,
				0.736648893495999, 0.191847792659746, -0.648492738712395,
			}),
		},
	} {
		var es EigenSym
		ok := es.Factorize(test.mat, true)
		if !ok {
			t.Errorf("bad factorization")
		}
		if !floats.EqualApprox(test.values, es.values, 1e-14) {
			t.Errorf("Eigenvalue mismatch")
		}
		if !EqualApprox(test.vectors, es.vectors, 1e-14) {
			t.Errorf("Eigenvector mismatch")
		}

		var es2 EigenSym
		es2.Factorize(test.mat, false)
		if !floats.EqualApprox(es2.values, es.values, 1e-14) {
			t.Errorf("Eigenvalue mismatch when no vectors computed")
		}
	}

	// Randomized tests
	rnd := rand.New(rand.NewSource(1))
	for _, n := range []int{3, 5, 10, 70} {
		for cas := 0; cas < 10; cas++ {
			a := make([]float64, n*n)
			for i := range a {
				a[i] = rnd.NormFloat64()
			}
			s := NewSymDense(n, a)
			var es EigenSym
			ok := es.Factorize(s, true)
			if !ok {
				t.Errorf("Bad test")
			}

			// Check that the eigenvectors are orthonormal.
			if !isOrthonormal(es.vectors, 1e-8) {
				t.Errorf("Eigenvectors not orthonormal")
			}

			// Check that the eigenvalues are actually eigenvalues.
			for i := 0; i < n; i++ {
				v := NewVecDense(n, Col(nil, i, es.vectors))
				var m VecDense
				m.MulVec(s, v)

				var scal VecDense
				scal.ScaleVec(es.values[i], v)

				if !EqualApprox(&m, &scal, 1e-8) {
					t.Errorf("Eigenvalue does not match")
				}
			}

			// Check that the eigenvalues are in ascending order.
			if !sort.Float64sAreSorted(es.values) {
				t.Errorf("Eigenvalues not ascending")
			}
		}
	}
}
