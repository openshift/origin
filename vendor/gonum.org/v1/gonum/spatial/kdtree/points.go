// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

import "math"

var (
	_ Interface  = Points(nil)
	_ Comparable = Point(nil)
)

// Point represents a point in a k-d space that satisfies the Comparable interface.
type Point []float64

// Compare returns the signed distance of p from the plane passing through c and
// perpendicular to the dimension d. The concrete type of c must be Point.
func (p Point) Compare(c Comparable, d Dim) float64 { q := c.(Point); return p[d] - q[d] }

// Dims returns the number of dimensions described by the receiver.
func (p Point) Dims() int { return len(p) }

// Distance returns the squared Euclidean distance between c and the receiver. The
// concrete type of c must be Point.
func (p Point) Distance(c Comparable) float64 {
	q := c.(Point)
	var sum float64
	for dim, c := range p {
		d := c - q[dim]
		sum += d * d
	}
	return sum
}

// Extend returns a bounding box that has been extended to include the receiver.
func (p Point) Extend(b *Bounding) *Bounding {
	if b == nil {
		b = &Bounding{append(Point(nil), p...), append(Point(nil), p...)}
	}
	min := b.Min.(Point)
	max := b.Max.(Point)
	for d, v := range p {
		min[d] = math.Min(min[d], v)
		max[d] = math.Max(max[d], v)
	}
	*b = Bounding{Min: min, Max: max}
	return b
}

// Points is a collection of point values that satisfies the Interface.
type Points []Point

func (p Points) Bounds() *Bounding {
	if p.Len() == 0 {
		return nil
	}
	min := append(Point(nil), p[0]...)
	max := append(Point(nil), p[0]...)
	for _, e := range p[1:] {
		for d, v := range e {
			min[d] = math.Min(min[d], v)
			max[d] = math.Max(max[d], v)
		}
	}
	return &Bounding{Min: min, Max: max}
}
func (p Points) Index(i int) Comparable         { return p[i] }
func (p Points) Len() int                       { return len(p) }
func (p Points) Pivot(d Dim) int                { return Plane{Points: p, Dim: d}.Pivot() }
func (p Points) Slice(start, end int) Interface { return p[start:end] }

// Plane is a wrapping type that allows a Points type be pivoted on a dimension.
// The Pivot method of Plane uses MedianOfRandoms sampling at most 100 elements
// to find a pivot element.
type Plane struct {
	Dim
	Points
}

// randoms is the maximum number of random values to sample for calculation of
// median of random elements.
const randoms = 100

func (p Plane) Less(i, j int) bool              { return p.Points[i][p.Dim] < p.Points[j][p.Dim] }
func (p Plane) Pivot() int                      { return Partition(p, MedianOfRandoms(p, randoms)) }
func (p Plane) Slice(start, end int) SortSlicer { p.Points = p.Points[start:end]; return p }
func (p Plane) Swap(i, j int)                   { p.Points[i], p.Points[j] = p.Points[j], p.Points[i] }
