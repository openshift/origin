// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import "gonum.org/v1/gonum/floats"

var (
	_ Method      = (*GradientDescent)(nil)
	_ localMethod = (*GradientDescent)(nil)
)

// GradientDescent implements the steepest descent optimization method that
// performs successive steps along the direction of the negative gradient.
type GradientDescent struct {
	// Linesearcher selects suitable steps along the descent direction.
	// If Linesearcher is nil, a reasonable default will be chosen.
	Linesearcher Linesearcher
	// StepSizer determines the initial step size along each direction.
	// If StepSizer is nil, a reasonable default will be chosen.
	StepSizer StepSizer
	// GradStopThreshold sets the threshold for stopping if the gradient norm
	// gets too small. If GradStopThreshold is 0 it is defaulted to 1e-12, and
	// if it is NaN the setting is not used.
	GradStopThreshold float64

	ls *LinesearchMethod

	status Status
	err    error
}

func (g *GradientDescent) Status() (Status, error) {
	return g.status, g.err
}

func (*GradientDescent) Uses(has Available) (uses Available, err error) {
	return has.gradient()
}

func (g *GradientDescent) Init(dim, tasks int) int {
	g.status = NotTerminated
	g.err = nil
	return 1
}

func (g *GradientDescent) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	g.status, g.err = localOptimizer{}.run(g, g.GradStopThreshold, operation, result, tasks)
	close(operation)
	return
}

func (g *GradientDescent) initLocal(loc *Location) (Operation, error) {
	if g.Linesearcher == nil {
		g.Linesearcher = &Backtracking{}
	}
	if g.StepSizer == nil {
		g.StepSizer = &QuadraticStepSize{}
	}

	if g.ls == nil {
		g.ls = &LinesearchMethod{}
	}
	g.ls.Linesearcher = g.Linesearcher
	g.ls.NextDirectioner = g

	return g.ls.Init(loc)
}

func (g *GradientDescent) iterateLocal(loc *Location) (Operation, error) {
	return g.ls.Iterate(loc)
}

func (g *GradientDescent) InitDirection(loc *Location, dir []float64) (stepSize float64) {
	copy(dir, loc.Gradient)
	floats.Scale(-1, dir)
	return g.StepSizer.Init(loc, dir)
}

func (g *GradientDescent) NextDirection(loc *Location, dir []float64) (stepSize float64) {
	copy(dir, loc.Gradient)
	floats.Scale(-1, dir)
	return g.StepSizer.StepSize(loc, dir)
}

func (*GradientDescent) needs() struct {
	Gradient bool
	Hessian  bool
} {
	return struct {
		Gradient bool
		Hessian  bool
	}{true, false}
}
