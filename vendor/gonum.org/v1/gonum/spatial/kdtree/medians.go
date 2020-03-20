// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

import (
	"sort"

	"golang.org/x/exp/rand"
)

// Partition partitions list such that all elements less than the value at
// pivot prior to the call are placed before that element and all elements
// greater than that value are placed after it. The final location of the
// element at pivot prior to the call is returned.
func Partition(list sort.Interface, pivot int) int {
	var index, last int
	if last = list.Len() - 1; last < 0 {
		return -1
	}
	list.Swap(pivot, last)
	for i := 0; i < last; i++ {
		if !list.Less(last, i) {
			list.Swap(index, i)
			index++
		}
	}
	list.Swap(last, index)
	return index
}

// SortSlicer satisfies the sort.Interface and is able to slice itself.
type SortSlicer interface {
	sort.Interface
	Slice(start, end int) SortSlicer
}

// Select partitions list such that all elements less than the kth element
// are placed before k in the resulting list and all elements greater than
// it are placed after the position k.
func Select(list SortSlicer, k int) int {
	var (
		start int
		end   = list.Len()
	)
	if k >= end {
		if k == 0 {
			return 0
		}
		panic("kdtree: index out of range")
	}
	if start == end-1 {
		return k
	}

	for {
		if start == end {
			panic("kdtree: internal inconsistency")
		}
		sub := list.Slice(start, end)
		pivot := Partition(sub, rand.Intn(sub.Len()))
		switch {
		case pivot == k:
			return k
		case k < pivot:
			end = pivot + start
		default:
			k -= pivot
			start += pivot
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// MedianOfMedians returns the index to the median value of the medians
// of groups of 5 consecutive elements.
func MedianOfMedians(list SortSlicer) int {
	n := list.Len() / 5
	for i := 0; i < n; i++ {
		left := i * 5
		sub := list.Slice(left, min(left+5, list.Len()-1))
		Select(sub, 2)
		list.Swap(i, left+2)
	}
	Select(list.Slice(0, min(n, list.Len()-1)), min(list.Len(), n/2))
	return n / 2
}

// MedianOfRandoms returns the index to the median value of up to n randomly
// chosen elements in list.
func MedianOfRandoms(list SortSlicer, n int) int {
	if l := list.Len(); l < n {
		n = l
	} else {
		rand.Shuffle(n, func(i, j int) { list.Swap(i, j) })
	}
	Select(list.Slice(0, n), n/2)
	return n / 2
}
