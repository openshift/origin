// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dual_test

import (
	"fmt"

	"gonum.org/v1/gonum/num/dual"
)

func Example_Number_fike() {
	// Calculate the value and derivative of the function
	// e^x/(sqrt(sin(x)^3 + cos(x)^3)).
	fn := func(x dual.Number) dual.Number {
		return dual.Mul(
			dual.Exp(x),
			dual.Inv(dual.Sqrt(
				dual.Add(
					dual.PowReal(dual.Sin(x), 3),
					dual.PowReal(dual.Cos(x), 3)))))
	}

	v := fn(dual.Number{Real: 1.5, Emag: 1})
	fmt.Printf("v=%.4f\n", v)
	fmt.Printf("fn(1.5)=%.4f\nfn'(1.5)=%.4f\n", v.Real, v.Emag)

	// Output:
	//
	// v=(4.4978+4.0534ϵ)
	// fn(1.5)=4.4978
	// fn'(1.5)=4.0534
}
