// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package barneshut

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/spatial/r3"
)

// Particle3 is a particle in a volume.
type Particle3 interface {
	Coord3() r3.Vec
	Mass() float64
}

// Force3 is a force modeling function for interactions between p1 and p2,
// m1 is the mass of p1 and m2 of p2. The vector v is the vector from p1 to
// p2. The returned value is the force vector acting on p1.
//
// In models where the identity of particles must be known, p1 and p2 may be
// compared. Force3 may be passed nil for p2 when the Barnes-Hut approximation
// is being used. A nil p2 indicates that the second mass center is an
// aggregate.
type Force3 func(p1, p2 Particle3, m1, m2 float64, v r3.Vec) r3.Vec

// Gravity3 returns a vector force on m1 by m2, equal to (m1⋅m2)/‖v‖²
// in the directions of v. Gravity3 ignores the identity of the interacting
// particles and returns a zero vector when the two particles are
// coincident, but performs no other sanity checks.
func Gravity3(_, _ Particle3, m1, m2 float64, v r3.Vec) r3.Vec {
	d2 := v.X*v.X + v.Y*v.Y + v.Z*v.Z
	if d2 == 0 {
		return r3.Vec{}
	}
	return v.Scale((m1 * m2) / (d2 * math.Sqrt(d2)))
}

// Volume implements Barnes-Hut force approximation calculations.
type Volume struct {
	root bucket

	Particles []Particle3
}

// NewVolume returns a new Volume. If the volume is too large to allow
// particle coordinates to be distinguished due to floating point
// precision limits, NewVolume will return a non-nil error.
func NewVolume(p []Particle3) (*Volume, error) {
	q := Volume{Particles: p}
	err := q.Reset()
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// Reset reconstructs the Barnes-Hut tree. Reset must be called if the
// Particles field or elements of Particles have been altered, unless
// ForceOn is called with theta=0 or no data structures have been
// previously built. If the volume is too large to allow particle
// coordinates to be distinguished due to floating point precision
// limits, Reset will return a non-nil error.
func (q *Volume) Reset() (err error) {
	if len(q.Particles) == 0 {
		q.root = bucket{}
		return nil
	}

	q.root = bucket{
		particle: q.Particles[0],
		center:   q.Particles[0].Coord3(),
		mass:     q.Particles[0].Mass(),
	}
	q.root.bounds.Min = q.root.center
	q.root.bounds.Max = q.root.center
	for _, e := range q.Particles[1:] {
		c := e.Coord3()
		if c.X < q.root.bounds.Min.X {
			q.root.bounds.Min.X = c.X
		}
		if c.X > q.root.bounds.Max.X {
			q.root.bounds.Max.X = c.X
		}
		if c.Y < q.root.bounds.Min.Y {
			q.root.bounds.Min.Y = c.Y
		}
		if c.Y > q.root.bounds.Max.Y {
			q.root.bounds.Max.Y = c.Y
		}
		if c.Z < q.root.bounds.Min.Z {
			q.root.bounds.Min.Z = c.Z
		}
		if c.Z > q.root.bounds.Max.Z {
			q.root.bounds.Max.Z = c.Z
		}
	}

	defer func() {
		switch r := recover(); r {
		case nil:
		case volumeTooBig:
			err = volumeTooBig
		default:
			panic(r)
		}
	}()

	// TODO(kortschak): Partially parallelise this by
	// choosing the direction and using one of eight
	// goroutines to work on each root octant.
	for _, e := range q.Particles[1:] {
		q.root.insert(e)
	}
	q.root.summarize()
	return nil
}

var volumeTooBig = errors.New("barneshut: volume too big")

// ForceOn returns a force vector on p given p's mass and the force function, f,
// using the Barnes-Hut theta approximation parameter.
//
// Calls to f will include p in the p1 position and a non-nil p2 if the force
// interaction is with a non-aggregate mass center, otherwise p2 will be nil.
//
// It is safe to call ForceOn concurrently.
func (q *Volume) ForceOn(p Particle3, theta float64, f Force3) (force r3.Vec) {
	var empty bucket
	if theta > 0 && q.root != empty {
		return q.root.forceOn(p, p.Coord3(), p.Mass(), theta, f)
	}

	// For the degenerate case, just iterate over the
	// slice of particles rather than walking the tree.
	var v r3.Vec
	m := p.Mass()
	pv := p.Coord3()
	for _, e := range q.Particles {
		v = v.Add(f(p, e, m, e.Mass(), e.Coord3().Sub(pv)))
	}
	return v
}

// bucket is an oct tree octant with Barnes-Hut extensions.
type bucket struct {
	particle Particle3

	bounds r3.Box

	nodes [8]*bucket

	center r3.Vec
	mass   float64
}

// insert inserts p into the subtree rooted at b.
func (b *bucket) insert(p Particle3) {
	if b.particle == nil {
		for _, q := range b.nodes {
			if q != nil {
				b.passDown(p)
				return
			}
		}
		b.particle = p
		b.center = p.Coord3()
		b.mass = p.Mass()
		return
	}

	b.passDown(p)
	b.passDown(b.particle)
	b.particle = nil
	b.center = r3.Vec{}
	b.mass = 0
}

func (b *bucket) passDown(p Particle3) {
	dir := octantOf(b.bounds, p)
	if b.nodes[dir] == nil {
		b.nodes[dir] = &bucket{bounds: splitVolume(b.bounds, dir)}
	}
	b.nodes[dir].insert(p)
}

const (
	lne = iota
	lse
	lsw
	lnw
	une
	use
	usw
	unw
)

// octantOf returns which octant of b that p should be placed in.
func octantOf(b r3.Box, p Particle3) int {
	center := r3.Vec{
		X: (b.Min.X + b.Max.X) / 2,
		Y: (b.Min.Y + b.Max.Y) / 2,
		Z: (b.Min.Z + b.Max.Z) / 2,
	}
	c := p.Coord3()
	if checkBounds && (c.X < b.Min.X || b.Max.X < c.X || c.Y < b.Min.Y || b.Max.Y < c.Y || c.Z < b.Min.Z || b.Max.Z < c.Z) {
		panic(fmt.Sprintf("p out of range %+v: %#v", b, p))
	}
	if c.X < center.X {
		if c.Y < center.Y {
			if c.Z < center.Z {
				return lnw
			} else {
				return unw
			}
		} else {
			if c.Z < center.Z {
				return lsw
			} else {
				return usw
			}
		}
	} else {
		if c.Y < center.Y {
			if c.Z < center.Z {
				return lne
			} else {
				return une
			}
		} else {
			if c.Z < center.Z {
				return lse
			} else {
				return use
			}
		}
	}
}

// splitVolume returns an octant subdivision of b in the given direction.
func splitVolume(b r3.Box, dir int) r3.Box {
	old := b
	halfX := (b.Max.X - b.Min.X) / 2
	halfY := (b.Max.Y - b.Min.Y) / 2
	halfZ := (b.Max.Z - b.Min.Z) / 2
	switch dir {
	case lne:
		b.Min.X += halfX
		b.Max.Y -= halfY
		b.Max.Z -= halfZ
	case lse:
		b.Min.X += halfX
		b.Min.Y += halfY
		b.Max.Z -= halfZ
	case lsw:
		b.Max.X -= halfX
		b.Min.Y += halfY
		b.Max.Z -= halfZ
	case lnw:
		b.Max.X -= halfX
		b.Max.Y -= halfY
		b.Max.Z -= halfZ
	case une:
		b.Min.X += halfX
		b.Max.Y -= halfY
		b.Min.Z += halfZ
	case use:
		b.Min.X += halfX
		b.Min.Y += halfY
		b.Min.Z += halfZ
	case usw:
		b.Max.X -= halfX
		b.Min.Y += halfY
		b.Min.Z += halfZ
	case unw:
		b.Max.X -= halfX
		b.Max.Y -= halfY
		b.Min.Z += halfZ
	}
	if b == old {
		panic(volumeTooBig)
	}
	return b
}

// summarize updates node masses and centers of mass.
func (b *bucket) summarize() (center r3.Vec, mass float64) {
	for _, d := range &b.nodes {
		if d == nil {
			continue
		}
		c, m := d.summarize()
		b.center.X += c.X * m
		b.center.Y += c.Y * m
		b.center.Z += c.Z * m
		b.mass += m
	}
	b.center.X /= b.mass
	b.center.Y /= b.mass
	b.center.Z /= b.mass
	return b.center, b.mass
}

// forceOn returns a force vector on p given p's mass m and the force
// calculation function, using the Barnes-Hut theta approximation parameter.
func (b *bucket) forceOn(p Particle3, pt r3.Vec, m, theta float64, f Force3) (vector r3.Vec) {
	s := ((b.bounds.Max.X - b.bounds.Min.X) + (b.bounds.Max.Y - b.bounds.Min.Y) + (b.bounds.Max.Z - b.bounds.Min.Z)) / 3
	d := math.Hypot(math.Hypot(pt.X-b.center.X, pt.Y-b.center.Y), pt.Z-b.center.Z)
	if s/d < theta || b.particle != nil {
		return f(p, b.particle, m, b.mass, b.center.Sub(pt))
	}

	var v r3.Vec
	for _, d := range &b.nodes {
		if d == nil {
			continue
		}
		v = v.Add(d.forceOn(p, pt, m, theta, f))
	}
	return v
}
