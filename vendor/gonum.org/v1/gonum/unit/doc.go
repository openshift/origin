// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run generate_unit.go

// Package unit provides a set of types and constants that facilitate
// the use of the International System of Units (SI).
//
// The unit package provides two main functionalities: compile-time type-safe
// base SI units and common derived units; and a system for dynamically
// extensible user-defined units.
//
// Static SI units
//
// This package provides a number of types representing either an SI base
// unit or a common combination of base units, named for the physical quantity
// it represents (Length, Mass, Pressure, etc.). Each type is defined from
// float64. The value of the float64 represents the quantity of that unit as
// expressed in SI base units (kilogram, metre, Pascal, etc.). For example,
//
// 	height := 1.6 * unit.Metre
// 	acc := unit.Acceleration(9.8)
//
// creates a variable named 'height' with a value of 1.6 metres, and
// a variable named 'acc' with a value of 9.8 metres per second squared.
// These types can be used to add compile-time safety to code. For
// example,
//
// 	func unitVolume(t unit.Temperature, p unit.Pressure) unit.Volume {
// 		...
// 	}
//
// 	func main(){
// 		t := 300 * unit.Kelvin
// 		p := 500 * unit.Kilo * unit.Pascal
// 		v := unitVolume(p, t) // compile-time error
// 	}
//
// gives a compile-time error (temperature type does not match pressure type)
// while the corresponding code using float64 runs without error.
//
// 	func float64Volume(temperature, pressure float64) float64 {
// 		...
// 	}
//
// 	func main(){
// 		t := 300.0 // Kelvin
// 		p := 500000.0 // Pascals
// 		v := float64Volume(p, t) // no error
// 	}
//
// Many types have constants defined representing named SI units (Metre,
// Kilogram, etc. ) or SI derived units (Pascal, Hz, etc.). The unit package
// additionally provides untyped constants for SI prefixes, so the following
// are all equivalent.
//
// 	l := 0.001 * unit.Metre
// 	k := 1 * unit.Milli * unit.Metre
// 	j := unit.Length(0.001)
//
// Additional SI-derived static units can also be defined by adding types that
// satisfy the Uniter interface described below.
//
// Dynamic user-extensible unit system
//
// The unit package also provides the Unit type, a representation of a general
// dimensional value. Unit can be used to help prevent errors of dimensionality
// when multiplying or dividing dimensional numbers defined a run time. New
// variables of type Unit can be created with the New function and the
// Dimensions map. For example, the code
//
// 	rate := unit.New(1 * unit.Milli, Dimensions{MoleDim: 1, TimeDim: -1})
//
// creates a variable "rate" which has a value of 1e-3 mol/s. Methods of
// unit can be used to modify this value, for example:
//
// 	rate.Mul(1 * unit.Centi * unit.Metre).Div(1 * unit.Milli * unit.Volt)
//
// To convert the unit back into a typed float64 value, the From methods
// of the dimensional types should be used. From will return an error if the
// dimensions do not match.
//
// 	var energy unit.Energy
// 	err := energy.From(acc)
//
// Domain-specific problems may need custom dimensions, and for this purpose
// NewDimension should be used to help avoid accidental overlap between
// packages. For example, results from a blood test may be measured in
// "White blood cells per slide". In this case, NewDimension should be
// used to create a 'WhiteBloodCell' dimension. NewDimension takes in a
// string which will be used for printing that dimension, and will return
// a unique dimension number.
//
// 	wbc := unit.NewDimension("WhiteBloodCell")
//
// NewDimension should not be used, however, to create the unit of 'Slide',
// because in this case slide is just a measurement of liquid volume. Instead,
// a constant could be defined.
//
// 	const Slide unit.Volume =  0.1 * unit.Micro * unit.Litre
//
// Note that unit cannot catch all errors related to dimensionality.
// Different physical ideas are sometimes expressed with the same dimensions
// and unit is incapable of catching these mismatches. For example, energy and
// torque are both expressed as force times distance (Newton-metres in SI),
// but it is wrong to say that a torque of 10 N·m is the same as 10 J, even
// though the dimensions agree. Despite this, using the defined types to
// represent units can help to catch errors at compile-time. For example,
// using unit.Torque allows you to define a statically typed function like so
//
// 	func LeverLength(apply unit.Force, want unit.Torque) unit.Length {
//		return unit.Length(float64(want)/float64(apply))
// 	}
//
// This will prevent an energy value being provided to LeverLength in place
// of a torque value.
package unit // import "gonum.org/v1/gonum/unit"
