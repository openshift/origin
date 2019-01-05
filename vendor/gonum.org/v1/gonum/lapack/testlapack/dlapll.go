// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/lapack"
)

type Dlapller interface {
	Dgesvder
	Dlapll(n int, x []float64, incX int, y []float64, incY int) float64
}

func DlapllTest(t *testing.T, impl Dlapller) {
	rnd := rand.New(rand.NewSource(1))
	for i, m := range []int{5, 6, 9, 300, 400, 600} {
		n := 2
		lda := n
		a := make([]float64, m*lda)
		for i := range a {
			a[i] = rnd.NormFloat64()
		}

		aCopy := make([]float64, len(a))
		copy(aCopy, a)

		got := impl.Dlapll(m, a[0:], 2, a[1:], 2)

		s := make([]float64, min(m, n))
		work := make([]float64, 1)
		impl.Dgesvd(lapack.SVDNone, lapack.SVDNone, m, n, aCopy, lda, s, nil, 0, nil, 0, work, -1)
		work = make([]float64, int(work[0]))
		impl.Dgesvd(lapack.SVDNone, lapack.SVDNone, m, n, aCopy, lda, s, nil, 0, nil, 0, work, len(work))
		want := s[len(s)-1]

		if !floats.EqualWithinAbsOrRel(got, want, 1e-14, 1e-14) {
			t.Errorf("unexpected ssmin for test %d: got:%f want:%f", i, got, want)
		}
	}
}
