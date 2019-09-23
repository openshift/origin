// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas/testblas"
	"gonum.org/v1/gonum/floats"
)

func TestCholesky(t *testing.T) {
	for _, test := range []struct {
		a *SymDense

		cond   float64
		want   *TriDense
		posdef bool
	}{
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			cond: 37,
			want: NewTriDense(3, true, []float64{
				2, 0.5, 0.5,
				0, 1.3228756555322954, 2.0788046015507495,
				0, 0, 1.195228609334394,
			}),
			posdef: true,
		},
	} {
		_, n := test.a.Dims()
		for _, chol := range []*Cholesky{
			{},
			{chol: NewTriDense(n-1, true, nil)},
			{chol: NewTriDense(n, true, nil)},
			{chol: NewTriDense(n+1, true, nil)},
		} {
			ok := chol.Factorize(test.a)
			if ok != test.posdef {
				t.Errorf("unexpected return from Cholesky factorization: got: ok=%t want: ok=%t", ok, test.posdef)
			}
			fc := DenseCopyOf(chol.chol)
			if !Equal(fc, test.want) {
				t.Error("incorrect Cholesky factorization")
			}
			if math.Abs(test.cond-chol.cond) > 1e-13 {
				t.Errorf("Condition number mismatch: Want %v, got %v", test.cond, chol.cond)
			}
			U := chol.UTo(nil)
			aCopy := DenseCopyOf(test.a)
			var a Dense
			a.Mul(U.TTri(), U)
			if !EqualApprox(&a, aCopy, 1e-14) {
				t.Error("unexpected Cholesky factor product")
			}

			L := chol.LTo(nil)
			a.Mul(L, L.TTri())
			if !EqualApprox(&a, aCopy, 1e-14) {
				t.Error("unexpected Cholesky factor product")
			}
		}
	}
}

func TestCholeskySolve(t *testing.T) {
	for _, test := range []struct {
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
		var chol Cholesky
		ok := chol.Factorize(test.a)
		if !ok {
			t.Fatal("unexpected Cholesky factorization failure: not positive definite")
		}

		var x Dense
		chol.Solve(&x, test.b)
		if !EqualApprox(&x, test.ans, 1e-12) {
			t.Error("incorrect Cholesky solve solution")
		}

		var ans Dense
		ans.Mul(test.a, &x)
		if !EqualApprox(&ans, test.b, 1e-12) {
			t.Error("incorrect Cholesky solve solution product")
		}
	}
}

func TestCholeskySolveChol(t *testing.T) {
	for _, test := range []struct {
		a, b *SymDense
	}{
		{
			a: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
			b: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
		},
		{
			a: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
			b: NewSymDense(2, []float64{
				2, 0,
				0, 2,
			}),
		},
		{
			a: NewSymDense(3, []float64{
				53, 59, 37,
				59, 83, 71,
				37, 71, 101,
			}),
			b: NewSymDense(3, []float64{
				2, -1, 0,
				-1, 2, -1,
				0, -1, 2,
			}),
		},
	} {
		var chola, cholb Cholesky
		ok := chola.Factorize(test.a)
		if !ok {
			t.Fatal("unexpected Cholesky factorization failure for a: not positive definite")
		}
		ok = cholb.Factorize(test.b)
		if !ok {
			t.Fatal("unexpected Cholesky factorization failure for b: not positive definite")
		}

		var x Dense
		chola.SolveChol(&x, &cholb)

		var ans Dense
		ans.Mul(test.a, &x)
		if !EqualApprox(&ans, test.b, 1e-12) {
			var y Dense
			y.Solve(test.a, test.b)
			t.Errorf("incorrect Cholesky solve solution product\ngot solution:\n%.4v\nwant solution\n%.4v",
				Formatted(&x), Formatted(&y))
		}
	}
}

func TestCholeskySolveVec(t *testing.T) {
	for _, test := range []struct {
		a   *SymDense
		b   *VecDense
		ans *VecDense
	}{
		{
			a: NewSymDense(2, []float64{
				1, 0,
				0, 1,
			}),
			b:   NewVecDense(2, []float64{5, 6}),
			ans: NewVecDense(2, []float64{5, 6}),
		},
		{
			a: NewSymDense(3, []float64{
				53, 59, 37,
				0, 83, 71,
				0, 0, 101,
			}),
			b:   NewVecDense(3, []float64{5, 6, 7}),
			ans: NewVecDense(3, []float64{0.20745069393718094, -0.17421475529583694, 0.11577794010226464}),
		},
	} {
		var chol Cholesky
		ok := chol.Factorize(test.a)
		if !ok {
			t.Fatal("unexpected Cholesky factorization failure: not positive definite")
		}

		var x VecDense
		chol.SolveVec(&x, test.b)
		if !EqualApprox(&x, test.ans, 1e-12) {
			t.Error("incorrect Cholesky solve solution")
		}

		var ans VecDense
		ans.MulVec(test.a, &x)
		if !EqualApprox(&ans, test.b, 1e-12) {
			t.Error("incorrect Cholesky solve solution product")
		}
	}
}

func TestCholeskyToSym(t *testing.T) {
	for _, test := range []*SymDense{
		NewSymDense(3, []float64{
			53, 59, 37,
			0, 83, 71,
			0, 0, 101,
		}),
	} {
		var chol Cholesky
		ok := chol.Factorize(test)
		if !ok {
			t.Fatal("unexpected Cholesky factorization failure: not positive definite")
		}
		s := chol.ToSym(nil)

		if !EqualApprox(s, test, 1e-12) {
			t.Errorf("Cholesky reconstruction not equal to original matrix.\nWant:\n% v\nGot:\n% v\n", Formatted(test), Formatted(s))
		}
	}
}

func TestCloneCholesky(t *testing.T) {
	for _, test := range []*SymDense{
		NewSymDense(3, []float64{
			53, 59, 37,
			0, 83, 71,
			0, 0, 101,
		}),
	} {
		var chol Cholesky
		ok := chol.Factorize(test)
		if !ok {
			panic("bad test")
		}
		var chol2 Cholesky
		chol2.Clone(&chol)

		if chol.cond != chol2.cond {
			t.Errorf("condition number mismatch from zero")
		}
		if !Equal(chol.chol, chol2.chol) {
			t.Errorf("chol mismatch from zero")
		}

		// Corrupt chol2 and try again
		chol2.cond = math.NaN()
		chol2.chol = NewTriDense(2, Upper, nil)
		chol2.Clone(&chol)
		if chol.cond != chol2.cond {
			t.Errorf("condition number mismatch from non-zero")
		}
		if !Equal(chol.chol, chol2.chol) {
			t.Errorf("chol mismatch from non-zero")
		}
	}
}

func TestCholeskyInverseTo(t *testing.T) {
	for _, n := range []int{1, 3, 5, 9} {
		data := make([]float64, n*n)
		for i := range data {
			data[i] = rand.NormFloat64()
		}
		var s SymDense
		s.SymOuterK(1, NewDense(n, n, data))

		var chol Cholesky
		ok := chol.Factorize(&s)
		if !ok {
			t.Errorf("Bad test, cholesky decomposition failed")
		}

		var sInv SymDense
		chol.InverseTo(&sInv)

		var ans Dense
		ans.Mul(&sInv, &s)
		if !equalApprox(eye(n), &ans, 1e-8, false) {
			var diff Dense
			diff.Sub(eye(n), &ans)
			t.Errorf("SymDense times Cholesky inverse not identity. Norm diff = %v", Norm(&diff, 2))
		}
	}
}

func TestCholeskySymRankOne(t *testing.T) {
	rand.Seed(1)
	for _, n := range []int{1, 2, 3, 4, 5, 7, 10, 20, 50, 100} {
		for k := 0; k < 50; k++ {
			// Construct a random positive definite matrix.
			data := make([]float64, n*n)
			for i := range data {
				data[i] = rand.NormFloat64()
			}
			var a SymDense
			a.SymOuterK(1, NewDense(n, n, data))

			// Construct random data for updating.
			xdata := make([]float64, n)
			for i := range xdata {
				xdata[i] = rand.NormFloat64()
			}
			x := NewVecDense(n, xdata)
			alpha := rand.NormFloat64()

			// Compute the updated matrix directly. If alpha > 0, there are no
			// issues. If alpha < 0, it could be that the final matrix is not
			// positive definite, so instead switch the two matrices.
			aUpdate := NewSymDense(n, nil)
			if alpha > 0 {
				aUpdate.SymRankOne(&a, alpha, x)
			} else {
				aUpdate.CopySym(&a)
				a.Reset()
				a.SymRankOne(aUpdate, -alpha, x)
			}

			// Compare the Cholesky decomposition computed with Cholesky.SymRankOne
			// with that computed from updating A directly.
			var chol Cholesky
			ok := chol.Factorize(&a)
			if !ok {
				t.Errorf("Bad random test, Cholesky factorization failed")
				continue
			}

			var cholUpdate Cholesky
			ok = cholUpdate.SymRankOne(&chol, alpha, x)
			if !ok {
				t.Errorf("n=%v, alpha=%v: unexpected failure", n, alpha)
				continue
			}

			var aCompare SymDense
			cholUpdate.ToSym(&aCompare)
			if !EqualApprox(&aCompare, aUpdate, 1e-13) {
				t.Errorf("n=%v, alpha=%v: mismatch between updated matrix and from Cholesky:\nupdated:\n%v\nfrom Cholesky:\n%v",
					n, alpha, Formatted(aUpdate), Formatted(&aCompare))
			}
		}
	}

	for i, test := range []struct {
		a     *SymDense
		alpha float64
		x     []float64

		wantOk bool
	}{
		{
			// Update (to positive definite matrix).
			a: NewSymDense(4, []float64{
				1, 1, 1, 1,
				0, 2, 3, 4,
				0, 0, 6, 10,
				0, 0, 0, 20,
			}),
			alpha:  1,
			x:      []float64{0, 0, 0, 1},
			wantOk: true,
		},
		{
			// Downdate to singular matrix.
			a: NewSymDense(4, []float64{
				1, 1, 1, 1,
				0, 2, 3, 4,
				0, 0, 6, 10,
				0, 0, 0, 20,
			}),
			alpha:  -1,
			x:      []float64{0, 0, 0, 1},
			wantOk: false,
		},
		{
			// Downdate to positive definite matrix.
			a: NewSymDense(4, []float64{
				1, 1, 1, 1,
				0, 2, 3, 4,
				0, 0, 6, 10,
				0, 0, 0, 20,
			}),
			alpha:  -1 / 2,
			x:      []float64{0, 0, 0, 1},
			wantOk: true,
		},
		{
			// Issue #453.
			a:      NewSymDense(1, []float64{1}),
			alpha:  -1,
			x:      []float64{0.25},
			wantOk: true,
		},
	} {
		var chol Cholesky
		ok := chol.Factorize(test.a)
		if !ok {
			t.Errorf("Case %v: bad test, Cholesky factorization failed", i)
			continue
		}

		x := NewVecDense(len(test.x), test.x)
		ok = chol.SymRankOne(&chol, test.alpha, x)
		if !ok {
			if test.wantOk {
				t.Errorf("Case %v: unexpected failure from SymRankOne", i)
			}
			continue
		}
		if ok && !test.wantOk {
			t.Errorf("Case %v: expected a failure from SymRankOne", i)
		}

		a := test.a
		a.SymRankOne(a, test.alpha, x)

		var achol SymDense
		chol.ToSym(&achol)
		if !EqualApprox(&achol, a, 1e-13) {
			t.Errorf("Case %v: mismatch between updated matrix and from Cholesky:\nupdated:\n%v\nfrom Cholesky:\n%v",
				i, Formatted(a), Formatted(&achol))
		}
	}
}

func TestCholeskyExtendVecSym(t *testing.T) {
	for cas, test := range []struct {
		a *SymDense
	}{
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
		},
	} {
		n := test.a.Symmetric()
		as := test.a.SliceSquare(0, n-1).(*SymDense)

		// Compute the full factorization to use later (do the full factorization
		// first to ensure the matrix is positive definite).
		var cholFull Cholesky
		ok := cholFull.Factorize(test.a)
		if !ok {
			panic("mat: bad test, matrix not positive definite")
		}

		var chol Cholesky
		ok = chol.Factorize(as)
		if !ok {
			panic("mat: bad test, subset is not positive definite")
		}
		row := NewVecDense(n, nil)
		for i := 0; i < n; i++ {
			row.SetVec(i, test.a.At(n-1, i))
		}

		var cholNew Cholesky
		ok = cholNew.ExtendVecSym(&chol, row)
		if !ok {
			t.Errorf("cas %v: update not positive definite", cas)
		}
		a := cholNew.ToSym(nil)
		if !EqualApprox(a, test.a, 1e-12) {
			t.Errorf("cas %v: mismatch", cas)
		}

		// test in-place
		ok = chol.ExtendVecSym(&chol, row)
		if !ok {
			t.Errorf("cas %v: in-place update not positive definite", cas)
		}
		if !equalChol(&chol, &cholNew) {
			t.Errorf("cas %v: Cholesky different in-place vs. new", cas)
		}

		// Test that the factorization is about right compared with the direct
		// full factorization. Use a high tolerance on the condition number
		// since the condition number with the updated rule is approximate.
		if !equalApproxChol(&chol, &cholFull, 1e-12, 0.3) {
			t.Errorf("cas %v: updated Cholesky does not match full", cas)
		}
	}
}

func TestCholeskyScale(t *testing.T) {
	for cas, test := range []struct {
		a *SymDense
		f float64
	}{
		{
			a: NewSymDense(3, []float64{
				4, 1, 1,
				0, 2, 3,
				0, 0, 6,
			}),
			f: 0.5,
		},
	} {
		var chol Cholesky
		ok := chol.Factorize(test.a)
		if !ok {
			t.Errorf("Case %v: bad test, Cholesky factorization failed", cas)
			continue
		}

		// Compare the update to a new Cholesky to an update in-place.
		var cholUpdate Cholesky
		cholUpdate.Scale(test.f, &chol)
		chol.Scale(test.f, &chol)
		if !equalChol(&chol, &cholUpdate) {
			t.Errorf("Case %d: cholesky mismatch new receiver", cas)
		}
		var sym SymDense
		chol.ToSym(&sym)
		var comp SymDense
		comp.ScaleSym(test.f, test.a)
		if !EqualApprox(&comp, &sym, 1e-14) {
			t.Errorf("Case %d: cholesky reconstruction doesn't match scaled matrix", cas)
		}

		var cholTest Cholesky
		cholTest.Factorize(&comp)
		if !equalApproxChol(&cholTest, &chol, 1e-12, 1e-12) {
			t.Errorf("Case %d: cholesky mismatch with scaled matrix. %v, %v", cas, cholTest.cond, chol.cond)
		}
	}
}

// equalApproxChol checks that the two Cholesky decompositions are equal.
func equalChol(a, b *Cholesky) bool {
	return Equal(a.chol, b.chol) && a.cond == b.cond
}

// equalApproxChol checks that the two Cholesky decompositions are approximately
// the same with the given tolerance on equality for the Triangular component and
// condition.
func equalApproxChol(a, b *Cholesky, matTol, condTol float64) bool {
	if !EqualApprox(a.chol, b.chol, matTol) {
		return false
	}
	return floats.EqualWithinAbsOrRel(a.cond, b.cond, condTol, condTol)
}

func BenchmarkCholeskySmall(b *testing.B) {
	benchmarkCholesky(b, 2)
}

func BenchmarkCholeskyMedium(b *testing.B) {
	benchmarkCholesky(b, testblas.MediumMat)
}

func BenchmarkCholeskyLarge(b *testing.B) {
	benchmarkCholesky(b, testblas.LargeMat)
}

func benchmarkCholesky(b *testing.B, n int) {
	base := make([]float64, n*n)
	for i := range base {
		base[i] = rand.Float64()
	}
	bm := NewDense(n, n, base)
	bm.Mul(bm.T(), bm)
	am := NewSymDense(n, bm.mat.Data)

	var chol Cholesky
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ok := chol.Factorize(am)
		if !ok {
			panic("not pos def")
		}
	}
}
