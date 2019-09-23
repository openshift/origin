// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package hyperdual provides the hyperdual numeric type and functions. Hyperdual
// numbers are an extension of the real numbers in the form a+bϵ₁+bϵ₂+dϵ₁ϵ₂ where
// ϵ₁^2=0 and ϵ₂^2=0, but ϵ₁≠0, ϵ₂≠0 and ϵ₁ϵ₂≠0.
//
// See https://doi.org/10.2514/6.2011-886 and http://adl.stanford.edu/hyperdual/ for
// details of their properties and uses.
package hyperdual // imports "gonum.org/v1/gonum/num/hyperdual"

// TODO(kortschak): Handle special cases properly.
//  - Pow
