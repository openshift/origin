// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

// A localMethod can optimize an objective function.
//
// It uses a reverse-communication interface between the optimization method
// and the caller. Method acts as a client that asks the caller to perform
// needed operations via Operation returned from Init and Iterate methods.
// This provides independence of the optimization algorithm on user-supplied
// data and their representation, and enables automation of common operations
// like checking for (various types of) convergence and maintaining statistics.
//
// A Method can command an Evaluation, a MajorIteration or NoOperation operations.
//
// An evaluation operation is one or more of the Evaluation operations
// (FuncEvaluation, GradEvaluation, etc.) which can be combined with
// the bitwise or operator. In an evaluation operation, the requested fields of
// Problem will be evaluated at the point specified in Location.X.
// The corresponding fields of Location will be filled with the results that
// can be retrieved upon the next call to Iterate. The Method interface
// requires that entries of Location are not modified aside from the commanded
// evaluations. Thus, the type implementing Method may use multiple Operations
// to set the Location fields at a particular x value.
//
// Instead of an Evaluation, a Method may declare MajorIteration. In
// a MajorIteration, the values in the fields of Location are treated as
// a potential optimizer. The convergence of the optimization routine
// (GradientThreshold, etc.) is checked at this new best point. In
// a MajorIteration, the fields of Location must be valid and consistent.
//
// A Method must not return InitIteration and PostIteration operations. These are
// reserved for the clients to be passed to Recorders. A Method must also not
// combine the Evaluation operations with the Iteration operations.
type localMethod interface {
	// Init initializes the method based on the initial data in loc, updates it
	// and returns the first operation to be carried out by the caller.
	// The initial location must be valid as specified by Needs.
	initLocal(loc *Location) (Operation, error)

	// Iterate retrieves data from loc, performs one iteration of the method,
	// updates loc and returns the next operation.
	iterateLocal(loc *Location) (Operation, error)

	Needser
}

type Needser interface {
	// Needs specifies information about the objective function needed by the
	// optimizer beyond just the function value. The information is used
	// internally for initialization and must match evaluation types returned
	// by Init and Iterate during the optimization process.
	Needs() struct {
		Gradient bool
		Hessian  bool
	}
}

// Statuser can report the status and any error. It is intended for methods as
// an additional error reporting mechanism apart from the errors returned from
// Init and Iterate.
type Statuser interface {
	Status() (Status, error)
}

// Linesearcher is a type that can perform a line search. It tries to find an
// (approximate) minimum of the objective function along the search direction
// dir_k starting at the most recent location x_k, i.e., it tries to minimize
// the function
//  φ(step) := f(x_k + step * dir_k) where step > 0.
// Typically, a Linesearcher will be used in conjunction with LinesearchMethod
// for performing gradient-based optimization through sequential line searches.
type Linesearcher interface {
	// Init initializes the Linesearcher and a new line search. Value and
	// derivative contain φ(0) and φ'(0), respectively, and step contains the
	// first trial step length. It returns an Operation that must be one of
	// FuncEvaluation, GradEvaluation, FuncEvaluation|GradEvaluation. The
	// caller must evaluate φ(step), φ'(step), or both, respectively, and pass
	// the result to Linesearcher in value and derivative arguments to Iterate.
	Init(value, derivative float64, step float64) Operation

	// Iterate takes in the values of φ and φ' evaluated at the previous step
	// and returns the next operation.
	//
	// If op is one of FuncEvaluation, GradEvaluation,
	// FuncEvaluation|GradEvaluation, the caller must evaluate φ(step),
	// φ'(step), or both, respectively, and pass the result to Linesearcher in
	// value and derivative arguments on the next call to Iterate.
	//
	// If op is MajorIteration, a sufficiently accurate minimum of φ has been
	// found at the previous step and the line search has concluded. Init must
	// be called again to initialize a new line search.
	//
	// If err is nil, op must not specify another operation. If err is not nil,
	// the values of op and step are undefined.
	Iterate(value, derivative float64) (op Operation, step float64, err error)
}

// NextDirectioner implements a strategy for computing a new line search
// direction at each major iteration. Typically, a NextDirectioner will be
// used in conjunction with LinesearchMethod for performing gradient-based
// optimization through sequential line searches.
type NextDirectioner interface {
	// InitDirection initializes the NextDirectioner at the given starting location,
	// putting the initial direction in place into dir, and returning the initial
	// step size. InitDirection must not modify Location.
	InitDirection(loc *Location, dir []float64) (step float64)

	// NextDirection updates the search direction and step size. Location is
	// the location seen at the conclusion of the most recent linesearch. The
	// next search direction is put in place into dir, and the next step size
	// is returned. NextDirection must not modify Location.
	NextDirection(loc *Location, dir []float64) (step float64)
}

// StepSizer can set the next step size of the optimization given the last Location.
// Returned step size must be positive.
type StepSizer interface {
	Init(loc *Location, dir []float64) float64
	StepSize(loc *Location, dir []float64) float64
}

// A Recorder can record the progress of the optimization, for example to print
// the progress to StdOut or to a log file. A Recorder must not modify any data.
type Recorder interface {
	Init() error
	Record(*Location, Operation, *Stats) error
}
