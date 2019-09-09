// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gonum

import (
	"testing"

	"gonum.org/v1/gonum/blas/testblas"
)

func TestZgemm(t *testing.T)  { testblas.ZgemmTest(t, impl) }
func TestZhemm(t *testing.T)  { testblas.ZhemmTest(t, impl) }
func TestZherk(t *testing.T)  { testblas.ZherkTest(t, impl) }
func TestZher2k(t *testing.T) { testblas.Zher2kTest(t, impl) }
func TestZsymm(t *testing.T)  { testblas.ZsymmTest(t, impl) }
func TestZsyrk(t *testing.T)  { testblas.ZsyrkTest(t, impl) }
func TestZsyr2k(t *testing.T) { testblas.Zsyr2kTest(t, impl) }
func TestZtrmm(t *testing.T)  { testblas.ZtrmmTest(t, impl) }
func TestZtrsm(t *testing.T)  { testblas.ZtrsmTest(t, impl) }
