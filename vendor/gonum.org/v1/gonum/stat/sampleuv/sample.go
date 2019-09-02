// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sampleuv

import (
	"errors"
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/stat/distuv"
)

var (
	badLengthMismatch = "sample: slice length mismatch"
)

var (
	_ Sampler = LatinHypercube{}
	_ Sampler = MetropolisHastings{}
	_ Sampler = (*Rejection)(nil)
	_ Sampler = IIDer{}

	_ WeightedSampler = SampleUniformWeighted{}
	_ WeightedSampler = Importance{}
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Sampler generates a batch of samples according to the rule specified by the
// implementing type. The number of samples generated is equal to len(batch),
// and the samples are stored in-place into the input.
type Sampler interface {
	Sample(batch []float64)
}

// WeightedSampler generates a batch of samples and their relative weights
// according to the rule specified by the implementing type. The number of samples
// generated is equal to len(batch), and the samples and weights
// are stored in-place into the inputs. The length of weights must equal
// len(batch), otherwise SampleWeighted will panic.
type WeightedSampler interface {
	SampleWeighted(batch, weights []float64)
}

// SampleUniformWeighted wraps a Sampler type to create a WeightedSampler where all
// weights are equal.
type SampleUniformWeighted struct {
	Sampler
}

// SampleWeighted generates len(batch) samples from the embedded Sampler type
// and sets all of the weights equal to 1. If len(batch) and len(weights)
// are not equal, SampleWeighted will panic.
func (w SampleUniformWeighted) SampleWeighted(batch, weights []float64) {
	if len(batch) != len(weights) {
		panic(badLengthMismatch)
	}
	w.Sample(batch)
	for i := range weights {
		weights[i] = 1
	}
}

// LatinHypercube is a type for sampling using Latin hypercube sampling
// from the given distribution. If src is not nil, it will be used to generate
// random numbers, otherwise rand.Float64 will be used.
//
// Latin hypercube sampling divides the cumulative distribution function into equally
// spaced bins and guarantees that one sample is generated per bin. Within each bin,
// the location is randomly sampled. The distuv.UnitUniform variable can be used
// for easy sampling from the unit hypercube.
type LatinHypercube struct {
	Q   distuv.Quantiler
	Src rand.Source
}

// Sample generates len(batch) samples using the LatinHypercube generation
// procedure.
func (l LatinHypercube) Sample(batch []float64) {
	latinHypercube(batch, l.Q, l.Src)
}

func latinHypercube(batch []float64, q distuv.Quantiler, src rand.Source) {
	n := len(batch)
	var perm []int
	var f64 func() float64
	if src != nil {
		r := rand.New(src)
		f64 = r.Float64
		perm = r.Perm(n)
	} else {
		f64 = rand.Float64
		perm = rand.Perm(n)
	}
	for i := range batch {
		v := f64()/float64(n) + float64(i)/float64(n)
		batch[perm[i]] = q.Quantile(v)
	}
}

// Importance is a type for performing importance sampling using the given
// Target and Proposal distributions.
//
// Importance sampling is a variance reduction technique where samples are
// generated from a proposal distribution, q(x), instead of the target distribution
// p(x). This allows relatively unlikely samples in p(x) to be generated more frequently.
//
// The importance sampling weight at x is given by p(x)/q(x). To reduce variance,
// a good proposal distribution will bound this sampling weight. This implies the
// support of q(x) should be at least as broad as p(x), and q(x) should be "fatter tailed"
// than p(x).
type Importance struct {
	Target   distuv.LogProber
	Proposal distuv.RandLogProber
}

// SampleWeighted generates len(batch) samples using the Importance sampling
// generation procedure.
//
// The length of weights must equal the length of batch, otherwise Importance will panic.
func (l Importance) SampleWeighted(batch, weights []float64) {
	importance(batch, weights, l.Target, l.Proposal)
}

func importance(batch, weights []float64, target distuv.LogProber, proposal distuv.RandLogProber) {
	if len(batch) != len(weights) {
		panic(badLengthMismatch)
	}
	for i := range batch {
		v := proposal.Rand()
		batch[i] = v
		weights[i] = math.Exp(target.LogProb(v) - proposal.LogProb(v))
	}
}

// ErrRejection is returned when the constant in Rejection is not sufficiently high.
var ErrRejection = errors.New("rejection: acceptance ratio above 1")

// Rejection is a type for sampling using the rejection sampling algorithm.
//
// Rejection sampling generates points from the target distribution by using
// the proposal distribution. At each step of the algorithm, the proposed point
// is accepted with probability
//  p = target(x) / (proposal(x) * c)
// where target(x) is the probability of the point according to the target distribution
// and proposal(x) is the probability according to the proposal distribution.
// The constant c must be chosen such that target(x) < proposal(x) * c for all x.
// The expected number of proposed samples is len(samples) * c.
//
// The number of proposed locations during sampling can be found with a call to
// Proposed. If there was an error during sampling, all elements of samples are
// set to NaN and the error can be accesssed with the Err method. If src != nil,
// it will be used to generate random numbers, otherwise rand.Float64 will be used.
//
// Target may return the true (log of) the probablity of the location, or it may return
// a value that is proportional to the probability (logprob + constant). This is
// useful for cases where the probability distribution is only known up to a normalization
// constant.
type Rejection struct {
	C        float64
	Target   distuv.LogProber
	Proposal distuv.RandLogProber
	Src      rand.Source

	err      error
	proposed int
}

// Err returns nil if the most recent call to sample was successful, and returns
// ErrRejection if it was not.
func (r *Rejection) Err() error {
	return r.err
}

// Proposed returns the number of samples proposed during the most recent call to
// Sample.
func (r *Rejection) Proposed() int {
	return r.proposed
}

// Sample generates len(batch) using the Rejection sampling generation procedure.
// Rejection sampling may fail if the constant is insufficiently high, as described
// in the type comment for Rejection. If the generation fails, the samples
// are set to math.NaN(), and a call to Err will return a non-nil value.
func (r *Rejection) Sample(batch []float64) {
	r.err = nil
	r.proposed = 0
	proposed, ok := rejection(batch, r.Target, r.Proposal, r.C, r.Src)
	if !ok {
		r.err = ErrRejection
	}
	r.proposed = proposed
}

func rejection(batch []float64, target distuv.LogProber, proposal distuv.RandLogProber, c float64, src rand.Source) (nProposed int, ok bool) {
	if c < 1 {
		panic("rejection: acceptance constant must be greater than 1")
	}
	f64 := rand.Float64
	if src != nil {
		f64 = rand.New(src).Float64
	}
	var idx int
	for {
		nProposed++
		v := proposal.Rand()
		qx := proposal.LogProb(v)
		px := target.LogProb(v)
		accept := math.Exp(px-qx) / c
		if accept > 1 {
			// Invalidate the whole result and return a failure.
			for i := range batch {
				batch[i] = math.NaN()
			}
			return nProposed, false
		}
		if accept > f64() {
			batch[idx] = v
			idx++
			if idx == len(batch) {
				break
			}
		}
	}
	return nProposed, true
}

// MHProposal defines a proposal distribution for Metropolis Hastings.
type MHProposal interface {
	// ConditionalDist returns the probability of the first argument conditioned on
	// being at the second argument
	//  p(x|y)
	ConditionalLogProb(x, y float64) (prob float64)

	// ConditionalRand generates a new random location conditioned being at the
	// location y.
	ConditionalRand(y float64) (x float64)
}

// MetropolisHastings is a type for generating samples using the Metropolis Hastings
// algorithm (http://en.wikipedia.org/wiki/Metropolis%E2%80%93Hastings_algorithm),
// with the given target and proposal distributions, starting at the location
// specified by Initial. If src != nil, it will be used to generate random
// numbers, otherwise rand.Float64 will be used.
//
// Metropolis-Hastings is a Markov-chain Monte Carlo algorithm that generates
// samples according to the distribution specified by target using the Markov
// chain implicitly defined by the proposal distribution. At each
// iteration, a proposal point is generated randomly from the current location.
// This proposal point is accepted with probability
//  p = min(1, (target(new) * proposal(current|new)) / (target(current) * proposal(new|current)))
// If the new location is accepted, it becomes the new current location.
// If it is rejected, the current location remains. This is the sample stored in
// batch, ignoring BurnIn and Rate (discussed below).
//
// The samples in Metropolis Hastings are correlated with one another through the
// Markov chain. As a result, the initial value can have a significant influence
// on the early samples, and so, typically, the first samples generated by the chain
// are ignored. This is known as "burn-in", and the number of samples ignored
// at the beginning is specified by BurnIn. The proper BurnIn value will depend
// on the mixing time of the Markov chain defined by the target and proposal
// distributions.
//
// Many choose to have a sampling "rate" where a number of samples
// are ignored in between each kept sample. This helps decorrelate
// the samples from one another, but also reduces the number of available samples.
// This value is specified by Rate. If Rate is 0 it is defaulted to 1 (keep
// every sample).
//
// The initial value is NOT changed during calls to Sample.
type MetropolisHastings struct {
	Initial  float64
	Target   distuv.LogProber
	Proposal MHProposal
	Src      rand.Source

	BurnIn int
	Rate   int
}

// Sample generates len(batch) samples using the Metropolis Hastings sample
// generation method. The initial location is NOT updated during the call to Sample.
func (m MetropolisHastings) Sample(batch []float64) {
	burnIn := m.BurnIn
	rate := m.Rate
	if rate == 0 {
		rate = 1
	}

	// Use the optimal size for the temporary memory to allow the fewest calls
	// to MetropolisHastings. The case where tmp shadows samples must be
	// aligned with the logic after burn-in so that tmp does not shadow samples
	// during the rate portion.
	tmp := batch
	if rate > len(batch) {
		tmp = make([]float64, rate)
	}

	// Perform burn-in.
	remaining := burnIn
	initial := m.Initial
	for remaining != 0 {
		newSamp := min(len(tmp), remaining)
		metropolisHastings(tmp[newSamp:], initial, m.Target, m.Proposal, m.Src)
		initial = tmp[newSamp-1]
		remaining -= newSamp
	}

	if rate == 1 {
		metropolisHastings(batch, initial, m.Target, m.Proposal, m.Src)
		return
	}

	if len(tmp) <= len(batch) {
		tmp = make([]float64, rate)
	}

	// Take a single sample from the chain
	metropolisHastings(batch[0:1], initial, m.Target, m.Proposal, m.Src)
	initial = batch[0]

	// For all of the other samples, first generate Rate samples and then actually
	// accept the last one.
	for i := 1; i < len(batch); i++ {
		metropolisHastings(tmp, initial, m.Target, m.Proposal, m.Src)
		v := tmp[rate-1]
		batch[i] = v
		initial = v
	}
}

func metropolisHastings(batch []float64, initial float64, target distuv.LogProber, proposal MHProposal, src rand.Source) {
	f64 := rand.Float64
	if src != nil {
		f64 = rand.New(src).Float64
	}
	current := initial
	currentLogProb := target.LogProb(initial)
	for i := range batch {
		proposed := proposal.ConditionalRand(current)
		proposedLogProb := target.LogProb(proposed)
		probTo := proposal.ConditionalLogProb(proposed, current)
		probBack := proposal.ConditionalLogProb(current, proposed)

		accept := math.Exp(proposedLogProb + probBack - probTo - currentLogProb)
		if accept > f64() {
			current = proposed
			currentLogProb = proposedLogProb
		}
		batch[i] = current
	}
}

// IIDer generates a set of independently and identically distributed samples from
// the input distribution.
type IIDer struct {
	Dist distuv.Rander
}

// Sample generates a set of identically and independently distributed samples.
func (iid IIDer) Sample(batch []float64) {
	for i := range batch {
		batch[i] = iid.Dist.Rand()
	}
}
