// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package layout

import (
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/spatial/barneshut"
	"gonum.org/v1/gonum/spatial/r2"
)

// EadesR2 implements the graph layout algorithm essentially as
// described in "A heuristic for graph drawing", Congressus
// numerantium 42:149-160.
// The implementation here uses the Barnes-Hut approximation for
// global repulsion calculation, and edge weights are considered
// when calculating adjacent node attraction.
type EadesR2 struct {
	// Updates is the number of updates to perform.
	Updates int

	// Repulsion is the strength of the global
	// repulsive force between nodes in the
	// layout. It corresponds to C3 in the paper.
	Repulsion float64

	// Rate is the gradient descent rate. It
	// corresponds to C4 in the paper.
	Rate float64

	// Theta is the Barnes-Hut theta constant.
	Theta float64

	// Src is the source of randomness used
	// to initialize the nodes' locations. If
	// Src is nil, the global random number
	// generator is used.
	Src rand.Source

	nodes   graph.Nodes
	indexOf map[int64]int

	particles []barneshut.Particle2
	forces    []r2.Vec
}

// Update is the EadesR2 spatial graph update function.
func (u *EadesR2) Update(g graph.Graph, layout LayoutR2) bool {
	if u.Updates <= 0 {
		return false
	}
	u.Updates--

	if !layout.IsInitialized() {
		var rnd func() float64
		if u.Src == nil {
			rnd = rand.Float64
		} else {
			rnd = rand.New(u.Src).Float64
		}
		u.nodes = g.Nodes()
		u.indexOf = make(map[int64]int, u.nodes.Len())
		u.particles = make([]barneshut.Particle2, 0, u.nodes.Len())
		u.forces = make([]r2.Vec, u.nodes.Len())
		for u.nodes.Next() {
			id := u.nodes.Node().ID()
			u.indexOf[id] = len(u.particles)
			u.particles = append(u.particles, eadesR2Node{id: id, pos: r2.Vec{X: rnd(), Y: rnd()}})
		}
	}
	u.nodes.Reset()

	// Apply global repulsion.
	plane, err := barneshut.NewPlane(u.particles)
	if err != nil {
		return false
	}
	var updated bool
	for i, p := range u.particles {
		f := plane.ForceOn(p, u.Theta, barneshut.Gravity2).Scale(-u.Repulsion)
		// Prevent marginal updates that can be caused by
		// floating point error when nodes are very far apart.
		if math.Hypot(f.X, f.Y) > 1e-12 {
			updated = true
		}
		u.forces[i] = f
	}

	// Handle edge weighting for attraction.
	var weight func(uid, vid int64) float64
	if wg, ok := g.(graph.Weighted); ok {
		if _, ok := g.(graph.Directed); ok {
			weight = func(xid, yid int64) float64 {
				var w float64
				f, ok := wg.Weight(xid, yid)
				if ok {
					w += f
				}
				r, ok := wg.Weight(yid, xid)
				if ok {
					w += r
				}
				return w
			}
		} else {
			weight = func(xid, yid int64) float64 {
				w, ok := wg.Weight(xid, yid)
				if ok {
					return w
				}
				return 0
			}
		}
	} else {
		// This is only called when the adjacency is known so just return unit.
		weight = func(_, _ int64) float64 { return 1 }
	}

	seen := make(map[[2]int64]bool)
	for u.nodes.Next() {
		xid := u.nodes.Node().ID()
		xidx := u.indexOf[xid]
		to := g.From(xid)
		for to.Next() {
			yid := to.Node().ID()
			if seen[[2]int64{xid, yid}] {
				continue
			}
			seen[[2]int64{yid, xid}] = true
			yidx := u.indexOf[yid]

			// Apply adjacent node attraction.
			v := u.particles[yidx].Coord2().Sub(u.particles[xidx].Coord2())
			f := v.Scale(weight(xid, yid) * math.Log(math.Hypot(v.X, v.Y)))
			if math.Hypot(f.X, f.Y) > 1e-12 {
				updated = true
			}
			u.forces[xidx] = u.forces[xidx].Add(f)
			u.forces[yidx] = u.forces[yidx].Sub(f)
		}
	}

	if !updated {
		return false
	}

	rate := u.Rate
	if rate == 0 {
		rate = 0.1
	}
	for i, f := range u.forces {
		n := u.particles[i].(eadesR2Node)
		n.pos = n.pos.Add(f.Scale(rate))
		u.particles[i] = n
		layout.SetCoord2(n.id, n.pos)
	}
	return true
}

type eadesR2Node struct {
	id  int64
	pos r2.Vec
}

func (p eadesR2Node) Coord2() r2.Vec { return p.pos }
func (p eadesR2Node) Mass() float64  { return 1 }
