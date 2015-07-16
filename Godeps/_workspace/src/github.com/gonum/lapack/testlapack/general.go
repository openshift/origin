// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"github.com/gonum/blas"
	"github.com/gonum/blas/blas64"
	"github.com/gonum/lapack"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractVMat collects the single reflectors from a into a matrix.
func extractVMat(m, n int, a []float64, lda int, direct lapack.Direct, store lapack.StoreV) blas64.General {
	k := min(m, n)
	switch {
	default:
		panic("not implemented")
	case direct == lapack.Forward && store == lapack.ColumnWise:
		v := blas64.General{
			Rows:   m,
			Cols:   k,
			Stride: k,
			Data:   make([]float64, m*k),
		}
		for i := 0; i < k; i++ {
			for j := 0; j < i; j++ {
				v.Data[j*v.Stride+i] = 0
			}
			v.Data[i*v.Stride+i] = 1
			for j := i + 1; j < m; j++ {
				v.Data[j*v.Stride+i] = a[j*lda+i]
			}
		}
		return v
	case direct == lapack.Forward && store == lapack.RowWise:
		v := blas64.General{
			Rows:   k,
			Cols:   n,
			Stride: n,
			Data:   make([]float64, k*n),
		}
		for i := 0; i < k; i++ {
			for j := 0; j < i; j++ {
				v.Data[i*v.Stride+j] = 0
			}
			v.Data[i*v.Stride+i] = 1
			for j := i + 1; j < n; j++ {
				v.Data[i*v.Stride+j] = a[i*lda+j]
			}
		}
		return v
	}
}

// constructVMat transforms the v matrix based on the storage.
func constructVMat(vMat blas64.General, store lapack.StoreV, direct lapack.Direct) blas64.General {
	m := vMat.Rows
	k := vMat.Cols
	switch {
	default:
		panic("not implemented")
	case store == lapack.ColumnWise && direct == lapack.Forward:
		ldv := k
		v := make([]float64, m*k)
		for i := 0; i < m; i++ {
			for j := 0; j < k; j++ {
				if j > i {
					v[i*ldv+j] = 0
				} else if j == i {
					v[i*ldv+i] = 1
				} else {
					v[i*ldv+j] = vMat.Data[i*vMat.Stride+j]
				}
			}
		}
		return blas64.General{
			Rows:   m,
			Cols:   k,
			Stride: k,
			Data:   v,
		}
	case store == lapack.RowWise && direct == lapack.Forward:
		ldv := m
		v := make([]float64, m*k)
		for i := 0; i < m; i++ {
			for j := 0; j < k; j++ {
				if j > i {
					v[j*ldv+i] = 0
				} else if j == i {
					v[j*ldv+i] = 1
				} else {
					v[j*ldv+i] = vMat.Data[i*vMat.Stride+j]
				}
			}
		}
		return blas64.General{
			Rows:   k,
			Cols:   m,
			Stride: m,
			Data:   v,
		}
	case store == lapack.ColumnWise && direct == lapack.Backward:
		rowsv := m
		ldv := k
		v := make([]float64, m*k)
		for i := 0; i < m; i++ {
			for j := 0; j < k; j++ {
				vrow := rowsv - i - 1
				vcol := k - j - 1
				if j > i {
					v[vrow*ldv+vcol] = 0
				} else if j == i {
					v[vrow*ldv+vcol] = 1
				} else {
					v[vrow*ldv+vcol] = vMat.Data[i*vMat.Stride+j]
				}
			}
		}
		return blas64.General{
			Rows:   rowsv,
			Cols:   ldv,
			Stride: ldv,
			Data:   v,
		}
	case store == lapack.RowWise && direct == lapack.Backward:
		rowsv := k
		ldv := m
		v := make([]float64, m*k)
		for i := 0; i < m; i++ {
			for j := 0; j < k; j++ {
				vcol := ldv - i - 1
				vrow := k - j - 1
				if j > i {
					v[vrow*ldv+vcol] = 0
				} else if j == i {
					v[vrow*ldv+vcol] = 1
				} else {
					v[vrow*ldv+vcol] = vMat.Data[i*vMat.Stride+j]
				}
			}
		}
		return blas64.General{
			Rows:   rowsv,
			Cols:   ldv,
			Stride: ldv,
			Data:   v,
		}
	}
}

func constructH(tau []float64, v blas64.General, store lapack.StoreV, direct lapack.Direct) blas64.General {
	m := v.Rows
	k := v.Cols
	if store == lapack.RowWise {
		m, k = k, m
	}
	h := blas64.General{
		Rows:   m,
		Cols:   m,
		Stride: m,
		Data:   make([]float64, m*m),
	}
	for i := 0; i < m; i++ {
		h.Data[i*m+i] = 1
	}
	for i := 0; i < k; i++ {
		vecData := make([]float64, m)
		if store == lapack.ColumnWise {
			for j := 0; j < m; j++ {
				vecData[j] = v.Data[j*v.Cols+i]
			}
		} else {
			for j := 0; j < m; j++ {
				vecData[j] = v.Data[i*v.Cols+j]
			}
		}
		vec := blas64.Vector{
			Inc:  1,
			Data: vecData,
		}

		hi := blas64.General{
			Rows:   m,
			Cols:   m,
			Stride: m,
			Data:   make([]float64, m*m),
		}
		for i := 0; i < m; i++ {
			hi.Data[i*m+i] = 1
		}
		// hi = I - tau * v * v^T
		blas64.Ger(-tau[i], vec, vec, hi)

		hcopy := blas64.General{
			Rows:   m,
			Cols:   m,
			Stride: m,
			Data:   make([]float64, m*m),
		}
		copy(hcopy.Data, h.Data)
		if direct == lapack.Forward {
			// H = H * H_I in forward mode
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, hcopy, hi, 0, h)
		} else {
			// H = H_I * H in backward mode
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, hi, hcopy, 0, h)
		}
	}
	return h
}

// constructQ constructs the Q matrix from the result of dgeqrf and dgeqr2
func constructQ(kind string, m, n int, a []float64, lda int, tau []float64) blas64.General {
	k := min(m, n)
	var sz int
	switch kind {
	case "QR":
		sz = m
	case "LQ":
		sz = n
	}

	q := blas64.General{
		Rows:   sz,
		Cols:   sz,
		Stride: sz,
		Data:   make([]float64, sz*sz),
	}
	for i := 0; i < sz; i++ {
		q.Data[i*sz+i] = 1
	}
	qCopy := blas64.General{
		Rows:   q.Rows,
		Cols:   q.Cols,
		Stride: q.Stride,
		Data:   make([]float64, len(q.Data)),
	}
	for i := 0; i < k; i++ {
		h := blas64.General{
			Rows:   sz,
			Cols:   sz,
			Stride: sz,
			Data:   make([]float64, sz*sz),
		}
		for j := 0; j < sz; j++ {
			h.Data[j*sz+j] = 1
		}
		vVec := blas64.Vector{
			Inc:  1,
			Data: make([]float64, sz),
		}
		for j := 0; j < i; j++ {
			vVec.Data[j] = 0
		}
		vVec.Data[i] = 1
		switch kind {
		case "QR":
			for j := i + 1; j < sz; j++ {
				vVec.Data[j] = a[lda*j+i]
			}
		case "LQ":
			for j := i + 1; j < sz; j++ {
				vVec.Data[j] = a[i*lda+j]
			}
		}
		blas64.Ger(-tau[i], vVec, vVec, h)
		copy(qCopy.Data, q.Data)
		// Mulitply q by the new h
		switch kind {
		case "QR":
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, qCopy, h, 0, q)
		case "LQ":
			blas64.Gemm(blas.NoTrans, blas.NoTrans, 1, h, qCopy, 0, q)
		}
	}
	return q
}
