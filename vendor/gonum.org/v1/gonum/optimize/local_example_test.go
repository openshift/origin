// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize_test

import (
	"fmt"
	"log"

	"gonum.org/v1/gonum/optimize"
	"gonum.org/v1/gonum/optimize/functions"
)

func ExampleMinimize() {
	p := optimize.Problem{
		Func: functions.ExtendedRosenbrock{}.Func,
		Grad: functions.ExtendedRosenbrock{}.Grad,
	}

	x := []float64{1.3, 0.7, 0.8, 1.9, 1.2}
	result, err := optimize.Minimize(p, x, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	if err = result.Status.Err(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("result.Status: %v\n", result.Status)
	fmt.Printf("result.X: %0.4g\n", result.X)
	fmt.Printf("result.F: %0.4g\n", result.F)
	fmt.Printf("result.Stats.FuncEvaluations: %d\n", result.Stats.FuncEvaluations)
	// Output:
	// result.Status: GradientThreshold
	// result.X: [1 1 1 1 1]
	// result.F: 4.98e-30
	// result.Stats.FuncEvaluations: 31
}
