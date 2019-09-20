// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"
)

// Converger returns the convergence of the optimization based on
// locations found during optimization. Converger must not modify the value of
// the provided Location in any of the methods.
type Converger interface {
	Init(dim int)
	Converged(loc *Location) Status
}

// NeverTerminate implements Converger, always reporting NotTerminated.
type NeverTerminate struct{}

func (NeverTerminate) Init(dim int) {}

func (NeverTerminate) Converged(loc *Location) Status {
	return NotTerminated
}

// FunctionConverge tests for insufficient improvement in the optimum value
// over the last iterations. A FunctionConvergence status is returned if
// there is no significant decrease for FunctionConverge.Iterations. A
// significant decrease is considered if
//   f < f_best
// and
//  f_best - f > FunctionConverge.Relative * maxabs(f, f_best) + FunctionConverge.Absolute
// If the decrease is significant, then the iteration counter is reset and
// f_best is updated.
//
// If FunctionConverge.Iterations == 0, it has no effect.
type FunctionConverge struct {
	Absolute   float64
	Relative   float64
	Iterations int

	first bool
	best  float64
	iter  int
}

func (fc *FunctionConverge) Init(dim int) {
	fc.first = true
	fc.best = 0
	fc.iter = 0
}

func (fc *FunctionConverge) Converged(l *Location) Status {
	f := l.F
	if fc.first {
		fc.best = f
		fc.first = false
		return NotTerminated
	}
	if fc.Iterations == 0 {
		return NotTerminated
	}
	maxAbs := math.Max(math.Abs(f), math.Abs(fc.best))
	if f < fc.best && fc.best-f > fc.Relative*maxAbs+fc.Absolute {
		fc.best = f
		fc.iter = 0
		return NotTerminated
	}
	fc.iter++
	if fc.iter < fc.Iterations {
		return NotTerminated
	}
	return FunctionConvergence
}
