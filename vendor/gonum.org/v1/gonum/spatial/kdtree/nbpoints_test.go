// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kdtree

var (
	_ Interface  = nbPoints{}
	_ Comparable = nbPoint{}
)

// nbRandoms is the maximum number of random values to sample for calculation of median of
// random elements.
var nbRandoms = 100

// nbPoint represents a point in a k-d space that satisfies the Comparable interface.
type nbPoint Point

func (p nbPoint) Compare(c Comparable, d Dim) float64 { q := c.(nbPoint); return p[d] - q[d] }
func (p nbPoint) Dims() int                           { return len(p) }
func (p nbPoint) Distance(c Comparable) float64 {
	q := c.(nbPoint)
	var sum float64
	for dim, c := range p {
		d := c - q[dim]
		sum += d * d
	}
	return sum
}

// nbPoints is a collection of point values that satisfies the Interface.
type nbPoints []nbPoint

func (p nbPoints) Index(i int) Comparable         { return p[i] }
func (p nbPoints) Len() int                       { return len(p) }
func (p nbPoints) Pivot(d Dim) int                { return nbPlane{nbPoints: p, Dim: d}.Pivot() }
func (p nbPoints) Slice(start, end int) Interface { return p[start:end] }

// nbPlane is a wrapping type that allows a Points type be pivoted on a dimension.
type nbPlane struct {
	Dim
	nbPoints
}

func (p nbPlane) Less(i, j int) bool              { return p.nbPoints[i][p.Dim] < p.nbPoints[j][p.Dim] }
func (p nbPlane) Pivot() int                      { return Partition(p, MedianOfRandoms(p, nbRandoms)) }
func (p nbPlane) Slice(start, end int) SortSlicer { p.nbPoints = p.nbPoints[start:end]; return p }
func (p nbPlane) Swap(i, j int) {
	p.nbPoints[i], p.nbPoints[j] = p.nbPoints[j], p.nbPoints[i]
}
