// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"
	"time"
)

// DefaultSettingsGlobal returns the default settings for a global optimization.
func DefaultSettingsGlobal() *Settings {
	return &Settings{
		FunctionThreshold: math.Inf(-1),
		FunctionConverge: &FunctionConverge{
			Absolute:   1e-10,
			Iterations: 100,
		},
	}
}

// Task is a type to communicate between the Method and the outer
// calling script.
type Task struct {
	ID int
	Op Operation
	*Location
}

// Method is a type which can search for an optimum of an objective function.
type Method interface {
	Needser
	// Init takes as input the problem dimension and number of available
	// concurrent tasks, and returns the number of concurrent processes to be used.
	// The returned value must be less than or equal to tasks.
	Init(dim, tasks int) int
	// Run runs an optimization. The method sends Tasks on
	// the operation channel (for performing function evaluations, major
	// iterations, etc.). The result of the tasks will be returned on Result.
	// See the documentation for Operation types for the possible tasks.
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
	// Method may have its own specific convergence criteria, which can
	// be communicated using a MethodDone operation. This will trigger a
	// PostIteration to be sent on result, and the MethodDone task will not be
	// returned on result. The Method must implement Statuser, and the
	// call to Status must return a Status other than NotTerminated.
	//
	// The operation and result tasks are guaranteed to have a buffer length
	// equal to the return from Init.
	Run(operation chan<- Task, result <-chan Task, tasks []Task)
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
// The third argument contains the settings for the minimization. The
// DefaultSettingsLocal and DefaultSettingsGlobal functions can be called for
// different default settings depending on the optimization method. If
// settings is nil, DefaultSettingsLocal is used. All settings will be honored
// for all Methods, even if that setting is counter-productive to the method.
// However, the information used to check the Settings, and the times at which
// they are checked, are controlled by the Method. For example, if the Method
// never evaluates the gradient of the function then GradientThreshold will not
// be checked. Minimize cannot guarantee strict adherence to the bounds
// specified when performing concurrent evaluations and updates.
//
// The final argument is the optimization method to use. If method == nil, then
// an appropriate default is chosen based on the properties of the other arguments
// (dimension, gradient-free or gradient-based, etc.).
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
		settings = DefaultSettingsLocal()
	}
	stats := &Stats{}
	dim := len(initX)
	err := checkOptimization(p, dim, method, settings.Recorder)
	if err != nil {
		return nil, err
	}

	optLoc := newLocation(dim, method)
	optLoc.F = math.Inf(1)

	if settings.FunctionConverge != nil {
		settings.FunctionConverge.Init()
	}

	initOp, initLoc := getInitLocation(dim, initX, settings.InitValues, method)

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
	status, err = minimize(&p, method, settings, stats, initOp, initLoc, optLoc, startTime)

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
		return &BFGS{}
	}
	return &NelderMead{}
}

// minimize performs an optimization. minimize updates the settings and optLoc,
// and returns the final Status and error.
func minimize(prob *Problem, method Method, settings *Settings, stats *Stats, initOp Operation, initLoc, optLoc *Location, startTime time.Time) (Status, error) {
	dim := len(optLoc.X)
	nTasks := settings.Concurrent
	if nTasks == 0 {
		nTasks = 1
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
			tasks[i].Location = newLocation(dim, method)
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
			status = performMajorIteration(optLoc, task.Location, stats, startTime, settings)
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
