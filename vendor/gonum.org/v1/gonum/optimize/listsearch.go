// Copyright ©2018 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"

	"gonum.org/v1/gonum/mat"
)

// ListSearch finds the optimum location from a specified list of possible
// optimum locations.
type ListSearch struct {
	// Locs is the list of locations to optimize. Each row of Locs is a location
	// to optimize. The number of columns of Locs must match the dimensions
	// passed to InitGlobal, and Locs must have at least one row.
	Locs mat.Matrix

	eval    int
	rows    int
	bestF   float64
	bestIdx int
}

func (*ListSearch) Needs() struct{ Gradient, Hessian bool } {
	return struct{ Gradient, Hessian bool }{false, false}
}

// InitGlobal initializes the method for optimization. The input dimension
// must match the number of columns of Locs.
func (l *ListSearch) Init(dim, tasks int) int {
	if dim <= 0 {
		panic(nonpositiveDimension)
	}
	if tasks < 0 {
		panic(negativeTasks)
	}
	r, c := l.Locs.Dims()
	if r == 0 {
		panic("listsearch: list matrix has no rows")
	}
	if c != dim {
		panic("listsearch: supplied dimension does not match list columns")
	}
	l.eval = 0
	l.rows = r
	l.bestF = math.Inf(1)
	l.bestIdx = -1
	return min(r, tasks)
}

func (l *ListSearch) sendNewLoc(operation chan<- Task, task Task) {
	task.Op = FuncEvaluation
	task.ID = l.eval
	mat.Row(task.X, l.eval, l.Locs)
	l.eval++
	operation <- task
}

func (l *ListSearch) updateMajor(operation chan<- Task, task Task) {
	// Update the best value seen so far, and send a MajorIteration.
	if task.F < l.bestF {
		l.bestF = task.F
		l.bestIdx = task.ID
	} else {
		task.F = l.bestF
		mat.Row(task.X, l.bestIdx, l.Locs)
	}
	task.Op = MajorIteration
	operation <- task
}

func (l *ListSearch) Status() (Status, error) {
	if l.eval < l.rows {
		return NotTerminated, nil
	}
	return MethodConverge, nil
}

func (l *ListSearch) Run(operation chan<- Task, result <-chan Task, tasks []Task) {
	// Send initial tasks to evaluate
	for _, task := range tasks {
		l.sendNewLoc(operation, task)
	}
	// Read from the channel until PostIteration is sent or until the list of
	// tasks is exhausted.
Loop:
	for {
		task := <-result
		switch task.Op {
		default:
			panic("unknown operation")
		case PostIteration:
			break Loop
		case MajorIteration:
			if l.eval == l.rows {
				task.Op = MethodDone
				operation <- task
				continue
			}
			l.sendNewLoc(operation, task)
		case FuncEvaluation:
			l.updateMajor(operation, task)
		}
	}

	// Post iteration was sent, or the list has been completed. Read in the final
	// list of tasks.
	for task := range result {
		switch task.Op {
		default:
			panic("unknown operation")
		case MajorIteration:
		case FuncEvaluation:
			l.updateMajor(operation, task)
		}
	}
	close(operation)
}
