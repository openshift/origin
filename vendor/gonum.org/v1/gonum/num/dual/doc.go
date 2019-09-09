// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dual provides the dual numeric type and functions. Dual numbers
// are an extension of the real numbers in the form a+bϵ where ϵ^2=0, but ϵ≠0.
//
// See https://en.wikipedia.org/wiki/Dual_number for details of their properties
// and uses.
package dual // imports "gonum.org/v1/gonum/num/dual"

// TODO(kortschak): Handle special cases properly.
//  - Pow
