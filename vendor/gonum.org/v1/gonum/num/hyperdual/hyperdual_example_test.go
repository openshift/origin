// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package hyperdual_test

import (
	"fmt"

	"gonum.org/v1/gonum/num/hyperdual"
)

func Example_Number_fike() {
	// Calculate the value and first and second derivatives
	// of the function e^x/(sqrt(sin(x)^3 + cos(x)^3)).
	fn := func(x hyperdual.Number) hyperdual.Number {
		return hyperdual.Mul(
			hyperdual.Exp(x),
			hyperdual.Inv(hyperdual.Sqrt(
				hyperdual.Add(
					hyperdual.PowReal(hyperdual.Sin(x), 3),
					hyperdual.PowReal(hyperdual.Cos(x), 3)))))
	}

	v := fn(hyperdual.Number{Real: 1.5, E1mag: 1, E2mag: 1})
	fmt.Printf("v=%.4f\n", v)
	fmt.Printf("fn(1.5)=%.4f\nfn'(1.5)=%.4f\nfn''(1.5)=%.4f\n", v.Real, v.E1mag, v.E1E2mag)

	// Output:
	//
	// v=(4.4978+4.0534ϵ₁+4.0534ϵ₂+9.4631ϵ₁ϵ₂)
	// fn(1.5)=4.4978
	// fn'(1.5)=4.0534
	// fn''(1.5)=9.4631
}
