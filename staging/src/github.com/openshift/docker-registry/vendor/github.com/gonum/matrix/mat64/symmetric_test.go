package mat64

import (
	"math/rand"

	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"gopkg.in/check.v1"
)

func (s *S) TestNewSymmetric(c *check.C) {
	for i, test := range []struct {
		data []float64
		N    int
		mat  *SymDense
	}{
		{
			data: []float64{
				1, 2, 3,
				4, 5, 6,
				7, 8, 9,
			},
			N: 3,
			mat: &SymDense{blas64.Symmetric{
				N:      3,
				Stride: 3,
				Uplo:   blas.Upper,
				Data:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
			}},
		},
	} {
		t := NewSymDense(test.N, test.data)
		rows, cols := t.Dims()
		c.Check(rows, check.Equals, test.N, check.Commentf("Test %d", i))
		c.Check(cols, check.Equals, test.N, check.Commentf("Test %d", i))
		c.Check(t, check.DeepEquals, test.mat, check.Commentf("Test %d", i))

		m := NewDense(test.N, test.N, test.data)
		c.Check(t.mat.Data, check.DeepEquals, m.mat.Data, check.Commentf("Test %d", i))

		c.Check(func() { NewSymDense(3, []float64{1, 2}) }, check.PanicMatches, ErrShape.Error())
	}
}

func (s *S) TestSymAtSet(c *check.C) {
	t := &SymDense{blas64.Symmetric{
		N:      3,
		Stride: 3,
		Uplo:   blas.Upper,
		Data:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
	}}
	rows, cols := t.Dims()
	// Check At out of bounds
	c.Check(func() { t.At(rows, 0) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.At(0, cols) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.At(rows+1, 0) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.At(0, cols+1) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.At(-1, 0) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.At(0, -1) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))

	// Check Set out of bounds
	c.Check(func() { t.SetSym(rows, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetSym(0, cols, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.SetSym(rows+1, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetSym(0, cols+1, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.SetSym(-1, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetSym(0, -1, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))

	c.Check(t.At(2, 1), check.Equals, 6.0)
	c.Check(t.At(1, 2), check.Equals, 6.0)
	t.SetSym(1, 2, 15)
	c.Check(t.At(2, 1), check.Equals, 15.0)
	c.Check(t.At(1, 2), check.Equals, 15.0)
	t.SetSym(2, 1, 12)
	c.Check(t.At(2, 1), check.Equals, 12.0)
	c.Check(t.At(1, 2), check.Equals, 12.0)
}

func (s *S) TestSymAdd(c *check.C) {
	for _, test := range []struct {
		n int
	}{
		{n: 1},
		{n: 2},
		{n: 3},
		{n: 4},
		{n: 5},
		{n: 10},
	} {
		n := test.n
		a := NewSymDense(n, nil)
		for i := range a.mat.Data {
			a.mat.Data[i] = rand.Float64()
		}
		b := NewSymDense(n, nil)
		for i := range a.mat.Data {
			b.mat.Data[i] = rand.Float64()
		}
		var m Dense
		m.Add(a, b)

		// Check with new receiver
		var s SymDense
		s.AddSym(a, b)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}

		// Check with equal receiver
		s.CopySym(a)
		s.AddSym(&s, b)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}
	}
}

func (s *S) TestCopy(c *check.C) {
	for _, test := range []struct {
		n int
	}{
		{n: 1},
		{n: 2},
		{n: 3},
		{n: 4},
		{n: 5},
		{n: 10},
	} {
		n := test.n
		a := NewSymDense(n, nil)
		for i := range a.mat.Data {
			a.mat.Data[i] = rand.Float64()
		}
		s := NewSymDense(n, nil)
		s.CopySym(a)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := a.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}
	}
}

func (s *S) TestSymRankOne(c *check.C) {
	for _, test := range []struct {
		n int
	}{
		{n: 1},
		{n: 2},
		{n: 3},
		{n: 4},
		{n: 5},
		{n: 10},
	} {
		n := test.n
		alpha := 2.0
		a := NewSymDense(n, nil)
		for i := range a.mat.Data {
			a.mat.Data[i] = rand.Float64()
		}
		x := make([]float64, n)
		for i := range x {
			x[i] = rand.Float64()
		}

		xMat := NewDense(n, 1, x)
		var m Dense
		m.MulTrans(xMat, false, xMat, true)
		m.Scale(alpha, &m)
		m.Add(&m, a)

		// Check with new receiver
		s := NewSymDense(n, nil)
		s.SymRankOne(a, alpha, x)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}

		// Check with reused receiver
		copy(s.mat.Data, a.mat.Data)
		s.SymRankOne(s, alpha, x)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}
	}
}

func (s *S) TestRankTwo(c *check.C) {
	for _, test := range []struct {
		n int
	}{
		{n: 1},
		{n: 2},
		{n: 3},
		{n: 4},
		{n: 5},
		{n: 10},
	} {
		n := test.n
		alpha := 2.0
		a := NewSymDense(n, nil)
		for i := range a.mat.Data {
			a.mat.Data[i] = rand.Float64()
		}
		x := make([]float64, n)
		y := make([]float64, n)
		for i := range x {
			x[i] = rand.Float64()
			y[i] = rand.Float64()
		}

		xMat := NewDense(n, 1, x)
		yMat := NewDense(n, 1, y)
		var m Dense
		m.MulTrans(xMat, false, yMat, true)
		var tmp Dense
		tmp.MulTrans(yMat, false, xMat, true)
		m.Add(&m, &tmp)
		m.Scale(alpha, &m)
		m.Add(&m, a)

		// Check with new receiver
		s := NewSymDense(n, nil)
		s.RankTwo(a, alpha, x, y)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}

		// Check with reused receiver
		copy(s.mat.Data, a.mat.Data)
		s.RankTwo(s, alpha, x, y)
		for i := 0; i < n; i++ {
			for j := i; j < n; j++ {
				v := m.At(i, j)
				c.Check(s.At(i, j), check.Equals, v)
			}
		}
	}
}
