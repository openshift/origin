// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"fmt"
	"math"
	"testing"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/functions"
)

type unconstrainedTest struct {
	// name is the name of the test.
	name string
	// p is the optimization problem to be solved.
	p Problem
	// x is the initial guess.
	x []float64
	// gradTol is the absolute gradient tolerance for the test. If gradTol == 0,
	// the default value of 1e-12 will be used.
	gradTol float64
	// fAbsTol is the absolute function convergence for the test. If fAbsTol == 0,
	// the default value of 1e-12 will be used.
	fAbsTol float64
	// fIter is the number of iterations for function convergence. If fIter == 0,
	// the default value of 20 will be used.
	fIter int
	// long indicates that the test takes long time to finish and will be
	// excluded if testing.Short returns true.
	long bool
}

func (t unconstrainedTest) String() string {
	dim := len(t.x)
	if dim <= 10 {
		// Print the initial X only for small-dimensional problems.
		return fmt.Sprintf("F: %v\nDim: %v\nInitial X: %v\nGradientThreshold: %v",
			t.name, dim, t.x, t.gradTol)
	}
	return fmt.Sprintf("F: %v\nDim: %v\nGradientThreshold: %v",
		t.name, dim, t.gradTol)
}

var gradFreeTests = []unconstrainedTest{
	{
		name: "Beale",
		p: Problem{
			Func: functions.Beale{}.Func,
		},
		x: []float64{1, 1},
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
		},
		x: []float64{1, 2, 1, 1, 1, 1},
	},
	{
		name: "BrownAndDennis",
		p: Problem{
			Func: functions.BrownAndDennis{}.Func,
		},
		x: []float64{25, 5, -5, -1},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
		},
		x: []float64{-10, 10},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
		},
		x: []float64{-5, 4, 16, 3},
	},
}

var gradientDescentTests = []unconstrainedTest{
	{
		name: "Beale",
		p: Problem{
			Func: functions.Beale{}.Func,
			Grad: functions.Beale{}.Grad,
		},
		x: []float64{1, 1},
	},
	{
		name: "Beale",
		p: Problem{
			Func: functions.Beale{}.Func,
			Grad: functions.Beale{}.Grad,
		},
		x: []float64{3.00001, 0.50001},
	},
	{
		name: "BiggsEXP2",
		p: Problem{
			Func: functions.BiggsEXP2{}.Func,
			Grad: functions.BiggsEXP2{}.Grad,
		},
		x: []float64{1, 2},
	},
	{
		name: "BiggsEXP2",
		p: Problem{
			Func: functions.BiggsEXP2{}.Func,
			Grad: functions.BiggsEXP2{}.Grad,
		},
		x: []float64{1.00001, 10.00001},
	},
	{
		name: "BiggsEXP3",
		p: Problem{
			Func: functions.BiggsEXP3{}.Func,
			Grad: functions.BiggsEXP3{}.Grad,
		},
		x: []float64{1, 2, 1},
	},
	{
		name: "BiggsEXP3",
		p: Problem{
			Func: functions.BiggsEXP3{}.Func,
			Grad: functions.BiggsEXP3{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 3.00001},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{-1.2, 1},
		gradTol: 1e-10,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1.00001, 1.00001},
		gradTol: 1e-10,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{-1.2, 1, -1.2},
		gradTol: 1e-10,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:    []float64{-120, 100, 50},
		long: true,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x: []float64{1, 1, 1},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1.00001, 1.00001, 1.00001},
		gradTol: 1e-8,
	},
	{
		name: "Gaussian",
		p: Problem{
			Func: functions.Gaussian{}.Func,
			Grad: functions.Gaussian{}.Grad,
		},
		x:       []float64{0.4, 1, 0},
		gradTol: 1e-9,
	},
	{
		name: "Gaussian",
		p: Problem{
			Func: functions.Gaussian{}.Func,
			Grad: functions.Gaussian{}.Grad,
		},
		x:       []float64{0.3989561, 1.0000191, 0},
		gradTol: 1e-9,
	},
	{
		name: "HelicalValley",
		p: Problem{
			Func: functions.HelicalValley{}.Func,
			Grad: functions.HelicalValley{}.Grad,
		},
		x: []float64{-1, 0, 0},
	},
	{
		name: "HelicalValley",
		p: Problem{
			Func: functions.HelicalValley{}.Func,
			Grad: functions.HelicalValley{}.Grad,
		},
		x: []float64{1.00001, 0.00001, 0.00001},
	},
	{
		name: "Trigonometric",
		p: Problem{
			Func: functions.Trigonometric{}.Func,
			Grad: functions.Trigonometric{}.Grad,
		},
		x:       []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		gradTol: 1e-7,
	},
	{
		name: "Trigonometric",
		p: Problem{
			Func: functions.Trigonometric{}.Func,
			Grad: functions.Trigonometric{}.Grad,
		},
		x: []float64{0.042964, 0.043976, 0.045093, 0.046338, 0.047744,
			0.049354, 0.051237, 0.195209, 0.164977, 0.060148},
		gradTol: 1e-8,
	},
	newVariablyDimensioned(2, 0),
	{
		name: "VariablyDimensioned",
		p: Problem{
			Func: functions.VariablyDimensioned{}.Func,
			Grad: functions.VariablyDimensioned{}.Grad,
		},
		x: []float64{1.00001, 1.00001},
	},
	newVariablyDimensioned(10, 0),
	{
		name: "VariablyDimensioned",
		p: Problem{
			Func: functions.VariablyDimensioned{}.Func,
			Grad: functions.VariablyDimensioned{}.Grad,
		},
		x: []float64{1.00001, 1.00001, 1.00001, 1.00001, 1.00001, 1.00001, 1.00001, 1.00001, 1.00001, 1.00001},
	},
}

var cgTests = []unconstrainedTest{
	{
		name: "BiggsEXP4",
		p: Problem{
			Func: functions.BiggsEXP4{}.Func,
			Grad: functions.BiggsEXP4{}.Grad,
		},
		x: []float64{1, 2, 1, 1},
	},
	{
		name: "BiggsEXP4",
		p: Problem{
			Func: functions.BiggsEXP4{}.Func,
			Grad: functions.BiggsEXP4{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001, 5.00001},
	},
	{
		name: "BiggsEXP5",
		p: Problem{
			Func: functions.BiggsEXP5{}.Func,
			Grad: functions.BiggsEXP5{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1},
		gradTol: 1e-7,
	},
	{
		name: "BiggsEXP5",
		p: Problem{
			Func: functions.BiggsEXP5{}.Func,
			Grad: functions.BiggsEXP5{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001},
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1, 1},
		gradTol: 1e-7,
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001, 3.00001},
		gradTol: 1e-8,
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{0, 10, 20},
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001},
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{100.00001, 100.00001, 0.00001},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{3, -1, 0, 3},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{0.00001, 0.00001, 0.00001, 0.00001},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x:       []float64{3, -1, 0, 3, 3, -1, 0, 3},
		gradTol: 1e-8,
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x: []float64{-1.2, 1, -1.2, 1},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1e4, 1e4},
		gradTol: 1e-10,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1.00001, 1.00001, 1.00001, 1.00001},
		gradTol: 1e-10,
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x:       []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		gradTol: 1e-9,
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x:       []float64{0.250007, 0.250007, 0.250007, 0.250007},
		gradTol: 1e-10,
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x: []float64{0.1581, 0.1581, 0.1581, 0.1581, 0.1581, 0.1581,
			0.1581, 0.1581, 0.1581, 0.1581},
		gradTol: 1e-10,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x:       []float64{0.5, 0.5, 0.5, 0.5},
		gradTol: 1e-8,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x:       []float64{0.19999, 0.19131, 0.4801, 0.51884},
		gradTol: 1e-8,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x: []float64{0.19998, 0.01035, 0.01960, 0.03208, 0.04993, 0.07651,
			0.11862, 0.19214, 0.34732, 0.36916},
		gradTol: 1e-6,
	},
	{
		name: "PowellBadlyScaled",
		p: Problem{
			Func: functions.PowellBadlyScaled{}.Func,
			Grad: functions.PowellBadlyScaled{}.Grad,
		},
		x:       []float64{1.09815e-05, 9.10614},
		gradTol: 1e-8,
	},
	newVariablyDimensioned(100, 1e-10),
	newVariablyDimensioned(1000, 1e-7),
	newVariablyDimensioned(10000, 1e-4),
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{0, 0, 0, 0, 0, 0},
		gradTol: 1e-6,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{-0.01572, 1.01243, -0.23299, 1.26043, -1.51372, 0.99299},
		gradTol: 1e-6,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		gradTol: 1e-6,
		long:    true,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x: []float64{-1.53070e-05, 0.99978, 0.01476, 0.14634, 1.00082,
			-2.61773, 4.10440, -3.14361, 1.05262},
		gradTol: 1e-6,
	},
	{
		name: "Wood",
		p: Problem{
			Func: functions.Wood{}.Func,
			Grad: functions.Wood{}.Grad,
		},
		x:       []float64{-3, -1, -3, -1},
		gradTol: 1e-6,
	},
}

var quasiNewtonTests = []unconstrainedTest{
	{
		name: "BiggsEXP4",
		p: Problem{
			Func: functions.BiggsEXP4{}.Func,
			Grad: functions.BiggsEXP4{}.Grad,
		},
		x: []float64{1, 2, 1, 1},
	},
	{
		name: "BiggsEXP4",
		p: Problem{
			Func: functions.BiggsEXP4{}.Func,
			Grad: functions.BiggsEXP4{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001, 5.00001},
	},
	{
		name: "BiggsEXP5",
		p: Problem{
			Func: functions.BiggsEXP5{}.Func,
			Grad: functions.BiggsEXP5{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1},
		gradTol: 1e-10,
	},
	{
		name: "BiggsEXP5",
		p: Problem{
			Func: functions.BiggsEXP5{}.Func,
			Grad: functions.BiggsEXP5{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001},
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1, 1},
		gradTol: 1e-8,
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001, 3.00001},
		gradTol: 1e-8,
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{0, 10, 20},
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{1.00001, 10.00001, 1.00001},
	},
	{
		name: "Box3D",
		p: Problem{
			Func: functions.Box3D{}.Func,
			Grad: functions.Box3D{}.Grad,
		},
		x: []float64{100.00001, 100.00001, 0.00001},
	},
	{
		name: "BrownBadlyScaled",
		p: Problem{
			Func: functions.BrownBadlyScaled{}.Func,
			Grad: functions.BrownBadlyScaled{}.Grad,
		},
		x: []float64{1, 1},
	},
	{
		name: "BrownBadlyScaled",
		p: Problem{
			Func: functions.BrownBadlyScaled{}.Func,
			Grad: functions.BrownBadlyScaled{}.Grad,
		},
		x: []float64{1.000001e6, 2.01e-6},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{3, -1, 0, 3},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{0.00001, 0.00001, 0.00001, 0.00001},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{3, -1, 0, 3, 3, -1, 0, 3},
	},
	{
		name: "ExtendedPowellSingular",
		p: Problem{
			Func: functions.ExtendedPowellSingular{}.Func,
			Grad: functions.ExtendedPowellSingular{}.Grad,
		},
		x: []float64{0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x: []float64{-1.2, 1, -1.2, 1},
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x: []float64{1.00001, 1.00001, 1.00001, 1.00001},
	},
	{
		name: "Gaussian",
		p: Problem{
			Func: functions.Gaussian{}.Func,
			Grad: functions.Gaussian{}.Grad,
		},
		x:       []float64{0.4, 1, 0},
		gradTol: 1e-11,
	},
	{
		name: "GulfResearchAndDevelopment",
		p: Problem{
			Func: functions.GulfResearchAndDevelopment{}.Func,
			Grad: functions.GulfResearchAndDevelopment{}.Grad,
		},
		x: []float64{5, 2.5, 0.15},
	},
	{
		name: "GulfResearchAndDevelopment",
		p: Problem{
			Func: functions.GulfResearchAndDevelopment{}.Func,
			Grad: functions.GulfResearchAndDevelopment{}.Grad,
		},
		x: []float64{50.00001, 25.00001, 1.50001},
	},
	{
		name: "GulfResearchAndDevelopment",
		p: Problem{
			Func: functions.GulfResearchAndDevelopment{}.Func,
			Grad: functions.GulfResearchAndDevelopment{}.Grad,
		},
		x: []float64{99.89529, 60.61453, 9.16124},
	},
	{
		name: "GulfResearchAndDevelopment",
		p: Problem{
			Func: functions.GulfResearchAndDevelopment{}.Func,
			Grad: functions.GulfResearchAndDevelopment{}.Grad,
		},
		x: []float64{201.66258, 60.61633, 10.22489},
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x:       []float64{0.250007, 0.250007, 0.250007, 0.250007},
		gradTol: 1e-9,
	},
	{
		name: "PenaltyI",
		p: Problem{
			Func: functions.PenaltyI{}.Func,
			Grad: functions.PenaltyI{}.Grad,
		},
		x: []float64{0.1581, 0.1581, 0.1581, 0.1581, 0.1581, 0.1581,
			0.1581, 0.1581, 0.1581, 0.1581},
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x:       []float64{0.5, 0.5, 0.5, 0.5},
		gradTol: 1e-10,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x:       []float64{0.19999, 0.19131, 0.4801, 0.51884},
		gradTol: 1e-10,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x:       []float64{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5},
		gradTol: 1e-9,
	},
	{
		name: "PenaltyII",
		p: Problem{
			Func: functions.PenaltyII{}.Func,
			Grad: functions.PenaltyII{}.Grad,
		},
		x: []float64{0.19998, 0.01035, 0.01960, 0.03208, 0.04993, 0.07651,
			0.11862, 0.19214, 0.34732, 0.36916},
		gradTol: 1e-9,
	},
	{
		name: "PowellBadlyScaled",
		p: Problem{
			Func: functions.PowellBadlyScaled{}.Func,
			Grad: functions.PowellBadlyScaled{}.Grad,
		},
		x: []float64{0, 1},
	},
	{
		name: "PowellBadlyScaled",
		p: Problem{
			Func: functions.PowellBadlyScaled{}.Func,
			Grad: functions.PowellBadlyScaled{}.Grad,
		},
		x:       []float64{1.09815e-05, 9.10614},
		gradTol: 1e-10,
	},
	newVariablyDimensioned(100, 1e-10),
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{0, 0, 0, 0, 0, 0},
		gradTol: 1e-7,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{-0.01572, 1.01243, -0.23299, 1.26043, -1.51372, 0.99299},
		gradTol: 1e-7,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x:       []float64{0, 0, 0, 0, 0, 0, 0, 0, 0},
		gradTol: 1e-8,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
		},
		x: []float64{-1.53070e-05, 0.99978, 0.01476, 0.14634, 1.00082,
			-2.61773, 4.10440, -3.14361, 1.05262},
		gradTol: 1e-8,
	},
}

var bfgsTests = []unconstrainedTest{
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1, 1},
		gradTol: 1e-10,
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001, 3.00001},
		gradTol: 1e-10,
	},
	{
		name: "BrownAndDennis",
		p: Problem{
			Func: functions.BrownAndDennis{}.Func,
			Grad: functions.BrownAndDennis{}.Grad,
		},
		x:       []float64{25, 5, -5, -1},
		gradTol: 1e-5,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1e5, 1e5},
		gradTol: 1e-10,
	},
	{
		name: "Gaussian",
		p: Problem{
			Func: functions.Gaussian{}.Func,
			Grad: functions.Gaussian{}.Grad,
		},
		x:       []float64{0.398, 1, 0},
		gradTol: 1e-11,
	},
	{
		name: "Wood",
		p: Problem{
			Func: functions.Wood{}.Func,
			Grad: functions.Wood{}.Grad,
		},
		x: []float64{-3, -1, -3, -1},
	},
}

var lbfgsTests = []unconstrainedTest{
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1, 2, 1, 1, 1, 1},
		gradTol: 1e-8,
	},
	{
		name: "BiggsEXP6",
		p: Problem{
			Func: functions.BiggsEXP6{}.Func,
			Grad: functions.BiggsEXP6{}.Grad,
		},
		x:       []float64{1.00001, 10.00001, 1.00001, 5.00001, 4.00001, 3.00001},
		gradTol: 1e-8,
	},
	{
		name: "ExtendedRosenbrock",
		p: Problem{
			Func: functions.ExtendedRosenbrock{}.Func,
			Grad: functions.ExtendedRosenbrock{}.Grad,
		},
		x:       []float64{1e7, 1e6},
		gradTol: 1e-10,
	},
	{
		name: "Gaussian",
		p: Problem{
			Func: functions.Gaussian{}.Func,
			Grad: functions.Gaussian{}.Grad,
		},
		x:       []float64{0.398, 1, 0},
		gradTol: 1e-10,
	},
	newVariablyDimensioned(1000, 1e-8),
	newVariablyDimensioned(10000, 1e-5),
}

var newtonTests = []unconstrainedTest{
	{
		name: "Beale",
		p: Problem{
			Func: functions.Beale{}.Func,
			Grad: functions.Beale{}.Grad,
			Hess: functions.Beale{}.Hess,
		},
		x: []float64{1, 1},
	},
	{
		name: "BrownAndDennis",
		p: Problem{
			Func: functions.BrownAndDennis{}.Func,
			Grad: functions.BrownAndDennis{}.Grad,
			Hess: functions.BrownAndDennis{}.Hess,
		},
		x:       []float64{25, 5, -5, -1},
		gradTol: 1e-10,
	},
	{
		name: "BrownBadlyScaled",
		p: Problem{
			Func: functions.BrownBadlyScaled{}.Func,
			Grad: functions.BrownBadlyScaled{}.Grad,
			Hess: functions.BrownBadlyScaled{}.Hess,
		},
		x: []float64{1, 1},
	},
	{
		name: "PowellBadlyScaled",
		p: Problem{
			Func: functions.PowellBadlyScaled{}.Func,
			Grad: functions.PowellBadlyScaled{}.Grad,
			Hess: functions.PowellBadlyScaled{}.Hess,
		},
		x:       []float64{0, 1},
		gradTol: 1e-10,
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
			Hess: functions.Watson{}.Hess,
		},
		x: []float64{0, 0, 0, 0, 0, 0},
	},
	{
		name: "Watson",
		p: Problem{
			Func: functions.Watson{}.Func,
			Grad: functions.Watson{}.Grad,
			Hess: functions.Watson{}.Hess,
		},
		x: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	},
	{
		name: "Wood",
		p: Problem{
			Func: functions.Wood{}.Func,
			Grad: functions.Wood{}.Grad,
			Hess: functions.Wood{}.Hess,
		},
		x: []float64{-3, -1, -3, -1},
	},
}

func newVariablyDimensioned(dim int, gradTol float64) unconstrainedTest {
	x := make([]float64, dim)
	for i := range x {
		x[i] = float64(dim-i-1) / float64(dim)
	}
	return unconstrainedTest{
		name: "VariablyDimensioned",
		p: Problem{
			Func: functions.VariablyDimensioned{}.Func,
			Grad: functions.VariablyDimensioned{}.Grad,
		},
		x:       x,
		gradTol: gradTol,
	}
}

func TestLocal(t *testing.T) {
	var tests []unconstrainedTest
	// Mix of functions with and without Grad method.
	tests = append(tests, gradFreeTests...)
	tests = append(tests, gradientDescentTests...)
	testLocal(t, tests, nil)
}

func TestNelderMead(t *testing.T) {
	var tests []unconstrainedTest
	// Mix of functions with and without Grad method.
	tests = append(tests, gradFreeTests...)
	tests = append(tests, gradientDescentTests...)
	testLocal(t, tests, &NelderMead{})
}

func TestGradientDescent(t *testing.T) {
	testLocal(t, gradientDescentTests, &GradientDescent{})
}

func TestGradientDescentBacktracking(t *testing.T) {
	testLocal(t, gradientDescentTests, &GradientDescent{
		Linesearcher: &Backtracking{
			DecreaseFactor: 0.1,
		},
	})
}

func TestGradientDescentBisection(t *testing.T) {
	testLocal(t, gradientDescentTests, &GradientDescent{
		Linesearcher: &Bisection{},
	})
}

func TestCG(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{})
}

func TestFletcherReevesQuadStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &FletcherReeves{},
		InitialStep: &QuadraticStepSize{},
	})
}

func TestFletcherReevesFirstOrderStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &FletcherReeves{},
		InitialStep: &FirstOrderStepSize{},
	})
}

func TestHestenesStiefelQuadStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &HestenesStiefel{},
		InitialStep: &QuadraticStepSize{},
	})
}

func TestHestenesStiefelFirstOrderStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &HestenesStiefel{},
		InitialStep: &FirstOrderStepSize{},
	})
}

func TestPolakRibiereQuadStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &PolakRibierePolyak{},
		InitialStep: &QuadraticStepSize{},
	})
}

func TestPolakRibiereFirstOrderStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &PolakRibierePolyak{},
		InitialStep: &FirstOrderStepSize{},
	})
}

func TestDaiYuanQuadStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &DaiYuan{},
		InitialStep: &QuadraticStepSize{},
	})
}

func TestDaiYuanFirstOrderStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &DaiYuan{},
		InitialStep: &FirstOrderStepSize{},
	})
}

func TestHagerZhangQuadStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &HagerZhang{},
		InitialStep: &QuadraticStepSize{},
	})
}

func TestHagerZhangFirstOrderStep(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, cgTests...)
	testLocal(t, tests, &CG{
		Variant:     &HagerZhang{},
		InitialStep: &FirstOrderStepSize{},
	})
}

func TestBFGS(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, quasiNewtonTests...)
	tests = append(tests, bfgsTests...)
	testLocal(t, tests, &BFGS{})
}

func TestLBFGS(t *testing.T) {
	var tests []unconstrainedTest
	tests = append(tests, gradientDescentTests...)
	tests = append(tests, quasiNewtonTests...)
	tests = append(tests, lbfgsTests...)
	testLocal(t, tests, &LBFGS{})
}

func TestNewton(t *testing.T) {
	testLocal(t, newtonTests, &Newton{})
}

func testLocal(t *testing.T, tests []unconstrainedTest, method Method) {
	for cas, test := range tests {
		if test.long && testing.Short() {
			continue
		}

		settings := DefaultSettingsLocal()
		settings.Recorder = nil
		if method != nil && method.Needs().Gradient {
			// Turn off function convergence checks for gradient-based methods.
			settings.FunctionConverge = nil
		} else {
			if test.fIter == 0 {
				test.fIter = 20
			}
			settings.FunctionConverge.Iterations = test.fIter
			if test.fAbsTol == 0 {
				test.fAbsTol = 1e-12
			}
			settings.FunctionConverge.Absolute = test.fAbsTol
		}
		if test.gradTol == 0 {
			test.gradTol = 1e-12
		}
		settings.GradientThreshold = test.gradTol

		result, err := Minimize(test.p, test.x, settings, method)
		if err != nil {
			t.Errorf("Case %d: error finding minimum (%v) for:\n%v", cas, err, test)
			continue
		}
		if result == nil {
			t.Errorf("Case %d: nil result without error for:\n%v", cas, test)
			continue
		}

		// Check that the function value at the found optimum location is
		// equal to result.F.
		optF := test.p.Func(result.X)
		if optF != result.F {
			t.Errorf("Case %d: Function value at the optimum location %v not equal to the returned value %v for:\n%v",
				cas, optF, result.F, test)
		}
		if result.Gradient != nil {
			// Evaluate the norm of the gradient at the found optimum location.
			g := make([]float64, len(test.x))
			test.p.Grad(g, result.X)

			if !floats.Equal(result.Gradient, g) {
				t.Errorf("Case %d: Gradient at the optimum location not equal to the returned value for:\n%v", cas, test)
			}

			optNorm := floats.Norm(g, math.Inf(1))
			// Check that the norm of the gradient at the found optimum location is
			// smaller than the tolerance.
			if optNorm >= settings.GradientThreshold {
				t.Errorf("Case %d: Norm of the gradient at the optimum location %v not smaller than tolerance %v for:\n%v",
					cas, optNorm, settings.GradientThreshold, test)
			}
		}

		if method == nil {
			// The tests below make sense only if the method used is known.
			continue
		}

		if !method.Needs().Gradient && !method.Needs().Hessian {
			// Gradient-free tests can correctly terminate only with
			// FunctionConvergence status.
			if result.Status != FunctionConvergence {
				t.Errorf("Status not %v, %v instead", FunctionConvergence, result.Status)
			}
		}

		// We are going to restart the solution using known initial data, so
		// evaluate them.
		settings.InitValues = &Location{}
		settings.InitValues.F = test.p.Func(test.x)
		if method.Needs().Gradient {
			settings.InitValues.Gradient = resize(settings.InitValues.Gradient, len(test.x))
			test.p.Grad(settings.InitValues.Gradient, test.x)
		}
		if method.Needs().Hessian {
			settings.InitValues.Hessian = mat.NewSymDense(len(test.x), nil)
			test.p.Hess(settings.InitValues.Hessian, test.x)
		}

		// Rerun the test again to make sure that it gets the same answer with
		// the same starting condition. Moreover, we are using the initial data.
		result2, err2 := Minimize(test.p, test.x, settings, method)
		if err2 != nil {
			t.Errorf("error finding minimum second time (%v) for:\n%v", err2, test)
			continue
		}
		if result2 == nil {
			t.Errorf("second time nil result without error for:\n%v", test)
			continue
		}

		// At the moment all the optimizers are deterministic, so check that we
		// get _exactly_ the same answer second time as well.
		if result.F != result2.F || !floats.Equal(result.X, result2.X) {
			t.Errorf("Different minimum second time for:\n%v", test)
		}

		// Check that providing initial data reduces the number of evaluations exactly by one.
		if result.FuncEvaluations != result2.FuncEvaluations+1 {
			t.Errorf("Providing initial data does not reduce the number of Func calls for:\n%v", test)
			continue
		}
		if method.Needs().Gradient {
			if result.GradEvaluations != result2.GradEvaluations+1 {
				t.Errorf("Providing initial data does not reduce the number of Grad calls for:\n%v", test)
				continue
			}
		}
		if method.Needs().Hessian {
			if result.HessEvaluations != result2.HessEvaluations+1 {
				t.Errorf("Providing initial data does not reduce the number of Hess calls for:\n%v", test)
				continue
			}
		}
	}
}

func TestIssue76(t *testing.T) {
	p := Problem{
		Func: functions.BrownAndDennis{}.Func,
		Grad: functions.BrownAndDennis{}.Grad,
	}
	// Location very close to the minimum.
	x := []float64{-11.594439904886773, 13.203630051265385, -0.40343948776868443, 0.2367787746745986}
	s := &Settings{
		FunctionThreshold: math.Inf(-1),
		GradientThreshold: 1e-14,
		MajorIterations:   1000000,
	}
	m := &GradientDescent{
		Linesearcher: &Backtracking{},
	}
	// We are not interested in the error, only in the returned status.
	r, _ := Minimize(p, x, s, m)
	// With the above stringent tolerance, the optimizer will never
	// successfully reach the minimum. Check if it terminated in a finite
	// number of steps.
	if r.Status == IterationLimit {
		t.Error("Issue https://github.com/gonum/optimize/issues/76 not fixed")
	}
}

func TestNelderMeadOneD(t *testing.T) {
	p := Problem{
		Func: func(x []float64) float64 { return x[0] * x[0] },
	}
	x := []float64{10}
	m := &NelderMead{}
	s := DefaultSettingsLocal()
	result, err := Minimize(p, x, s, m)
	if err != nil {
		t.Errorf(err.Error())
	}
	if !floats.EqualApprox(result.X, []float64{0}, 1e-10) {
		t.Errorf("Minimum not found")
	}
	if m.reflection != 1 {
		t.Errorf("Wrong value of reflection")
	}
	if m.expansion != 2 {
		t.Errorf("Wrong value of expansion")
	}
	if m.contraction != 0.5 {
		t.Errorf("Wrong value of contraction")
	}
	if m.shrink != 0.5 {
		t.Errorf("Wrong value of shrink")
	}
}
