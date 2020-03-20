// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package barneshut

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/spatial/r2"
)

type particle2 struct {
	x, y, m float64
	name    string
}

func (p particle2) Coord2() r2.Vec { return r2.Vec{X: p.x, Y: p.y} }
func (p particle2) Mass() float64  { return p.m }

var planeTests = []struct {
	name      string
	particles []particle2
	want      *Plane
}{
	{
		name:      "nil",
		particles: nil,
		want:      &Plane{},
	},
	{
		name:      "empty",
		particles: []particle2{},
		want:      &Plane{Particles: []Particle2{}},
	},
	{
		name:      "one",
		particles: []particle2{{m: 1}}, // Must have a mass to avoid vacuum decay.
		want: &Plane{
			root: tile{
				particle: particle2{x: 0, y: 0, m: 1},
				bounds:   r2.Box{Min: r2.Vec{X: 0, Y: 0}, Max: r2.Vec{X: 0, Y: 0}},
				center:   r2.Vec{X: 0, Y: 0},
				mass:     1,
			},

			Particles: []Particle2{
				particle2{m: 1},
			},
		},
	},
	{
		name: "3 corners",
		particles: []particle2{
			{x: 1, y: 1, m: 1},
			{x: -1, y: 1, m: 1},
			{x: -1, y: -1, m: 1},
		},
		want: &Plane{
			root: tile{
				bounds: r2.Box{Min: r2.Vec{X: -1, Y: -1}, Max: r2.Vec{X: 1, Y: 1}},
				nodes: [4]*tile{
					se: {
						particle: particle2{x: 1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: 0, Y: 0}, Max: r2.Vec{X: 1, Y: 1}},
						center:   r2.Vec{X: 1, Y: 1}, mass: 1,
					},
					sw: {
						particle: particle2{x: -1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -1, Y: 0}, Max: r2.Vec{X: 0, Y: 1}},
						center:   r2.Vec{X: -1, Y: 1}, mass: 1,
					},
					nw: {
						particle: particle2{x: -1, y: -1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -1, Y: -1}, Max: r2.Vec{X: 0, Y: 0}},
						center:   r2.Vec{X: -1, Y: -1}, mass: 1,
					},
				},
				center: r2.Vec{X: -0.3333333333333333, Y: 0.3333333333333333},
				mass:   3,
			},

			Particles: []Particle2{
				particle2{x: 1, y: 1, m: 1},
				particle2{x: -1, y: 1, m: 1},
				particle2{x: -1, y: -1, m: 1},
			},
		},
	},
	{
		name: "4 corners",
		particles: []particle2{
			{x: 1, y: 1, m: 1},
			{x: -1, y: 1, m: 1},
			{x: 1, y: -1, m: 1},
			{x: -1, y: -1, m: 1},
		},
		want: &Plane{
			root: tile{
				bounds: r2.Box{Min: r2.Vec{X: -1, Y: -1}, Max: r2.Vec{X: 1, Y: 1}},
				nodes: [4]*tile{
					{
						particle: particle2{x: 1, y: -1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: 0, Y: -1}, Max: r2.Vec{X: 1, Y: 0}},
						center:   r2.Vec{X: 1, Y: -1},
						mass:     1,
					},
					{
						particle: particle2{x: 1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: 0, Y: 0}, Max: r2.Vec{X: 1, Y: 1}},
						center:   r2.Vec{X: 1, Y: 1},
						mass:     1,
					},
					{
						particle: particle2{x: -1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -1, Y: 0}, Max: r2.Vec{X: 0, Y: 1}},
						center:   r2.Vec{X: -1, Y: 1},
						mass:     1,
					},
					{
						particle: particle2{x: -1, y: -1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -1, Y: -1}, Max: r2.Vec{X: 0, Y: 0}},
						center:   r2.Vec{X: -1, Y: -1},
						mass:     1,
					},
				},
				center: r2.Vec{X: 0, Y: 0},
				mass:   4,
			},

			Particles: []Particle2{
				particle2{x: 1, y: 1, m: 1},
				particle2{x: -1, y: 1, m: 1},
				particle2{x: 1, y: -1, m: 1},
				particle2{x: -1, y: -1, m: 1},
			},
		},
	},
	{
		name: "5 corners",
		particles: []particle2{
			{x: 1, y: 1, m: 1},
			{x: -1, y: 1, m: 1},
			{x: 1, y: -1, m: 1},
			{x: -1, y: -1, m: 1},
			{x: -1.1, y: -1, m: 1},
		},
		want: &Plane{
			root: tile{
				bounds: r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: 1, Y: 1}},
				nodes: [4]*tile{
					{
						particle: particle2{x: 1, y: -1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -0.050000000000000044, Y: -1}, Max: r2.Vec{X: 1, Y: 0}},
						center:   r2.Vec{X: 1, Y: -1},
						mass:     1,
					},
					{
						particle: particle2{x: 1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -0.050000000000000044, Y: 0}, Max: r2.Vec{X: 1, Y: 1}},
						center:   r2.Vec{X: 1, Y: 1},
						mass:     1,
					},
					{
						particle: particle2{x: -1, y: 1, m: 1},
						bounds:   r2.Box{Min: r2.Vec{X: -1.1, Y: 0}, Max: r2.Vec{X: -0.050000000000000044, Y: 1}},
						center:   r2.Vec{X: -1, Y: 1},
						mass:     1,
					},
					{
						bounds: r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: -0.050000000000000044, Y: 0}},
						nodes: [4]*tile{
							nw: {
								bounds: r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: -0.5750000000000001, Y: -0.5}},
								nodes: [4]*tile{
									nw: {
										bounds: r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: -0.8375000000000001, Y: -0.75}},
										nodes: [4]*tile{
											nw: {
												bounds: r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: -0.9687500000000001, Y: -0.875}},
												nodes: [4]*tile{
													ne: {
														particle: particle2{x: -1, y: -1, m: 1},
														bounds:   r2.Box{Min: r2.Vec{X: -1.034375, Y: -1}, Max: r2.Vec{X: -0.9687500000000001, Y: -0.9375}},
														center:   r2.Vec{X: -1, Y: -1},
														mass:     1,
													},
													nw: {
														particle: particle2{x: -1.1, y: -1, m: 1},
														bounds:   r2.Box{Min: r2.Vec{X: -1.1, Y: -1}, Max: r2.Vec{X: -1.034375, Y: -0.9375}},
														center:   r2.Vec{X: -1.1, Y: -1},
														mass:     1,
													},
												},
												center: r2.Vec{X: -1.05, Y: -1},
												mass:   2,
											},
										},
										center: r2.Vec{X: -1.05, Y: -1},
										mass:   2,
									},
								},
								center: r2.Vec{X: -1.05, Y: -1},
								mass:   2,
							},
						},
						center: r2.Vec{X: -1.05, Y: -1},
						mass:   2,
					},
				},
				center: r2.Vec{X: -0.22000000000000003, Y: -0.2},
				mass:   5,
			},

			Particles: []Particle2{
				particle2{x: 1, y: 1, m: 1},
				particle2{x: -1, y: 1, m: 1},
				particle2{x: 1, y: -1, m: 1},
				particle2{x: -1, y: -1, m: 1},
				particle2{x: -1.1, y: -1, m: 1},
			},
		},
	},
	{
		// Note that the code here subdivides the space differently to
		// how it is split in the example, since Plane makes a minimum
		// bounding box based on the data, while the example does not.
		name: "http://arborjs.org/docs/barnes-hut example",
		particles: []particle2{
			{x: 64.5, y: 81.5, m: 1, name: "A"},
			{x: 242, y: 34, m: 1, name: "B"},
			{x: 199, y: 69, m: 1, name: "C"},
			{x: 285, y: 106.5, m: 1, name: "D"},
			{x: 170, y: 194.5, m: 1, name: "E"},
			{x: 42.5, y: 334.5, m: 1, name: "F"},
			{x: 147, y: 309, m: 1, name: "G"},
			{x: 236.5, y: 324, m: 1, name: "H"},
		},
		want: &Plane{
			root: tile{
				bounds: r2.Box{Min: r2.Vec{X: 42.5, Y: 34}, Max: r2.Vec{X: 285, Y: 334.5}},
				nodes: [4]*tile{
					{
						bounds: r2.Box{Min: r2.Vec{X: 163.75, Y: 34}, Max: r2.Vec{X: 285, Y: 184.25}},
						nodes: [4]*tile{
							ne: {
								bounds: r2.Box{Min: r2.Vec{X: 224.375, Y: 34}, Max: r2.Vec{X: 285, Y: 109.125}},
								nodes: [4]*tile{
									se: {
										particle: particle2{x: 285, y: 106.5, m: 1, name: "D"},
										bounds:   r2.Box{Min: r2.Vec{X: 254.6875, Y: 71.5625}, Max: r2.Vec{X: 285, Y: 109.125}},
										center:   r2.Vec{X: 285, Y: 106.5},
										mass:     1,
									},
									nw: {
										particle: particle2{x: 242, y: 34, m: 1, name: "B"},
										bounds:   r2.Box{Min: r2.Vec{X: 224.375, Y: 34}, Max: r2.Vec{X: 254.6875, Y: 71.5625}},
										center:   r2.Vec{X: 242, Y: 34},
										mass:     1,
									},
								},
								center: r2.Vec{X: 263.5, Y: 70.25},
								mass:   2,
							},
							nw: {
								particle: particle2{x: 199, y: 69, m: 1, name: "C"},
								bounds:   r2.Box{Min: r2.Vec{X: 163.75, Y: 34}, Max: r2.Vec{X: 224.375, Y: 109.125}},
								center:   r2.Vec{X: 199, Y: 69},
								mass:     1,
							},
						},
						center: r2.Vec{X: 242, Y: 69.83333333333333},
						mass:   3,
					},
					{
						bounds: r2.Box{Min: r2.Vec{X: 163.75, Y: 184.25}, Max: r2.Vec{X: 285, Y: 334.5}},
						nodes: [4]*tile{
							se: {
								particle: particle2{x: 236.5, y: 324, m: 1, name: "H"},
								bounds:   r2.Box{Min: r2.Vec{X: 224.375, Y: 259.375}, Max: r2.Vec{X: 285, Y: 334.5}},
								center:   r2.Vec{X: 236.5, Y: 324},
								mass:     1,
							},
							nw: {
								particle: particle2{x: 170, y: 194.5, m: 1, name: "E"},
								bounds:   r2.Box{Min: r2.Vec{X: 163.75, Y: 184.25}, Max: r2.Vec{X: 224.375, Y: 259.375}},
								center:   r2.Vec{X: 170, Y: 194.5},
								mass:     1,
							},
						},
						center: r2.Vec{X: 203.25, Y: 259.25},
						mass:   2,
					},
					{
						bounds: r2.Box{Min: r2.Vec{X: 42.5, Y: 184.25}, Max: r2.Vec{X: 163.75, Y: 334.5}},
						nodes: [4]*tile{
							se: {
								particle: particle2{x: 147, y: 309, m: 1, name: "G"},
								bounds:   r2.Box{Min: r2.Vec{X: 103.125, Y: 259.375}, Max: r2.Vec{X: 163.75, Y: 334.5}},
								center:   r2.Vec{X: 147, Y: 309},
								mass:     1,
							},
							sw: {
								particle: particle2{x: 42.5, y: 334.5, m: 1, name: "F"},
								bounds:   r2.Box{Min: r2.Vec{X: 42.5, Y: 259.375}, Max: r2.Vec{X: 103.125, Y: 334.5}},
								center:   r2.Vec{X: 42.5, Y: 334.5},
								mass:     1,
							},
						},
						center: r2.Vec{X: 94.75, Y: 321.75},
						mass:   2,
					},
					{
						particle: particle2{x: 64.5, y: 81.5, m: 1, name: "A"},
						bounds:   r2.Box{Min: r2.Vec{X: 42.5, Y: 34}, Max: r2.Vec{X: 163.75, Y: 184.25}},
						center:   r2.Vec{X: 64.5, Y: 81.5},
						mass:     1,
					},
				},
				center: r2.Vec{X: 173.3125, Y: 181.625},
				mass:   8,
			},

			Particles: []Particle2{
				particle2{x: 64.5, y: 81.5, m: 1, name: "A"},
				particle2{x: 242, y: 34, m: 1, name: "B"},
				particle2{x: 199, y: 69, m: 1, name: "C"},
				particle2{x: 285, y: 106.5, m: 1, name: "D"},
				particle2{x: 170, y: 194.5, m: 1, name: "E"},
				particle2{x: 42.5, y: 334.5, m: 1, name: "F"},
				particle2{x: 147, y: 309, m: 1, name: "G"},
				particle2{x: 236.5, y: 324, m: 1, name: "H"},
			},
		},
	},
}

func TestPlane(t *testing.T) {
	const tol = 1e-15

	for _, test := range planeTests {
		var particles []Particle2
		if test.particles != nil {
			particles = make([]Particle2, len(test.particles))
		}
		for i, p := range test.particles {
			particles[i] = p
		}

		got, err := NewPlane(particles)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		if test.want != nil && !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}

		// Recursively check all internal centers of mass.
		walkPlane(&got.root, func(tl *tile) {
			var sub []Particle2
			walkPlane(tl, func(tl *tile) {
				if tl.particle != nil {
					sub = append(sub, tl.particle)
				}
			})
			center, mass := centerOfMass2(sub)
			if !floats.EqualWithinAbsOrRel(center.X, tl.center.X, tol, tol) || !floats.EqualWithinAbsOrRel(center.Y, tl.center.Y, tol, tol) {
				t.Errorf("unexpected result for %q for center of mass: got:%f want:%f", test.name, tl.center, center)
			}
			if !floats.EqualWithinAbsOrRel(mass, tl.mass, tol, tol) {
				t.Errorf("unexpected result for %q for total mass: got:%f want:%f", test.name, tl.mass, mass)
			}
		})
	}
}

func centerOfMass2(particles []Particle2) (center r2.Vec, mass float64) {
	for _, p := range particles {
		m := p.Mass()
		mass += m
		c := p.Coord2()
		center.X += c.X * m
		center.Y += c.Y * m
	}
	if mass != 0 {
		center.X /= mass
		center.Y /= mass
	}
	return center, mass
}

func walkPlane(t *tile, fn func(*tile)) {
	if t == nil {
		return
	}
	fn(t)
	for _, q := range t.nodes {
		walkPlane(q, fn)
	}
}

func TestPlaneForceOn(t *testing.T) {
	const (
		size = 1000
		tol  = 0.07
	)
	for _, n := range []int{3e3, 1e4, 3e4} {
		rnd := rand.New(rand.NewSource(1))
		particles := make([]Particle2, n)
		for i := range particles {
			particles[i] = particle2{x: size * rnd.Float64(), y: size * rnd.Float64(), m: 1}
		}

		moved := make([]r2.Vec, n)
		for i, p := range particles {
			var v r2.Vec
			m := p.Mass()
			pv := p.Coord2()
			for _, e := range particles {
				v = v.Add(Gravity2(p, e, m, e.Mass(), e.Coord2().Sub(pv)))
			}
			moved[i] = p.Coord2().Add(v)
		}

		plane, err := NewPlane(particles)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		for _, theta := range []float64{0, 0.3, 0.6, 0.9} {
			t.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(t *testing.T) {
				var ssd, sd float64
				var calls int
				for i, p := range particles {
					v := plane.ForceOn(p, theta, func(p1, p2 Particle2, m1, m2 float64, v r2.Vec) r2.Vec {
						calls++
						return Gravity2(p1, p2, m1, m2, v)
					})
					pos := p.Coord2().Add(v)
					d := moved[i].Sub(pos)
					ssd += d.X*d.X + d.Y*d.Y
					sd += math.Hypot(d.X, d.Y)
				}
				rmsd := math.Sqrt(ssd / float64(len(particles)))
				if rmsd > tol {
					t.Error("RMSD for approximation too high")
				}
				t.Logf("rmsd=%.4v md=%.4v calls/particle=%.5v",
					rmsd, sd/float64(len(particles)), float64(calls)/float64(len(particles)))
			})
		}
	}
}

var (
	fv2sink   r2.Vec
	planeSink *Plane
)

func BenchmarkNewPlane(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5, 1e6} {
		rnd := rand.New(rand.NewSource(1))
		particles := make([]Particle2, n)
		for i := range particles {
			particles[i] = particle2{x: rnd.Float64(), y: rnd.Float64(), m: 1}
		}
		b.ResetTimer()
		var err error
		b.Run(fmt.Sprintf("%d-body", len(particles)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				planeSink, err = NewPlane(particles)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkPlaneForceOn(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5} {
		for _, theta := range []float64{0, 0.1, 0.5, 1, 1.5} {
			if n > 1e4 && theta < 0.5 {
				// Don't run unreasonably long benchmarks.
				continue
			}
			rnd := rand.New(rand.NewSource(1))
			particles := make([]Particle2, n)
			for i := range particles {
				particles[i] = particle2{x: rnd.Float64(), y: rnd.Float64(), m: 1}
			}
			plane, err := NewPlane(particles)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			b.ResetTimer()
			b.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for _, p := range particles {
						fv2sink = plane.ForceOn(p, theta, Gravity2)
					}
				}
			})
		}
	}
}

func BenchmarkPlaneFull(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5} {
		for _, theta := range []float64{0, 0.1, 0.5, 1, 1.5} {
			if n > 1e4 && theta < 0.5 {
				// Don't run unreasonably long benchmarks.
				continue
			}
			rnd := rand.New(rand.NewSource(1))
			particles := make([]Particle2, n)
			for i := range particles {
				particles[i] = particle2{x: rnd.Float64(), y: rnd.Float64(), m: 1}
			}
			b.ResetTimer()
			b.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					plane, err := NewPlane(particles)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
					for _, p := range particles {
						fv2sink = plane.ForceOn(p, theta, Gravity2)
					}
				}
			})
		}
	}
}
