// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
)

func TestWithoutReplacement(t *testing.T) {
	for cas, test := range []struct {
		N      int
		K      int
		Src    *rand.Rand
		Trials int
		Tol    float64
	}{
		{
			// Test with perm and source.
			N: 10, K: 5, Src: rand.New(rand.NewSource(1)),
			Trials: 100000, Tol: 1e-3,
		},
		{
			// Test without perm and with source.
			N: 10, K: 3, Src: rand.New(rand.NewSource(1)),
			Trials: 100000, Tol: 1e-3,
		},
	} {
		dist := make([]float64, test.N)
		for trial := 0; trial < test.Trials; trial++ {
			idxs := make([]int, test.K)
			WithoutReplacement(idxs, test.N, test.Src)

			allDiff := true
			for i := 0; i < len(idxs); i++ {
				v := idxs[i]
				for j := i + 1; j < len(idxs); j++ {
					if v == idxs[j] {
						allDiff = false
						break
					}
				}
			}
			if !allDiff {
				t.Errorf("Cas %d: Repeat in sampling. Idxs =%v", cas, idxs)
			}
			for _, v := range idxs {
				dist[v]++
			}
		}
		div := 1 / (float64(test.Trials) * float64(test.K))
		floats.Scale(div, dist)
		want := make([]float64, test.N)
		for i := range want {
			want[i] = 1 / float64(test.N)
		}
		if !floats.EqualApprox(want, dist, test.Tol) {
			t.Errorf("Cas %d: biased sampling. Want = %v, got = %v", cas, want, dist)
		}
	}
}
