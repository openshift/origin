// Copyright ©2013 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"math"

	"gopkg.in/check.v1"
)

func (s *S) TestSVD(c *check.C) {
	for _, t := range []struct {
		a *Dense

		epsilon float64
		small   float64

		wantu bool
		u     *Dense

		sigma []float64

		wantv bool
		v     *Dense
	}{
		{
			a: NewDense(4, 2, []float64{2, 4, 1, 3, 0, 0, 0, 0}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			wantu: true,
			u: NewDense(4, 2, []float64{
				0.8174155604703632, -0.5760484367663209,
				0.5760484367663209, 0.8174155604703633,
				0, 0,
				0, 0,
			}),

			sigma: []float64{5.464985704219041, 0.365966190626258},

			wantv: true,
			v: NewDense(2, 2, []float64{
				0.4045535848337571, -0.9145142956773044,
				0.9145142956773044, 0.4045535848337571,
			}),
		},
		{
			a: NewDense(4, 2, []float64{2, 4, 1, 3, 0, 0, 0, 0}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			wantu: true,
			u: NewDense(4, 2, []float64{
				0.8174155604703632, -0.5760484367663209,
				0.5760484367663209, 0.8174155604703633,
				0, 0,
				0, 0,
			}),

			sigma: []float64{5.464985704219041, 0.365966190626258},

			wantv: false,
		},
		{
			a: NewDense(4, 2, []float64{2, 4, 1, 3, 0, 0, 0, 0}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			wantu: false,

			sigma: []float64{5.464985704219041, 0.365966190626258},

			wantv: true,
			v: NewDense(2, 2, []float64{
				0.4045535848337571, -0.9145142956773044,
				0.9145142956773044, 0.4045535848337571,
			}),
		},
		{
			a: NewDense(4, 2, []float64{2, 4, 1, 3, 0, 0, 0, 0}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			sigma: []float64{5.464985704219041, 0.365966190626258},
		},
		{ // Issue #5.
			a: NewDense(3, 11, []float64{
				1, 1, 0, 1, 0, 0, 0, 0, 0, 11, 1,
				1, 0, 0, 0, 0, 0, 1, 0, 0, 12, 2,
				1, 1, 0, 0, 0, 0, 0, 0, 1, 13, 3,
			}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			sigma: []float64{21.259500881097434, 1.5415021616856566, 1.2873979074613628},

			wantu: true,
			u: NewDense(3, 3, []float64{
				0.5224167862273765, -0.7864430360363114, 0.3295270133658976,
				0.5739526766688285, 0.03852203026050301, -0.8179818935216693,
				0.6306021141833781, 0.6164603833618163, 0.4715056408282468,
			}),

			wantv: true,
			v: NewDense(11, 3, []float64{
				0.08123293141915189, -0.08528085505260324, -0.013165501690885152,
				0.05423546426886932, -0.1102707844980355, 0.622210623111631,
				0, 0, 0,
				0.0245733326078166, -0.510179651760153, 0.25596360803140994,
				0, 0, 0,
				0, 0, 0,
				0.026997467150282436, 0.024989929445430496, -0.6353761248025164,
				0, 0, 0,
				0.029662131661052707, 0.3999088672621176, 0.3662470150802212,
				0.9798839760830571, -0.11328174160898856, -0.047702613241813366,
				0.16755466189153964, 0.7395268089170608, 0.08395240366704032,
			}),
		},
		{ // Issue #5: test that correct matrices are constructed.
			a: NewDense(3, 11, []float64{
				1, 1, 0, 1, 0, 0, 0, 0, 0, 11, 1,
				1, 0, 0, 0, 0, 0, 1, 0, 0, 12, 2,
				1, 1, 0, 0, 0, 0, 0, 0, 1, 13, 3,
			}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			sigma: []float64{21.259500881097434, 1.5415021616856566, 1.2873979074613628},

			wantu: true,
			u: NewDense(3, 3, []float64{
				0.5224167862273765, -0.7864430360363114, 0.3295270133658976,
				0.5739526766688285, 0.03852203026050301, -0.8179818935216693,
				0.6306021141833781, 0.6164603833618163, 0.4715056408282468,
			}),
		},
		{ // Issue #5: test that correct matrices are constructed.
			a: NewDense(3, 11, []float64{
				1, 1, 0, 1, 0, 0, 0, 0, 0, 11, 1,
				1, 0, 0, 0, 0, 0, 1, 0, 0, 12, 2,
				1, 1, 0, 0, 0, 0, 0, 0, 1, 13, 3,
			}),

			epsilon: math.Pow(2, -52.0),
			small:   math.Pow(2, -966.0),

			sigma: []float64{21.259500881097434, 1.5415021616856566, 1.2873979074613628},

			wantv: true,
			v: NewDense(11, 3, []float64{
				0.08123293141915189, -0.08528085505260324, -0.013165501690885152,
				0.05423546426886932, -0.1102707844980355, 0.622210623111631,
				0, 0, 0,
				0.0245733326078166, -0.510179651760153, 0.25596360803140994,
				0, 0, 0,
				0, 0, 0,
				0.026997467150282436, 0.024989929445430496, -0.6353761248025164,
				0, 0, 0,
				0.029662131661052707, 0.3999088672621176, 0.3662470150802212,
				0.9798839760830571, -0.11328174160898856, -0.047702613241813366,
				0.16755466189153964, 0.7395268089170608, 0.08395240366704032,
			}),
		},
	} {
		svd := SVD(DenseCopyOf(t.a), t.epsilon, t.small, t.wantu, t.wantv)
		if t.sigma != nil {
			c.Check(svd.Sigma, check.DeepEquals, t.sigma)
		}
		s := svd.S()

		if svd.U != nil {
			c.Check(svd.U.Equals(t.u), check.Equals, true)
		} else {
			c.Check(t.wantu, check.Equals, false)
			c.Check(t.u, check.IsNil)
		}
		if svd.V != nil {
			c.Check(svd.V.Equals(t.v), check.Equals, true)
		} else {
			c.Check(t.wantv, check.Equals, false)
			c.Check(t.v, check.IsNil)
		}

		if t.wantu && t.wantv {
			c.Assert(svd.U, check.NotNil)
			c.Assert(svd.V, check.NotNil)
			vt := &Dense{}
			vt.TCopy(svd.V)
			var tmp, got Dense
			tmp.Mul(svd.U, s)
			got.Mul(&tmp, vt)
			c.Check(got.EqualsApprox(t.a, 1e-12), check.Equals, true)
		}
	}
}
