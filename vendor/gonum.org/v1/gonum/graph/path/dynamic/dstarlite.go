// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dynamic

import (
	"container/heap"
	"fmt"
	"math"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

// DStarLite implements the D* Lite dynamic re-planning path search algorithm.
//
//  doi:10.1109/tro.2004.838026 and ISBN:0-262-51129-0 pp476-483
//
type DStarLite struct {
	s, t *dStarLiteNode
	last *dStarLiteNode

	model       WorldModel
	queue       dStarLiteQueue
	keyModifier float64

	weight    path.Weighting
	heuristic path.Heuristic
}

// WorldModel is a mutable weighted directed graph that returns nodes identified
// by id number.
type WorldModel interface {
	graph.WeightedBuilder
	graph.WeightedDirected
}

// NewDStarLite returns a new DStarLite planner for the path from s to t in g using the
// heuristic h. The world model, m, is used to store shortest path information during path
// planning. The world model must be an empty graph when NewDStarLite is called.
//
// If h is nil, the DStarLite will use the g.HeuristicCost method if g implements
// path.HeuristicCoster, falling back to path.NullHeuristic otherwise. If the graph does not
// implement graph.Weighter, path.UniformCost is used. NewDStarLite will panic if g has
// a negative edge weight.
func NewDStarLite(s, t graph.Node, g graph.Graph, h path.Heuristic, m WorldModel) *DStarLite {
	/*
	   procedure Initialize()
	   {02”} U = ∅;
	   {03”} k_m = 0;
	   {04”} for all s ∈ S rhs(s) = g(s) = ∞;
	   {05”} rhs(s_goal) = 0;
	   {06”} U.Insert(s_goal, [h(s_start, s_goal); 0]);
	*/

	d := &DStarLite{
		s: newDStarLiteNode(s),
		t: newDStarLiteNode(t), // badKey is overwritten below.

		model: m,

		heuristic: h,
	}
	d.t.rhs = 0

	/*
		procedure Main()
		{29”} s_last = s_start;
		{30”} Initialize();
	*/
	d.last = d.s

	if wg, ok := g.(graph.Weighted); ok {
		d.weight = wg.Weight
	} else {
		d.weight = path.UniformCost(g)
	}
	if d.heuristic == nil {
		if g, ok := g.(path.HeuristicCoster); ok {
			d.heuristic = g.HeuristicCost
		} else {
			d.heuristic = path.NullHeuristic
		}
	}

	d.queue.insert(d.t, key{d.heuristic(s, t), 0})

	for _, n := range graph.NodesOf(g.Nodes()) {
		switch n.ID() {
		case d.s.ID():
			d.model.AddNode(d.s)
		case d.t.ID():
			d.model.AddNode(d.t)
		default:
			d.model.AddNode(newDStarLiteNode(n))
		}
	}
	for _, u := range graph.NodesOf(d.model.Nodes()) {
		uid := u.ID()
		for _, v := range graph.NodesOf(g.From(uid)) {
			vid := v.ID()
			w := edgeWeight(d.weight, uid, vid)
			if w < 0 {
				panic("D* Lite: negative edge weight")
			}
			d.model.SetWeightedEdge(simple.WeightedEdge{F: u, T: d.model.Node(vid), W: w})
		}
	}

	/*
		procedure Main()
		{31”} ComputeShortestPath();
	*/
	d.findShortestPath()

	return d
}

// edgeWeight is a helper function that returns the weight of the edge between
// two connected nodes, u and v, using the provided weight function. It panics
// if there is no edge between u and v.
func edgeWeight(weight path.Weighting, uid, vid int64) float64 {
	w, ok := weight(uid, vid)
	if !ok {
		panic("D* Lite: unexpected invalid weight")
	}
	return w
}

// keyFor is the CalculateKey procedure in the D* Lite papers.
func (d *DStarLite) keyFor(s *dStarLiteNode) key {
	/*
	   procedure CalculateKey(s)
	   {01”} return [min(g(s), rhs(s)) + h(s_start, s) + k_m; min(g(s), rhs(s))];
	*/
	k := key{1: math.Min(s.g, s.rhs)}
	k[0] = k[1] + d.heuristic(d.s.Node, s.Node) + d.keyModifier
	return k
}

// update is the UpdateVertex procedure in the D* Lite papers.
func (d *DStarLite) update(u *dStarLiteNode) {
	/*
	   procedure UpdateVertex(u)
	   {07”} if (g(u) != rhs(u) AND u ∈ U) U.Update(u,CalculateKey(u));
	   {08”} else if (g(u) != rhs(u) AND u /∈ U) U.Insert(u,CalculateKey(u));
	   {09”} else if (g(u) = rhs(u) AND u ∈ U) U.Remove(u);
	*/
	inQueue := u.inQueue()
	switch {
	case inQueue && u.g != u.rhs:
		d.queue.update(u, d.keyFor(u))
	case !inQueue && u.g != u.rhs:
		d.queue.insert(u, d.keyFor(u))
	case inQueue && u.g == u.rhs:
		d.queue.remove(u)
	}
}

// findShortestPath is the ComputeShortestPath procedure in the D* Lite papers.
func (d *DStarLite) findShortestPath() {
	/*
	   procedure ComputeShortestPath()
	   {10”} while (U.TopKey() < CalculateKey(s_start) OR rhs(s_start) > g(s_start))
	   {11”} u = U.Top();
	   {12”} k_old = U.TopKey();
	   {13”} k_new = CalculateKey(u);
	   {14”} if(k_old < k_new)
	   {15”}   U.Update(u, k_new);
	   {16”} else if (g(u) > rhs(u))
	   {17”}   g(u) = rhs(u);
	   {18”}   U.Remove(u);
	   {19”}   for all s ∈ Pred(u)
	   {20”}     if (s != s_goal) rhs(s) = min(rhs(s), c(s, u) + g(u));
	   {21”}     UpdateVertex(s);
	   {22”} else
	   {23”}   g_old = g(u);
	   {24”}   g(u) = ∞;
	   {25”}   for all s ∈ Pred(u) ∪ {u}
	   {26”}     if (rhs(s) = c(s, u) + g_old)
	   {27”}       if (s != s_goal) rhs(s) = min s'∈Succ(s)(c(s, s') + g(s'));
	   {28”}     UpdateVertex(s);
	*/
	for d.queue.Len() != 0 { // We use d.queue.Len since d.queue does not return an infinite key when empty.
		u := d.queue.top()
		if !u.key.less(d.keyFor(d.s)) && d.s.rhs <= d.s.g {
			break
		}
		uid := u.ID()
		switch kNew := d.keyFor(u); {
		case u.key.less(kNew):
			d.queue.update(u, kNew)
		case u.g > u.rhs:
			u.g = u.rhs
			d.queue.remove(u)
			for _, _s := range graph.NodesOf(d.model.To(uid)) {
				s := _s.(*dStarLiteNode)
				sid := s.ID()
				if sid != d.t.ID() {
					s.rhs = math.Min(s.rhs, edgeWeight(d.model.Weight, sid, uid)+u.g)
				}
				d.update(s)
			}
		default:
			gOld := u.g
			u.g = math.Inf(1)
			for _, _s := range append(graph.NodesOf(d.model.To(uid)), u) {
				s := _s.(*dStarLiteNode)
				sid := s.ID()
				if s.rhs == edgeWeight(d.model.Weight, sid, uid)+gOld {
					if s.ID() != d.t.ID() {
						s.rhs = math.Inf(1)
						for _, t := range graph.NodesOf(d.model.From(sid)) {
							tid := t.ID()
							s.rhs = math.Min(s.rhs, edgeWeight(d.model.Weight, sid, tid)+t.(*dStarLiteNode).g)
						}
					}
				}
				d.update(s)
			}
		}
	}
}

// Step performs one movement step along the best path towards the goal.
// It returns false if no further progression toward the goal can be
// achieved, either because the goal has been reached or because there
// is no path.
func (d *DStarLite) Step() bool {
	/*
	   procedure Main()
	   {32”} while (s_start != s_goal)
	   {33”} // if (rhs(s_start) = ∞) then there is no known path
	   {34”}   s_start = argmin s'∈Succ(s_start)(c(s_start, s') + g(s'));
	*/
	if d.s.ID() == d.t.ID() {
		return false
	}
	if math.IsInf(d.s.rhs, 1) {
		return false
	}

	// We use rhs comparison to break ties
	// between coequally weighted nodes.
	rhs := math.Inf(1)
	min := math.Inf(1)

	var next *dStarLiteNode
	dsid := d.s.ID()
	for _, _s := range graph.NodesOf(d.model.From(dsid)) {
		s := _s.(*dStarLiteNode)
		w := edgeWeight(d.model.Weight, dsid, s.ID()) + s.g
		if w < min || (w == min && s.rhs < rhs) {
			next = s
			min = w
			rhs = s.rhs
		}
	}
	d.s = next

	/*
	   procedure Main()
	   {35”}   Move to s_start;
	*/
	return true
}

// MoveTo moves to n in the world graph.
func (d *DStarLite) MoveTo(n graph.Node) {
	d.last = d.s
	d.s = d.model.Node(n.ID()).(*dStarLiteNode)
	d.keyModifier += d.heuristic(d.last, d.s)
}

// UpdateWorld updates or adds edges in the world graph. UpdateWorld will
// panic if changes include a negative edge weight.
func (d *DStarLite) UpdateWorld(changes []graph.Edge) {
	/*
	   procedure Main()
	   {36”}   Scan graph for changed edge costs;
	   {37”}   if any edge costs changed
	   {38”}     k_m = k_m + h(s_last, s_start);
	   {39”}     s_last = s_start;
	   {40”}     for all directed edges (u, v) with changed edge costs
	   {41”}       c_old = c(u, v);
	   {42”}       Update the edge cost c(u, v);
	   {43”}       if (c_old > c(u, v))
	   {44”}         if (u != s_goal) rhs(u) = min(rhs(u), c(u, v) + g(v));
	   {45”}       else if (rhs(u) = c_old + g(v))
	   {46”}         if (u != s_goal) rhs(u) = min s'∈Succ(u)(c(u, s') + g(s'));
	   {47”}       UpdateVertex(u);
	   {48”}     ComputeShortestPath()
	*/
	if len(changes) == 0 {
		return
	}
	d.keyModifier += d.heuristic(d.last, d.s)
	d.last = d.s
	for _, e := range changes {
		from := e.From()
		fid := from.ID()
		to := e.To()
		tid := to.ID()
		c, _ := d.weight(fid, tid)
		if c < 0 {
			panic("D* Lite: negative edge weight")
		}
		cOld, _ := d.model.Weight(fid, tid)
		u := d.worldNodeFor(from)
		v := d.worldNodeFor(to)
		d.model.SetWeightedEdge(simple.WeightedEdge{F: u, T: v, W: c})
		uid := u.ID()
		if cOld > c {
			if uid != d.t.ID() {
				u.rhs = math.Min(u.rhs, c+v.g)
			}
		} else if u.rhs == cOld+v.g {
			if uid != d.t.ID() {
				u.rhs = math.Inf(1)
				for _, t := range graph.NodesOf(d.model.From(uid)) {
					u.rhs = math.Min(u.rhs, edgeWeight(d.model.Weight, uid, t.ID())+t.(*dStarLiteNode).g)
				}
			}
		}
		d.update(u)
	}
	d.findShortestPath()
}

func (d *DStarLite) worldNodeFor(n graph.Node) *dStarLiteNode {
	switch w := d.model.Node(n.ID()).(type) {
	case *dStarLiteNode:
		return w
	case graph.Node:
		panic(fmt.Sprintf("D* Lite: illegal world model node type: %T", w))
	default:
		return newDStarLiteNode(n)
	}
}

// Here returns the current location.
func (d *DStarLite) Here() graph.Node {
	return d.s.Node
}

// Path returns the path from the current location to the goal and the
// weight of the path.
func (d *DStarLite) Path() (p []graph.Node, weight float64) {
	u := d.s
	p = []graph.Node{u.Node}
	for u.ID() != d.t.ID() {
		if math.IsInf(u.rhs, 1) {
			return nil, math.Inf(1)
		}

		// We use stored rhs comparison to break
		// ties between calculated rhs-coequal nodes.
		rhsMin := math.Inf(1)
		min := math.Inf(1)
		var (
			next *dStarLiteNode
			cost float64
		)
		uid := u.ID()
		for _, _v := range graph.NodesOf(d.model.From(uid)) {
			v := _v.(*dStarLiteNode)
			vid := v.ID()
			w := edgeWeight(d.model.Weight, uid, vid)
			if rhs := w + v.g; rhs < min || (rhs == min && v.rhs < rhsMin) {
				next = v
				min = rhs
				rhsMin = v.rhs
				cost = w
			}
		}
		if next == nil {
			return nil, math.NaN()
		}
		u = next
		weight += cost
		p = append(p, u.Node)
	}
	return p, weight
}

/*
The pseudocode uses the following functions to manage the priority
queue:

      * U.Top() returns a vertex with the smallest priority of all
        vertices in priority queue U.
      * U.TopKey() returns the smallest priority of all vertices in
        priority queue U. (If is empty, then U.TopKey() returns [∞;∞].)
      * U.Pop() deletes the vertex with the smallest priority in
        priority queue U and returns the vertex.
      * U.Insert(s, k) inserts vertex s into priority queue with
        priority k.
      * U.Update(s, k) changes the priority of vertex s in priority
        queue U to k. (It does nothing if the current priority of vertex
        s already equals k.)
      * Finally, U.Remove(s) removes vertex s from priority queue U.
*/

// key is a D* Lite priority queue key.
type key [2]float64

var badKey = key{math.NaN(), math.NaN()}

// less returns whether k is less than other. From ISBN:0-262-51129-0 pp476-483:
//
//  k ≤ k' iff k₁ < k'₁ OR (k₁ == k'₁ AND k₂ ≤ k'₂)
//
func (k key) less(other key) bool {
	if k != k || other != other {
		panic("D* Lite: poisoned key")
	}
	return k[0] < other[0] || (k[0] == other[0] && k[1] < other[1])
}

// dStarLiteNode adds D* Lite accounting to a graph.Node.
type dStarLiteNode struct {
	graph.Node
	key key
	idx int
	rhs float64
	g   float64
}

// newDStarLiteNode returns a dStarLite node that is in a legal state
// for existence outside the DStarLite priority queue.
func newDStarLiteNode(n graph.Node) *dStarLiteNode {
	return &dStarLiteNode{
		Node: n,
		rhs:  math.Inf(1),
		g:    math.Inf(1),
		key:  badKey,
		idx:  -1,
	}
}

// inQueue returns whether the node is in the queue.
func (q *dStarLiteNode) inQueue() bool {
	return q.idx >= 0
}

// dStarLiteQueue is a D* Lite priority queue.
type dStarLiteQueue []*dStarLiteNode

func (q dStarLiteQueue) Less(i, j int) bool {
	return q[i].key.less(q[j].key)
}

func (q dStarLiteQueue) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
	q[i].idx = i
	q[j].idx = j
}

func (q dStarLiteQueue) Len() int {
	return len(q)
}

func (q *dStarLiteQueue) Push(x interface{}) {
	n := x.(*dStarLiteNode)
	n.idx = len(*q)
	*q = append(*q, n)
}

func (q *dStarLiteQueue) Pop() interface{} {
	n := (*q)[len(*q)-1]
	n.idx = -1
	*q = (*q)[:len(*q)-1]
	return n
}

// top returns the top node in the queue. Note that instead of
// returning a key [∞;∞] when q is empty, the caller checks for
// an empty queue by calling q.Len.
func (q dStarLiteQueue) top() *dStarLiteNode {
	return q[0]
}

// insert puts the node u into the queue with the key k.
func (q *dStarLiteQueue) insert(u *dStarLiteNode, k key) {
	u.key = k
	heap.Push(q, u)
}

// update updates the node in the queue identified by id with the key k.
func (q *dStarLiteQueue) update(n *dStarLiteNode, k key) {
	n.key = k
	heap.Fix(q, n.idx)
}

// remove removes the node identified by id from the queue.
func (q *dStarLiteQueue) remove(n *dStarLiteNode) {
	heap.Remove(q, n.idx)
	n.key = badKey
	n.idx = -1
}
