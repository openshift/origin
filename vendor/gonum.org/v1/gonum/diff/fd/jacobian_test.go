// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fd

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

func vecFunc13(y, x []float64) {
	y[0] = 5*x[0] + x[2]*math.Sin(x[1]) + 1
}
func vecFunc13Jac(jac *mat.Dense, x []float64) {
	jac.Set(0, 0, 5)
	jac.Set(0, 1, x[2]*math.Cos(x[1]))
	jac.Set(0, 2, math.Sin(x[1]))
}

func vecFunc22(y, x []float64) {
	y[0] = x[0]*x[0]*x[1] + 1
	y[1] = 5*x[0] + math.Sin(x[1]) + 1
}
func vecFunc22Jac(jac *mat.Dense, x []float64) {
	jac.Set(0, 0, 2*x[0]*x[1])
	jac.Set(0, 1, x[0]*x[0])
	jac.Set(1, 0, 5)
	jac.Set(1, 1, math.Cos(x[1]))
}

func vecFunc43(y, x []float64) {
	y[0] = x[0] + 1
	y[1] = 5*x[2] + 1
	y[2] = 4*x[1]*x[1] - 2*x[2] + 1
	y[3] = x[2]*math.Sin(x[0]) + 1
}
func vecFunc43Jac(jac *mat.Dense, x []float64) {
	jac.Set(0, 0, 1)
	jac.Set(0, 1, 0)
	jac.Set(0, 2, 0)
	jac.Set(1, 0, 0)
	jac.Set(1, 1, 0)
	jac.Set(1, 2, 5)
	jac.Set(2, 0, 0)
	jac.Set(2, 1, 8*x[1])
	jac.Set(2, 2, -2)
	jac.Set(3, 0, x[2]*math.Cos(x[0]))
	jac.Set(3, 1, 0)
	jac.Set(3, 2, math.Sin(x[0]))
}

func TestJacobian(t *testing.T) {
	rand.Seed(1)

	// Test with default settings.
	for tc, test := range []struct {
		m, n int
		f    func([]float64, []float64)
		jac  func(*mat.Dense, []float64)
	}{
		{
			m:   1,
			n:   3,
			f:   vecFunc13,
			jac: vecFunc13Jac,
		},
		{
			m:   2,
			n:   2,
			f:   vecFunc22,
			jac: vecFunc22Jac,
		},
		{
			m:   4,
			n:   3,
			f:   vecFunc43,
			jac: vecFunc43Jac,
		},
	} {
		const tol = 1e-6

		x := randomSlice(test.n, 10)
		xcopy := make([]float64, test.n)
		copy(xcopy, x)

		want := mat.NewDense(test.m, test.n, nil)
		test.jac(want, x)

		got := mat.NewDense(test.m, test.n, nil)
		fillNaNDense(got)
		Jacobian(got, test.f, x, nil)
		if !mat.EqualApprox(want, got, tol) {
			t.Errorf("Case %d (default settings): unexpected Jacobian.\nwant: %v\ngot:  %v",
				tc, mat.Formatted(want, mat.Prefix("      ")), mat.Formatted(got, mat.Prefix("      ")))
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %d (default settings): x modified", tc)
		}
	}

	// Test with non-default settings.
	for tc, test := range []struct {
		m, n    int
		f       func([]float64, []float64)
		jac     func(*mat.Dense, []float64)
		tol     float64
		formula Formula
	}{
		{
			m:       1,
			n:       3,
			f:       vecFunc13,
			jac:     vecFunc13Jac,
			tol:     1e-6,
			formula: Forward,
		},
		{
			m:       1,
			n:       3,
			f:       vecFunc13,
			jac:     vecFunc13Jac,
			tol:     1e-6,
			formula: Backward,
		},
		{
			m:       1,
			n:       3,
			f:       vecFunc13,
			jac:     vecFunc13Jac,
			tol:     1e-9,
			formula: Central,
		},
		{
			m:       2,
			n:       2,
			f:       vecFunc22,
			jac:     vecFunc22Jac,
			tol:     1e-6,
			formula: Forward,
		},
		{
			m:       2,
			n:       2,
			f:       vecFunc22,
			jac:     vecFunc22Jac,
			tol:     1e-6,
			formula: Backward,
		},
		{
			m:       2,
			n:       2,
			f:       vecFunc22,
			jac:     vecFunc22Jac,
			tol:     1e-9,
			formula: Central,
		},
		{
			m:       4,
			n:       3,
			f:       vecFunc43,
			jac:     vecFunc43Jac,
			tol:     1e-6,
			formula: Forward,
		},
		{
			m:       4,
			n:       3,
			f:       vecFunc43,
			jac:     vecFunc43Jac,
			tol:     1e-6,
			formula: Backward,
		},
		{
			m:       4,
			n:       3,
			f:       vecFunc43,
			jac:     vecFunc43Jac,
			tol:     1e-9,
			formula: Central,
		},
	} {
		x := randomSlice(test.n, 10)
		xcopy := make([]float64, test.n)
		copy(xcopy, x)

		want := mat.NewDense(test.m, test.n, nil)
		test.jac(want, x)

		got := mat.NewDense(test.m, test.n, nil)
		fillNaNDense(got)
		Jacobian(got, test.f, x, &JacobianSettings{
			Formula: test.formula,
		})
		if !mat.EqualApprox(want, got, test.tol) {
			t.Errorf("Case %d: unexpected Jacobian.\nwant: %v\ngot:  %v",
				tc, mat.Formatted(want, mat.Prefix("      ")), mat.Formatted(got, mat.Prefix("      ")))
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %d: x modified", tc)
		}

		fillNaNDense(got)
		Jacobian(got, test.f, x, &JacobianSettings{
			Formula:    test.formula,
			Concurrent: true,
		})
		if !mat.EqualApprox(want, got, test.tol) {
			t.Errorf("Case %d (concurrent): unexpected Jacobian.\nwant: %v\ngot:  %v",
				tc, mat.Formatted(want, mat.Prefix("      ")), mat.Formatted(got, mat.Prefix("      ")))
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %d (concurrent): x modified", tc)
		}

		fillNaNDense(got)
		origin := make([]float64, test.m)
		test.f(origin, x)
		Jacobian(got, test.f, x, &JacobianSettings{
			Formula:     test.formula,
			OriginValue: origin,
		})
		if !mat.EqualApprox(want, got, test.tol) {
			t.Errorf("Case %d (origin): unexpected Jacobian.\nwant: %v\ngot:  %v",
				tc, mat.Formatted(want, mat.Prefix("      ")), mat.Formatted(got, mat.Prefix("      ")))
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %d (origin): x modified", tc)
		}

		fillNaNDense(got)
		Jacobian(got, test.f, x, &JacobianSettings{
			Formula:     test.formula,
			OriginValue: origin,
			Concurrent:  true,
		})
		if !mat.EqualApprox(want, got, test.tol) {
			t.Errorf("Case %d (concurrent, origin): unexpected Jacobian.\nwant: %v\ngot:  %v",
				tc, mat.Formatted(want, mat.Prefix("      ")), mat.Formatted(got, mat.Prefix("      ")))
		}
		if !floats.Equal(x, xcopy) {
			t.Errorf("Case %d (concurrent, origin): x modified", tc)
		}
	}
}

// randomSlice returns a slice of n elements from the interval [-bound,bound).
func randomSlice(n int, bound float64) []float64 {
	x := make([]float64, n)
	for i := range x {
		x[i] = 2*bound*rand.Float64() - bound
	}
	return x
}

// fillNaNDense fills the matrix m with NaN values.
func fillNaNDense(m *mat.Dense) {
	r, c := m.Dims()
	for i := 0; i < r; i++ {
		for j := 0; j < c; j++ {
			m.Set(i, j, math.NaN())
		}
	}
}
