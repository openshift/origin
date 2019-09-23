// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"errors"
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/mat"
)

const defaultGradientAbsTol = 1e-6

// Operation represents the set of operations commanded by Method at each
// iteration. It is a bitmap of various Iteration and Evaluation constants.
// Individual constants must NOT be combined together by the binary OR operator
// except for the Evaluation operations.
type Operation uint64

// Supported Operations.
const (
	// NoOperation specifies that no evaluation or convergence check should
	// take place.
	NoOperation Operation = 0
	// InitIteration is sent to Recorder to indicate the initial location.
	// All fields of the location to record must be valid.
	// Method must not return it.
	InitIteration Operation = 1 << (iota - 1)
	// PostIteration is sent to Recorder to indicate the final location
	// reached during an optimization run.
	// All fields of the location to record must be valid.
	// Method must not return it.
	PostIteration
	// MajorIteration indicates that the next candidate location for
	// an optimum has been found and convergence should be checked.
	MajorIteration
	// MethodDone declares that the method is done running. A method must
	// be a Statuser in order to use this iteration, and after returning
	// MethodDone, the Status must return other than NotTerminated.
	MethodDone
	// FuncEvaluation specifies that the objective function
	// should be evaluated.
	FuncEvaluation
	// GradEvaluation specifies that the gradient
	// of the objective function should be evaluated.
	GradEvaluation
	// HessEvaluation specifies that the Hessian
	// of the objective function should be evaluated.
	HessEvaluation
	// signalDone is used internally to signal completion.
	signalDone

	// Mask for the evaluating operations.
	evalMask = FuncEvaluation | GradEvaluation | HessEvaluation
)

func (op Operation) isEvaluation() bool {
	return op&evalMask != 0 && op&^evalMask == 0
}

func (op Operation) String() string {
	if op&evalMask != 0 {
		return fmt.Sprintf("Evaluation(Func: %t, Grad: %t, Hess: %t, Extra: 0b%b)",
			op&FuncEvaluation != 0,
			op&GradEvaluation != 0,
			op&HessEvaluation != 0,
			op&^(evalMask))
	}
	s, ok := operationNames[op]
	if ok {
		return s
	}
	return fmt.Sprintf("Operation(%d)", op)
}

var operationNames = map[Operation]string{
	NoOperation:    "NoOperation",
	InitIteration:  "InitIteration",
	MajorIteration: "MajorIteration",
	PostIteration:  "PostIteration",
	MethodDone:     "MethodDone",
	signalDone:     "signalDone",
}

// Location represents a location in the optimization procedure.
type Location struct {
	X        []float64
	F        float64
	Gradient []float64
	Hessian  *mat.SymDense
}

// Result represents the answer of an optimization run. It contains the optimum
// location as well as the Status at convergence and Statistics taken during the
// run.
type Result struct {
	Location
	Stats
	Status Status
}

// Stats contains the statistics of the run.
type Stats struct {
	MajorIterations int           // Total number of major iterations
	FuncEvaluations int           // Number of evaluations of Func
	GradEvaluations int           // Number of evaluations of Grad
	HessEvaluations int           // Number of evaluations of Hess
	Runtime         time.Duration // Total runtime of the optimization
}

// complementEval returns an evaluating operation that evaluates fields of loc
// not evaluated by eval.
func complementEval(loc *Location, eval Operation) (complEval Operation) {
	if eval&FuncEvaluation == 0 {
		complEval = FuncEvaluation
	}
	if loc.Gradient != nil && eval&GradEvaluation == 0 {
		complEval |= GradEvaluation
	}
	if loc.Hessian != nil && eval&HessEvaluation == 0 {
		complEval |= HessEvaluation
	}
	return complEval
}

// Problem describes the optimization problem to be solved.
type Problem struct {
	// Func evaluates the objective function at the given location. Func
	// must not modify x.
	Func func(x []float64) float64

	// Grad evaluates the gradient at x and stores the result in-place in grad.
	// Grad must not modify x.
	Grad func(grad []float64, x []float64)

	// Hess evaluates the Hessian at x and stores the result in-place in hess.
	// Hess must not modify x.
	Hess func(hess mat.MutableSymmetric, x []float64)

	// Status reports the status of the objective function being optimized and any
	// error. This can be used to terminate early, for example when the function is
	// not able to evaluate itself. The user can use one of the pre-provided Status
	// constants, or may call NewStatus to create a custom Status value.
	Status func() (Status, error)
}

// TODO(btracey): Think about making this an exported function when the
// constraint interface is designed.
func (p Problem) satisfies(method Needser) error {
	if method.Needs().Gradient && p.Grad == nil {
		return errors.New("optimize: problem does not provide needed Grad function")
	}
	if method.Needs().Hessian && p.Hess == nil {
		return errors.New("optimize: problem does not provide needed Hess function")
	}
	return nil
}

// Settings represents settings of the optimization run. It contains initial
// settings, convergence information, and Recorder information. In general, users
// should use DefaultSettings rather than constructing a Settings literal.
//
// If Recorder is nil, no information will be recorded.
type Settings struct {
	// InitValues specifies properties (function value, gradient, etc.) known
	// at the initial location passed to Minimize. If InitValues is non-nil, then
	// the function value F must be provided, the location X must not be specified
	// and other fields may be specified.
	InitValues *Location

	// FunctionThreshold is the threshold for acceptably small values of the
	// objective function. FunctionThreshold status is returned if
	// the objective function is less than this value.
	// The default value is -inf.
	FunctionThreshold float64

	// GradientThreshold determines the accuracy to which the minimum is found.
	// GradientThreshold status is returned if the infinity norm of
	// the gradient is less than this value.
	// Has no effect if gradient information is not used.
	// The default value is 1e-6.
	GradientThreshold float64

	// FunctionConverge tests that the function value decreases by a
	// significant amount over the specified number of iterations.
	//
	// If f < f_best and
	//  f_best - f > FunctionConverge.Relative * maxabs(f, f_best) + FunctionConverge.Absolute
	// then a significant decrease has occurred, and f_best is updated.
	//
	// If there is no significant decrease for FunctionConverge.Iterations
	// major iterations, FunctionConvergence status is returned.
	//
	// If this is nil or if FunctionConverge.Iterations == 0, it has no effect.
	FunctionConverge *FunctionConverge

	// MajorIterations is the maximum number of iterations allowed.
	// IterationLimit status is returned if the number of major iterations
	// equals or exceeds this value.
	// If it equals zero, this setting has no effect.
	// The default value is 0.
	MajorIterations int

	// Runtime is the maximum runtime allowed. RuntimeLimit status is returned
	// if the duration of the run is longer than this value. Runtime is only
	// checked at MajorIterations of the Method.
	// If it equals zero, this setting has no effect.
	// The default value is 0.
	Runtime time.Duration

	// FuncEvaluations is the maximum allowed number of function evaluations.
	// FunctionEvaluationLimit status is returned if the total number of calls
	// to Func equals or exceeds this number.
	// If it equals zero, this setting has no effect.
	// The default value is 0.
	FuncEvaluations int

	// GradEvaluations is the maximum allowed number of gradient evaluations.
	// GradientEvaluationLimit status is returned if the total number of calls
	// to Grad equals or exceeds this number.
	// If it equals zero, this setting has no effect.
	// The default value is 0.
	GradEvaluations int

	// HessEvaluations is the maximum allowed number of Hessian evaluations.
	// HessianEvaluationLimit status is returned if the total number of calls
	// to Hess equals or exceeds this number.
	// If it equals zero, this setting has no effect.
	// The default value is 0.
	HessEvaluations int

	Recorder Recorder

	// Concurrent represents how many concurrent evaluations are possible.
	Concurrent int
}

// DefaultSettingsLocal returns a new Settings struct that contains default settings
// for running a local optimization.
func DefaultSettingsLocal() *Settings {
	return &Settings{
		GradientThreshold: defaultGradientAbsTol,
		FunctionThreshold: math.Inf(-1),
		FunctionConverge: &FunctionConverge{
			Absolute:   1e-10,
			Iterations: 20,
		},
	}
}

// resize takes x and returns a slice of length dim. It returns a resliced x
// if cap(x) >= dim, and a new slice otherwise.
func resize(x []float64, dim int) []float64 {
	if dim > cap(x) {
		return make([]float64, dim)
	}
	return x[:dim]
}

func resizeSymDense(m *mat.SymDense, dim int) *mat.SymDense {
	if m == nil || cap(m.RawSymmetric().Data) < dim*dim {
		return mat.NewSymDense(dim, nil)
	}
	return mat.NewSymDense(dim, m.RawSymmetric().Data[:dim*dim])
}
