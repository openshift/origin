// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cgo

import (
	"testing"

	"github.com/gonum/lapack/testlapack"
)

var impl = Implementation{}

func TestDpotrf(t *testing.T) {
	testlapack.DpotrfTest(t, impl)
}
