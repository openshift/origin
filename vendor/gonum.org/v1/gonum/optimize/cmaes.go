// Copyright ©2017 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"math"
	"sort"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat/distmv"
)

// TODO(btracey): If we ever implement the traditional CMA-ES algorithm, provide
// the base explanation there, and modify this description to just
// describe the differences.

// CmaEsChol implements the covariance matrix adaptation evolution strategy (CMA-ES)
// based on the Cholesky decomposition. The full algorithm is described in
//  Krause, Oswin, Dídac Rodríguez Arbonès, and Christian Igel. "CMA-ES with
//  optimal covariance update and storage complexity." Advances in Neural
//  Information Processing Systems. 2016.
//  https://papers.nips.cc/paper/6457-cma-es-with-optimal-covariance-update-and-storage-complexity.pdf
// CMA-ES is a global optimization method that progressively adapts a population
// of samples. CMA-ES combines techniques from local optimization with global
// optimization. Specifically, the CMA-ES algorithm uses an initial multivariate
// normal distribution to generate a population of input locations. The input locations
// with the lowest function values are used to update the parameters of the normal
// distribution, a new set of input locations are generated, and this procedure
// is iterated until convergence. The initial sampling distribution will have
// a mean specified by the initial x location, and a covariance specified by
// the InitCholesky field.
//
// As the normal distribution is progressively updated according to the best samples,
// it can be that the mean of the distribution is updated in a gradient-descent
// like fashion, followed by a shrinking covariance.
// It is recommended that the algorithm be run multiple times (with different
// InitMean) to have a better chance of finding the global minimum.
//
// The CMA-ES-Chol algorithm differs from the standard CMA-ES algorithm in that
// it directly updates the Cholesky decomposition of the normal distribution.
// This changes the runtime from O(dimension^3) to O(dimension^2*population)
// The evolution of the multi-variate normal will be similar to the baseline
// CMA-ES algorithm, but the covariance update equation is not identical.
//
// For more information about the CMA-ES algorithm, see
//  https://en.wikipedia.org/wiki/CMA-ES
//  https://arxiv.org/pdf/1604.00772.pdf
type CmaEsChol struct {
	// InitStepSize sets the initial size of the covariance matrix adaptation.
	// If InitStepSize is 0, a default value of 0.5 is used. InitStepSize cannot
	// be negative, or CmaEsChol will panic.
	InitStepSize float64
	// Population sets the population size for the algorithm. If Population is
	// 0, a default value of 4 + math.Floor(3*math.Log(float64(dim))) is used.
	// Population cannot be negative or CmaEsChol will panic.
	Population int
	// InitCholesky specifies the Cholesky decomposition of the covariance
	// matrix for the initial sampling distribution. If InitCholesky is nil,
	// a default value of I is used. If it is non-nil, then it must have
	// InitCholesky.Size() be equal to the problem dimension.
	InitCholesky *mat.Cholesky
	// StopLogDet sets the threshold for stopping the optimization if the
	// distribution becomes too peaked. The log determinant is a measure of the
	// (log) "volume" of the normal distribution, and when it is too small
	// the samples are almost the same. If the log determinant of the covariance
	// matrix becomes less than StopLogDet, the optimization run is concluded.
	// If StopLogDet is 0, a default value of dim*log(1e-16) is used.
	// If StopLogDet is NaN, the stopping criterion is not used, though
	// this can cause numeric instabilities in the algorithm.
	StopLogDet float64
	// ForgetBest, when true, does not track the best overall function value found,
	// instead returning the new best sample in each iteration. If ForgetBest
	// is false, then the minimum value returned will be the lowest across all
	// iterations, regardless of when that sample was generated.
	ForgetBest bool
	// Src allows a random number generator to be supplied for generating samples.
	// If Src is nil the generator in golang.org/x/math/rand is used.
	Src rand.Source

	// Fixed algorithm parameters.
	dim                 int
	pop                 int
	weights             []float64
	muEff               float64
	cc, cs, c1, cmu, ds float64
	eChi                float64

	// Function data.
	xs *mat.Dense
	fs []float64

	// Adaptive algorithm parameters.
	invSigma float64 // inverse of the sigma parameter
	pc, ps   []float64
	mean     []float64
	chol     mat.Cholesky

	// Overall best.
	bestX []float64
	bestF float64

	// Synchronization.
	sentIdx     int
	receivedIdx int
	operation   chan<- Task
	updateErr   error
}

var (
	_ Statuser = (*CmaEsChol)(nil)
	_ Method   = (*CmaEsChol)(nil)
)

func (cma *CmaEsChol) Needs() struct{ Gradient, Hessian bool } {
	return struct{ Gradient, Hessian bool }{false, false}
}

func (cma *CmaEsChol) methodConverged() Status {
	sd := cma.StopLogDet
	switch {
	case math.IsNaN(sd):
		return NotTerminated
	case sd == 0:
		sd = float64(cma.dim) * -36.8413614879 // ln(1e-16)
	}
	if cma.chol.LogDet() < sd {
		return MethodConverge
	}
	return NotTerminated
}

// Status returns the status of the method.
func (cma *CmaEsChol) Status() (Status, error) {
	if cma.updateErr != nil {
		return Failure, cma.updateErr
	}
	return cma.methodConverged(), nil
}

func (cma *CmaEsChol) Init(dim, tasks int) int {
	if dim <= 0 {
		panic(nonpositiveDimension)
	}
	if tasks < 0 {
		panic(negativeTasks)
	}

	// Set fixed algorithm parameters.
	// Parameter values are from https://arxiv.org/pdf/1604.00772.pdf .
	cma.dim = dim
	cma.pop = cma.Population
	n := float64(dim)
	if cma.pop == 0 {
		cma.pop = 4 + int(3*math.Log(n)) // Note the implicit floor.
	} else if cma.pop < 0 {
		panic("cma-es-chol: negative population size")
	}
	mu := cma.pop / 2
	cma.weights = resize(cma.weights, mu)
	for i := range cma.weights {
		v := math.Log(float64(mu)+0.5) - math.Log(float64(i)+1)
		cma.weights[i] = v
	}
	floats.Scale(1/floats.Sum(cma.weights), cma.weights)
	cma.muEff = 0
	for _, v := range cma.weights {
		cma.muEff += v * v
	}
	cma.muEff = 1 / cma.muEff

	cma.cc = (4 + cma.muEff/n) / (n + 4 + 2*cma.muEff/n)
	cma.cs = (cma.muEff + 2) / (n + cma.muEff + 5)
	cma.c1 = 2 / ((n+1.3)*(n+1.3) + cma.muEff)
	cma.cmu = math.Min(1-cma.c1, 2*(cma.muEff-2+1/cma.muEff)/((n+2)*(n+2)+cma.muEff))
	cma.ds = 1 + 2*math.Max(0, math.Sqrt((cma.muEff-1)/(n+1))-1) + cma.cs
	// E[chi] is taken from https://en.wikipedia.org/wiki/CMA-ES (there
	// listed as E[||N(0,1)||]).
	cma.eChi = math.Sqrt(n) * (1 - 1.0/(4*n) + 1/(21*n*n))

	// Allocate memory for function data.
	cma.xs = mat.NewDense(cma.pop, dim, nil)
	cma.fs = resize(cma.fs, cma.pop)

	// Allocate and initialize adaptive parameters.
	cma.invSigma = 1 / cma.InitStepSize
	if cma.InitStepSize == 0 {
		cma.invSigma = 10.0 / 3
	} else if cma.InitStepSize < 0 {
		panic("cma-es-chol: negative initial step size")
	}
	cma.pc = resize(cma.pc, dim)
	for i := range cma.pc {
		cma.pc[i] = 0
	}
	cma.ps = resize(cma.ps, dim)
	for i := range cma.ps {
		cma.ps[i] = 0
	}
	cma.mean = resize(cma.mean, dim) // mean location initialized at the start of Run

	if cma.InitCholesky != nil {
		if cma.InitCholesky.Size() != dim {
			panic("cma-es-chol: incorrect InitCholesky size")
		}
		cma.chol.Clone(cma.InitCholesky)
	} else {
		// Set the initial Cholesky to I.
		b := mat.NewDiagonal(dim, nil)
		for i := 0; i < dim; i++ {
			b.SetSymBand(i, i, 1)
		}
		var chol mat.Cholesky
		ok := chol.Factorize(b)
		if !ok {
			panic("cma-es-chol: bad cholesky. shouldn't happen")
		}
		cma.chol = chol
	}

	cma.bestX = resize(cma.bestX, dim)
	cma.bestF = math.Inf(1)

	cma.sentIdx = 0
	cma.receivedIdx = 0
	cma.operation = nil
	cma.updateErr = nil
	t := min(tasks, cma.pop)
	return t
}

func (cma *CmaEsChol) sendInitTasks(tasks []Task) {
	for i, task := range tasks {
		cma.sendTask(i, task)
	}
	cma.sentIdx = len(tasks)
}

// sendTask generates a sample and sends the task. It does not update the cma index.
func (cma *CmaEsChol) sendTask(idx int, task Task) {
	task.ID = idx
	task.Op = FuncEvaluation
	distmv.NormalRand(cma.xs.RawRowView(idx), cma.mean, &cma.chol, cma.Src)
	copy(task.X, cma.xs.RawRowView(idx))
	cma.operation <- task
}

// bestIdx returns the best index in the functions. Returns -1 if all values
// are NaN.
func (cma *CmaEsChol) bestIdx() int {
	best := -1
	bestVal := math.Inf(1)
	for i, v := range cma.fs {
		if math.IsNaN(v) {
			continue
		}
		// Use equality in case somewhere evaluates to +inf.
		if v <= bestVal {
			best = i
			bestVal = v
		}
	}
	return best
}

// findBestAndUpdateTask finds the best task in the current list, updates the
// new best overall, and then stores the best location into task.
func (cma *CmaEsChol) findBestAndUpdateTask(task Task) Task {
	// Find and update the best location.
	// Don't use floats because there may be NaN values.
	best := cma.bestIdx()
	bestF := math.NaN()
	bestX := cma.xs.RawRowView(0)
	if best != -1 {
		bestF = cma.fs[best]
		bestX = cma.xs.RawRowView(best)
	}
	if cma.ForgetBest {
		task.F = bestF
		copy(task.X, bestX)
	} else {
		if bestF < cma.bestF {
			cma.bestF = bestF
			copy(cma.bestX, bestX)
		}
		task.F = cma.bestF
		copy(task.X, cma.bestX)
	}
	return task
}

func (cma *CmaEsChol) Run(operations chan<- Task, results <-chan Task, tasks []Task) {
	copy(cma.mean, tasks[0].X)
	cma.operation = operations
	// Send the initial tasks. We know there are at most as many tasks as elements
	// of the population.
	cma.sendInitTasks(tasks)

Loop:
	for {
		result := <-results
		switch result.Op {
		default:
			panic("unknown operation")
		case PostIteration:
			break Loop
		case MajorIteration:
			// The last thing we did was update all of the tasks and send the
			// major iteration. Now we can send a group of tasks again.
			cma.sendInitTasks(tasks)
		case FuncEvaluation:
			cma.receivedIdx++
			cma.fs[result.ID] = result.F
			switch {
			case cma.sentIdx < cma.pop:
				// There are still tasks to evaluate. Send the next.
				cma.sendTask(cma.sentIdx, result)
				cma.sentIdx++
			case cma.receivedIdx < cma.pop:
				// All the tasks have been sent, but not all of them have been received.
				// Need to wait until all are back.
				continue Loop
			default:
				// All of the evaluations have been received.
				if cma.receivedIdx != cma.pop {
					panic("bad logic")
				}
				cma.receivedIdx = 0
				cma.sentIdx = 0

				task := cma.findBestAndUpdateTask(result)
				// Update the parameters and send a MajorIteration or a convergence.
				err := cma.update()
				// Kill the existing data.
				for i := range cma.fs {
					cma.fs[i] = math.NaN()
					cma.xs.Set(i, 0, math.NaN())
				}
				switch {
				case err != nil:
					cma.updateErr = err
					task.Op = MethodDone
				case cma.methodConverged() != NotTerminated:
					task.Op = MethodDone
				default:
					task.Op = MajorIteration
					task.ID = -1
				}
				operations <- task
			}
		}
	}

	// Been told to stop. Clean up.
	// Need to see best of our evaluated tasks so far. Should instead just
	// collect, then see.
	for task := range results {
		switch task.Op {
		case MajorIteration:
		case FuncEvaluation:
			cma.fs[task.ID] = task.F
		default:
			panic("unknown operation")
		}
	}
	// Send the new best value if the evaluation is better than any we've
	// found so far. Keep this separate from findBestAndUpdateTask so that
	// we only send an iteration if we find a better location.
	if !cma.ForgetBest {
		best := cma.bestIdx()
		if best != -1 && cma.fs[best] < cma.bestF {
			task := tasks[0]
			task.F = cma.fs[best]
			copy(task.X, cma.xs.RawRowView(best))
			task.Op = MajorIteration
			task.ID = -1
			operations <- task
		}
	}
	close(operations)
}

// update computes the new parameters (mean, cholesky, etc.). Does not update
// any of the synchronization parameters (taskIdx).
func (cma *CmaEsChol) update() error {
	// Sort the function values to find the elite samples.
	ftmp := make([]float64, cma.pop)
	copy(ftmp, cma.fs)
	indexes := make([]int, cma.pop)
	for i := range indexes {
		indexes[i] = i
	}
	sort.Sort(bestSorter{F: ftmp, Idx: indexes})

	meanOld := make([]float64, len(cma.mean))
	copy(meanOld, cma.mean)

	// m_{t+1} = \sum_{i=1}^mu w_i x_i
	for i := range cma.mean {
		cma.mean[i] = 0
	}
	for i, w := range cma.weights {
		idx := indexes[i] // index of teh 1337 sample.
		floats.AddScaled(cma.mean, w, cma.xs.RawRowView(idx))
	}
	meanDiff := make([]float64, len(cma.mean))
	floats.SubTo(meanDiff, cma.mean, meanOld)

	// p_{c,t+1} = (1-c_c) p_{c,t} + \sqrt(c_c*(2-c_c)*mueff) (m_{t+1}-m_t)/sigma_t
	floats.Scale(1-cma.cc, cma.pc)
	scaleC := math.Sqrt(cma.cc*(2-cma.cc)*cma.muEff) * cma.invSigma
	floats.AddScaled(cma.pc, scaleC, meanDiff)

	// p_{sigma, t+1} = (1-c_sigma) p_{sigma,t} + \sqrt(c_s*(2-c_s)*mueff) A_t^-1 (m_{t+1}-m_t)/sigma_t
	floats.Scale(1-cma.cs, cma.ps)
	// First compute A_t^-1 (m_{t+1}-m_t), then add the scaled vector.
	tmp := make([]float64, cma.dim)
	tmpVec := mat.NewVecDense(cma.dim, tmp)
	diffVec := mat.NewVecDense(cma.dim, meanDiff)
	err := tmpVec.SolveVec(cma.chol.RawU().T(), diffVec)
	if err != nil {
		return err
	}
	scaleS := math.Sqrt(cma.cs*(2-cma.cs)*cma.muEff) * cma.invSigma
	floats.AddScaled(cma.ps, scaleS, tmp)

	// Compute the update to A.
	scaleChol := 1 - cma.c1 - cma.cmu
	if scaleChol == 0 {
		scaleChol = math.SmallestNonzeroFloat64 // enough to kill the old data, but still non-zero.
	}
	cma.chol.Scale(scaleChol, &cma.chol)
	cma.chol.SymRankOne(&cma.chol, cma.c1, mat.NewVecDense(cma.dim, cma.pc))
	for i, w := range cma.weights {
		idx := indexes[i]
		floats.SubTo(tmp, cma.xs.RawRowView(idx), meanOld)
		cma.chol.SymRankOne(&cma.chol, cma.cmu*w*cma.invSigma, tmpVec)
	}

	// sigma_{t+1} = sigma_t exp(c_sigma/d_sigma * norm(p_{sigma,t+1}/ E[chi] -1)
	normPs := floats.Norm(cma.ps, 2)
	cma.invSigma /= math.Exp(cma.cs / cma.ds * (normPs/cma.eChi - 1))
	return nil
}

type bestSorter struct {
	F   []float64
	Idx []int
}

func (b bestSorter) Len() int {
	return len(b.F)
}
func (b bestSorter) Less(i, j int) bool {
	return b.F[i] < b.F[j]
}
func (b bestSorter) Swap(i, j int) {
	b.F[i], b.F[j] = b.F[j], b.F[i]
	b.Idx[i], b.Idx[j] = b.Idx[j], b.Idx[i]
}
