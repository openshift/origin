// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrZeroDimensional signifies an optimization was called with an input of length 0.
	ErrZeroDimensional = errors.New("optimize: zero dimensional input")

	// ErrLinesearcherFailure signifies that a Linesearcher has iterated too
	// many times. This may occur if the gradient tolerance is set too low.
	ErrLinesearcherFailure = errors.New("linesearch: failed to converge")

	// ErrNonDescentDirection signifies that LinesearchMethod has received a
	// search direction from a NextDirectioner in which the function is not
	// decreasing.
	ErrNonDescentDirection = errors.New("linesearch: non-descent search direction")

	// ErrNoProgress signifies that LinesearchMethod cannot make further
	// progress because there is no change in location after Linesearcher step
	// due to floating-point arithmetic.
	ErrNoProgress = errors.New("linesearch: no change in location after Linesearcher step")

	// ErrLinesearcherBound signifies that a Linesearcher reached a step that
	// lies out of allowed bounds.
	ErrLinesearcherBound = errors.New("linesearch: step out of bounds")
)

// ErrFunc is returned when an initial function value is invalid. The error
// state may be either +Inf or NaN. ErrFunc satisfies the error interface.
type ErrFunc float64

func (err ErrFunc) Error() string {
	switch {
	case math.IsInf(float64(err), 1):
		return "optimize: initial function value is infinite"
	case math.IsNaN(float64(err)):
		return "optimize: initial function value is NaN"
	default:
		panic("optimize: bad ErrFunc")
	}
}

// ErrGrad is returned when an initial gradient is invalid. The error gradient
// may be either ±Inf or NaN. ErrGrad satisfies the error interface.
type ErrGrad struct {
	Grad  float64 // Grad is the invalid gradient value.
	Index int     // Index is the position at which the invalid gradient was found.
}

func (err ErrGrad) Error() string {
	switch {
	case math.IsInf(err.Grad, 0):
		return fmt.Sprintf("optimize: initial gradient is infinite at position %d", err.Index)
	case math.IsNaN(err.Grad):
		return fmt.Sprintf("optimize: initial gradient is NaN at position %d", err.Index)
	default:
		panic("optimize: bad ErrGrad")
	}
}

// List of shared panic strings
var (
	badProblem = "optimize: objective function is undefined"
)
