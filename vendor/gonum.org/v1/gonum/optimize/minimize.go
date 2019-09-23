// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"fmt"
	"math"
	"time"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
)

const (
	nonpositiveDimension string = "optimize: non-positive input dimension"
	negativeTasks        string = "optimize: negative input number of tasks"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Task is a type to communicate between the Method and the outer
// calling script.
type Task struct {
	ID int
	Op Operation
	*Location
}

// Location represents a location in the optimization procedure.
type Location struct {
	X        []float64
	F        float64
	Gradient []float64
	Hessian  mat.Symmetric
}

// Method is a type which can search for an optimum of an objective function.
type Method interface {
	// Init initializes the method for optimization. The inputs are
	// the problem dimension and number of available concurrent tasks.
	//
	// Init returns the number of concurrent processes to use, which must be
	// less than or equal to tasks.
	Init(dim, tasks int) (concurrent int)
	// Run runs an optimization. The method sends Tasks on
	// the operation channel (for performing function evaluations, major
	// iterations, etc.). The result of the tasks will be returned on Result.
	// See the documentation for Operation types for the possible operations.
	//
	// The caller of Run will signal the termination of the optimization
	// (i.e. convergence from user settings) by sending a task with a PostIteration
	// Op field on result. More tasks may still be sent on operation after this
	// occurs, but only MajorIteration operations will still be conducted
	// appropriately. Thus, it can not be guaranteed that all Evaluations sent
	// on operation will be evaluated, however if an Evaluation is started,
	// the results of that evaluation will be sent on results.
	//
	// The Method must read from the result channel until it is closed.
	// During this, the Method may want to send new MajorIteration(s) on
	// operation. Method then must close operation, and return from Run.
	// These steps must establish a "happens-before" relationship between result
	// being closed (externally) and Run closing operation, for example
	// by using a range loop to read from result even if no results are expected.
	//
	// The last parameter to Run is a slice of tasks with length equal to
	// the return from Init. Task has an ID field which may be
	// set and modified by Method, and must not be modified by the caller.
	// The first element of tasks contains information about the initial location.
	// The Location.X field is always valid. The Operation field specifies which
	// other values of Location are known. If Operation == NoOperation, none of
	// the values should be used, otherwise the Evaluation operations will be
	// composed to specify the valid fields. Methods are free to use or
	// ignore these values.
	//
	// Successful execution of an Operation may require the Method to modify
	// fields a Location. MajorIteration calls will not modify the values in
	// the Location, but Evaluation operations will. Methods are encouraged to
	// leave Location fields untouched to allow memory re-use. If data needs to
	// be stored, the respective field should be set to nil -- Methods should
	// not allocate Location memory themselves.
	//
	// Method may have its own specific convergence criteria, which can
	// be communicated using a MethodDone operation. This will trigger a
	// PostIteration to be sent on result, and the MethodDone task will not be
	// returned on result. The Method must implement Statuser, and the
	// call to Status must return a Status other than NotTerminated.
	//
	// The operation and result tasks are guaranteed to have a buffer length
	// equal to the return from Init.
	Run(operation chan<- Task, result <-chan Task, tasks []Task)
	// Uses checks if the Method is suited to the optimization problem. The
	// input is the available functions in Problem to call, and the returns are
	// the functions which may be used and an error if there is a mismatch
	// between the Problem and the Method's capabilities.
	Uses(has Available) (uses Available, err error)
}

// Minimize uses an optimizer to search for a minimum of a function. A
// maximization problem can be transformed into a minimization problem by
// multiplying the function by -1.
//
// The first argument represents the problem to be minimized. Its fields are
// routines that evaluate the objective function, gradient, and other
// quantities related to the problem. The objective function, p.Func, must not
// be nil. The optimization method used may require other fields to be non-nil
// as specified by method.Needs. Minimize will panic if these are not met. The
// method can be determined automatically from the supplied problem which is
// described below.
//
// If p.Status is not nil, it is called before every evaluation. If the
// returned Status is other than NotTerminated or if the error is not nil, the
// optimization run is terminated.
//
// The second argument specifies the initial location for the optimization.
// Some Methods do not require an initial location, but initX must still be
// specified for the dimension of the optimization problem.
//
// The third argument contains the settings for the minimization. If settings
// is nil, the zero value will be used, see the documentation of the Settings
// type for more information, and see the warning below. All settings will be
// honored for all Methods, even if that setting is counter-productive to the
// method. Minimize cannot guarantee strict adherence to the evaluation bounds
// specified when performing concurrent evaluations and updates.
//
// The final argument is the optimization method to use. If method == nil, then
// an appropriate default is chosen based on the properties of the other arguments
// (dimension, gradient-free or gradient-based, etc.). If method is not nil,
// Minimize panics if the Problem is not consistent with the Method (Uses
// returns an error).
//
// Minimize returns a Result struct and any error that occurred. See the
// documentation of Result for more information.
//
// See the documentation for Method for the details on implementing a method.
//
// Be aware that the default settings of Minimize are to accurately find the
// minimum. For certain functions and optimization methods, this can take many
// function evaluations. The Settings input struct can be used to limit this,
// for example by modifying the maximum function evaluations or gradient tolerance.
func Minimize(p Problem, initX []float64, settings *Settings, method Method) (*Result, error) {
	startTime := time.Now()
	if method == nil {
		method = getDefaultMethod(&p)
	}
	if settings == nil {
		settings = &Settings{}
	}
	stats := &Stats{}
	dim := len(initX)
	err := checkOptimization(p, dim, settings.Recorder)
	if err != nil {
		return nil, err
	}

	optLoc := newLocation(dim) // This must have an allocated X field.
	optLoc.F = math.Inf(1)

	initOp, initLoc := getInitLocation(dim, initX, settings.InitValues)

	converger := settings.Converger
	if converger == nil {
		converger = defaultFunctionConverge()
	}
	converger.Init(dim)

	stats.Runtime = time.Since(startTime)

	// Send initial location to Recorder
	if settings.Recorder != nil {
		err = settings.Recorder.Record(optLoc, InitIteration, stats)
		if err != nil {
			return nil, err
		}
	}

	// Run optimization
	var status Status
	status, err = minimize(&p, method, settings, converger, stats, initOp, initLoc, optLoc, startTime)

	// Cleanup and collect results
	if settings.Recorder != nil && err == nil {
		err = settings.Recorder.Record(optLoc, PostIteration, stats)
	}
	stats.Runtime = time.Since(startTime)
	return &Result{
		Location: *optLoc,
		Stats:    *stats,
		Status:   status,
	}, err
}

func getDefaultMethod(p *Problem) Method {
	if p.Grad != nil {
		return &LBFGS{}
	}
	return &NelderMead{}
}

// minimize performs an optimization. minimize updates the settings and optLoc,
// and returns the final Status and error.
func minimize(prob *Problem, method Method, settings *Settings, converger Converger, stats *Stats, initOp Operation, initLoc, optLoc *Location, startTime time.Time) (Status, error) {
	dim := len(optLoc.X)
	nTasks := settings.Concurrent
	if nTasks == 0 {
		nTasks = 1
	}
	has := availFromProblem(*prob)
	_, initErr := method.Uses(has)
	if initErr != nil {
		panic(fmt.Sprintf("optimize: specified method inconsistent with Problem: %v", initErr))
	}
	newNTasks := method.Init(dim, nTasks)
	if newNTasks > nTasks {
		panic("optimize: too many tasks returned by Method")
	}
	nTasks = newNTasks

	// Launch the method. The method communicates tasks using the operations
	// channel, and results is used to return the evaluated results.
	operations := make(chan Task, nTasks)
	results := make(chan Task, nTasks)
	go func() {
		tasks := make([]Task, nTasks)
		tasks[0].Location = initLoc
		tasks[0].Op = initOp
		for i := 1; i < len(tasks); i++ {
			tasks[i].Location = newLocation(dim)
		}
		method.Run(operations, results, tasks)
	}()

	// Algorithmic Overview:
	// There are three pieces to performing a concurrent optimization,
	// the distributor, the workers, and the stats combiner. At a high level,
	// the distributor reads in tasks sent by method, sending evaluations to the
	// workers, and forwarding other operations to the statsCombiner. The workers
	// read these forwarded evaluation tasks, evaluate the relevant parts of Problem
	// and forward the results on to the stats combiner. The stats combiner reads
	// in results from the workers, as well as tasks from the distributor, and
	// uses them to update optimization statistics (function evaluations, etc.)
	// and to check optimization convergence.
	//
	// The complicated part is correctly shutting down the optimization. The
	// procedure is as follows. First, the stats combiner closes done and sends
	// a PostIteration to the method. The distributor then reads that done has
	// been closed, and closes the channel with the workers. At this point, no
	// more evaluation operations will be executed. As the workers finish their
	// evaluations, they forward the results onto the stats combiner, and then
	// signal their shutdown to the stats combiner. When all workers have successfully
	// finished, the stats combiner closes the results channel, signaling to the
	// method that all results have been collected. At this point, the method
	// may send MajorIteration(s) to update an optimum location based on these
	// last returned results, and then the method will close the operations channel.
	// The Method must ensure that the closing of results happens before the
	// closing of operations in order to ensure proper shutdown order.
	// Now that no more tasks will be commanded by the method, the distributor
	// closes statsChan, and with no more statistics to update the optimization
	// concludes.

	workerChan := make(chan Task) // Delegate tasks to the workers.
	statsChan := make(chan Task)  // Send evaluation updates.
	done := make(chan struct{})   // Communicate the optimization is done.

	// Read tasks from the method and distribute as appropriate.
	distributor := func() {
		for {
			select {
			case task := <-operations:
				switch task.Op {
				case InitIteration:
					panic("optimize: Method returned InitIteration")
				case PostIteration:
					panic("optimize: Method returned PostIteration")
				case NoOperation, MajorIteration, MethodDone:
					statsChan <- task
				default:
					if !task.Op.isEvaluation() {
						panic("optimize: expecting evaluation operation")
					}
					workerChan <- task
				}
			case <-done:
				// No more evaluations will be sent, shut down the workers, and
				// read the final tasks.
				close(workerChan)
				for task := range operations {
					if task.Op == MajorIteration {
						statsChan <- task
					}
				}
				close(statsChan)
				return
			}
		}
	}
	go distributor()

	// Evaluate the Problem concurrently.
	worker := func() {
		x := make([]float64, dim)
		for task := range workerChan {
			evaluate(prob, task.Location, task.Op, x)
			statsChan <- task
		}
		// Signal successful worker completion.
		statsChan <- Task{Op: signalDone}
	}
	for i := 0; i < nTasks; i++ {
		go worker()
	}

	var (
		workersDone int // effective wg for the workers
		status      Status
		err         error
		finalStatus Status
		finalError  error
	)

	// Update optimization statistics and check convergence.
	var methodDone bool
	for task := range statsChan {
		switch task.Op {
		default:
			if !task.Op.isEvaluation() {
				panic("minimize: evaluation task expected")
			}
			updateEvaluationStats(stats, task.Op)
			status, err = checkEvaluationLimits(prob, stats, settings)
		case signalDone:
			workersDone++
			if workersDone == nTasks {
				close(results)
			}
			continue
		case NoOperation:
			// Just send the task back.
		case MajorIteration:
			status = performMajorIteration(optLoc, task.Location, stats, converger, startTime, settings)
		case MethodDone:
			methodDone = true
			status = MethodConverge
		}
		if settings.Recorder != nil && status == NotTerminated && err == nil {
			stats.Runtime = time.Since(startTime)
			// Allow err to be overloaded if the Recorder fails.
			err = settings.Recorder.Record(task.Location, task.Op, stats)
			if err != nil {
				status = Failure
			}
		}
		// If this is the first termination status, trigger the conclusion of
		// the optimization.
		if status != NotTerminated || err != nil {
			select {
			case <-done:
			default:
				finalStatus = status
				finalError = err
				results <- Task{
					Op: PostIteration,
				}
				close(done)
			}
		}

		// Send the result back to the Problem if there are still active workers.
		if workersDone != nTasks && task.Op != MethodDone {
			results <- task
		}
	}
	// This code block is here rather than above to ensure Status() is not called
	// before Method.Run closes operations.
	if methodDone {
		statuser, ok := method.(Statuser)
		if !ok {
			panic("optimize: method returned MethodDone but is not a Statuser")
		}
		finalStatus, finalError = statuser.Status()
		if finalStatus == NotTerminated {
			panic("optimize: method returned MethodDone but a NotTerminated status")
		}
	}
	return finalStatus, finalError
}

func defaultFunctionConverge() *FunctionConverge {
	return &FunctionConverge{
		Absolute:   1e-10,
		Iterations: 100,
	}
}

// newLocation allocates a new locatian structure with an X field of the
// appropriate size.
func newLocation(dim int) *Location {
	return &Location{
		X: make([]float64, dim),
	}
}

// getInitLocation checks the validity of initLocation and initOperation and
// returns the initial values as a *Location.
func getInitLocation(dim int, initX []float64, initValues *Location) (Operation, *Location) {
	loc := newLocation(dim)
	if initX == nil {
		if initValues != nil {
			panic("optimize: initValues is non-nil but no initial location specified")
		}
		return NoOperation, loc
	}
	copy(loc.X, initX)
	if initValues == nil {
		return NoOperation, loc
	} else {
		if initValues.X != nil {
			panic("optimize: location specified in InitValues (only use InitX)")
		}
	}
	loc.F = initValues.F
	op := FuncEvaluation
	if initValues.Gradient != nil {
		if len(initValues.Gradient) != dim {
			panic("optimize: initial gradient does not match problem dimension")
		}
		loc.Gradient = initValues.Gradient
		op |= GradEvaluation
	}
	if initValues.Hessian != nil {
		if initValues.Hessian.Symmetric() != dim {
			panic("optimize: initial Hessian does not match problem dimension")
		}
		loc.Hessian = initValues.Hessian
		op |= HessEvaluation
	}
	return op, loc
}

func checkOptimization(p Problem, dim int, recorder Recorder) error {
	if p.Func == nil {
		panic(badProblem)
	}
	if dim <= 0 {
		panic("optimize: impossible problem dimension")
	}
	if p.Status != nil {
		_, err := p.Status()
		if err != nil {
			return err
		}
	}
	if recorder != nil {
		err := recorder.Init()
		if err != nil {
			return err
		}
	}
	return nil
}

// evaluate evaluates the routines specified by the Operation at loc.X, and stores
// the answer into loc. loc.X is copied into x before evaluating in order to
// prevent the routines from modifying it.
func evaluate(p *Problem, loc *Location, op Operation, x []float64) {
	if !op.isEvaluation() {
		panic(fmt.Sprintf("optimize: invalid evaluation %v", op))
	}
	copy(x, loc.X)
	if op&FuncEvaluation != 0 {
		loc.F = p.Func(x)
	}
	if op&GradEvaluation != 0 {
		loc.Gradient = p.Grad(loc.Gradient, x)
	}
	if op&HessEvaluation != 0 {
		loc.Hessian = p.Hess(loc.Hessian, x)
	}
}

// updateEvaluationStats updates the statistics based on the operation.
func updateEvaluationStats(stats *Stats, op Operation) {
	if op&FuncEvaluation != 0 {
		stats.FuncEvaluations++
	}
	if op&GradEvaluation != 0 {
		stats.GradEvaluations++
	}
	if op&HessEvaluation != 0 {
		stats.HessEvaluations++
	}
}

// checkLocationConvergence checks if the current optimal location satisfies
// any of the convergence criteria based on the function location.
//
// checkLocationConvergence returns NotTerminated if the Location does not satisfy
// the convergence criteria given by settings. Otherwise a corresponding status is
// returned.
// Unlike checkLimits, checkConvergence is called only at MajorIterations.
func checkLocationConvergence(loc *Location, settings *Settings, converger Converger) Status {
	if math.IsInf(loc.F, -1) {
		return FunctionNegativeInfinity
	}
	if loc.Gradient != nil && settings.GradientThreshold > 0 {
		norm := floats.Norm(loc.Gradient, math.Inf(1))
		if norm < settings.GradientThreshold {
			return GradientThreshold
		}
	}
	return converger.Converged(loc)
}

// checkEvaluationLimits checks the optimization limits after an evaluation
// Operation. It checks the number of evaluations (of various kinds) and checks
// the status of the Problem, if applicable.
func checkEvaluationLimits(p *Problem, stats *Stats, settings *Settings) (Status, error) {
	if p.Status != nil {
		status, err := p.Status()
		if err != nil || status != NotTerminated {
			return status, err
		}
	}
	if settings.FuncEvaluations > 0 && stats.FuncEvaluations >= settings.FuncEvaluations {
		return FunctionEvaluationLimit, nil
	}
	if settings.GradEvaluations > 0 && stats.GradEvaluations >= settings.GradEvaluations {
		return GradientEvaluationLimit, nil
	}
	if settings.HessEvaluations > 0 && stats.HessEvaluations >= settings.HessEvaluations {
		return HessianEvaluationLimit, nil
	}
	return NotTerminated, nil
}

// checkIterationLimits checks the limits on iterations affected by MajorIteration.
func checkIterationLimits(loc *Location, stats *Stats, settings *Settings) Status {
	if settings.MajorIterations > 0 && stats.MajorIterations >= settings.MajorIterations {
		return IterationLimit
	}
	if settings.Runtime > 0 && stats.Runtime >= settings.Runtime {
		return RuntimeLimit
	}
	return NotTerminated
}

// performMajorIteration does all of the steps needed to perform a MajorIteration.
// It increments the iteration count, updates the optimal location, and checks
// the necessary convergence criteria.
func performMajorIteration(optLoc, loc *Location, stats *Stats, converger Converger, startTime time.Time, settings *Settings) Status {
	optLoc.F = loc.F
	copy(optLoc.X, loc.X)
	if loc.Gradient == nil {
		optLoc.Gradient = nil
	} else {
		if optLoc.Gradient == nil {
			optLoc.Gradient = make([]float64, len(loc.Gradient))
		}
		copy(optLoc.Gradient, loc.Gradient)
	}
	stats.MajorIterations++
	stats.Runtime = time.Since(startTime)
	status := checkLocationConvergence(optLoc, settings, converger)
	if status != NotTerminated {
		return status
	}
	return checkIterationLimits(optLoc, stats, settings)
}
