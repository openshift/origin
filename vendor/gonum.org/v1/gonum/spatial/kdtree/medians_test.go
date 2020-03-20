// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

import (
	"sort"
	"testing"

	"golang.org/x/exp/rand"
)

type ints []int

func (a ints) Len() int                  { return len(a) }
func (a ints) Less(i, j int) bool        { return a[i] < a[j] }
func (a ints) Slice(s, e int) SortSlicer { return a[s:e] }
func (a ints) Swap(i, j int)             { a[i], a[j] = a[j], a[i] }

func TestPartition(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	for p := 0; p < 100; p++ {
		list := make(ints, 1e5)
		for i := range list {
			list[i] = rnd.Int()
		}
		pi := Partition(list, rnd.Intn(list.Len()))
		for i := 0; i < pi; i++ {
			if list[i] > list[pi] {
				t.Errorf("unexpected partition sort order p[%d] > p[%d]: %d > %d", i, pi, list[i], list[pi])
			}
		}
		for i := pi + 1; i < len(list); i++ {
			if list[i] <= list[pi] {
				t.Errorf("unexpected partition sort order p[%d] <= p[%d]: %d <= %d", i, pi, list[i], list[pi])
			}
		}
	}
}

func TestPartitionCollision(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	for p := 0; p < 10; p++ {
		list := make(ints, 10)
		for i := range list {
			list[i] = rnd.Intn(5)
		}
		pi := Partition(list, p)
		for i := 0; i < pi; i++ {
			if list[i] > list[pi] {
				t.Errorf("unexpected partition sort order p[%d] > p[%d]: %d > %d", i, pi, list[i], list[pi])
			}
		}
		for i := pi + 1; i < len(list); i++ {
			if list[i] <= list[pi] {
				t.Errorf("unexpected partition sort order p[%d] <= p[%d]: %d <= %d", i, pi, list[i], list[pi])
			}
		}
	}
}

func sortSelection(list ints, k int) int {
	sort.Sort(list)
	return list[k]
}

func TestSelect(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	for k := 0; k < 2121; k++ {
		list := make(ints, 2121)
		for i := range list {
			list[i] = rnd.Intn(1000)
		}
		Select(list, k)
		sorted := append(ints(nil), list...)
		want := sortSelection(sorted, k)
		if list[k] != want {
			t.Errorf("unexpected result from Select(..., %d): got:%v want:%d", k, list[k], want)
		}
	}
}

func TestMedianOfMedians(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	list := make(ints, 1e4)
	for i := range list {
		list[i] = rnd.Int()
	}
	p := MedianOfMedians(list)
	med := list[p]
	sort.Sort(list)
	var found bool
	for _, v := range list[len(list)*3/10 : len(list)*7/10+1] {
		if v == med {
			found = true
			break
		}
	}
	if !found {
		t.Error("failed to find median")
	}
}

func TestMedianOfRandoms(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))

	list := make(ints, 1e4)
	for i := range list {
		list[i] = rnd.Int()
	}
	p := MedianOfRandoms(list, randoms)
	med := list[p]
	sort.Sort(list)
	var found bool
	for _, v := range list[len(list)*3/10 : len(list)*7/10+1] {
		if v == med {
			found = true
			break
		}
	}
	if !found {
		t.Error("failed to find median")
	}
}

var benchSink int

func BenchmarkMedianOfMedians(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		list := make(ints, 1e4)
		for i := range list {
			list[i] = rnd.Int()
		}
		b.StartTimer()
		benchSink = MedianOfMedians(list)
	}
}

func BenchmarkPartitionMedianOfMedians(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		list := make(ints, 1e4)
		for i := range list {
			list[i] = rnd.Int()
		}
		b.StartTimer()
		benchSink = Partition(list, MedianOfMedians(list))
	}
}

func BenchmarkMedianOfRandoms(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))

	b.StopTimer()
	list := make(ints, 1e4)
	for i := range list {
		list[i] = rnd.Int()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		benchSink = MedianOfRandoms(list, list.Len()/1e3)
	}
}

func BenchmarkPartitionMedianOfRandoms(b *testing.B) {
	rnd := rand.New(rand.NewSource(1))

	b.StopTimer()
	list := make(ints, 1e4)
	for i := range list {
		list[i] = rnd.Int()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		benchSink = Partition(list, MedianOfRandoms(list, list.Len()/1e3))
	}
}
