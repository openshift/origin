// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package quat

func lift(v float64) Number {
	return Number{Real: v}
}

func split(q Number) (float64, Number) {
	return q.Real, Number{Imag: q.Imag, Jmag: q.Jmag, Kmag: q.Kmag}
}

func join(w float64, uv Number) Number {
	uv.Real = w
	return uv
}

func unit(q Number) Number {
	return Scale(1/Abs(q), q)
}
