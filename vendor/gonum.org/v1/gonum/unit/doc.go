// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run internal/autogen.go

// Package unit provides a set of types and constants that facilitate
// the use of the International System of Units (SI).
//
// Unit provides two main functionalities.
//
// 1)
// It provides a number of types representing either an SI base unit
// or a common combination of base units, named for the unit it
// represents (Length, Mass, Pressure, etc.). Each type has
// a float64 as the underlying unit, and its value represents the
// number of that underlying unit (Kilogram, Meter, Pascal, etc.).
// For example,
//		height := 1.6 * unit.Meter
//		acc := unit.Acceleration(9.8)
// creates a variable named 'height' with a value of 1.6 meters, and
// a variable named 'acc' with a value of 9.8 meters per second squared.
// These types can be used to add compile-time safety to code. For
// example,
//		func UnitDensity(t unit.Temperature, pressure unit.Pressure) (unit.Density){
//			...
//		}
//		func main(){
//			t := 300 * unit.Kelvin
//			p := 5 * unit.Bar
//			rho := UnitDensity(p, t) // compile-time error
//		}
// gives a compile-time error (temperature type does not match pressure type)
// while the corresponding code using float64 runs without error.
//		func Float64Density(temperature, pressure float64) (float64){
//			...
//		}
//		func main(){
//			t := 300.0 // degrees kelvin
//			p := 50000.0 // Pascals
//			rho := Float64Density(p, t) // no error
//		}
// Many types have constants defined representing named SI units (Meter,
// Kilogram, etc. ) or SI derived units (Bar, Hz, etc.). The Unit package
// additionally provides untyped constants for SI prefixes, so the following
// are all equivalent.
//		l := 0.001 * unit.Meter
//		k := 1 * unit.Milli * unit.Meter
//		j := unit.Length(0.001)
//
// 2)
// Unit provides the type "Unit", meant to represent a general dimensional
// value. unit.Unit can be used to help prevent errors of dimensionality
// when multiplying or dividing dimensional numbers. This package also
// provides the "Uniter" interface which is satisfied by any type which can
// be converted to a unit. New varibles of type Unit can be created with
// the New function and the Dimensions map. For example, the code
//		acc := New(9.81, Dimensions{LengthDim:1, TimeDim: -2})
// creates a variable "acc" which has a value of 9.81 m/s^2. Methods of
// unit can be used to modify this value, for example:
// 		acc.Mul(1.0 * unit.Kilogram).Mul(1 * unit.Meter)
// To convert the unit back into a typed float64 value, the From methods
// of the dimensional types should be used. From will return an error if the
// dimensions do not match.
// 		var energy unit.Energy
//		err := (*energy).From(acc)
// Domain-specific problems may need custom dimensions, and for this purpose
// NewDimension should be used to help avoid accidental overlap between
// packages. For example, results from a blood test may be measured in
// "White blood cells per slide". In this case, NewDimension should be
// used to create a 'WhiteBloodCell' dimension. NewDimension takes in a
// string which will be used for printing that dimension, and will return
// a unique dimension number. NewDimension should not be
// used, however, to create the unit of 'Slide', because in this case slide
// is just a measurement of area. Instead, a constant could be defined.
//		const Slide unit.Area =  0.001875 // m^2
// Note that Unit cannot catch all errors related to dimensionality.
// Different physical ideas are sometimes expressed with the same dimensions
// and Unit is incapable of catching these mismatches. For example, energy and
// torque are both expressed as force times distance (Newton-meters in SI),
// but it is wrong to say that a torque of 10 N-m is the same as 10 J, even
// though the dimensions agree.
package unit // import "gonum.org/v1/gonum/unit"
