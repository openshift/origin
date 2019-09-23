// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"gonum.org/v1/gonum/stat/distmv"
)

// GuessAndCheck is a global optimizer that evaluates the function at random
// locations. Not a good optimizer, but useful for comparison and debugging.
type GuessAndCheck struct {
	Rander distmv.Rander

	bestF float64
	bestX []float64
}

func (g *GuessAndCheck) Needs() struct{ Gradient, Hessian bool } {
	return struct{ Gradient, Hessian bool }{false, false}
}

func (g *GuessAndCheck) Init(dim, tasks int) int {
	if dim <= 0 {
		panic(nonpositiveDimension)
	}
	if tasks < 0 {
		panic(negativeTasks)
	}
	g.bestF = math.Inf(1)
	g.bestX = resize(g.bestX, dim)
	return tasks
}

func (g *GuessAndCheck) sendNewLoc(operation chan<- Task, task Task) {
	g.Rander.Rand(task.X)
	task.Op = FuncEvaluation
	operation <- task
}

func (g *GuessAndCheck) updateMajor(operation chan<- Task, task Task) {
	// Update the best value seen so far, and send a MajorIteration.
	if task.F < g.bestF {
		g.bestF = task.F
		copy(g.bestX, task.X)
	} else {
		task.F = g.bestF
		copy(task.X, g.bestX)
	}
	task.Op = MajorIteration
	operation <- task
}

func (g *GuessAndCheck) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	// Send initial tasks to evaluate
	for _, task := range tasks {
		g.sendNewLoc(operation, task)
	}

	// Read from the channel until PostIteration is sent.
Loop:
	for {
		task := <-result
		switch task.Op {
		default:
			panic("unknown operation")
		case PostIteration:
			break Loop
		case MajorIteration:
			g.sendNewLoc(operation, task)
		case FuncEvaluation:
			g.updateMajor(operation, task)
		}
	}

	// PostIteration was sent. Update the best new values.
	for task := range result {
		switch task.Op {
		default:
			panic("unknown operation")
		case MajorIteration:
		case FuncEvaluation:
			g.updateMajor(operation, task)
		}
	}
	close(operation)
}
