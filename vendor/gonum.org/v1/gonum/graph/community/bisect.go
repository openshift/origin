// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"errors"
	"fmt"
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
)

// Interval is an interval of resolutions with a common score.
type Interval struct {
	// Low and High delimit the interval
	// such that the interval is [low, high).
	Low, High float64

	// Score is the score of the interval.
	Score float64

	// Reduced is the best scoring
	// community membership found for the
	// interval.
	Reduced
}

// Reduced is a graph reduction.
type Reduced interface {
	// Communities returns the community
	// structure of the reduction.
	Communities() [][]graph.Node
}

// Size is a score function that is the reciprocal of the number of communities.
func Size(g ReducedGraph) float64 { return 1 / float64(len(g.Structure())) }

// Weight is a score function that is the sum of community weights. The concrete
// type of g must be a pointer to a ReducedUndirected or a ReducedDirected, otherwise
// Weight will panic.
func Weight(g ReducedGraph) float64 {
	var w float64
	switch g := g.(type) {
	case *ReducedUndirected:
		for _, n := range g.nodes {
			w += n.weight
		}
	case *ReducedDirected:
		for _, n := range g.nodes {
			w += n.weight
		}
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
	return w
}

// ModularScore returns a modularized scoring function for Profile based on the
// graph g and the given score function. The effort parameter determines how
// many attempts will be made to get an improved score for any given resolution.
func ModularScore(g graph.Graph, score func(ReducedGraph) float64, effort int, src rand.Source) func(float64) (float64, Reduced) {
	return func(resolution float64) (float64, Reduced) {
		max := math.Inf(-1)
		var best Reduced
		for i := 0; i < effort; i++ {
			r := Modularize(g, resolution, src)
			s := score(r)
			if s > max {
				max = s
				best = r
			}
		}
		return max, best
	}
}

// SizeMultiplex is a score function that is the reciprocal of the number of communities.
func SizeMultiplex(g ReducedMultiplex) float64 { return 1 / float64(len(g.Structure())) }

// WeightMultiplex is a score function that is the sum of community weights. The concrete
// type of g must be pointer to a ReducedUndirectedMultiplex or a ReducedDirectedMultiplex,
// otherwise WeightMultiplex will panic.
func WeightMultiplex(g ReducedMultiplex) float64 {
	var w float64
	switch g := g.(type) {
	case *ReducedUndirectedMultiplex:
		for _, n := range g.nodes {
			for _, lw := range n.weights {
				w += lw
			}
		}
	case *ReducedDirectedMultiplex:
		for _, n := range g.nodes {
			for _, lw := range n.weights {
				w += lw
			}
		}
	default:
		panic(fmt.Sprintf("community: invalid graph type: %T", g))
	}
	return w
}

// ModularMultiplexScore returns a modularized scoring function for Profile based
// on the graph g and the given score function. The effort parameter determines how
// many attempts will be made to get an improved score for any given resolution.
func ModularMultiplexScore(g Multiplex, weights []float64, all bool, score func(ReducedMultiplex) float64, effort int, src rand.Source) func(float64) (float64, Reduced) {
	return func(resolution float64) (float64, Reduced) {
		max := math.Inf(-1)
		var best Reduced
		for i := 0; i < effort; i++ {
			r := ModularizeMultiplex(g, weights, []float64{resolution}, all, src)
			s := score(r)
			if s > max {
				max = s
				best = r
			}
		}
		return max, best
	}
}

// Profile returns an approximate profile of score values in the resolution domain [low,high)
// at the given granularity. The score is calculated by bisecting calls to fn. If log is true,
// log space bisection is used, otherwise bisection is linear. The function fn should be
// monotonically decreasing in at least 1/grain evaluations. Profile will attempt to detect
// non-monotonicity during the bisection.
//
// Since exact modularity optimization is known to be NP-hard and Profile calls modularization
// routines repeatedly, it is unlikely to return the exact resolution profile.
func Profile(fn func(float64) (float64, Reduced), log bool, grain, low, high float64) (profile []Interval, err error) {
	if low >= high {
		return nil, errors.New("community: zero or negative width domain")
	}

	defer func() {
		r := recover()
		e, ok := r.(nonDecreasing)
		if ok {
			err = e
			return
		}
		if r != nil {
			panic(r)
		}
	}()
	left, comm := fn(low)
	right, _ := fn(high)
	for i := 1; i < int(1/grain); i++ {
		rt, _ := fn(high)
		right = math.Max(right, rt)
	}
	profile = bisect(fn, log, grain, low, left, high, right, comm)

	// We may have missed some non-monotonicity,
	// so merge low score discordant domains into
	// their lower resolution neighbours.
	return fixUp(profile), nil
}

type nonDecreasing int

func (n nonDecreasing) Error() string {
	return fmt.Sprintf("community: profile does not reliably monotonically decrease: tried %d times", n)
}

func bisect(fn func(float64) (float64, Reduced), log bool, grain, low, scoreLow, high, scoreHigh float64, comm Reduced) []Interval {
	if low >= high {
		panic("community: zero or negative width domain")
	}
	if math.IsNaN(scoreLow) || math.IsNaN(scoreHigh) {
		return nil
	}

	// Heuristically determine a reasonable number
	// of times to try to get a higher value.
	maxIter := int(1 / grain)

	lowComm := comm
	for n := 0; scoreLow < scoreHigh; n++ {
		if n > maxIter {
			panic(nonDecreasing(n))
		}
		scoreLow, lowComm = fn(low)
	}

	if scoreLow == scoreHigh || tooSmall(low, high, grain, log) {
		return []Interval{{Low: low, High: high, Score: scoreLow, Reduced: lowComm}}
	}

	var mid float64
	if log {
		mid = math.Sqrt(low * high)
	} else {
		mid = (low + high) / 2
	}

	scoreMid := math.Inf(-1)
	var midComm Reduced
	for n := 0; scoreMid < scoreHigh; n++ {
		if n > maxIter {
			panic(nonDecreasing(n))
		}
		scoreMid, midComm = fn(mid)
	}

	lower := bisect(fn, log, grain, low, scoreLow, mid, scoreMid, lowComm)
	higher := bisect(fn, log, grain, mid, scoreMid, high, scoreHigh, midComm)
	for n := 0; lower[len(lower)-1].Score < higher[0].Score; n++ {
		if n > maxIter {
			panic(nonDecreasing(n))
		}
		lower[len(lower)-1].Score, lower[len(lower)-1].Reduced = fn(low)
	}

	if lower[len(lower)-1].Score == higher[0].Score {
		higher[0].Low = lower[len(lower)-1].Low
		lower = lower[:len(lower)-1]
		if len(lower) == 0 {
			return higher
		}
	}
	return append(lower, higher...)
}

// fixUp non-monotonically decreasing interval scores.
func fixUp(profile []Interval) []Interval {
	max := profile[len(profile)-1].Score
	for i := len(profile) - 2; i >= 0; i-- {
		if profile[i].Score > max {
			max = profile[i].Score
			continue
		}
		profile[i+1].Low = profile[i].Low
		profile = append(profile[:i], profile[i+1:]...)
	}
	return profile
}

func tooSmall(low, high, grain float64, log bool) bool {
	if log {
		return math.Log(high/low) < grain
	}
	return high-low < grain
}
