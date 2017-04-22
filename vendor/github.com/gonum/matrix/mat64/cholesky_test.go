// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"math"
	"math/rand"
	"testing"

	"gopkg.in/check.v1"
)

func (s *S) TestCholesky(c *check.C) {
	for _, t := range []struct {
		a     *SymDense
		upper bool
		f     *TriDense

		want *TriDense
		pd   bool
	}{
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			upper: false,
			f:     &TriDense{},

			want: NewTriDense(3, false, []float64{
				2, 0, 0,
				0.5, 1.3228756555322954, 0,
				0.5, 2.0788046015507495, 1.195228609334394,
			}),
			pd: true,
		},
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			upper: true,
			f:     &TriDense{},

			want: NewTriDense(3, true, []float64{
				2, 0.5, 0.5,
				0, 1.3228756555322954, 2.0788046015507495,
				0, 0, 1.195228609334394,
			}),
			pd: true,
		},
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			upper: false,
			f:     NewTriDense(3, false, nil),

			want: NewTriDense(3, false, []float64{
				2, 0, 0,
				0.5, 1.3228756555322954, 0,
				0.5, 2.0788046015507495, 1.195228609334394,
			}),
			pd: true,
		},
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			upper: true,
			f:     NewTriDense(3, true, nil),

			want: NewTriDense(3, true, []float64{
				2, 0.5, 0.5,
				0, 1.3228756555322954, 2.0788046015507495,
				0, 0, 1.195228609334394,
			}),
			pd: true,
		},
		{
			// Test case for issue #119.
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			upper: false,
			f: NewTriDense(3, false, []float64{
				math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(),
			}),

			want: NewTriDense(3, false, []float64{
				2, 0, 0,
				0.5, 1.3228756555322954, 0,
				0.5, 2.0788046015507495, 1.195228609334394,
			}),
			pd: true,
		},
	} {
		ok := t.f.Cholesky(t.a, t.upper)
		c.Check(ok, check.Equals, t.pd)
		fc := DenseCopyOf(t.f)
		c.Check(fc.Equals(t.want), check.Equals, true)

		ft := &Dense{}
		ft.TCopy(t.f)

		if t.upper {
			fc.Mul(ft, fc)
		} else {
			fc.Mul(fc, ft)
		}
		c.Check(fc.EqualsApprox(t.a, 1e-12), check.Equals, true)

		var x Dense
		x.SolveCholesky(t.f, eye())

		var res Dense
		res.Mul(t.a, &x)
		c.Check(res.EqualsApprox(eye(), 1e-12), check.Equals, true)

		x = Dense{}
		x.SolveTri(t.f, t.upper, eye())
		x.SolveTri(t.f, !t.upper, &x)

		res.Mul(t.a, &x)
		c.Check(res.EqualsApprox(eye(), 1e-12), check.Equals, true)
	}
}

func (s *S) TestCholeskySolve(c *check.C) {
	for _, t := range []struct {
		a   *SymDense
		b   *Dense
		ans *Dense
	}{
		{
			a: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
			b:   NewDense(2, 1, []float64{5, 6}),
			ans: NewDense(2, 1, []float64{5, 6}),
		},
		{
			a: NewSymDense(3, []float64{
				53, 59, 37,
				0, 83, 71,
				37, 71, 101,
			}),
			b:   NewDense(3, 1, []float64{5, 6, 7}),
			ans: NewDense(3, 1, []float64{0.20745069393718094, -0.17421475529583694, 0.11577794010226464}),
		},
	} {
		var f TriDense
		ok := f.Cholesky(t.a, false)
		c.Assert(ok, check.Equals, true)

		var x Dense
		x.SolveCholesky(&f, t.b)
		c.Check(x.EqualsApprox(t.ans, 1e-12), check.Equals, true)

		x = Dense{}
		x.SolveTri(&f, false, t.b)
		x.SolveTri(&f, true, &x)
		c.Check(x.EqualsApprox(t.ans, 1e-12), check.Equals, true)
	}
}

func (s *S) TestCholeskySolveVec(c *check.C) {
	for _, t := range []struct {
		a   *SymDense
		b   *Vector
		ans *Vector
	}{
		{
			a: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
			b:   NewVector(2, []float64{5, 6}),
			ans: NewVector(2, []float64{5, 6}),
		},
		{
			a: NewSymDense(3, []float64{
				53, 59, 37,
				0, 83, 71,
				0, 0, 101,
			}),
			b:   NewVector(3, []float64{5, 6, 7}),
			ans: NewVector(3, []float64{0.20745069393718094, -0.17421475529583694, 0.11577794010226464}),
		},
	} {
		var f TriDense
		ok := f.Cholesky(t.a, false)
		c.Assert(ok, check.Equals, true)

		var x Vector
		x.SolveCholeskyVec(&f, t.b)
		c.Check(x.EqualsApproxVec(t.ans, 1e-12), check.Equals, true)

		var fl TriDense
		ok = fl.Cholesky(t.a, true)
		c.Assert(ok, check.Equals, true)

		var xl Vector
		xl.SolveCholeskyVec(&fl, t.b)
		c.Check(xl.EqualsApproxVec(t.ans, 1e-12), check.Equals, true)
	}
}

func BenchmarkCholeskySmall(b *testing.B) {
	benchmarkCholesky(b, 2)
}

func BenchmarkCholeskyMedium(b *testing.B) {
	benchmarkCholesky(b, Med)
}

func BenchmarkCholeskyLarge(b *testing.B) {
	benchmarkCholesky(b, Lg)
}

func benchmarkCholesky(b *testing.B, n int) {
	base := make([]float64, n*n)
	for i := range base {
		base[i] = rand.Float64()
	}
	bm := NewDense(n, n, base)
	bm.MulTrans(bm, true, bm, false)
	am := NewSymDense(n, bm.mat.Data)

	t := NewTriDense(n, true, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok := t.Cholesky(am, true)
		if !ok {
			panic("not pos def")
		}
	}
}
