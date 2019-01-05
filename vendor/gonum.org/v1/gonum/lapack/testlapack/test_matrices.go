// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testlapack

import (
	"math"

	"golang.org/x/exp/rand"

	"gonum.org/v1/gonum/blas/blas64"
)

// A123 is the non-symmetric singular matrix
//      [ 1 2 3 ]
//  A = [ 4 5 6 ]
//      [ 7 8 9 ]
// It has three distinct real eigenvalues.
type A123 struct{}

func (A123) Matrix() blas64.General {
	return blas64.General{
		Rows:   3,
		Cols:   3,
		Stride: 3,
		Data: []float64{
			1, 2, 3,
			4, 5, 6,
			7, 8, 9,
		},
	}
}

func (A123) Eigenvalues() []complex128 {
	return []complex128{16.116843969807043, -1.116843969807043, 0}
}

func (A123) LeftEV() blas64.General {
	return blas64.General{
		Rows:   3,
		Cols:   3,
		Stride: 3,
		Data: []float64{
			-0.464547273387671, -0.570795531228578, -0.677043789069485,
			-0.882905959653586, -0.239520420054206, 0.403865119545174,
			0.408248290463862, -0.816496580927726, 0.408248290463863,
		},
	}
}

func (A123) RightEV() blas64.General {
	return blas64.General{
		Rows:   3,
		Cols:   3,
		Stride: 3,
		Data: []float64{
			-0.231970687246286, -0.785830238742067, 0.408248290463864,
			-0.525322093301234, -0.086751339256628, -0.816496580927726,
			-0.818673499356181, 0.612327560228810, 0.408248290463863,
		},
	}
}

// AntisymRandom is a anti-symmetric random matrix. All its eigenvalues are
// imaginary with one zero if the order is odd.
type AntisymRandom struct {
	mat blas64.General
}

func NewAntisymRandom(n int, rnd *rand.Rand) AntisymRandom {
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			r := rnd.NormFloat64()
			a.Data[i*a.Stride+j] = r
			a.Data[j*a.Stride+i] = -r
		}
	}
	return AntisymRandom{a}
}

func (a AntisymRandom) Matrix() blas64.General {
	return cloneGeneral(a.mat)
}

func (AntisymRandom) Eigenvalues() []complex128 {
	return nil
}

// Circulant is a generally non-symmetric matrix given by
//  A[i,j] = 1 + (j-i+n)%n.
// For example, for n=5,
//      [ 1 2 3 4 5 ]
//      [ 5 1 2 3 4 ]
//  A = [ 4 5 1 2 3 ]
//      [ 3 4 5 1 2 ]
//      [ 2 3 4 5 1 ]
// It has real and complex eigenvalues, some possibly repeated.
type Circulant int

func (c Circulant) Matrix() blas64.General {
	n := int(c)
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			a.Data[i*a.Stride+j] = float64(1 + (j-i+n)%n)
		}
	}
	return a
}

func (c Circulant) Eigenvalues() []complex128 {
	n := int(c)
	w := rootsOfUnity(n)
	ev := make([]complex128, n)
	for k := 0; k < n; k++ {
		ev[k] = complex(float64(n), 0)
	}
	for i := n - 1; i > 0; i-- {
		for k := 0; k < n; k++ {
			ev[k] = ev[k]*w[k] + complex(float64(i), 0)
		}
	}
	return ev
}

// Clement is a generally non-symmetric matrix given by
//  A[i,j] = i+1,  if j == i+1,
//         = n-i,  if j == i-1,
//         = 0,    otherwise.
// For example, for n=5,
//      [ . 1 . . . ]
//      [ 4 . 2 . . ]
//  A = [ . 3 . 3 . ]
//      [ . . 2 . 4 ]
//      [ . . . 1 . ]
// It has n distinct real eigenvalues.
type Clement int

func (c Clement) Matrix() blas64.General {
	n := int(c)
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		if i < n-1 {
			a.Data[i*a.Stride+i+1] = float64(i + 1)
		}
		if i > 0 {
			a.Data[i*a.Stride+i-1] = float64(n - i)
		}
	}
	return a
}

func (c Clement) Eigenvalues() []complex128 {
	n := int(c)
	ev := make([]complex128, n)
	for i := range ev {
		ev[i] = complex(float64(-n+2*i+1), 0)
	}
	return ev
}

// Creation is a singular non-symmetric matrix given by
//  A[i,j] = i,  if j == i-1,
//         = 0,  otherwise.
// For example, for n=5,
//      [ . . . . . ]
//      [ 1 . . . . ]
//  A = [ . 2 . . . ]
//      [ . . 3 . . ]
//      [ . . . 4 . ]
// Zero is its only eigenvalue.
type Creation int

func (c Creation) Matrix() blas64.General {
	n := int(c)
	a := zeros(n, n, n)
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = float64(i)
	}
	return a
}

func (c Creation) Eigenvalues() []complex128 {
	return make([]complex128, int(c))
}

// Diagonal is a diagonal matrix given by
//  A[i,j] = i+1,  if i == j,
//         = 0,    otherwise.
// For example, for n=5,
//      [ 1 . . . . ]
//      [ . 2 . . . ]
//  A = [ . . 3 . . ]
//      [ . . . 4 . ]
//      [ . . . . 5 ]
// It has n real eigenvalues {1,...,n}.
type Diagonal int

func (d Diagonal) Matrix() blas64.General {
	n := int(d)
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		a.Data[i*a.Stride+i] = float64(i)
	}
	return a
}

func (d Diagonal) Eigenvalues() []complex128 {
	n := int(d)
	ev := make([]complex128, n)
	for i := range ev {
		ev[i] = complex(float64(i), 0)
	}
	return ev
}

// Downshift is a non-singular upper Hessenberg matrix given by
//  A[i,j] = 1,  if (i-j+n)%n == 1,
//         = 0,  otherwise.
// For example, for n=5,
//      [ . . . . 1 ]
//      [ 1 . . . . ]
//  A = [ . 1 . . . ]
//      [ . . 1 . . ]
//      [ . . . 1 . ]
// Its eigenvalues are the complex roots of unity.
type Downshift int

func (d Downshift) Matrix() blas64.General {
	n := int(d)
	a := zeros(n, n, n)
	a.Data[n-1] = 1
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = 1
	}
	return a
}

func (d Downshift) Eigenvalues() []complex128 {
	return rootsOfUnity(int(d))
}

// Fibonacci is an upper Hessenberg matrix with 3 distinct real eigenvalues. For
// example, for n=5,
//      [ . 1 . . . ]
//      [ 1 1 . . . ]
//  A = [ . 1 1 . . ]
//      [ . . 1 1 . ]
//      [ . . . 1 1 ]
type Fibonacci int

func (f Fibonacci) Matrix() blas64.General {
	n := int(f)
	a := zeros(n, n, n)
	if n > 1 {
		a.Data[1] = 1
	}
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = 1
		a.Data[i*a.Stride+i] = 1
	}
	return a
}

func (f Fibonacci) Eigenvalues() []complex128 {
	n := int(f)
	ev := make([]complex128, n)
	if n == 0 || n == 1 {
		return ev
	}
	phi := 0.5 * (1 + math.Sqrt(5))
	ev[0] = complex(phi, 0)
	for i := 1; i < n-1; i++ {
		ev[i] = 1 + 0i
	}
	ev[n-1] = complex(1-phi, 0)
	return ev
}

// Gear is a singular non-symmetric matrix with real eigenvalues. For example,
// for n=5,
//      [ . 1 . . 1 ]
//      [ 1 . 1 . . ]
//  A = [ . 1 . 1 . ]
//      [ . . 1 . 1 ]
//      [-1 . . 1 . ]
type Gear int

func (g Gear) Matrix() blas64.General {
	n := int(g)
	a := zeros(n, n, n)
	if n == 1 {
		return a
	}
	for i := 0; i < n-1; i++ {
		a.Data[i*a.Stride+i+1] = 1
	}
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = 1
	}
	a.Data[n-1] = 1
	a.Data[(n-1)*a.Stride] = -1
	return a
}

func (g Gear) Eigenvalues() []complex128 {
	n := int(g)
	ev := make([]complex128, n)
	if n == 0 || n == 1 {
		return ev
	}
	if n == 2 {
		ev[0] = complex(0, 1)
		ev[1] = complex(0, -1)
		return ev
	}
	w := 0
	ev[w] = math.Pi / 2
	w++
	phi := (n - 1) / 2
	for p := 1; p <= phi; p++ {
		ev[w] = complex(float64(2*p)*math.Pi/float64(n), 0)
		w++
	}
	phi = n / 2
	for p := 1; p <= phi; p++ {
		ev[w] = complex(float64(2*p-1)*math.Pi/float64(n), 0)
		w++
	}
	for i, v := range ev {
		ev[i] = complex(2*math.Cos(real(v)), 0)
	}
	return ev
}

// Grcar is an upper Hessenberg matrix given by
//  A[i,j] = -1  if i == j+1,
//         = 1   if i <= j and j <= i+k,
//         = 0   otherwise.
// For example, for n=5 and k=2,
//      [  1  1  1  .  . ]
//      [ -1  1  1  1  . ]
//  A = [  . -1  1  1  1 ]
//      [  .  . -1  1  1 ]
//      [  .  .  . -1  1 ]
// The matrix has sensitive eigenvalues but they are not given explicitly.
type Grcar struct {
	N int
	K int
}

func (g Grcar) Matrix() blas64.General {
	n := g.N
	a := zeros(n, n, n)
	for k := 0; k <= g.K; k++ {
		for i := 0; i < n-k; i++ {
			a.Data[i*a.Stride+i+k] = 1
		}
	}
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = -1
	}
	return a
}

func (Grcar) Eigenvalues() []complex128 {
	return nil
}

// Hanowa is a non-symmetric non-singular matrix of even order given by
//  A[i,j] = alpha    if i == j,
//         = -i-1     if i < n/2 and j == i + n/2,
//         = i+1-n/2  if i >= n/2 and j == i - n/2,
//         = 0        otherwise.
// The matrix has complex eigenvalues.
type Hanowa struct {
	N     int // Order of the matrix, must be even.
	Alpha float64
}

func (h Hanowa) Matrix() blas64.General {
	if h.N&0x1 != 0 {
		panic("lapack: matrix order must be even")
	}
	n := h.N
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		a.Data[i*a.Stride+i] = h.Alpha
	}
	for i := 0; i < n/2; i++ {
		a.Data[i*a.Stride+i+n/2] = float64(-i - 1)
	}
	for i := n / 2; i < n; i++ {
		a.Data[i*a.Stride+i-n/2] = float64(i + 1 - n/2)
	}
	return a
}

func (h Hanowa) Eigenvalues() []complex128 {
	if h.N&0x1 != 0 {
		panic("lapack: matrix order must be even")
	}
	n := int(h.N)
	ev := make([]complex128, n)
	for i := 0; i < n/2; i++ {
		ev[2*i] = complex(h.Alpha, float64(-i-1))
		ev[2*i+1] = complex(h.Alpha, float64(i+1))
	}
	return ev
}

// Lesp is a tridiagonal, generally non-symmetric matrix given by
//  A[i,j] = -2*i-5   if i == j,
//         = 1/(i+1)  if i == j-1,
//         = j+1      if i == j+1.
// For example, for n=5,
//      [  -5    2    .    .    . ]
//      [ 1/2   -7    3    .    . ]
//  A = [   .  1/3   -9    4    . ]
//      [   .    .  1/4  -11    5 ]
//      [   .    .    .  1/5  -13 ].
// The matrix has sensitive eigenvalues but they are not given explicitly.
type Lesp int

func (l Lesp) Matrix() blas64.General {
	n := int(l)
	a := zeros(n, n, n)
	for i := 0; i < n; i++ {
		a.Data[i*a.Stride+i] = float64(-2*i - 5)
	}
	for i := 0; i < n-1; i++ {
		a.Data[i*a.Stride+i+1] = float64(i + 2)
	}
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = 1 / float64(i+1)
	}
	return a
}

func (Lesp) Eigenvalues() []complex128 {
	return nil
}

// Rutis is the 4×4 non-symmetric matrix
//      [ 4 -5  0  3 ]
//  A = [ 0  4 -3 -5 ]
//      [ 5 -3  4  0 ]
//      [ 3  0  5  4 ]
// It has two distinct real eigenvalues and a pair of complex eigenvalues.
type Rutis struct{}

func (Rutis) Matrix() blas64.General {
	return blas64.General{
		Rows:   4,
		Cols:   4,
		Stride: 4,
		Data: []float64{
			4, -5, 0, 3,
			0, 4, -3, -5,
			5, -3, 4, 0,
			3, 0, 5, 4,
		},
	}
}

func (Rutis) Eigenvalues() []complex128 {
	return []complex128{12, 1 + 5i, 1 - 5i, 2}
}

// Tris is a tridiagonal matrix given by
//  A[i,j] = x  if i == j-1,
//         = y  if i == j,
//         = z  if i == j+1.
// If x*z is negative, the matrix has complex eigenvalues.
type Tris struct {
	N       int
	X, Y, Z float64
}

func (t Tris) Matrix() blas64.General {
	n := t.N
	a := zeros(n, n, n)
	for i := 1; i < n; i++ {
		a.Data[i*a.Stride+i-1] = t.X
	}
	for i := 0; i < n; i++ {
		a.Data[i*a.Stride+i] = t.Y
	}
	for i := 0; i < n-1; i++ {
		a.Data[i*a.Stride+i+1] = t.Z
	}
	return a
}

func (t Tris) Eigenvalues() []complex128 {
	n := int(t.N)
	ev := make([]complex128, n)
	for i := range ev {
		angle := float64(i+1) * math.Pi / float64(n+1)
		arg := t.X * t.Z
		if arg >= 0 {
			ev[i] = complex(t.Y+2*math.Sqrt(arg)*math.Cos(angle), 0)
		} else {
			ev[i] = complex(t.Y, 2*math.Sqrt(-arg)*math.Cos(angle))
		}
	}
	return ev
}

// Wilk4 is a 4×4 lower triangular matrix with 4 distinct real eigenvalues.
type Wilk4 struct{}

func (Wilk4) Matrix() blas64.General {
	return blas64.General{
		Rows:   4,
		Cols:   4,
		Stride: 4,
		Data: []float64{
			0.9143e-4, 0.0, 0.0, 0.0,
			0.8762, 0.7156e-4, 0.0, 0.0,
			0.7943, 0.8143, 0.9504e-4, 0.0,
			0.8017, 0.6123, 0.7165, 0.7123e-4,
		},
	}
}

func (Wilk4) Eigenvalues() []complex128 {
	return []complex128{
		0.9504e-4, 0.9143e-4, 0.7156e-4, 0.7123e-4,
	}
}

// Wilk12 is a 12×12 lower Hessenberg matrix with 12 distinct real eigenvalues.
type Wilk12 struct{}

func (Wilk12) Matrix() blas64.General {
	return blas64.General{
		Rows:   12,
		Cols:   12,
		Stride: 12,
		Data: []float64{
			12, 11, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			11, 11, 10, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			10, 10, 10, 9, 0, 0, 0, 0, 0, 0, 0, 0,
			9, 9, 9, 9, 8, 0, 0, 0, 0, 0, 0, 0,
			8, 8, 8, 8, 8, 7, 0, 0, 0, 0, 0, 0,
			7, 7, 7, 7, 7, 7, 6, 0, 0, 0, 0, 0,
			6, 6, 6, 6, 6, 6, 6, 5, 0, 0, 0, 0,
			5, 5, 5, 5, 5, 5, 5, 5, 4, 0, 0, 0,
			4, 4, 4, 4, 4, 4, 4, 4, 4, 3, 0, 0,
			3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 2, 0,
			2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1,
			1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
		},
	}
}

func (Wilk12) Eigenvalues() []complex128 {
	return []complex128{
		32.2288915015722210,
		20.1989886458770691,
		12.3110774008685340,
		6.9615330855671154,
		3.5118559485807528,
		1.5539887091319704,
		0.6435053190136506,
		0.2847497205488856,
		0.1436465181918488,
		0.0812276683076552,
		0.0495074140194613,
		0.0310280683208907,
	}
}

// Wilk20 is a 20×20 lower Hessenberg matrix. If the parameter is 0, the matrix
// has 20 distinct real eigenvalues. If the parameter is 1e-10, the matrix has 6
// real eigenvalues and 7 pairs of complex eigenvalues.
type Wilk20 float64

func (w Wilk20) Matrix() blas64.General {
	a := zeros(20, 20, 20)
	for i := 0; i < 20; i++ {
		a.Data[i*a.Stride+i] = float64(i + 1)
	}
	for i := 0; i < 19; i++ {
		a.Data[i*a.Stride+i+1] = 20
	}
	a.Data[19*a.Stride] = float64(w)
	return a
}

func (w Wilk20) Eigenvalues() []complex128 {
	if float64(w) == 0 {
		ev := make([]complex128, 20)
		for i := range ev {
			ev[i] = complex(float64(i+1), 0)
		}
		return ev
	}
	return nil
}

// Zero is a matrix with all elements equal to zero.
type Zero int

func (z Zero) Matrix() blas64.General {
	n := int(z)
	return zeros(n, n, n)
}

func (z Zero) Eigenvalues() []complex128 {
	n := int(z)
	return make([]complex128, n)
}
