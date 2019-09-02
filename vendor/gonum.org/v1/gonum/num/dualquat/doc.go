// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dualquat provides the dual quaternion numeric type and functions.
//
// Dual quaternions provide a system for rigid transformation with interpolation
// and blending in ℝ³. See https://www.cs.utah.edu/~ladislav/kavan06dual/kavan06dual.pdf and
// https://en.wikipedia.org/wiki/Dual_quaternion for more details.
package dualquat // imports "gonum.org/v1/gonum/num/dualquat"

// TODO(kortschak): Handle special cases properly.
//  - Pow
