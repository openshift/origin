// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import (
	"sort"

	"golang.org/x/exp/rand"
)

// WithoutReplacement samples len(idxs) integers from [0, n) without replacement.
// That is, upon return the elements of idxs will be unique integers. If source
// is non-nil it will be used to generate random numbers, otherwise the default
// source from the math/rand package will be used.
//
// WithoutReplacement will panic if len(idxs) > n.
func WithoutReplacement(idxs []int, n int, src rand.Source) {
	if len(idxs) == 0 {
		panic("withoutreplacement: zero length input")
	}
	if len(idxs) > n {
		panic("withoutreplacement: impossible size inputs")
	}

	// There are two algorithms. One is to generate a random permutation
	// and take the first len(idxs) elements. The second is to generate
	// individual random numbers for each element and check uniqueness. The first
	// method scales as O(n), and the second scales as O(len(idxs)^2). Choose
	// the algorithm accordingly.
	if n < len(idxs)*len(idxs) {
		var perm []int
		if src != nil {
			perm = rand.New(src).Perm(n)
		} else {
			perm = rand.Perm(n)
		}
		copy(idxs, perm)
		return
	}

	// Instead, generate the random numbers directly.
	sorted := make([]int, 0, len(idxs))
	for i := range idxs {
		var r int
		if src != nil {
			r = rand.New(src).Intn(n - i)
		} else {
			r = rand.Intn(n - i)
		}
		for _, v := range sorted {
			if r >= v {
				r++
			}
		}
		idxs[i] = r
		sorted = append(sorted, r)
		sort.Ints(sorted)
	}
}
