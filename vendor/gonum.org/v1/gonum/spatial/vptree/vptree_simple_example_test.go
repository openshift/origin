// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vptree_test

import (
	"fmt"
	"log"

	"gonum.org/v1/gonum/spatial/vptree"
)

func ExampleTree() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := []vptree.Comparable{
		vptree.Point{2, 3},
		vptree.Point{5, 4},
		vptree.Point{9, 6},
		vptree.Point{4, 7},
		vptree.Point{8, 1},
		vptree.Point{7, 2},
	}

	t, err := vptree.New(points, 3, nil)
	if err != nil {
		log.Fatal(err)
	}
	q := vptree.Point{8, 7}
	p, d := t.Nearest(q)
	fmt.Printf("%v is closest point to %v, d=%f\n", p, q, d)
	// Output:
	// [9 6] is closest point to [8 7], d=1.414214
}

func ExampleTree_Do() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := []vptree.Comparable{
		vptree.Point{2, 3},
		vptree.Point{5, 4},
		vptree.Point{9, 6},
		vptree.Point{4, 7},
		vptree.Point{8, 1},
		vptree.Point{7, 2},
	}

	// Print all points in the data set within 3 of (3, 5).
	t, err := vptree.New(points, 0, nil)
	if err != nil {
		log.Fatal(err)
	}
	q := vptree.Point{3, 5}
	t.Do(func(c vptree.Comparable, _ int) (done bool) {
		// Compare each distance and output points
		// with a Euclidean distance less than or
		// equal to 3. Distance returns the
		// Euclidean distance between points.
		if q.Distance(c) <= 3 {
			fmt.Println(c)
		}
		return
	})
	// Unordered output:
	// [2 3]
	// [4 7]
	// [5 4]
}
