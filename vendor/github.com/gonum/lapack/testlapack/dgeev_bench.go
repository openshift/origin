// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.7

package testlapack

import (
	"math/rand"
	"testing"

	"github.com/gonum/blas/blas64"
	"github.com/gonum/lapack"
)

var resultGeneral blas64.General

func DgeevBenchmark(b *testing.B, impl Dgeever) {
	rnd := rand.New(rand.NewSource(1))
	benchmarks := []struct {
		name string
		a    blas64.General
	}{
		{"AntisymRandom3", NewAntisymRandom(3, rnd).Matrix()},
		{"AntisymRandom4", NewAntisymRandom(4, rnd).Matrix()},
		{"AntisymRandom5", NewAntisymRandom(5, rnd).Matrix()},
		{"AntisymRandom10", NewAntisymRandom(10, rnd).Matrix()},
		{"AntisymRandom50", NewAntisymRandom(50, rnd).Matrix()},
		{"AntisymRandom100", NewAntisymRandom(100, rnd).Matrix()},
		{"AntisymRandom200", NewAntisymRandom(200, rnd).Matrix()},
		{"AntisymRandom500", NewAntisymRandom(500, rnd).Matrix()},
		{"Circulant3", Circulant(3).Matrix()},
		{"Circulant4", Circulant(4).Matrix()},
		{"Circulant5", Circulant(5).Matrix()},
		{"Circulant10", Circulant(10).Matrix()},
		{"Circulant50", Circulant(50).Matrix()},
		{"Circulant100", Circulant(100).Matrix()},
		{"Circulant200", Circulant(200).Matrix()},
		{"Circulant500", Circulant(500).Matrix()},
	}
	for _, bm := range benchmarks {
		n := bm.a.Rows
		a := zeros(n, n, n)
		vl := zeros(n, n, n)
		vr := zeros(n, n, n)
		wr := make([]float64, n)
		wi := make([]float64, n)
		work := make([]float64, 1)
		impl.Dgeev(lapack.ComputeLeftEV, lapack.ComputeRightEV, n, nil, n, nil, nil, nil, n, nil, n, work, -1)
		work = make([]float64, int(work[0]))
		b.Run(bm.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				copyGeneral(a, bm.a)
				b.StartTimer()
				impl.Dgeev(lapack.ComputeLeftEV, lapack.ComputeRightEV, n, a.Data, a.Stride, wr, wi,
					vl.Data, vl.Stride, vr.Data, vr.Stride, work, len(work))
			}
			resultGeneral = a
		})
	}
}
