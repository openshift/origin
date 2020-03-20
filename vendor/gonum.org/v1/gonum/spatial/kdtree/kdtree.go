// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
)

// Interface is the set of methods required for construction of efficiently
// searchable k-d trees. A k-d tree may be constructed without using the
// Interface type, but it is likely to have reduced search performance.
type Interface interface {
	// Index returns the ith element of the list of points.
	Index(i int) Comparable

	// Len returns the length of the list.
	Len() int

	// Pivot partitions the list based on the dimension specified.
	Pivot(Dim) int

	// Slice returns a slice of the list using zero-based half
	// open indexing equivalent to built-in slice indexing.
	Slice(start, end int) Interface
}

// Bounder returns a bounding volume containing the list of points. Bounds may return nil.
type Bounder interface {
	Bounds() *Bounding
}

type bounder interface {
	Interface
	Bounder
}

// Dim is an index into a point's coordinates.
type Dim int

// Comparable is the element interface for values stored in a k-d tree.
type Comparable interface {
	// Compare returns the signed distance of a from the plane passing through
	// b and perpendicular to the dimension d.
	//
	// Given c = a.Compare(b, d):
	//  c = a_d - b_d
	//
	Compare(Comparable, Dim) float64

	// Dims returns the number of dimensions described in the Comparable.
	Dims() int

	// Distance returns the squared Euclidean distance between the receiver and
	// the parameter.
	Distance(Comparable) float64
}

// Extender is a Comparable that can increase a bounding volume to include the
// point represented by the Comparable.
type Extender interface {
	Comparable

	// Extend returns a bounding box that has been extended to include the
	// receiver. Extend may return nil.
	Extend(*Bounding) *Bounding
}

// Bounding represents a volume bounding box.
type Bounding struct {
	Min, Max Comparable
}

// Contains returns whether c is within the volume of the Bounding. A nil Bounding
// returns true.
func (b *Bounding) Contains(c Comparable) bool {
	if b == nil {
		return true
	}
	for d := Dim(0); d < Dim(c.Dims()); d++ {
		if c.Compare(b.Min, d) < 0 || 0 < c.Compare(b.Max, d) {
			return false
		}
	}
	return true
}

// Node holds a single point value in a k-d tree.
type Node struct {
	Point       Comparable
	Plane       Dim
	Left, Right *Node
	*Bounding
}

func (n *Node) String() string {
	if n == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%.3f %d", n.Point, n.Plane)
}

// Tree implements a k-d tree creation and nearest neighbor search.
type Tree struct {
	Root  *Node
	Count int
}

// New returns a k-d tree constructed from the values in p. If p is a Bounder and
// bounding is true, bounds are determined for each node.
// The ordering of elements in p may be altered after New returns.
func New(p Interface, bounding bool) *Tree {
	if p, ok := p.(bounder); ok && bounding {
		return &Tree{
			Root:  buildBounded(p, 0, bounding),
			Count: p.Len(),
		}
	}
	return &Tree{
		Root:  build(p, 0),
		Count: p.Len(),
	}
}

func build(p Interface, plane Dim) *Node {
	if p.Len() == 0 {
		return nil
	}

	piv := p.Pivot(plane)
	d := p.Index(piv)
	np := (plane + 1) % Dim(d.Dims())

	return &Node{
		Point:    d,
		Plane:    plane,
		Left:     build(p.Slice(0, piv), np),
		Right:    build(p.Slice(piv+1, p.Len()), np),
		Bounding: nil,
	}
}

func buildBounded(p bounder, plane Dim, bounding bool) *Node {
	if p.Len() == 0 {
		return nil
	}

	piv := p.Pivot(plane)
	d := p.Index(piv)
	np := (plane + 1) % Dim(d.Dims())

	b := p.Bounds()
	return &Node{
		Point:    d,
		Plane:    plane,
		Left:     buildBounded(p.Slice(0, piv).(bounder), np, bounding),
		Right:    buildBounded(p.Slice(piv+1, p.Len()).(bounder), np, bounding),
		Bounding: b,
	}
}

// Insert adds a point to the tree, updating the bounding volumes if bounding is
// true, and the tree is empty or the tree already has bounding volumes stored,
// and c is an Extender. No rebalancing of the tree is performed.
func (t *Tree) Insert(c Comparable, bounding bool) {
	t.Count++
	if t.Root != nil {
		bounding = t.Root.Bounding != nil
	}
	if c, ok := c.(Extender); ok && bounding {
		t.Root = t.Root.insertBounded(c, 0, bounding)
		return
	} else if !ok && t.Root != nil {
		// If we are not rebounding, mark the tree as non-bounded.
		t.Root.Bounding = nil
	}
	t.Root = t.Root.insert(c, 0)
}

func (n *Node) insert(c Comparable, d Dim) *Node {
	if n == nil {
		return &Node{
			Point:    c,
			Plane:    d,
			Bounding: nil,
		}
	}

	d = (n.Plane + 1) % Dim(c.Dims())
	if c.Compare(n.Point, n.Plane) <= 0 {
		n.Left = n.Left.insert(c, d)
	} else {
		n.Right = n.Right.insert(c, d)
	}

	return n
}

func (n *Node) insertBounded(c Extender, d Dim, bounding bool) *Node {
	if n == nil {
		var b *Bounding
		if bounding {
			b = c.Extend(b)
		}
		return &Node{
			Point:    c,
			Plane:    d,
			Bounding: b,
		}
	}

	if bounding {
		n.Bounding = c.Extend(n.Bounding)
	}
	d = (n.Plane + 1) % Dim(c.Dims())
	if c.Compare(n.Point, n.Plane) <= 0 {
		n.Left = n.Left.insertBounded(c, d, bounding)
	} else {
		n.Right = n.Right.insertBounded(c, d, bounding)
	}

	return n
}

// Len returns the number of elements in the tree.
func (t *Tree) Len() int { return t.Count }

// Contains returns whether a Comparable is in the bounds of the tree. If no bounding has
// been constructed Contains returns true.
func (t *Tree) Contains(c Comparable) bool {
	if t.Root.Bounding == nil {
		return true
	}
	return t.Root.Contains(c)
}

var inf = math.Inf(1)

// Nearest returns the nearest value to the query and the distance between them.
func (t *Tree) Nearest(q Comparable) (Comparable, float64) {
	if t.Root == nil {
		return nil, inf
	}
	n, dist := t.Root.search(q, inf)
	if n == nil {
		return nil, inf
	}
	return n.Point, dist
}

func (n *Node) search(q Comparable, dist float64) (*Node, float64) {
	if n == nil {
		return nil, inf
	}

	c := q.Compare(n.Point, n.Plane)
	dist = math.Min(dist, q.Distance(n.Point))

	bn := n
	if c <= 0 {
		ln, ld := n.Left.search(q, dist)
		if ld < dist {
			bn, dist = ln, ld
		}
		if c*c < dist {
			rn, rd := n.Right.search(q, dist)
			if rd < dist {
				bn, dist = rn, rd
			}
		}
		return bn, dist
	}
	rn, rd := n.Right.search(q, dist)
	if rd < dist {
		bn, dist = rn, rd
	}
	if c*c < dist {
		ln, ld := n.Left.search(q, dist)
		if ld < dist {
			bn, dist = ln, ld
		}
	}
	return bn, dist
}

// ComparableDist holds a Comparable and a distance to a specific query. A nil Comparable
// is used to mark the end of the heap, so clients should not store nil values except for
// this purpose.
type ComparableDist struct {
	Comparable Comparable
	Dist       float64
}

// Heap is a max heap sorted on Dist.
type Heap []ComparableDist

func (h *Heap) Max() ComparableDist  { return (*h)[0] }
func (h *Heap) Len() int             { return len(*h) }
func (h *Heap) Less(i, j int) bool   { return (*h)[i].Comparable == nil || (*h)[i].Dist > (*h)[j].Dist }
func (h *Heap) Swap(i, j int)        { (*h)[i], (*h)[j] = (*h)[j], (*h)[i] }
func (h *Heap) Push(x interface{})   { (*h) = append(*h, x.(ComparableDist)) }
func (h *Heap) Pop() (i interface{}) { i, *h = (*h)[len(*h)-1], (*h)[:len(*h)-1]; return i }

// NKeeper is a Keeper that retains the n best ComparableDists that have been passed to Keep.
type NKeeper struct {
	Heap
}

// NewNKeeper returns an NKeeper with the max value of the heap set to infinite distance. The
// returned NKeeper is able to retain at most n values.
func NewNKeeper(n int) *NKeeper {
	k := NKeeper{make(Heap, 1, n)}
	k.Heap[0].Dist = inf
	return &k
}

// Keep adds c to the heap if its distance is less than the maximum value of the heap. If adding
// c would increase the size of the heap beyond the initial maximum length, the maximum value of
// the heap is dropped.
func (k *NKeeper) Keep(c ComparableDist) {
	if c.Dist <= k.Heap[0].Dist { // Favour later finds to displace sentinel.
		if len(k.Heap) == cap(k.Heap) {
			heap.Pop(k)
		}
		heap.Push(k, c)
	}
}

// DistKeeper is a Keeper that retains the ComparableDists within the specified distance of the
// query that it is called to Keep.
type DistKeeper struct {
	Heap
}

// NewDistKeeper returns an DistKeeper with the maximum value of the heap set to d.
func NewDistKeeper(d float64) *DistKeeper { return &DistKeeper{Heap{{Dist: d}}} }

// Keep adds c to the heap if its distance is less than or equal to the max value of the heap.
func (k *DistKeeper) Keep(c ComparableDist) {
	if c.Dist <= k.Heap[0].Dist {
		heap.Push(k, c)
	}
}

// Keeper implements a conditional max heap sorted on the Dist field of the ComparableDist type.
// kd search is guided by the distance stored in the max value of the heap.
type Keeper interface {
	Keep(ComparableDist) // Keep conditionally pushes the provided ComparableDist onto the heap.
	Max() ComparableDist // Max returns the maximum element of the Keeper.
	heap.Interface
}

// NearestSet finds the nearest values to the query accepted by the provided Keeper, k.
// k must be able to return a ComparableDist specifying the maximum acceptable distance
// when Max() is called, and retains the results of the search in min sorted order after
// the call to NearestSet returns.
// If a sentinel ComparableDist with a nil Comparable is used by the Keeper to mark the
// maximum distance, NearestSet will remove it before returning.
func (t *Tree) NearestSet(k Keeper, q Comparable) {
	if t.Root == nil {
		return
	}
	t.Root.searchSet(q, k)

	// Check whether we have retained a sentinel
	// and flag removal if we have.
	removeSentinel := k.Len() != 0 && k.Max().Comparable == nil

	sort.Sort(sort.Reverse(k))

	// This abuses the interface to drop the max.
	// It is reasonable to do this because we know
	// that the maximum value will now be at element
	// zero, which is removed by the Pop method.
	if removeSentinel {
		k.Pop()
	}
}

func (n *Node) searchSet(q Comparable, k Keeper) {
	if n == nil {
		return
	}

	c := q.Compare(n.Point, n.Plane)
	k.Keep(ComparableDist{Comparable: n.Point, Dist: q.Distance(n.Point)})
	if c <= 0 {
		n.Left.searchSet(q, k)
		if c*c <= k.Max().Dist {
			n.Right.searchSet(q, k)
		}
		return
	}
	n.Right.searchSet(q, k)
	if c*c <= k.Max().Dist {
		n.Left.searchSet(q, k)
	}
}

// Operation is a function that operates on a Comparable. The bounding volume and tree depth
// of the point is also provided. If done is returned true, the Operation is indicating that no
// further work needs to be done and so the Do function should traverse no further.
type Operation func(Comparable, *Bounding, int) (done bool)

// Do performs fn on all values stored in the tree. A boolean is returned indicating whether the
// Do traversal was interrupted by an Operation returning true. If fn alters stored values' sort
// relationships, future tree operation behaviors are undefined.
func (t *Tree) Do(fn Operation) bool {
	if t.Root == nil {
		return false
	}
	return t.Root.do(fn, 0)
}

func (n *Node) do(fn Operation, depth int) (done bool) {
	if n.Left != nil {
		done = n.Left.do(fn, depth+1)
		if done {
			return
		}
	}
	done = fn(n.Point, n.Bounding, depth)
	if done {
		return
	}
	if n.Right != nil {
		done = n.Right.do(fn, depth+1)
	}
	return
}

// DoBounded performs fn on all values stored in the tree that are within the specified bound.
// If b is nil, the result is the same as a Do. A boolean is returned indicating whether the
// DoBounded traversal was interrupted by an Operation returning true. If fn alters stored
// values' sort relationships future tree operation behaviors are undefined.
func (t *Tree) DoBounded(b *Bounding, fn Operation) bool {
	if t.Root == nil {
		return false
	}
	if b == nil {
		return t.Root.do(fn, 0)
	}
	return t.Root.doBounded(fn, b, 0)
}

func (n *Node) doBounded(fn Operation, b *Bounding, depth int) (done bool) {
	if n.Left != nil && b.Min.Compare(n.Point, n.Plane) < 0 {
		done = n.Left.doBounded(fn, b, depth+1)
		if done {
			return
		}
	}
	if b.Contains(n.Point) {
		done = fn(n.Point, n.Bounding, depth)
		if done {
			return
		}
	}
	if n.Right != nil && 0 < b.Max.Compare(n.Point, n.Plane) {
		done = n.Right.doBounded(fn, b, depth+1)
	}
	return
}
