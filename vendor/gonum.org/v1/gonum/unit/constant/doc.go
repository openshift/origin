// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run generate_defined_types.go
//go:generate go run generate_constants.go defined_types.go

// Package constant provides fundamental constants satisfying unit.Uniter.
//
// Constant values reflect the values published at https://physics.nist.gov/cuu/index.html
// and are subject to change when the published values are updated.
package constant
