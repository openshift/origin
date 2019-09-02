// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stat_test

import (
	"fmt"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/stat"
)

func ExampleLinearRegression() {
	var (
		xs      = make([]float64, 100)
		ys      = make([]float64, 100)
		weights []float64
	)

	line := func(x float64) float64 {
		return 1 + 3*x
	}

	for i := range xs {
		xs[i] = float64(i)
		ys[i] = line(xs[i]) + 0.1*rand.NormFloat64()
	}

	// Do not force the regression line to pass through the origin.
	origin := false

	alpha, beta := stat.LinearRegression(xs, ys, weights, origin)
	r2 := stat.RSquared(xs, ys, weights, alpha, beta)

	fmt.Printf("Estimated slope is:  %v\n", alpha)
	fmt.Printf("Estimated offset is: %v\n", beta)
	fmt.Printf("R^2: %v\n", r2)

	// Output:
	// Estimated slope is:  0.988572424633503
	// Estimated offset is: 3.0001541344029676
	// R^2: 0.9999991095061128
}
