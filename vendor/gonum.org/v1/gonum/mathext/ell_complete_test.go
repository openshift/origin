// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mathext

import (
	"math"
	"testing"
)

// TestCompleteKE checks if the Legendre's relation for m=0.0001(0.0001)0.9999
// is satisfied with accuracy 1e-14.
func TestCompleteKE(t *testing.T) {
	const tol = 1.0e-14

	for m := 1; m <= 9999; m++ {
		mf := float64(m) / 10000
		mp := 1 - mf
		K, Kp := CompleteK(mf), CompleteK(mp)
		E, Ep := CompleteE(mf), CompleteE(mp)
		legendre := math.Abs(E*Kp + Ep*K - K*Kp - math.Pi/2)
		if legendre > tol {
			t.Fatalf("legendre > tol: m=%v, legendre=%v, tol=%v", mf, legendre, tol)
		}
	}
}

// TestCompleteBD checks if the relations between two associate elliptic integrals B(m), D(m)
// and more common Legendre's elliptic integrals K(m), E(m) are satisfied with accuracy 1e-14
// for m=0.0001(0.0001)0.9999.
//
// K(m) and E(m) can be computed without cancellation problems as following:
//	K(m) = B(m) + D(m),
//	E(m) = B(m) + (1-m)D(m).
func TestCompleteBD(t *testing.T) {
	const tol = 1.0e-14

	for m := 1; m <= 9999; m++ {
		mf := float64(m) / 10000
		B, D := CompleteB(mf), CompleteD(mf)
		K, E := CompleteK(mf), CompleteE(mf)
		difference1 := math.Abs(K - (B + D))
		difference2 := math.Abs(E - (B + (1-mf)*D))
		if difference1 > tol {
			t.Fatalf("difference1 > tol: m=%v, difference1=%v, tol=%v", mf, difference1, tol)
		}
		if difference2 > tol {
			t.Fatalf("difference2 > tol: m=%v, difference2=%v, tol=%v", mf, difference2, tol)
		}
	}
}
