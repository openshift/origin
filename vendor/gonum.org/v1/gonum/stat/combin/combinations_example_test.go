// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package combin_test

import (
	"fmt"

	"gonum.org/v1/gonum/stat/combin"
)

func ExampleCartesian() {
	fmt.Println("Generate Cartesian products for given lengths:")
	lens := []int{1, 2, 3}
	list := combin.Cartesian(lens)
	for i, v := range list {
		fmt.Println(i, v)
	}
	// This is easy, but the number of combinations  can be very large,
	// and generating all at once can use a lot of memory.
	// For big data sets, consider using CartesianGenerator instead.

	// Output:
	// Generate Cartesian products for given lengths:
	// 0 [0 0 0]
	// 1 [0 0 1]
	// 2 [0 0 2]
	// 3 [0 1 0]
	// 4 [0 1 1]
	// 5 [0 1 2]
}

func ExampleCartesianGenerator() {
	fmt.Println("Generate products for given lengths:")
	lens := []int{1, 2, 3}
	gen := combin.NewCartesianGenerator(lens)

	// Now loop over all products.
	var i int
	for gen.Next() {
		fmt.Println(i, gen.Product(nil))
		i++
	}

	// Output:
	// Generate products for given lengths:
	// 0 [0 0 0]
	// 1 [0 0 1]
	// 2 [0 0 2]
	// 3 [0 1 0]
	// 4 [0 1 1]
	// 5 [0 1 2]
}

func ExampleCombinations() {
	// combin provides several ways to work with the combinations of
	// different objects. Combinations generates them directly.
	fmt.Println("Generate list:")
	n := 5
	k := 3
	list := combin.Combinations(n, k)
	for i, v := range list {
		fmt.Println(i, v)
	}
	// This is easy, but the number of combinations  can be very large,
	// and generating all at once can use a lot of memory.

	// Output:
	// Generate list:
	// 0 [0 1 2]
	// 1 [0 1 3]
	// 2 [0 1 4]
	// 3 [0 2 3]
	// 4 [0 2 4]
	// 5 [0 3 4]
	// 6 [1 2 3]
	// 7 [1 2 4]
	// 8 [1 3 4]
	// 9 [2 3 4]
}

func ExampleCombinations_index() {
	// The integer slices returned from Combinations can be used to index
	// into a data structure.
	data := []string{"a", "b", "c", "d", "e"}
	cs := combin.Combinations(len(data), 2)
	for _, c := range cs {
		fmt.Printf("%s%s\n", data[c[0]], data[c[1]])
	}

	// Output:
	// ab
	// ac
	// ad
	// ae
	// bc
	// bd
	// be
	// cd
	// ce
	// de
}

func ExampleCombinationGenerator() {
	// combin provides several ways to work with the combinations of
	// different objects. CombinationGenerator constructs an iterator
	// for the combinations.
	n := 5
	k := 3
	gen := combin.NewCombinationGenerator(n, k)
	idx := 0
	for gen.Next() {
		fmt.Println(idx, gen.Combination(nil)) // can also store in-place.
		idx++
	}
	// Output:
	// 0 [0 1 2]
	// 1 [0 1 3]
	// 2 [0 1 4]
	// 3 [0 2 3]
	// 4 [0 2 4]
	// 5 [0 3 4]
	// 6 [1 2 3]
	// 7 [1 2 4]
	// 8 [1 3 4]
	// 9 [2 3 4]
}

func ExampleIndexToCombination() {
	// combin provides several ways to work with the combinations of
	// different objects. IndexToCombination allows random access into
	// the combination order. Combined with CombinationIndex this
	// provides a correspondence between integers and combinations.
	n := 5
	k := 3
	comb := make([]int, k)
	for i := 0; i < combin.Binomial(n, k); i++ {
		combin.IndexToCombination(comb, i, n, k) // can also use nil.
		idx := combin.CombinationIndex(comb, n, k)
		fmt.Println(i, comb, idx)
	}

	// Output:
	// 0 [0 1 2] 0
	// 1 [0 1 3] 1
	// 2 [0 1 4] 2
	// 3 [0 2 3] 3
	// 4 [0 2 4] 4
	// 5 [0 3 4] 5
	// 6 [1 2 3] 6
	// 7 [1 2 4] 7
	// 8 [1 3 4] 8
	// 9 [2 3 4] 9
}

func ExamplePermutations() {
	// combin provides several ways to work with the permutations of
	// different objects. Permutations generates them directly.
	fmt.Println("Generate list:")
	n := 4
	k := 3
	list := combin.Permutations(n, k)
	for i, v := range list {
		fmt.Println(i, v)
	}
	// This is easy, but the number of permutations can be very large,
	// and generating all at once can use a lot of memory.

	// Output:
	// Generate list:
	// 0 [0 1 2]
	// 1 [0 2 1]
	// 2 [1 0 2]
	// 3 [1 2 0]
	// 4 [2 0 1]
	// 5 [2 1 0]
	// 6 [0 1 3]
	// 7 [0 3 1]
	// 8 [1 0 3]
	// 9 [1 3 0]
	// 10 [3 0 1]
	// 11 [3 1 0]
	// 12 [0 2 3]
	// 13 [0 3 2]
	// 14 [2 0 3]
	// 15 [2 3 0]
	// 16 [3 0 2]
	// 17 [3 2 0]
	// 18 [1 2 3]
	// 19 [1 3 2]
	// 20 [2 1 3]
	// 21 [2 3 1]
	// 22 [3 1 2]
	// 23 [3 2 1]
}

func ExamplePermutations_index() {
	// The integer slices returned from Permutations can be used to index
	// into a data structure.
	data := []string{"a", "b", "c", "d"}
	cs := combin.Permutations(len(data), 2)
	for _, c := range cs {
		fmt.Printf("%s%s\n", data[c[0]], data[c[1]])
	}

	// Output:
	// ab
	// ba
	// ac
	// ca
	// ad
	// da
	// bc
	// cb
	// bd
	// db
	// cd
	// dc
}

func ExamplePermutationGenerator() {
	// combin provides several ways to work with the permutations of
	// different objects. PermutationGenerator constructs an iterator
	// for the permutations.
	n := 4
	k := 3
	gen := combin.NewPermutationGenerator(n, k)
	idx := 0
	for gen.Next() {
		fmt.Println(idx, gen.Permutation(nil)) // can also store in-place.
		idx++
	}

	// Output:
	// 0 [0 1 2]
	// 1 [0 2 1]
	// 2 [1 0 2]
	// 3 [1 2 0]
	// 4 [2 0 1]
	// 5 [2 1 0]
	// 6 [0 1 3]
	// 7 [0 3 1]
	// 8 [1 0 3]
	// 9 [1 3 0]
	// 10 [3 0 1]
	// 11 [3 1 0]
	// 12 [0 2 3]
	// 13 [0 3 2]
	// 14 [2 0 3]
	// 15 [2 3 0]
	// 16 [3 0 2]
	// 17 [3 2 0]
	// 18 [1 2 3]
	// 19 [1 3 2]
	// 20 [2 1 3]
	// 21 [2 3 1]
	// 22 [3 1 2]
	// 23 [3 2 1]
}

func ExampleIndexToPermutation() {
	// combin provides several ways to work with the permutations of
	// different objects. IndexToPermutation allows random access into
	// the permutation order. Combined with PermutationIndex this
	// provides a correspondence between integers and permutations.
	n := 4
	k := 3
	comb := make([]int, k)
	for i := 0; i < combin.NumPermutations(n, k); i++ {
		combin.IndexToPermutation(comb, i, n, k) // can also use nil.
		idx := combin.PermutationIndex(comb, n, k)
		fmt.Println(i, comb, idx)
	}

	// Output:
	// 0 [0 1 2] 0
	// 1 [0 2 1] 1
	// 2 [1 0 2] 2
	// 3 [1 2 0] 3
	// 4 [2 0 1] 4
	// 5 [2 1 0] 5
	// 6 [0 1 3] 6
	// 7 [0 3 1] 7
	// 8 [1 0 3] 8
	// 9 [1 3 0] 9
	// 10 [3 0 1] 10
	// 11 [3 1 0] 11
	// 12 [0 2 3] 12
	// 13 [0 3 2] 13
	// 14 [2 0 3] 14
	// 15 [2 3 0] 15
	// 16 [3 0 2] 16
	// 17 [3 2 0] 17
	// 18 [1 2 3] 18
	// 19 [1 3 2] 19
	// 20 [2 1 3] 20
	// 21 [2 3 1] 21
	// 22 [3 1 2] 22
	// 23 [3 2 1] 23
}
