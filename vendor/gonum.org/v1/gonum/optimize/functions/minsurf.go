// Copyright ©2015 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package functions

import (
	"fmt"
	"math"
)

// MinimalSurface implements a finite element approximation to a minimal
// surface problem: determine the surface with minimal area and given boundary
// values in a unit square centered at the origin.
//
// References:
//  Averick, M.B., Carter, R.G., Moré, J.J., Xue, G.-L.: The Minpack-2 Test
//  Problem Collection. Preprint MCS-P153-0692, Argonne National Laboratory (1992)
type MinimalSurface struct {
	bottom, top  []float64
	left, right  []float64
	origin, step [2]float64
}

// NewMinimalSurface creates a new discrete minimal surface problem and
// precomputes its boundary values. The problem is discretized on a rectilinear
// grid with nx×ny nodes which means that the problem dimension is (nx-2)(ny-2).
func NewMinimalSurface(nx, ny int) *MinimalSurface {
	ms := &MinimalSurface{
		bottom: make([]float64, nx),
		top:    make([]float64, nx),
		left:   make([]float64, ny),
		right:  make([]float64, ny),
		origin: [2]float64{-0.5, -0.5},
		step:   [2]float64{1 / float64(nx-1), 1 / float64(ny-1)},
	}

	ms.initBoundary(ms.bottom, ms.origin[0], ms.origin[1], ms.step[0], 0)
	startY := ms.origin[1] + float64(ny-1)*ms.step[1]
	ms.initBoundary(ms.top, ms.origin[0], startY, ms.step[0], 0)
	ms.initBoundary(ms.left, ms.origin[0], ms.origin[1], 0, ms.step[1])
	startX := ms.origin[0] + float64(nx-1)*ms.step[0]
	ms.initBoundary(ms.right, startX, ms.origin[1], 0, ms.step[1])

	return ms
}

// Func returns the area of the surface represented by the vector x.
func (ms *MinimalSurface) Func(x []float64) (area float64) {
	nx, ny := ms.Dims()
	if len(x) != (nx-2)*(ny-2) {
		panic("functions: problem size mismatch")
	}

	hx, hy := ms.Steps()
	for j := 0; j < ny-1; j++ {
		for i := 0; i < nx-1; i++ {
			vLL := ms.at(i, j, x)
			vLR := ms.at(i+1, j, x)
			vUL := ms.at(i, j+1, x)
			vUR := ms.at(i+1, j+1, x)

			dvLdx := (vLR - vLL) / hx
			dvLdy := (vUL - vLL) / hy
			dvUdx := (vUR - vUL) / hx
			dvUdy := (vUR - vLR) / hy

			fL := math.Sqrt(1 + dvLdx*dvLdx + dvLdy*dvLdy)
			fU := math.Sqrt(1 + dvUdx*dvUdx + dvUdy*dvUdy)
			area += fL + fU
		}
	}
	area *= 0.5 * hx * hy
	return area
}

// Grad evaluates the area gradient of the surface represented by the vector.
func (ms *MinimalSurface) Grad(grad, x []float64) {
	nx, ny := ms.Dims()
	if len(x) != (nx-2)*(ny-2) {
		panic("functions: problem size mismatch")
	}
	if grad != nil && len(x) != len(grad) {
		panic("functions: unexpected size mismatch")
	}

	for i := range grad {
		grad[i] = 0
	}
	hx, hy := ms.Steps()
	for j := 0; j < ny-1; j++ {
		for i := 0; i < nx-1; i++ {
			vLL := ms.at(i, j, x)
			vLR := ms.at(i+1, j, x)
			vUL := ms.at(i, j+1, x)
			vUR := ms.at(i+1, j+1, x)

			dvLdx := (vLR - vLL) / hx
			dvLdy := (vUL - vLL) / hy
			dvUdx := (vUR - vUL) / hx
			dvUdy := (vUR - vLR) / hy

			fL := math.Sqrt(1 + dvLdx*dvLdx + dvLdy*dvLdy)
			fU := math.Sqrt(1 + dvUdx*dvUdx + dvUdy*dvUdy)

			if grad != nil {
				if i > 0 {
					if j > 0 {
						grad[ms.index(i, j)] -= (dvLdx/hx + dvLdy/hy) / fL
					}
					if j < ny-2 {
						grad[ms.index(i, j+1)] += (dvLdy/hy)/fL - (dvUdx/hx)/fU
					}
				}
				if i < nx-2 {
					if j > 0 {
						grad[ms.index(i+1, j)] += (dvLdx/hx)/fL - (dvUdy/hy)/fU
					}
					if j < ny-2 {
						grad[ms.index(i+1, j+1)] += (dvUdx/hx + dvUdy/hy) / fU
					}
				}
			}
		}

	}
	cellSize := 0.5 * hx * hy
	for i := range grad {
		grad[i] *= cellSize
	}
}

// InitX returns a starting location for the minimization problem. Length of
// the returned slice is (nx-2)(ny-2).
func (ms *MinimalSurface) InitX() []float64 {
	nx, ny := ms.Dims()
	x := make([]float64, (nx-2)*(ny-2))
	for j := 1; j < ny-1; j++ {
		for i := 1; i < nx-1; i++ {
			x[ms.index(i, j)] = (ms.left[j] + ms.bottom[i]) / 2
		}
	}
	return x
}

// ExactX returns the exact solution to the _continuous_ minimization problem
// projected on the interior nodes of the grid. Length of the returned slice is
// (nx-2)(ny-2).
func (ms *MinimalSurface) ExactX() []float64 {
	nx, ny := ms.Dims()
	v := make([]float64, (nx-2)*(ny-2))
	for j := 1; j < ny-1; j++ {
		for i := 1; i < nx-1; i++ {
			v[ms.index(i, j)] = ms.ExactSolution(ms.x(i), ms.y(j))
		}
	}
	return v
}

// ExactSolution returns the value of the exact solution to the minimal surface
// problem at (x,y). The exact solution is
//  F_exact(x,y) = U^2(x,y) - V^2(x,y),
// where U and V are the unique solutions to the equations
//  x =  u + uv^2 - u^3/3,
//  y = -v - u^2v + v^3/3.
func (ms *MinimalSurface) ExactSolution(x, y float64) float64 {
	var u = [2]float64{x, -y}
	var f [2]float64
	var jac [2][2]float64
	for k := 0; k < 100; k++ {
		f[0] = u[0] + u[0]*u[1]*u[1] - u[0]*u[0]*u[0]/3 - x
		f[1] = -u[1] - u[0]*u[0]*u[1] + u[1]*u[1]*u[1]/3 - y
		fNorm := math.Hypot(f[0], f[1])
		if fNorm < 1e-13 {
			break
		}
		jac[0][0] = 1 + u[1]*u[1] - u[0]*u[0]
		jac[0][1] = 2 * u[0] * u[1]
		jac[1][0] = -2 * u[0] * u[1]
		jac[1][1] = -1 - u[0]*u[0] + u[1]*u[1]
		det := jac[0][0]*jac[1][1] - jac[0][1]*jac[1][0]
		u[0] -= (jac[1][1]*f[0] - jac[0][1]*f[1]) / det
		u[1] -= (jac[0][0]*f[1] - jac[1][0]*f[0]) / det
	}
	return u[0]*u[0] - u[1]*u[1]
}

// Dims returns the size of the underlying rectilinear grid.
func (ms *MinimalSurface) Dims() (nx, ny int) {
	return len(ms.bottom), len(ms.left)
}

// Steps returns the spatial step sizes of the underlying rectilinear grid.
func (ms *MinimalSurface) Steps() (hx, hy float64) {
	return ms.step[0], ms.step[1]
}

func (ms *MinimalSurface) x(i int) float64 {
	return ms.origin[0] + float64(i)*ms.step[0]
}

func (ms *MinimalSurface) y(j int) float64 {
	return ms.origin[1] + float64(j)*ms.step[1]
}

func (ms *MinimalSurface) at(i, j int, x []float64) float64 {
	nx, ny := ms.Dims()
	if i < 0 || i >= nx {
		panic(fmt.Sprintf("node [%v,%v] not on grid", i, j))
	}
	if j < 0 || j >= ny {
		panic(fmt.Sprintf("node [%v,%v] not on grid", i, j))
	}

	if i == 0 {
		return ms.left[j]
	}
	if j == 0 {
		return ms.bottom[i]
	}
	if i == nx-1 {
		return ms.right[j]
	}
	if j == ny-1 {
		return ms.top[i]
	}
	return x[ms.index(i, j)]
}

// index maps an interior grid node (i, j) to a one-dimensional index and
// returns it.
func (ms *MinimalSurface) index(i, j int) int {
	nx, ny := ms.Dims()
	if i <= 0 || i >= nx-1 {
		panic(fmt.Sprintf("[%v,%v] is not an interior node", i, j))
	}
	if j <= 0 || j >= ny-1 {
		panic(fmt.Sprintf("[%v,%v] is not an interior node", i, j))
	}

	return i - 1 + (j-1)*(nx-2)
}

// initBoundary initializes with the exact solution the boundary b whose i-th
// element b[i] is located at [startX+i×hx, startY+i×hy].
func (ms *MinimalSurface) initBoundary(b []float64, startX, startY, hx, hy float64) {
	for i := range b {
		x := startX + float64(i)*hx
		y := startY + float64(i)*hy
		b[i] = ms.ExactSolution(x, y)
	}
}
