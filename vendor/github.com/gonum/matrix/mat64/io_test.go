// Copyright ©2015 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mat64

import (
	"gopkg.in/check.v1"
)

func (s *S) TestDenseRW(c *check.C) {
	for i, test := range []*Dense{
		NewDense(0, 0, []float64{}),
		NewDense(2, 2, []float64{1, 2, 3, 4}),
		NewDense(2, 3, []float64{1, 2, 3, 4, 5, 6}),
		NewDense(3, 2, []float64{1, 2, 3, 4, 5, 6}),
		NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}),
		NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}).View(0, 0, 2, 2).(*Dense),
		NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}).View(1, 1, 2, 2).(*Dense),
		NewDense(3, 3, []float64{1, 2, 3, 4, 5, 6, 7, 8, 9}).View(0, 1, 3, 2).(*Dense),
	} {
		buf, err := test.MarshalBinary()
		c.Check(err, check.Equals, nil, check.Commentf("error encoding test #%d: %v\n", i, err))

		nrows, ncols := test.Dims()
		sz := nrows*ncols*sizeFloat64 + 2*sizeInt64
		c.Check(len(buf), check.Equals, sz, check.Commentf("encoded size test #%d: want=%d got=%d\n", i, sz, len(buf)))

		var got Dense
		err = got.UnmarshalBinary(buf)
		c.Check(err, check.Equals, nil, check.Commentf("error decoding test #%d: %v\n", i, err))

		c.Check(got.Equals(test), check.Equals, true, check.Commentf("r/w test #%d failed\nwant=%#v\n got=%#v\n", i, test, &got))
	}
}
