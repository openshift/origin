// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/optimize/functions"
)

func TestListSearch(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	for cas, test := range []struct {
		r, c       int
		shortEvals int
		fun        func([]float64) float64
	}{
		{
			r:   100,
			c:   10,
			fun: functions.ExtendedRosenbrock{}.Func,
		},
	} {
		// Generate a random list of items.
		r, c := test.r, test.c
		locs := mat.NewDense(r, c, nil)
		for i := 0; i < r; i++ {
			for j := 0; j < c; j++ {
				locs.Set(i, j, rnd.NormFloat64())
			}
		}

		// Evaluate all of the items in the list and find the minimum value.
		fs := make([]float64, r)
		for i := 0; i < r; i++ {
			fs[i] = test.fun(locs.RawRowView(i))
		}
		minIdx := floats.MinIdx(fs)

		// Check that the global minimum is found under normal conditions.
		p := Problem{Func: test.fun}
		method := &ListSearch{
			Locs: locs,
		}
		settings := &Settings{}
		initX := make([]float64, c)
		result, err := Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != MethodConverge {
			t.Errorf("cas %v: status should be MethodConverge", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdx)) {
			t.Errorf("cas %v: did not find minimum of whole list", cas)
		}

		// Check that the optimization works concurrently.
		concurrent := 6
		settings.Concurrent = concurrent
		result, err = Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != MethodConverge {
			t.Errorf("cas %v: status should be MethodConverge", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdx)) {
			t.Errorf("cas %v: did not find minimum of whole list concurrent", cas)
		}

		// Check that the optimization works concurrently with more than the number of samples.
		settings.Concurrent = test.r + concurrent
		result, err = Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != MethodConverge {
			t.Errorf("cas %v: status should be MethodConverge", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdx)) {
			t.Errorf("cas %v: did not find minimum of whole list concurrent", cas)
		}

		// Check that cleanup happens properly by setting the minimum location
		// to the last sample.
		swapSamples(locs, fs, minIdx, test.r-1)
		minIdx = test.r - 1
		settings.Concurrent = concurrent
		result, err = Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != MethodConverge {
			t.Errorf("cas %v: status should be MethodConverge", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdx)) {
			t.Errorf("cas %v: did not find minimum of whole list last sample", cas)
		}

		// Test that the correct optimum is found when the optimization ends early.
		// Note that the above test swapped the list minimum to the last sample,
		// so it's guaranteed that the minimum of the shortened list is not the
		// same as the minimum of the whole list.
		evals := test.r / 3
		minIdxFirst := floats.MinIdx(fs[:evals])
		settings.Concurrent = 0
		settings.FuncEvaluations = evals
		result, err = Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != FunctionEvaluationLimit {
			t.Errorf("cas %v: status was not FunctionEvaluationLimit", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdxFirst)) {
			t.Errorf("cas %v: did not find minimum of shortened list serial", cas)
		}

		// Test the same but concurrently. We can't guarantee a specific number
		// of function evaluations concurrently, so make sure that the list optimum
		// is not between [evals:evals+concurrent]
		for floats.MinIdx(fs[:evals]) != floats.MinIdx(fs[:evals+concurrent]) {
			// Swap the minimum index with a random element.
			minIdxFirst := floats.MinIdx(fs[:evals+concurrent])
			new := rnd.Intn(evals)
			swapSamples(locs, fs, minIdxFirst, new)
		}

		minIdxFirst = floats.MinIdx(fs[:evals])
		settings.Concurrent = concurrent
		result, err = Minimize(p, initX, settings, method)
		if err != nil {
			t.Errorf("cas %v: error optimizing: %s", cas, err)
		}
		if result.Status != FunctionEvaluationLimit {
			t.Errorf("cas %v: status was not FunctionEvaluationLimit", cas)
		}
		if !floats.Equal(result.X, locs.RawRowView(minIdxFirst)) {
			t.Errorf("cas %v: did not find minimum of shortened list concurrent", cas)
		}
	}
}

func swapSamples(m *mat.Dense, f []float64, i, j int) {
	f[i], f[j] = f[j], f[i]
	row := mat.Row(nil, i, m)
	m.SetRow(i, m.RawRowView(j))
	m.SetRow(j, row)
}
