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
	"gonum.org/v1/gonum/spatial/r3"
)

type particle3 struct {
	x, y, z, m float64
	name       string
}

func (p particle3) Coord3() r3.Vec { return r3.Vec{X: p.x, Y: p.y, Z: p.z} }
func (p particle3) Mass() float64  { return p.m }

var volumeTests = []struct {
	name      string
	particles []particle3
	want      *Volume
}{
	{
		name:      "nil",
		particles: nil,
		want:      &Volume{},
	},
	{
		name:      "empty",
		particles: []particle3{},
		want:      &Volume{Particles: []Particle3{}},
	},
	{
		name:      "one",
		particles: []particle3{{m: 1}}, // Must have a mass to avoid vacuum decay.
		want: &Volume{
			root: bucket{
				particle: particle3{x: 0, y: 0, z: 0, m: 1},
				bounds:   r3.Box{Min: r3.Vec{X: 0, Y: 0, Z: 0}, Max: r3.Vec{X: 0, Y: 0, Z: 0}},
				center:   r3.Vec{X: 0, Y: 0, Z: 0},
				mass:     1,
			},

			Particles: []Particle3{
				particle3{x: 0, y: 0, z: 0, m: 1},
			},
		},
	},
	{
		name: "3 corners",
		particles: []particle3{
			{x: 1, y: 1, z: 1, m: 1},
			{x: -1, y: 1, z: 0, m: 1},
			{x: -1, y: -1, z: -1, m: 1},
		},
		want: &Volume{
			root: bucket{
				bounds: r3.Box{Min: r3.Vec{X: -1, Y: -1, Z: -1}, Max: r3.Vec{X: 1, Y: 1, Z: 1}},
				nodes: [8]*bucket{
					lnw: {
						particle: particle3{x: -1, y: -1, z: -1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -1, Y: -1, Z: -1}, Max: r3.Vec{X: 0, Y: 0, Z: 0}},
						center:   r3.Vec{X: -1, Y: -1, Z: -1},
						mass:     1,
					},
					use: {
						particle: particle3{x: 1, y: 1, z: 1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: 0, Y: 0, Z: 0}, Max: r3.Vec{X: 1, Y: 1, Z: 1}},
						center:   r3.Vec{X: 1, Y: 1, Z: 1},
						mass:     1,
					},
					usw: {
						particle: particle3{x: -1, y: 1, z: 0, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -1, Y: 0, Z: 0}, Max: r3.Vec{X: 0, Y: 1, Z: 1}},
						center:   r3.Vec{X: -1, Y: 1, Z: 0},
						mass:     1,
					},
				},
				center: r3.Vec{X: -0.3333333333333333, Y: 0.3333333333333333, Z: 0},
				mass:   3,
			},

			Particles: []Particle3{
				particle3{x: 1, y: 1, z: 1, m: 1},
				particle3{x: -1, y: 1, z: 0, m: 1},
				particle3{x: -1, y: -1, z: -1, m: 1},
			},
		},
	},
	{
		name: "4 corners",
		particles: []particle3{
			{x: 1, y: 1, z: -1, m: 1},
			{x: -1, y: 1, z: 1, m: 1},
			{x: 1, y: -1, z: 1, m: 1},
			{x: -1, y: -1, z: -1, m: 1},
		},
		want: &Volume{
			root: bucket{
				bounds: r3.Box{Min: r3.Vec{X: -1, Y: -1, Z: -1}, Max: r3.Vec{X: 1, Y: 1, Z: 1}},
				nodes: [8]*bucket{
					lse: {
						particle: particle3{x: 1, y: 1, z: -1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: 0, Y: 0, Z: -1}, Max: r3.Vec{X: 1, Y: 1, Z: 0}},
						center:   r3.Vec{X: 1, Y: 1, Z: -1},
						mass:     1,
					},
					lnw: {
						particle: particle3{x: -1, y: -1, z: -1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -1, Y: -1, Z: -1}, Max: r3.Vec{X: 0, Y: 0, Z: 0}},
						center:   r3.Vec{X: -1, Y: -1, Z: -1},
						mass:     1,
					},
					une: {
						particle: particle3{x: 1, y: -1, z: 1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: 0, Y: -1, Z: 0}, Max: r3.Vec{X: 1, Y: 0, Z: 1}},
						center:   r3.Vec{X: 1, Y: -1, Z: 1},
						mass:     1,
					},
					usw: {
						particle: particle3{x: -1, y: 1, z: 1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -1, Y: 0, Z: 0}, Max: r3.Vec{X: 0, Y: 1, Z: 1}},
						center:   r3.Vec{X: -1, Y: 1, Z: 1},
						mass:     1,
					},
				},
				center: r3.Vec{X: 0, Y: 0, Z: 0},
				mass:   4,
			},

			Particles: []Particle3{
				particle3{x: 1, y: 1, z: -1, m: 1},
				particle3{x: -1, y: 1, z: 1, m: 1},
				particle3{x: 1, y: -1, z: 1, m: 1},
				particle3{x: -1, y: -1, z: -1, m: 1},
			},
		},
	},
	{
		name: "5 corners",
		particles: []particle3{
			{x: 1, y: 1, z: -1, m: 1},
			{x: -1, y: 1, z: 1, m: 1},
			{x: 1, y: -1, z: 1, m: 1},
			{x: -1, y: -1, z: -1, m: 1},
			{x: -1.1, y: -1, z: -1.1, m: 1},
		},
		want: &Volume{
			root: bucket{
				bounds: r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: 1, Y: 1, Z: 1}},
				nodes: [8]*bucket{
					lse: {
						particle: particle3{x: 1, y: 1, z: -1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -0.050000000000000044, Y: 0, Z: -1.1}, Max: r3.Vec{X: 1, Y: 1, Z: -0.050000000000000044}},
						center:   r3.Vec{X: 1, Y: 1, Z: -1},
						mass:     1,
					},
					lnw: {
						bounds: r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: -0.050000000000000044, Y: 0, Z: -0.050000000000000044}},
						nodes: [8]*bucket{
							lnw: {
								bounds: r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: -0.5750000000000001, Y: -0.5, Z: -0.5750000000000001}},
								nodes: [8]*bucket{
									lnw: {
										bounds: r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: -0.8375000000000001, Y: -0.75, Z: -0.8375000000000001}},
										nodes: [8]*bucket{
											lnw: {
												bounds: r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: -0.9687500000000001, Y: -0.875, Z: -0.9687500000000001}},
												nodes: [8]*bucket{
													lnw: {
														particle: particle3{x: -1.1, y: -1, z: -1.1, m: 1},
														bounds:   r3.Box{Min: r3.Vec{X: -1.1, Y: -1, Z: -1.1}, Max: r3.Vec{X: -1.034375, Y: -0.9375, Z: -1.034375}},
														center:   r3.Vec{X: -1.1, Y: -1, Z: -1.1},
														mass:     1,
													},
													une: {
														particle: particle3{x: -1, y: -1, z: -1, m: 1},
														bounds:   r3.Box{Min: r3.Vec{X: -1.034375, Y: -1, Z: -1.034375}, Max: r3.Vec{X: -0.9687500000000001, Y: -0.9375, Z: -0.9687500000000001}},
														center:   r3.Vec{X: -1, Y: -1, Z: -1},
														mass:     1,
													},
												},
												center: r3.Vec{X: -1.05, Y: -1, Z: -1.05},
												mass:   2,
											},
										},
										center: r3.Vec{X: -1.05, Y: -1, Z: -1.05},
										mass:   2,
									},
								},
								center: r3.Vec{X: -1.05, Y: -1, Z: -1.05},
								mass:   2,
							},
						},
						center: r3.Vec{X: -1.05, Y: -1, Z: -1.05},
						mass:   2,
					},
					une: {
						particle: particle3{x: 1, y: -1, z: 1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -0.050000000000000044, Y: -1, Z: -0.050000000000000044}, Max: r3.Vec{X: 1, Y: 0, Z: 1}},
						center:   r3.Vec{X: 1, Y: -1, Z: 1},
						mass:     1,
					},
					usw: {
						particle: particle3{x: -1, y: 1, z: 1, m: 1},
						bounds:   r3.Box{Min: r3.Vec{X: -1.1, Y: 0, Z: -0.050000000000000044}, Max: r3.Vec{X: -0.050000000000000044, Y: 1, Z: 1}},
						center:   r3.Vec{X: -1, Y: 1, Z: 1},
						mass:     1,
					},
				},
				center: r3.Vec{X: -0.22000000000000003, Y: -0.2, Z: -0.22000000000000003},
				mass:   5,
			},
			Particles: []Particle3{
				particle3{x: 1, y: 1, z: -1, m: 1},
				particle3{x: -1, y: 1, z: 1, m: 1},
				particle3{x: 1, y: -1, z: 1, m: 1},
				particle3{x: -1, y: -1, z: -1, m: 1},
				particle3{x: -1.1, y: -1, z: -1.1, m: 1},
			},
		},
	},
	{
		// This case is derived from the 2D example of the same name,
		// but with a monotonic increase in Z position according to name.
		name: "http://arborjs.org/docs/barnes-hut example",
		particles: []particle3{
			{x: 64.5, y: 81.5, z: 0, m: 1, name: "A"},
			{x: 242, y: 34, z: 40, m: 1, name: "B"},
			{x: 199, y: 69, z: 80, m: 1, name: "C"},
			{x: 285, y: 106.5, z: 120, m: 1, name: "D"},
			{x: 170, y: 194.5, z: 160, m: 1, name: "E"},
			{x: 42.5, y: 334.5, z: 200, m: 1, name: "F"},
			{x: 147, y: 309, z: 240, m: 1, name: "G"},
			{x: 236.5, y: 324, z: 280, m: 1, name: "H"},
		},
		want: &Volume{
			root: bucket{
				bounds: r3.Box{Min: r3.Vec{X: 42.5, Y: 34, Z: 0}, Max: r3.Vec{X: 285, Y: 334.5, Z: 280}},
				nodes: [8]*bucket{
					lne: {
						bounds: r3.Box{Min: r3.Vec{X: 163.75, Y: 34, Z: 0}, Max: r3.Vec{X: 285, Y: 184.25, Z: 140}},
						nodes: [8]*bucket{
							lne: {
								particle: particle3{x: 242, y: 34, z: 40, m: 1, name: "B"},
								bounds:   r3.Box{Min: r3.Vec{X: 224.375, Y: 34, Z: 0}, Max: r3.Vec{X: 285, Y: 109.125, Z: 70}},
								center:   r3.Vec{X: 242, Y: 34, Z: 40},
								mass:     1,
							},
							une: {
								particle: particle3{x: 285, y: 106.5, z: 120, m: 1, name: "D"},
								bounds:   r3.Box{Min: r3.Vec{X: 224.375, Y: 34, Z: 70}, Max: r3.Vec{X: 285, Y: 109.125, Z: 140}},
								center:   r3.Vec{X: 285, Y: 106.5, Z: 120},
								mass:     1,
							},
							unw: {
								particle: particle3{x: 199, y: 69, z: 80, m: 1, name: "C"},
								bounds:   r3.Box{Min: r3.Vec{X: 163.75, Y: 34, Z: 70}, Max: r3.Vec{X: 224.375, Y: 109.125, Z: 140}},
								center:   r3.Vec{X: 199, Y: 69, Z: 80},
								mass:     1,
							},
						},
						center: r3.Vec{X: 242, Y: 69.83333333333333, Z: 80},
						mass:   3,
					},
					lnw: {
						particle: particle3{x: 64.5, y: 81.5, z: 0, m: 1, name: "A"},
						bounds:   r3.Box{Min: r3.Vec{X: 42.5, Y: 34, Z: 0}, Max: r3.Vec{X: 163.75, Y: 184.25, Z: 140}},
						center:   r3.Vec{X: 64.5, Y: 81.5, Z: 0},
						mass:     1,
					},
					(*bucket)(nil),
					use: {
						bounds: r3.Box{Min: r3.Vec{X: 163.75, Y: 184.25, Z: 140}, Max: r3.Vec{X: 285, Y: 334.5, Z: 280}},
						nodes: [8]*bucket{
							lnw: {
								particle: particle3{x: 170, y: 194.5, z: 160, m: 1, name: "E"},
								bounds:   r3.Box{Min: r3.Vec{X: 163.75, Y: 184.25, Z: 140}, Max: r3.Vec{X: 224.375, Y: 259.375, Z: 210}},
								center:   r3.Vec{X: 170, Y: 194.5, Z: 160},
								mass:     1,
							},
							use: {
								particle: particle3{x: 236.5, y: 324, z: 280, m: 1, name: "H"},
								bounds:   r3.Box{Min: r3.Vec{X: 224.375, Y: 259.375, Z: 210}, Max: r3.Vec{X: 285, Y: 334.5, Z: 280}},
								center:   r3.Vec{X: 236.5, Y: 324, Z: 280},
								mass:     1,
							},
						},
						center: r3.Vec{X: 203.25, Y: 259.25, Z: 220},
						mass:   2,
					},
					usw: {
						bounds: r3.Box{Min: r3.Vec{X: 42.5, Y: 184.25, Z: 140}, Max: r3.Vec{X: 163.75, Y: 334.5, Z: 280}},
						nodes: [8]*bucket{
							lsw: {
								particle: particle3{x: 42.5, y: 334.5, z: 200, m: 1, name: "F"},
								bounds:   r3.Box{Min: r3.Vec{X: 42.5, Y: 259.375, Z: 140}, Max: r3.Vec{X: 103.125, Y: 334.5, Z: 210}},
								center:   r3.Vec{X: 42.5, Y: 334.5, Z: 200},
								mass:     1,
							},
							use: {
								particle: particle3{x: 147, y: 309, z: 240, m: 1, name: "G"},
								bounds:   r3.Box{Min: r3.Vec{X: 103.125, Y: 259.375, Z: 210}, Max: r3.Vec{X: 163.75, Y: 334.5, Z: 280}},
								center:   r3.Vec{X: 147, Y: 309, Z: 240},
								mass:     1,
							},
						},
						center: r3.Vec{X: 94.75, Y: 321.75, Z: 220},
						mass:   2,
					},
				},
				center: r3.Vec{X: 173.3125, Y: 181.625, Z: 140},
				mass:   8,
			},

			Particles: []Particle3{
				particle3{x: 64.5, y: 81.5, z: 0, m: 1, name: "A"},
				particle3{x: 242, y: 34, z: 40, m: 1, name: "B"},
				particle3{x: 199, y: 69, z: 80, m: 1, name: "C"},
				particle3{x: 285, y: 106.5, z: 120, m: 1, name: "D"},
				particle3{x: 170, y: 194.5, z: 160, m: 1, name: "E"},
				particle3{x: 42.5, y: 334.5, z: 200, m: 1, name: "F"},
				particle3{x: 147, y: 309, z: 240, m: 1, name: "G"},
				particle3{x: 236.5, y: 324, z: 280, m: 1, name: "H"},
			},
		},
	},
}

func TestVolume(t *testing.T) {
	const tol = 1e-15

	for _, test := range volumeTests {
		var particles []Particle3
		if test.particles != nil {
			particles = make([]Particle3, len(test.particles))
		}
		for i, p := range test.particles {
			particles[i] = p
		}

		got, err := NewVolume(particles)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		if test.want != nil && !reflect.DeepEqual(got, test.want) {
			t.Errorf("unexpected result for %q: got:%v want:%v", test.name, got, test.want)
		}

		// Recursively check all internal centers of mass.
		walkVolume(&got.root, func(b *bucket) {
			var sub []Particle3
			walkVolume(b, func(b *bucket) {
				if b.particle != nil {
					sub = append(sub, b.particle)
				}
			})
			center, mass := centerOfMass3(sub)
			if !floats.EqualWithinAbsOrRel(center.X, b.center.X, tol, tol) || !floats.EqualWithinAbsOrRel(center.Y, b.center.Y, tol, tol) || !floats.EqualWithinAbsOrRel(center.Z, b.center.Z, tol, tol) {
				t.Errorf("unexpected result for %q for center of mass: got:%f want:%f", test.name, b.center, center)
			}
			if !floats.EqualWithinAbsOrRel(mass, b.mass, tol, tol) {
				t.Errorf("unexpected result for %q for total mass: got:%f want:%f", test.name, b.mass, mass)
			}
		})
	}
}

func centerOfMass3(particles []Particle3) (center r3.Vec, mass float64) {
	for _, p := range particles {
		m := p.Mass()
		mass += m
		c := p.Coord3()
		center.X += c.X * m
		center.Y += c.Y * m
		center.Z += c.Z * m
	}
	if mass != 0 {
		center.X /= mass
		center.Y /= mass
		center.Z /= mass
	}
	return center, mass
}

func walkVolume(t *bucket, fn func(*bucket)) {
	if t == nil {
		return
	}
	fn(t)
	for _, q := range t.nodes {
		walkVolume(q, fn)
	}
}

func TestVolumeForceOn(t *testing.T) {
	const (
		size = 1000
		tol  = 1e-3
	)
	for _, n := range []int{3e3, 1e4, 3e4} {
		rnd := rand.New(rand.NewSource(1))
		particles := make([]Particle3, n)
		for i := range particles {
			particles[i] = particle3{x: size * rnd.Float64(), y: size * rnd.Float64(), z: size * rnd.Float64(), m: 1}
		}

		moved := make([]r3.Vec, n)
		for i, p := range particles {
			var v r3.Vec
			m := p.Mass()
			pv := p.Coord3()
			for _, e := range particles {
				v = v.Add(Gravity3(p, e, m, e.Mass(), e.Coord3().Sub(pv)))
			}
			moved[i] = p.Coord3().Add(v)
		}

		volume, err := NewVolume(particles)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}
		for _, theta := range []float64{0, 0.3, 0.6, 0.9} {
			t.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(t *testing.T) {
				var ssd, sd float64
				var calls int
				for i, p := range particles {
					v := volume.ForceOn(p, theta, func(p1, p2 Particle3, m1, m2 float64, v r3.Vec) r3.Vec {
						calls++
						return Gravity3(p1, p2, m1, m2, v)
					})
					pos := p.Coord3().Add(v)
					d := moved[i].Sub(pos)
					ssd += d.X*d.X + d.Y*d.Y + d.Z*d.Z
					sd += math.Hypot(math.Hypot(d.X, d.Y), d.Z)
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
	fv3sink    r3.Vec
	volumeSink *Volume
)

func BenchmarkNewVolume(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5, 1e6} {
		rnd := rand.New(rand.NewSource(1))
		particles := make([]Particle3, n)
		for i := range particles {
			particles[i] = particle3{x: rnd.Float64(), y: rnd.Float64(), z: rnd.Float64(), m: 1}
		}
		b.ResetTimer()
		var err error
		b.Run(fmt.Sprintf("%d-body", len(particles)), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				volumeSink, err = NewVolume(particles)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

func BenchmarkVolumeForceOn(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5} {
		for _, theta := range []float64{0, 0.1, 0.5, 1, 1.5} {
			if n > 1e4 && theta < 0.5 {
				// Don't run unreasonably long benchmarks.
				continue
			}
			rnd := rand.New(rand.NewSource(1))
			particles := make([]Particle3, n)
			for i := range particles {
				particles[i] = particle3{x: rnd.Float64(), y: rnd.Float64(), z: rnd.Float64(), m: 1}
			}
			volume, err := NewVolume(particles)
			if err != nil {
				b.Fatalf("unexpected error: %v", err)
			}
			b.ResetTimer()
			b.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					for _, p := range particles {
						fv3sink = volume.ForceOn(p, theta, Gravity3)
					}
				}
			})
		}
	}
}

func BenchmarkVolumeFull(b *testing.B) {
	for _, n := range []int{1e3, 1e4, 1e5} {
		for _, theta := range []float64{0, 0.1, 0.5, 1, 1.5} {
			if n > 1e4 && theta < 0.5 {
				// Don't run unreasonably long benchmarks.
				continue
			}
			rnd := rand.New(rand.NewSource(1))
			particles := make([]Particle3, n)
			for i := range particles {
				particles[i] = particle3{x: rnd.Float64(), y: rnd.Float64(), z: rnd.Float64(), m: 1}
			}
			b.ResetTimer()
			b.Run(fmt.Sprintf("%d-body/theta=%v", len(particles), theta), func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					volume, err := NewVolume(particles)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
					for _, p := range particles {
						fv3sink = volume.ForceOn(p, theta, Gravity3)
					}
				}
			})
		}
	}
}
