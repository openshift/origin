// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dense

import (
	"container/heap"
	"log"
	"slices"

	"honnef.co/go/tools/analysis/dfa"
	"honnef.co/go/tools/internal/xtools-internal/graph"
)

// Forward performs a forward monotone analysis over a control flow graph.
//
// The entry map provides initial state for entry blocks (blocks with zero
// predecessors). For each edge, it calls transfer(fact, edge), where fact is
// the analysis state on entry to edge.Pred. The transfer function must return
// the outgoing analysis state of the edge (which may be fact, if the edge has
// no effect on the  analysis state).
func Forward[L dfa.Semilattice[Fact], Fact any, NodeID comparable](g graph.Graph[NodeID], entry map[NodeID]Fact, transfer func(from, to NodeID, fact Fact) Fact) *Analysis[Fact, NodeID] {
	cg, nodeMap := graph.Compact(g)

	nNodes := cg.NumNodes()
	fb := &fwdBuilder[L, Fact, NodeID]{
		cfg:      cg,
		nodeMap:  nodeMap,
		transfer: transfer,
		blocks:   make([]blockInfo[Fact], nNodes),
	}
	fb.queue.init(cg)

	// Initialize each node.
	totalEdges := 0
	for ni := range nNodes {
		b := &fb.blocks[ni]

		// Construct back-edges.
		//
		// I experimented with making Graph support iterating over in-edges, but
		// in practice that just meant each Graph implementation had a copy of
		// this logic. So instead we keep Graph as simple as possible and
		// compute the auxiliary data in the algorithm. One drawback of this is
		// that, for the [Transpose] graph, this information is redundant with
		// the underlying graph. We could potentially special-case that.
		outs := 0
		for succID := range cg.Out(ni) {
			succ := &fb.blocks[succID]
			succ.preds = append(succ.preds, blockEdge{ni, outs})
			outs++
			totalEdges++
		}

		// Initialize in & out states.
		fact, ok := entry[nodeMap.Value(ni)]
		if !ok {
			fact = fb.l.Ident()
		}
		b.in = fact
		b.out = slices.Repeat([]Fact{fb.l.Ident()}, outs)

		// Enqueue block.
		//
		// It's tempting to enqueue only the entry blocks, but this is wrong.
		// The entry map may be empty if there are no interesting entry states,
		// but the transfer function may still introduce interesting states
		// anywhere.
		b.dirty = true
		fb.queue.enqueue(ni)
	}

	// Propagate over blocks.
	fb.propagate()

	// Collect the final analysis results.
	a := Analysis[Fact, NodeID]{
		nodeMap: nodeMap,
		ins:     make([]Fact, nNodes),
		edges:   make([]edgeFact[Fact], 0, totalEdges),
	}
	for pred := range nNodes {
		a.ins[pred] = fb.blocks[pred].in
		i := 0
		for succ := range cg.Out(pred) {
			edge := edge{pred, succ}
			a.edges = append(a.edges, edgeFact[Fact]{edge, fb.blocks[pred].out[i]})
			i++
		}
	}
	slices.SortFunc(a.edges, func(a, b edgeFact[Fact]) int { return a.edge.compare(b.edge) })
	return &a
}

// fwdBuilder is the state used during [Forward] analysis.
type fwdBuilder[L dfa.Semilattice[Fact], Fact any, NodeID comparable] struct {
	l L // Lattice

	cfg     graph.Graph[int]     // Control flow graph (compact)
	nodeMap *graph.Index[NodeID] // Map from cfg to original NodeIDs

	// transfer is the edge transfer function.
	transfer func(from, to NodeID, fact Fact) Fact

	blocks []blockInfo[Fact]

	queue nodeHeap
}

type blockInfo[Fact any] struct {
	dirty bool // The in fact has never been propagated.

	preds []blockEdge

	in  Fact
	out []Fact // Corresponds to i'th out edge
}

type blockEdge struct {
	node int
	i    int // Out edge index
}

// nodeHeap implements a heap of NodeIDs, ordered topologically.
//
// We use this ordering so forward analysis converges more quickly.
type nodeHeap struct {
	heap        []int   // Remaining nodes in the current sweep
	deferred    []int   // Nodes of next sweep
	inQueue     []int64 // Bitmap over node IDs
	prio        []int   // NodeID -> priority
	currentPrio int     // Priority of last dequeued node, or -1
}

func (h *nodeHeap) init(g graph.Graph[int]) {
	nNodes := g.NumNodes()
	*h = nodeHeap{
		inQueue:     make([]int64, (nNodes+63)/64),
		prio:        make([]int, nNodes),
		currentPrio: -1,
	}
	for p, nid := range graph.ReversePostorder(g) {
		h.prio[nid] = p
	}
}

func (h *nodeHeap) enqueue(nid int) {
	if h.inQueue[nid/64]&(1<<(nid%64)) != 0 {
		return
	}
	h.inQueue[nid/64] |= 1 << (nid % 64)

	if h.currentPrio >= 0 && h.prio[nid] <= h.currentPrio {
		// This is a retreating edge, self-edge, or other update to a node
		// already passed in this sweep. Coalesce it into the next sweep.
		h.deferred = append(h.deferred, nid)
	} else {
		heap.Push(h, nid)
	}
}

func (h *nodeHeap) dequeue() int {
	if len(h.heap) == 0 {
		// Start the next RPO sweep.
		h.heap, h.deferred = h.deferred, h.heap[:0]
		h.currentPrio = -1
		heap.Init(h)
	}

	nid := h.heap[0]
	heap.Pop(h)
	h.inQueue[nid/64] &^= 1 << (nid % 64)
	h.currentPrio = h.prio[nid]
	return nid
}

func (h *nodeHeap) pending() bool     { return len(h.heap) != 0 || len(h.deferred) != 0 }
func (h nodeHeap) Len() int           { return len(h.heap) }
func (h nodeHeap) Less(i, j int) bool { return h.prio[h.heap[i]] < h.prio[h.heap[j]] }
func (h nodeHeap) Swap(i, j int)      { h.heap[i], h.heap[j] = h.heap[j], h.heap[i] }
func (h *nodeHeap) Push(x any)        { h.heap = append(h.heap, x.(int)) }
func (h *nodeHeap) Pop() any {
	n := len(h.heap)
	x := h.heap[n-1]
	h.heap = h.heap[:n-1]
	return x
}

func (fb *fwdBuilder[L, Fact, NodeID]) merge(a, b Fact) Fact {
	if fb.l.Equals(a, b) {
		return a
	}
	return fb.l.Merge(a, b)
}

func (fb *fwdBuilder[L, Fact, NodeID]) propagate() {
	for fb.queue.pending() {
		bi := fb.queue.dequeue()
		block := &fb.blocks[bi]

		// Merge predecessor facts to compute updated "in" fact.
		var in Fact
		first := true
		for _, edge := range block.preds {
			pred := &fb.blocks[edge.node]
			var edgeFact Fact
			if pred.dirty {
				// We haven't visited this predecessor yet, so it doesn't have
				// meaningful out facts.
				edgeFact = fb.l.Ident()
			} else {
				edgeFact = pred.out[edge.i]
			}
			if first {
				if debug {
					log.Printf("propagate to node %d", bi)
				}
				in = edgeFact
				first = false
			} else {
				in = fb.merge(in, edgeFact)
			}
			if debug {
				log.Printf("  from node %d: %v", edge.node, edgeFact)
			}
		}
		if first {
			// No predecessors.
			if debug {
				log.Printf("node %d gets initial state", bi)
			}
			in = block.in
		}

		if !block.dirty && fb.l.Equals(in, block.in) {
			// No change to block input, which means the transfer function
			// results also won't change from the last time we ran it.
			if debug {
				log.Printf("  initial state unchanged: %v", in)
			}
			continue
		}
		if debug {
			log.Printf("  new initial state: %v", in)
		}
		block.in = in

		// Apply transfer function.
		predID := fb.nodeMap.Value(bi)
		i := 0
		for succNum := range fb.cfg.Out(bi) {
			edgeFact := fb.transfer(predID, fb.nodeMap.Value(succNum), in)
			if block.dirty || !fb.l.Equals(block.out[i], edgeFact) {
				// Out fact changed, so recompute the target block.
				if debug {
					log.Printf("  to node %d: %v", succNum, edgeFact)
				}
				block.out[i] = edgeFact
				fb.queue.enqueue(succNum)
			} else {
				if debug {
					log.Printf("  to node %d: no change", succNum)
				}
			}
			i++
		}

		block.dirty = false
	}
}
