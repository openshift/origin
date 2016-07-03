package mat64

import (
	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"gopkg.in/check.v1"
)

func (s *S) TestNewTriangular(c *check.C) {
	for i, test := range []struct {
		data  []float64
		N     int
		upper bool
		mat   *TriDense
	}{
		{
			data: []float64{
				1, 2, 3,
				4, 5, 6,
				7, 8, 9,
			},
			N:     3,
			upper: true,
			mat: &TriDense{blas64.Triangular{
				N:      3,
				Stride: 3,
				Uplo:   blas.Upper,
				Data:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
				Diag:   blas.NonUnit,
			}},
		},
	} {
		t := NewTriDense(test.N, test.upper, test.data)
		rows, cols := t.Dims()
		c.Check(rows, check.Equals, test.N, check.Commentf("Test %d", i))
		c.Check(cols, check.Equals, test.N, check.Commentf("Test %d", i))
		c.Check(t, check.DeepEquals, test.mat, check.Commentf("Test %d", i))
	}
}
func (s *S) TestTriAtSet(c *check.C) {
	t := &TriDense{blas64.Triangular{
		N:      3,
		Stride: 3,
		Uplo:   blas.Upper,
		Data:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Diag:   blas.NonUnit,
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
	c.Check(func() { t.SetTri(rows, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetTri(0, cols, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.SetTri(rows+1, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetTri(0, cols+1, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.SetTri(-1, 0, 1.2) }, check.PanicMatches, ErrRowAccess.Error(), check.Commentf("Test row out of bounds"))
	c.Check(func() { t.SetTri(0, -1, 1.2) }, check.PanicMatches, ErrColAccess.Error(), check.Commentf("Test col out of bounds"))
	c.Check(func() { t.SetTri(2, 1, 1.2) }, check.PanicMatches, "mat64: triangular set out of bounds", check.Commentf("Test lower access"))
	t.mat.Uplo = blas.Lower
	c.Check(func() { t.SetTri(1, 2, 1.2) }, check.PanicMatches, "mat64: triangular set out of bounds", check.Commentf("Test upper access"))
	c.Check(t.At(2, 1), check.Equals, 8.0)
	t.SetTri(2, 1, 15)
	c.Check(t.At(2, 1), check.Equals, 15.0)
	t.mat.Uplo = blas.Upper
	c.Check(t.At(1, 2), check.Equals, 6.0)
	t.SetTri(1, 2, 15)
	c.Check(t.At(1, 2), check.Equals, 15.0)
}
