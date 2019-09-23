// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"testing"

	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/functions"
	"gonum.org/v1/gonum/stat/distmv"
)

func TestGuessAndCheck(t *testing.T) {
	dim := 30
	problem := Problem{
		Func: functions.ExtendedRosenbrock{}.Func,
	}
	mu := make([]float64, dim)
	sigma := mat.NewSymDense(dim, nil)
	for i := 0; i < dim; i++ {
		sigma.SetSym(i, i, 1)
	}
	d, ok := distmv.NewNormal(mu, sigma, nil)
	if !ok {
		panic("bad test")
	}
	initX := make([]float64, dim)
	Minimize(problem, initX, nil, &GuessAndCheck{Rander: d})

	settings := DefaultSettingsGlobal()
	settings.Concurrent = 5
	settings.MajorIterations = 15
	Minimize(problem, initX, settings, &GuessAndCheck{Rander: d})
}
