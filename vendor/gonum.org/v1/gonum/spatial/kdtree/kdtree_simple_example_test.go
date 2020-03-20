// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree_test

import (
	"fmt"
	"math"

	"gonum.org/v1/gonum/spatial/kdtree"
)

func ExampleTree() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := kdtree.Points{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}

	t := kdtree.New(points, false)
	q := kdtree.Point{8, 7}
	p, d := t.Nearest(q)
	fmt.Printf("%v is closest point to %v, d=%f\n", p, q, math.Sqrt(d))
	// Output:
	// [9 6] is closest point to [8 7], d=1.414214
}

func ExampleTree_bounds() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := kdtree.Points{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}

	t := kdtree.New(points, true)
	fmt.Printf("Bounding box of points is %+v\n", t.Root.Bounding)
	// Output:
	// Bounding box of points is &{Min:[2 1] Max:[9 7]}
}

func ExampleTree_Do() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := kdtree.Points{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}

	// Print all points in the data set within 3 of (3, 5).
	t := kdtree.New(points, false)
	q := kdtree.Point{3, 5}
	t.Do(func(c kdtree.Comparable, _ *kdtree.Bounding, _ int) (done bool) {
		// Compare each distance and output points
		// with a Euclidean distance less than 3.
		// Distance returns the square of the
		// Euclidean distance between points.
		if q.Distance(c) <= 3*3 {
			fmt.Println(c)
		}
		return
	})
	// Unordered output:
	// [2 3]
	// [4 7]
	// [5 4]
}

func ExampleTree_DoBounded() {
	// Example data from https://en.wikipedia.org/wiki/K-d_tree
	points := kdtree.Points{{2, 3}, {5, 4}, {9, 6}, {4, 7}, {8, 1}, {7, 2}}

	// Find all points within the bounding box ((3, 3), (6, 8))
	// and print them with their bounding boxes and tree depth.
	t := kdtree.New(points, true) // Construct tree with bounding boxes.
	b := &kdtree.Bounding{
		Min: kdtree.Point{3, 3},
		Max: kdtree.Point{6, 8},
	}
	t.DoBounded(b, func(c kdtree.Comparable, bound *kdtree.Bounding, depth int) (done bool) {
		fmt.Printf("p=%v bound=%+v depth=%d\n", c, bound, depth)
		return
	})
	// Output:
	// p=[5 4] bound=&{Min:[2 3] Max:[5 7]} depth=1
	// p=[4 7] bound=&{Min:[4 7] Max:[4 7]} depth=2
}
