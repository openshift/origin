// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package testquad provides integrals for testing quadrature algorithms.
package testquad

import (
	"fmt"
	"math"
)

// Integral is a definite integral
//  ∫_a^b f(x)dx
// with a known value.
type Integral struct {
	Name  string
	A, B  float64               // Integration limits
	F     func(float64) float64 // Integrand
	Value float64
}

// Constant returns the integral of a constant function
//  ∫_{-1}^2 alpha dx
func Constant(alpha float64) Integral {
	return Integral{
		Name: fmt.Sprintf("∫_{-1}^{2} %vdx", alpha),
		A:    -1,
		B:    2,
		F: func(float64) float64 {
			return alpha
		},
		Value: 3 * alpha,
	}
}

// Poly returns the integral of a polynomial
//  ∫_{-1}^2 x^degree dx
func Poly(degree int) Integral {
	d := float64(degree)
	return Integral{
		Name: fmt.Sprintf("∫_{-1}^{2} x^%vdx", degree),
		A:    -1,
		B:    2,
		F: func(x float64) float64 {
			return math.Pow(x, d)
		},
		Value: (math.Pow(2, d+1) - math.Pow(-1, d+1)) / (d + 1),
	}
}

// Sin returns the integral
//  ∫_0^1 sin(x)dx
func Sin() Integral {
	return Integral{
		Name: "∫_0^1 sin(x)dx",
		A:    0,
		B:    1,
		F: func(x float64) float64 {
			return math.Sin(x)
		},
		Value: 1 - math.Cos(1),
	}
}

// XExpMinusX returns the integral
//  ∫_0^1 x*exp(-x)dx
func XExpMinusX() Integral {
	return Integral{
		Name: "∫_0^1 x*exp(-x)dx",
		A:    0,
		B:    1,
		F: func(x float64) float64 {
			return x * math.Exp(-x)
		},
		Value: (math.E - 2) / math.E,
	}
}

// Sqrt returns the integral
//  ∫_0^1 sqrt(x)dx
func Sqrt() Integral {
	return Integral{
		Name: "∫_0^1 sqrt(x)dx",
		A:    0,
		B:    1,
		F: func(x float64) float64 {
			return math.Sqrt(x)
		},
		Value: 2 / 3.0,
	}
}

// ExpOverX2Plus1 returns the integral
//  ∫_0^1 exp(x)/(x*x+1)dx
func ExpOverX2Plus1() Integral {
	return Integral{
		Name: "∫_0^1 exp(x)/(x*x+1)dx",
		A:    0,
		B:    1,
		F: func(x float64) float64 {
			return math.Exp(x) / (x*x + 1)
		},
		Value: 1.270724139833620220138,
	}
}
