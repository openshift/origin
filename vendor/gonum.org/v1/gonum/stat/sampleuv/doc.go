// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sampleuv implements advanced sampling routines from explicit and implicit
// probability distributions.
//
// Each sampling routine is implemented as a stateless function with a
// complementary wrapper type. The wrapper types allow the sampling routines
// to implement interfaces.
package sampleuv // import "gonum.org/v1/gonum/stat/sampleuv"
