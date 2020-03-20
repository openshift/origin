// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import (
	"math"
	"reflect"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas/blas64"
)

func TestNewDiagDense(t *testing.T) {
	for i, test := range []struct {
		data  []float64
		n     int
		mat   *DiagDense
		dense *Dense
	}{
		{
			data: []float64{1, 2, 3, 4, 5, 6},
			n:    6,
			mat: &DiagDense{
				mat: blas64.Vector{N: 6, Inc: 1, Data: []float64{1, 2, 3, 4, 5, 6}},
			},
			dense: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
	} {
		band := NewDiagDense(test.n, test.data)
		rows, cols := band.Dims()

		if rows != test.n {
			t.Errorf("unexpected number of rows for test %d: got: %d want: %d", i, rows, test.n)
		}
		if cols != test.n {
			t.Errorf("unexpected number of cols for test %d: got: %d want: %d", i, cols, test.n)
		}
		if !reflect.DeepEqual(band, test.mat) {
			t.Errorf("unexpected value via reflect for test %d: got: %v want: %v", i, band, test.mat)
		}
		if !Equal(band, test.mat) {
			t.Errorf("unexpected value via mat.Equal for test %d: got: %v want: %v", i, band, test.mat)
		}
		if !Equal(band, test.dense) {
			t.Errorf("unexpected value via mat.Equal(band, dense) for test %d:\ngot:\n% v\nwant:\n% v", i, Formatted(band), Formatted(test.dense))
		}
	}
}

func TestDiagDenseZero(t *testing.T) {
	// Elements that equal 1 should be set to zero, elements that equal -1
	// should remain unchanged.
	for _, test := range []*DiagDense{
		{
			mat: blas64.Vector{
				N:   5,
				Inc: 2,
				Data: []float64{
					1, -1,
					1, -1,
					1, -1,
					1, -1,
					1,
				},
			},
		},
	} {
		dataCopy := make([]float64, len(test.mat.Data))
		copy(dataCopy, test.mat.Data)
		test.Zero()
		for i, v := range test.mat.Data {
			if dataCopy[i] != -1 && v != 0 {
				t.Errorf("Matrix not zeroed in bounds")
			}
			if dataCopy[i] == -1 && v != -1 {
				t.Errorf("Matrix zeroed out of bounds")
			}
		}
	}
}

func TestDiagonalStride(t *testing.T) {
	for _, test := range []struct {
		diag  *DiagDense
		dense *Dense
	}{
		{
			diag: &DiagDense{
				mat: blas64.Vector{N: 6, Inc: 1, Data: []float64{1, 2, 3, 4, 5, 6}},
			},
			dense: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			diag: &DiagDense{
				mat: blas64.Vector{N: 6, Inc: 2, Data: []float64{
					1, 0,
					2, 0,
					3, 0,
					4, 0,
					5, 0,
					6,
				}},
			},
			dense: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			diag: &DiagDense{
				mat: blas64.Vector{N: 6, Inc: 5, Data: []float64{
					1, 0, 0, 0, 0,
					2, 0, 0, 0, 0,
					3, 0, 0, 0, 0,
					4, 0, 0, 0, 0,
					5, 0, 0, 0, 0,
					6,
				}},
			},
			dense: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
	} {
		if !Equal(test.diag, test.dense) {
			t.Errorf("unexpected value via mat.Equal for stride %d: got: %v want: %v",
				test.diag.mat.Inc, test.diag, test.dense)
		}
	}
}

func TestDiagFrom(t *testing.T) {
	for i, test := range []struct {
		mat  Matrix
		want *Dense
	}{
		{
			mat: NewDiagDense(6, []float64{1, 2, 3, 4, 5, 6}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewBandDense(6, 6, 1, 1, []float64{
				math.NaN(), 1, math.NaN(),
				math.NaN(), 2, math.NaN(),
				math.NaN(), 3, math.NaN(),
				math.NaN(), 4, math.NaN(),
				math.NaN(), 5, math.NaN(),
				math.NaN(), 6, math.NaN(),
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewDense(6, 6, []float64{
				1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), 5, math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 6,
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewDense(6, 4, []float64{
				1, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4,
				math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(),
			}),
			want: NewDense(4, 4, []float64{
				1, 0, 0, 0,
				0, 2, 0, 0,
				0, 0, 3, 0,
				0, 0, 0, 4,
			}),
		},
		{
			mat: NewDense(4, 6, []float64{
				1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
			}),
			want: NewDense(4, 4, []float64{
				1, 0, 0, 0,
				0, 2, 0, 0,
				0, 0, 3, 0,
				0, 0, 0, 4,
			}),
		},
		{
			mat: NewSymBandDense(6, 1, []float64{
				1, math.NaN(),
				2, math.NaN(),
				3, math.NaN(),
				4, math.NaN(),
				5, math.NaN(),
				6, math.NaN(),
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewSymDense(6, []float64{
				1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), 5, math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 6,
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewTriBandDense(6, 2, Upper, []float64{
				1, math.NaN(), math.NaN(),
				2, math.NaN(), math.NaN(),
				3, math.NaN(), math.NaN(),
				4, math.NaN(), math.NaN(),
				5, math.NaN(), math.NaN(),
				6, math.NaN(), math.NaN(),
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewTriBandDense(6, 2, Lower, []float64{
				math.NaN(), math.NaN(), 1,
				math.NaN(), math.NaN(), 2,
				math.NaN(), math.NaN(), 3,
				math.NaN(), math.NaN(), 4,
				math.NaN(), math.NaN(), 5,
				math.NaN(), math.NaN(), 6,
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewTriDense(6, Upper, []float64{
				1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), 5, math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 6,
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat: NewTriDense(6, Lower, []float64{
				1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), 5, math.NaN(),
				math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 6,
			}),
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
		{
			mat:  NewVecDense(6, []float64{1, 2, 3, 4, 5, 6}),
			want: NewDense(1, 1, []float64{1}),
		},
		{
			mat: &basicMatrix{
				mat: blas64.General{
					Rows:   6,
					Cols:   6,
					Stride: 6,
					Data: []float64{
						1, math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(),
						math.NaN(), 2, math.NaN(), math.NaN(), math.NaN(), math.NaN(),
						math.NaN(), math.NaN(), 3, math.NaN(), math.NaN(), math.NaN(),
						math.NaN(), math.NaN(), math.NaN(), 4, math.NaN(), math.NaN(),
						math.NaN(), math.NaN(), math.NaN(), math.NaN(), 5, math.NaN(),
						math.NaN(), math.NaN(), math.NaN(), math.NaN(), math.NaN(), 6,
					},
				},
				capRows: 6,
				capCols: 6,
			},
			want: NewDense(6, 6, []float64{
				1, 0, 0, 0, 0, 0,
				0, 2, 0, 0, 0, 0,
				0, 0, 3, 0, 0, 0,
				0, 0, 0, 4, 0, 0,
				0, 0, 0, 0, 5, 0,
				0, 0, 0, 0, 0, 6,
			}),
		},
	} {
		var got DiagDense
		got.DiagFrom(test.mat)
		if !Equal(&got, test.want) {
			r, c := test.mat.Dims()
			t.Errorf("unexpected value via mat.Equal for %d×%d %T test %d:\ngot:\n% v\nwant:\n% v",
				r, c, test.mat, i, Formatted(&got), Formatted(test.want))
		}
	}
}

// diagDenseViewer takes the view of the Diagonal with the underlying Diagonal
// as the DiagDense type.
type diagDenseViewer interface {
	Matrix
	DiagView() Diagonal
}

func testDiagView(t *testing.T, cas int, test diagDenseViewer) {
	// Check the DiagView matches the Diagonal.
	r, c := test.Dims()
	diagView := test.DiagView()
	for i := 0; i < min(r, c); i++ {
		if diagView.At(i, i) != test.At(i, i) {
			t.Errorf("Diag mismatch case %d, element %d", cas, i)
		}
	}

	// Check that changes to the diagonal are reflected.
	offset := 10.0
	diag := diagView.(*DiagDense)
	for i := 0; i < min(r, c); i++ {
		v := test.At(i, i)
		diag.SetDiag(i, v+offset)
		if test.At(i, i) != v+offset {
			t.Errorf("Diag set mismatch case %d, element %d", cas, i)
		}
	}

	// Check that DiagView and DiagFrom match.
	var diag2 DiagDense
	diag2.DiagFrom(test)
	if !Equal(diag, &diag2) {
		t.Errorf("Cas %d: DiagView and DiagFrom mismatch", cas)
	}
}

func TestDiagonalAtSet(t *testing.T) {
	for _, n := range []int{1, 3, 8} {
		for _, nilstart := range []bool{true, false} {
			var diag *DiagDense
			if nilstart {
				diag = NewDiagDense(n, nil)
			} else {
				data := make([]float64, n)
				diag = NewDiagDense(n, data)
				// Test the data is used.
				for i := range data {
					data[i] = -float64(i) - 1
					v := diag.At(i, i)
					if v != data[i] {
						t.Errorf("Diag shadow mismatch. Got %v, want %v", v, data[i])
					}
				}
			}
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					if i != j {
						if diag.At(i, j) != 0 {
							t.Errorf("Diag returned non-zero off diagonal element at %d, %d", i, j)
						}
					}
					v := float64(i) + 1
					diag.SetDiag(i, v)
					v2 := diag.At(i, i)
					if v2 != v {
						t.Errorf("Diag at/set mismatch. Got %v, want %v", v, v2)
					}
				}
			}
		}
	}
}

func randDiagDense(size int, rnd *rand.Rand) *DiagDense {
	t := NewDiagDense(size, nil)
	for i := 0; i < size; i++ {
		t.SetDiag(i, rnd.Float64())
	}
	return t
}
