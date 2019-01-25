// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functions

import "math"

// This file implements functions from the Virtual Library of Simulation Experiments.
//  https://www.sfu.ca/~ssurjano/optimization.html
// In many cases gradients and Hessians have been added. In some cases, these
// are not defined at certain points or manifolds. The gradient in these locations
// has been set to 0.

// Ackley implements the Ackley function, a function of arbitrary dimension that
// has many local minima. It has a single global minimum of 0 at 0. Its typical
// domain is the hypercube of [-32.768, 32.768]^d.
//  f(x) = -20 * exp(-0.2 sqrt(1/d sum_i x_i^2)) - exp(1/d sum_i cos(2π x_i)) + 20 + exp(1)
// where d is the input dimension.
//
// Reference:
//  https://www.sfu.ca/~ssurjano/ackley.html (obtained June 2017)
type Ackley struct{}

func (Ackley) Func(x []float64) float64 {
	var ss, sc float64
	for _, v := range x {
		ss += v * v
		sc += math.Cos(2 * math.Pi * v)
	}
	id := 1 / float64(len(x))
	return -20*math.Exp(-0.2*math.Sqrt(id*ss)) - math.Exp(id*sc) + 20 + math.E
}

// Bukin6 implements Bukin's 6th function. The function is two-dimensional, with
// the typical domain as x_0 ∈ [-15, -5], x_1 ∈ [-3, 3]. The function has a unique
// global minimum at [-10, 1], and many local minima.
//  f(x) = 100 * sqrt(|x_1 - 0.01*x_0^2|) + 0.01*|x_0+10|
// Reference:
//  https://www.sfu.ca/~ssurjano/bukin6.html (obtained June 2017)
type Bukin6 struct{}

func (Bukin6) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	return 100*math.Sqrt(math.Abs(x[1]-0.01*x[0]*x[0])) + 0.01*math.Abs(x[0]+10)
}

// CamelThree implements the three-hump camel function, a two-dimensional function
// with three local minima, one of which is global.
// The function is given by
//  f(x) = 2*x_0^2 - 1.05*x_0^4 + x_0^6/6 + x_0*x_1 + x_1^2
// with the global minimum at
//  x^* = (0, 0)
//  f(x^*) = 0
// The typical domain is x_i ∈ [-5, 5] for all i.
// Reference:
//  https://www.sfu.ca/~ssurjano/camel3.html (obtained December 2017)
type CamelThree struct{}

func (c CamelThree) Func(x []float64) float64 {
	if len(x) != 2 {
		panic("camelthree: dimension must be 2")
	}
	x0 := x[0]
	x1 := x[1]
	x02 := x0 * x0
	x04 := x02 * x02
	return 2*x02 - 1.05*x04 + x04*x02/6 + x0*x1 + x1*x1
}

// CamelSix implements the six-hump camel function, a two-dimensional function.
// with six local minima, two of which are global.
// The function is given by
//  f(x) = (4 - 2.1*x_0^2 + x_0^4/3)*x_0^2 + x_0*x_1 + (-4 + 4*x_1^2)*x_1^2
// with the global minima at
//  x^* = (0.0898, -0.7126), (-0.0898, 0.7126)
//  f(x^*) = -1.0316
// The typical domain is x_0 ∈ [-3, 3], x_1 ∈ [-2, 2].
// Reference:
//  https://www.sfu.ca/~ssurjano/camel6.html (obtained December 2017)
type CamelSix struct{}

func (c CamelSix) Func(x []float64) float64 {
	if len(x) != 2 {
		panic("camelsix: dimension must be 2")
	}
	x0 := x[0]
	x1 := x[1]
	x02 := x0 * x0
	x12 := x1 * x1
	return (4-2.1*x02+x02*x02/3)*x02 + x0*x1 + (-4+4*x12)*x12
}

// CrossInTray implements the cross-in-tray function. The cross-in-tray function
// is a two-dimensional function with many local minima, and four global minima
// at (±1.3491, ±1.3491). The function is typically evaluated in the square
// [-10,10]^2.
//  f(x) = -0.001(|sin(x_0)sin(x_1)exp(|100-sqrt((x_0^2+x_1^2)/π)|)|+1)^0.1
// Reference:
//  https://www.sfu.ca/~ssurjano/crossit.html (obtained June 2017)
type CrossInTray struct{}

func (CrossInTray) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	exp := math.Abs(100 - math.Sqrt((x0*x0+x1*x1)/math.Pi))
	return -0.0001 * math.Pow(math.Abs(math.Sin(x0)*math.Sin(x1)*math.Exp(exp))+1, 0.1)
}

// DixonPrice implements the DixonPrice function, a function of arbitrary dimension
// Its typical domain is the hypercube of [-10, 10]^d.
// The function is given by
//  f(x) = (x_0-1)^2 + \sum_{i=1}^{d-1} (i+1) * (2*x_i^2-x_{i-1})^2
// where d is the input dimension. There is a single global minimum, which has
// a location and value of
//  x_i^* = 2^{-(2^{i+1}-2)/(2^{i+1})} for i = 0, ..., d-1.
//  f(x^*) = 0
// Reference:
//  https://www.sfu.ca/~ssurjano/dixonpr.html (obtained June 2017)
type DixonPrice struct{}

func (DixonPrice) Func(x []float64) float64 {
	xp := x[0]
	v := (xp - 1) * (xp - 1)
	for i := 1; i < len(x); i++ {
		xn := x[i]
		tmp := (2*xn*xn - xp)
		v += float64(i+1) * tmp * tmp
		xp = xn
	}
	return v
}

// DropWave implements the drop-wave function, a two-dimensional function with
// many local minima and one global minimum at 0. The function is typically evaluated
// in the square [-5.12, 5.12]^2.
//  f(x) = - (1+cos(12*sqrt(x0^2+x1^2))) / (0.5*(x0^2+x1^2)+2)
// Reference:
//  https://www.sfu.ca/~ssurjano/drop.html (obtained June 2017)
type DropWave struct{}

func (DropWave) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	num := 1 + math.Cos(12*math.Sqrt(x0*x0+x1*x1))
	den := 0.5*(x0*x0+x1*x1) + 2
	return -num / den
}

// Eggholder implements the Eggholder function, a two-dimensional function with
// many local minima and one global minimum at [512, 404.2319]. The function
// is typically evaluated in the square [-512, 512]^2.
//  f(x) = -(x_1+47)*sin(sqrt(|x_1+x_0/2+47|))-x_1*sin(sqrt(|x_0-(x_1+47)|))
// Reference:
//  https://www.sfu.ca/~ssurjano/egg.html (obtained June 2017)
type Eggholder struct{}

func (Eggholder) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	return -(x1+47)*math.Sin(math.Sqrt(math.Abs(x1+x0/2+47))) -
		x0*math.Sin(math.Sqrt(math.Abs(x0-x1-47)))
}

// GramacyLee implements the Gramacy-Lee function, a one-dimensional function
// with many local minima. The function is typically evaluated on the domain [0.5, 2.5].
//  f(x) = sin(10πx)/(2x) + (x-1)^4
// Reference:
//  https://www.sfu.ca/~ssurjano/grlee12.html (obtained June 2017)
type GramacyLee struct{}

func (GramacyLee) Func(x []float64) float64 {
	if len(x) != 1 {
		panic(badInputDim)
	}
	x0 := x[0]
	return math.Sin(10*math.Pi*x0)/(2*x0) + math.Pow(x0-1, 4)
}

// Griewank implements the Griewank function, a function of arbitrary dimension that
// has many local minima. It has a single global minimum of 0 at 0. Its typical
// domain is the hypercube of [-600, 600]^d.
//  f(x) = \sum_i x_i^2/4000 - \prod_i cos(x_i/sqrt(i)) + 1
// where d is the input dimension.
//
// Reference:
//  https://www.sfu.ca/~ssurjano/griewank.html (obtained June 2017)
type Griewank struct{}

func (Griewank) Func(x []float64) float64 {
	var ss float64
	pc := 1.0
	for i, v := range x {
		ss += v * v
		pc *= math.Cos(v / math.Sqrt(float64(i+1)))
	}
	return ss/4000 - pc + 1
}

// HolderTable implements the Holder table function. The Holder table function
// is a two-dimensional function with many local minima, and four global minima
// at (±8.05502, ±9.66459). The function is typically evaluated in the square [-10,10]^2.
//  f(x) = -|sin(x_0)cos(x1)exp(|1-sqrt(x_0^2+x1^2)/π|)|
// Reference:
//  https://www.sfu.ca/~ssurjano/holder.html (obtained June 2017)
type HolderTable struct{}

func (HolderTable) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	return -math.Abs(math.Sin(x0) * math.Cos(x1) * math.Exp(math.Abs(1-math.Sqrt(x0*x0+x1*x1)/math.Pi)))
}

// Langermann2 implements the two-dimensional version of the Langermann function.
// The Langermann function has many local minima. The function is typically
// evaluated in the square [0,10]^2.
//  f(x) = \sum_1^5 c_i exp(-(1/π)\sum_{j=1}^2(x_j-A_{ij})^2) * cos(π\sum_{j=1}^2 (x_j - A_{ij})^2)
//  c = [5]float64{1,2,5,2,3}
//  A = [5][2]float64{{3,5},{5,2},{2,1},{1,4},{7,9}}
// Reference:
//  https://www.sfu.ca/~ssurjano/langer.html (obtained June 2017)
type Langermann2 struct{}

func (Langermann2) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	var (
		c = [5]float64{1, 2, 5, 2, 3}
		A = [5][2]float64{{3, 5}, {5, 2}, {2, 1}, {1, 4}, {7, 9}}
	)
	var f float64
	for i, cv := range c {
		var ss float64
		for j, av := range A[i] {
			xja := x[j] - av
			ss += xja * xja
		}
		f += cv * math.Exp(-(1/math.Pi)*ss) * math.Cos(math.Pi*ss)
	}
	return f
}

// Levy implements the Levy function, a function of arbitrary dimension that
// has many local minima. It has a single global minimum of 0 at 1. Its typical
// domain is the hypercube of [-10, 10]^d.
//  f(x) = sin^2(π*w_0) + \sum_{i=0}^{d-2}(w_i-1)^2*[1+10sin^2(π*w_i+1)] +
//            (w_{d-1}-1)^2*[1+sin^2(2π*w_{d-1})]
//   w_i = 1 + (x_i-1)/4
// where d is the input dimension.
//
// Reference:
//  https://www.sfu.ca/~ssurjano/levy.html (obtained June 2017)
type Levy struct{}

func (Levy) Func(x []float64) float64 {
	w1 := 1 + (x[0]-1)/4
	s1 := math.Sin(math.Pi * w1)
	sum := s1 * s1
	for i := 0; i < len(x)-1; i++ {
		wi := 1 + (x[i]-1)/4
		s := math.Sin(math.Pi*wi + 1)
		sum += (wi - 1) * (wi - 1) * (1 + 10*s*s)
	}
	wd := 1 + (x[len(x)-1]-1)/4
	sd := math.Sin(2 * math.Pi * wd)
	return sum + (wd-1)*(wd-1)*(1+sd*sd)
}

// Levy13 implements the Levy-13 function, a two-dimensional function
// with many local minima. It has a single global minimum of 0 at 1. Its typical
// domain is the square [-10, 10]^2.
//  f(x) = sin^2(3π*x_0) + (x_0-1)^2*[1+sin^2(3π*x_1)] + (x_1-1)^2*[1+sin^2(2π*x_1)]
// Reference:
//  https://www.sfu.ca/~ssurjano/levy13.html (obtained June 2017)
type Levy13 struct{}

func (Levy13) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	s0 := math.Sin(3 * math.Pi * x0)
	s1 := math.Sin(3 * math.Pi * x1)
	s2 := math.Sin(2 * math.Pi * x1)
	return s0*s0 + (x0-1)*(x0-1)*(1+s1*s1) + (x1-1)*(x1-1)*(1+s2*s2)
}

// Rastrigin implements the Rastrigen function, a function of arbitrary dimension
// that has many local minima. It has a single global minimum of 0 at 0. Its typical
// domain is the hypercube of [-5.12, 5.12]^d.
//  f(x) = 10d + \sum_i [x_i^2 - 10cos(2π*x_i)]
// where d is the input dimension.
//
// Reference:
//  https://www.sfu.ca/~ssurjano/rastr.html (obtained June 2017)
type Rastrigin struct{}

func (Rastrigin) Func(x []float64) float64 {
	sum := 10 * float64(len(x))
	for _, v := range x {
		sum += v*v - 10*math.Cos(2*math.Pi*v)
	}
	return sum
}

// Schaffer2 implements the second Schaffer function, a two-dimensional function
// with many local minima. It has a single global minimum of 0 at 0. Its typical
// domain is the square [-100, 100]^2.
//  f(x) = 0.5 + (sin^2(x_0^2-x_1^2)-0.5) / (1+0.001*(x_0^2+x_1^2))^2
// Reference:
//  https://www.sfu.ca/~ssurjano/schaffer2.html (obtained June 2017)
type Schaffer2 struct{}

func (Schaffer2) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	s := math.Sin(x0*x0 - x1*x1)
	den := 1 + 0.001*(x0*x0+x1*x1)
	return 0.5 + (s*s-0.5)/(den*den)
}

// Schaffer4 implements the fourth Schaffer function, a two-dimensional function
// with many local minima. Its typical domain is the square [-100, 100]^2.
//  f(x) = 0.5 + (cos(sin(|x_0^2-x_1^2|))-0.5) / (1+0.001*(x_0^2+x_1^2))^2
// Reference:
//  https://www.sfu.ca/~ssurjano/schaffer4.html (obtained June 2017)
type Schaffer4 struct{}

func (Schaffer4) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	den := 1 + 0.001*(x0*x0+x1*x1)
	return 0.5 + (math.Cos(math.Sin(math.Abs(x0*x0-x1*x1)))-0.5)/(den*den)
}

// Schwefel implements the Schwefel function, a function of arbitrary dimension
// that has many local minima. Its typical domain is the hypercube of [-500, 500]^d.
//  f(x) = 418.9829*d - \sum_i x_i*sin(sqrt(|x_i|))
// where d is the input dimension.
//
// Reference:
//  https://www.sfu.ca/~ssurjano/schwef.html (obtained June 2017)
type Schwefel struct{}

func (Schwefel) Func(x []float64) float64 {
	var sum float64
	for _, v := range x {
		sum += v * math.Sin(math.Sqrt(math.Abs(v)))
	}
	return 418.9829*float64(len(x)) - sum
}

// Shubert implements the Shubert function, a two-dimensional function
// with many local minima and many global minima. Its typical domain is the
// square [-10, 10]^2.
//  f(x) = (sum_{i=1}^5 i cos((i+1)*x_0+i)) * (\sum_{i=1}^5 i cos((i+1)*x_1+i))
// Reference:
//  https://www.sfu.ca/~ssurjano/shubert.html (obtained June 2017)
type Shubert struct{}

func (Shubert) Func(x []float64) float64 {
	if len(x) != 2 {
		panic(badInputDim)
	}
	x0 := x[0]
	x1 := x[1]
	var s0, s1 float64
	for i := 1.0; i <= 5.0; i++ {
		s0 += i * math.Cos((i+1)*x0+i)
		s1 += i * math.Cos((i+1)*x1+i)
	}
	return s0 * s1
}
