// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fourier

import "gonum.org/v1/gonum/fourier/internal/fftpack"

// DCT implements Discrete Cosine Transform for real sequences.
type DCT struct {
	work []float64
	ifac [15]int
}

// NewDCT returns a DCT initialized for work on sequences of length n.
// NewDCT will panic is n is not greater than 1.
func NewDCT(n int) *DCT {
	var t DCT
	t.Reset(n)
	return &t
}

// Len returns the length of the acceptable input.
func (t *DCT) Len() int { return len(t.work) / 3 }

// Reset reinitializes the DCT for work on sequences of length n.
// Reset will panic is n is not greater than 1.
func (t *DCT) Reset(n int) {
	if n <= 1 {
		panic("fourier: n less than 2")
	}
	if 3*n <= cap(t.work) {
		t.work = t.work[:3*n]
	} else {
		t.work = make([]float64, 3*n)
	}
	fftpack.Costi(n, t.work, t.ifac[:])
}

// Transform computes the Discrete Fourier Cosine Transform of
// the input data, src, placing the result in dst and returning it.
// This transform is unnormalized; a call to Transform followed by
// another call to Transform will multiply the input sequence by 2*(n-1),
// where n is the length of the sequence.
//
// If the length of src is not t.Len(), Transform will panic.
// If dst is nil, a new slice is allocated and returned. If dst is not nil and
// the length of dst does not equal t.Len(), FFT will panic.
// It is safe to use the same slice for dst and src.
func (t *DCT) Transform(dst, src []float64) []float64 {
	if len(src) != t.Len() {
		panic("fourier: sequence length mismatch")
	}
	if dst == nil {
		dst = make([]float64, t.Len())
	} else if len(dst) != t.Len() {
		panic("fourier: destination length mismatch")
	}
	copy(dst, src)
	fftpack.Cost(len(dst), dst, t.work, t.ifac[:])
	return dst
}

// DST implements Discrete Sine Transform for real sequences.
type DST struct {
	work []float64
	ifac [15]int
}

// NewDST returns a DST initialized for work on sequences of length n.
func NewDST(n int) *DST {
	var t DST
	t.Reset(n)
	return &t
}

// Len returns the length of the acceptable input.
func (t *DST) Len() int { return (2*len(t.work)+1)/5 - 1 }

// Reset reinitializes the DCT for work on sequences of length n.
func (t *DST) Reset(n int) {
	if 5*(n+1)/2 <= cap(t.work) {
		t.work = t.work[:5*(n+1)/2]
	} else {
		t.work = make([]float64, 5*(n+1)/2)
	}
	fftpack.Sinti(n, t.work, t.ifac[:])
}

// Transform computes the Discrete Fourier Sine Transform of the input
// data, src, placing the result in dst and returning it.
// This transform is unnormalized; a call to Transform followed by
// another call to Transform will multiply the input sequence by 2*(n-1),
// where n is the length of the sequence.
//
// If the length of src is not t.Len(), Transform will panic.
// If dst is nil, a new slice is allocated and returned. If dst is not nil and
// the length of dst does not equal t.Len(), FFT will panic.
// It is safe to use the same slice for dst and src.
func (t *DST) Transform(dst, src []float64) []float64 {
	if len(src) != t.Len() {
		panic("fourier: sequence length mismatch")
	}
	if dst == nil {
		dst = make([]float64, t.Len())
	} else if len(dst) != t.Len() {
		panic("fourier: destination length mismatch")
	}
	copy(dst, src)
	fftpack.Sint(len(dst), dst, t.work, t.ifac[:])
	return dst
}
