// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package samplemv

import (
	"math"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distmv"
)

func TestHalton(t *testing.T) {
	for cas, test := range []struct {
		n int
		d int
	}{
		{10, 1},
		{100, 2},
		{1000, 3},
	} {
		src := rand.New(rand.NewSource(1))
		// Generate the samples.
		batch := mat.NewDense(test.n, test.d, nil)
		Halton{Kind: Owen, Q: distmv.NewUnitUniform(test.d, nil), Src: src}.Sample(batch)

		// In each dimension, the samples should be stratefied according to the
		// prime for that dimension. There should be at most 1 sample per
		// 1/b^k block, where k is log(n)/log(b).
		for d := 0; d < test.d; d++ {
			b := float64(nthPrime(d))
			fk := math.Log(float64(test.n)) / math.Log(b)
			k := math.Ceil(fk)
			den := math.Pow(b, k)
			m := make(map[int]int)
			for i := 0; i < test.n; i++ {
				bucket := int(batch.At(i, d) * den)
				m[bucket]++
			}
			for bucket, n := range m {
				if n > 1 {
					t.Errorf("case %d: bucket %v has %v samples", cas, bucket, n)
				}
			}
		}
	}
}
