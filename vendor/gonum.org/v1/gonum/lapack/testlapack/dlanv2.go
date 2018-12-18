// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"fmt"
	"math"
	"testing"

	"golang.org/x/exp/rand"
)

type Dlanv2er interface {
	Dlanv2(a, b, c, d float64) (aa, bb, cc, dd float64, rt1r, rt1i, rt2r, rt2i float64, cs, sn float64)
}

func Dlanv2Test(t *testing.T, impl Dlanv2er) {
	rnd := rand.New(rand.NewSource(1))
	t.Run("UpperTriangular", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			a := rnd.NormFloat64()
			b := rnd.NormFloat64()
			d := rnd.NormFloat64()
			dlanv2Test(t, impl, a, b, 0, d)
		}
	})
	t.Run("LowerTriangular", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			a := rnd.NormFloat64()
			c := rnd.NormFloat64()
			d := rnd.NormFloat64()
			dlanv2Test(t, impl, a, 0, c, d)
		}
	})
	t.Run("StandardSchur", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			a := rnd.NormFloat64()
			b := rnd.NormFloat64()
			c := rnd.NormFloat64()
			if math.Signbit(b) == math.Signbit(c) {
				c = -c
			}
			dlanv2Test(t, impl, a, b, c, a)
		}
	})
	t.Run("General", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			a := rnd.NormFloat64()
			b := rnd.NormFloat64()
			c := rnd.NormFloat64()
			d := rnd.NormFloat64()
			dlanv2Test(t, impl, a, b, c, d)
		}
	})
}

func dlanv2Test(t *testing.T, impl Dlanv2er, a, b, c, d float64) {
	aa, bb, cc, dd, rt1r, rt1i, rt2r, rt2i, cs, sn := impl.Dlanv2(a, b, c, d)

	mat := fmt.Sprintf("[%v %v; %v %v]", a, b, c, d)
	if cc == 0 {
		// The eigenvalues are real, so check that the imaginary parts
		// are zero.
		if rt1i != 0 || rt2i != 0 {
			t.Errorf("Unexpected complex eigenvalues for %v", mat)
		}
	} else {
		// The eigenvalues are complex, so check that documented
		// conditions hold.
		if aa != dd {
			t.Errorf("Diagonal elements not equal for %v: got [%v %v]", mat, aa, dd)
		}
		if bb*cc >= 0 {
			t.Errorf("Non-diagonal elements have the same sign for %v: got [%v %v]", mat, bb, cc)
		} else {
			// Compute the absolute value of the imaginary part.
			im := math.Sqrt(-bb * cc)
			// Check that ±im is close to one of the returned
			// imaginary parts.
			if math.Abs(rt1i-im) > 1e-14 && math.Abs(rt1i+im) > 1e-14 {
				t.Errorf("Unexpected imaginary part of eigenvalue for %v: got %v, want %v or %v", mat, rt1i, im, -im)
			}
			if math.Abs(rt2i-im) > 1e-14 && math.Abs(rt2i+im) > 1e-14 {
				t.Errorf("Unexpected imaginary part of eigenvalue for %v: got %v, want %v or %v", mat, rt2i, im, -im)
			}
		}
	}
	// Check that the returned real parts are consistent.
	if rt1r != aa && rt1r != dd {
		t.Errorf("Unexpected real part of eigenvalue for %v: got %v, want %v or %v", mat, rt1r, aa, dd)
	}
	if rt2r != aa && rt2r != dd {
		t.Errorf("Unexpected real part of eigenvalue for %v: got %v, want %v or %v", mat, rt2r, aa, dd)
	}
	// Check that the columns of the orthogonal matrix have unit norm.
	if math.Abs(math.Hypot(cs, sn)-1) > 1e-14 {
		t.Errorf("Unexpected unitary matrix for %v: got cs %v, sn %v", mat, cs, sn)
	}

	// Re-compute the original matrix [a b; c d] from its factorization.
	gota := cs*(aa*cs-bb*sn) - sn*(cc*cs-dd*sn)
	gotb := cs*(aa*sn+bb*cs) - sn*(cc*sn+dd*cs)
	gotc := sn*(aa*cs-bb*sn) + cs*(cc*cs-dd*sn)
	gotd := sn*(aa*sn+bb*cs) + cs*(cc*sn+dd*cs)
	if math.Abs(gota-a) > 1e-14 ||
		math.Abs(gotb-b) > 1e-14 ||
		math.Abs(gotc-c) > 1e-14 ||
		math.Abs(gotd-d) > 1e-14 {
		t.Errorf("Unexpected factorization: got [%v %v; %v %v], want [%v %v; %v %v]", gota, gotb, gotc, gotd, a, b, c, d)
	}
}
