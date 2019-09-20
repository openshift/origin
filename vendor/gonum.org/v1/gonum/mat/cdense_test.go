// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat

import "testing"

func TestCDenseNewAtSet(t *testing.T) {
	for cas, test := range []struct {
		a          []complex128
		rows, cols int
	}{
		{
			a: []complex128{0, 0, 0,
				0, 0, 0,
				0, 0, 0},
			rows: 3,
			cols: 3,
		},
	} {
		aCopy := make([]complex128, len(test.a))
		copy(aCopy, test.a)
		mZero := NewCDense(test.rows, test.cols, nil)
		rows, cols := mZero.Dims()
		if rows != test.rows {
			t.Errorf("unexpected number of rows for test %d: got: %d want: %d", cas, rows, test.rows)
		}
		if cols != test.cols {
			t.Errorf("unexpected number of cols for test %d: got: %d want: %d", cas, cols, test.cols)
		}
		m := NewCDense(test.rows, test.cols, aCopy)
		for i := 0; i < test.rows; i++ {
			for j := 0; j < test.cols; j++ {
				v := m.At(i, j)
				idx := i*test.rows + j
				if v != test.a[idx] {
					t.Errorf("unexpected get value for test %d at i=%d, j=%d: got: %v, want: %v", cas, i, j, v, test.a[idx])
				}
				add := complex(float64(i+1), float64(j+1))
				m.Set(i, j, v+add)
				if m.At(i, j) != test.a[idx]+add {
					t.Errorf("unexpected set value for test %d at i=%d, j=%d: got: %v, want: %v", cas, i, j, v, test.a[idx]+add)
				}
			}
		}
	}
}
