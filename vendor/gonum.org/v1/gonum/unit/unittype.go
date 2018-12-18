// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit

import (
	"bytes"
	"fmt"
	"sort"
)

// Uniter is a type that can be converted to a Unit.
type Uniter interface {
	Unit() *Unit
}

// Dimension is a type representing an SI base dimension or a distinct
// orthogonal dimension. Non-SI dimensions can be created using the NewDimension
// function, typically within an init function.
type Dimension int

// String returns the string for the dimension.
func (d Dimension) String() string {
	switch {
	case d == reserved:
		return "reserved"
	case d < Dimension(len(symbols)):
		return symbols[d]
	default:
		panic("unit: illegal dimension")
	}
}

const (
	// SI Base Units
	reserved Dimension = iota
	CurrentDim
	LengthDim
	LuminousIntensityDim
	MassDim
	TemperatureDim
	TimeDim
	// Other common SI Dimensions
	AngleDim // e.g. radians
)

var (
	symbols = []string{
		CurrentDim:           "A",
		LengthDim:            "m",
		LuminousIntensityDim: "cd",
		MassDim:              "kg",
		TemperatureDim:       "K",
		TimeDim:              "s",
		AngleDim:             "rad",
	}

	// dimensions guarantees there aren't two identical symbols
	// SI symbol list from http://lamar.colostate.edu/~hillger/basic.htm
	dimensions = map[string]Dimension{
		"A":   CurrentDim,
		"m":   LengthDim,
		"cd":  LuminousIntensityDim,
		"kg":  MassDim,
		"K":   TemperatureDim,
		"s":   TimeDim,
		"rad": AngleDim,

		// Reserve common SI symbols
		// base units
		"mol": reserved,
		// prefixes
		"Y":  reserved,
		"Z":  reserved,
		"E":  reserved,
		"P":  reserved,
		"T":  reserved,
		"G":  reserved,
		"M":  reserved,
		"k":  reserved,
		"h":  reserved,
		"da": reserved,
		"d":  reserved,
		"c":  reserved,
		"μ":  reserved,
		"n":  reserved,
		"p":  reserved,
		"f":  reserved,
		"a":  reserved,
		"z":  reserved,
		"y":  reserved,
		// SI Derived units with special symbols
		"sr":  reserved,
		"F":   reserved,
		"C":   reserved,
		"S":   reserved,
		"H":   reserved,
		"V":   reserved,
		"Ω":   reserved,
		"J":   reserved,
		"N":   reserved,
		"Hz":  reserved,
		"lx":  reserved,
		"lm":  reserved,
		"Wb":  reserved,
		"W":   reserved,
		"Pa":  reserved,
		"Bq":  reserved,
		"Gy":  reserved,
		"Sv":  reserved,
		"kat": reserved,
		// Units in use with SI
		"ha": reserved,
		"L":  reserved,
		"l":  reserved,
		// Units in Use Temporarily with SI
		"bar": reserved,
		"b":   reserved,
		"Ci":  reserved,
		"R":   reserved,
		"rd":  reserved,
		"rem": reserved,
	}
)

// Dimensions represent the dimensionality of the unit in powers
// of that dimension. If a key is not present, the power of that
// dimension is zero. Dimensions is used in conjunction with New.
type Dimensions map[Dimension]int

func (d Dimensions) String() string {
	// Map iterates randomly, but print should be in a fixed order. Can't use
	// dimension number, because for user-defined dimension that number may
	// not be fixed from run to run.
	atoms := make(unitPrinters, 0, len(d))
	for dimension, power := range d {
		if power != 0 {
			atoms = append(atoms, atom{dimension, power})
		}
	}
	sort.Sort(atoms)
	var b bytes.Buffer
	for i, a := range atoms {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "%s", a.Dimension)
		if a.pow != 1 {
			fmt.Fprintf(&b, "^%d", a.pow)
		}
	}

	return b.String()
}

type atom struct {
	Dimension
	pow int
}

type unitPrinters []atom

func (u unitPrinters) Len() int {
	return len(u)
}

func (u unitPrinters) Less(i, j int) bool {
	// Order first by positive powers, then by name.
	if u[i].pow*u[j].pow < 0 {
		return u[i].pow > 0
	}
	return u[i].String() < u[j].String()
}

func (u unitPrinters) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

// NewDimension creates a new orthogonal dimension with the given symbol, and
// returns the value of that dimension. The input symbol must not overlap with
// any of the any of the SI base units or other symbols of common use in SI ("kg",
// "J", etc.), and must not overlap with any other dimensions created by calls
// to NewDimension. The SymbolExists function can check if the symbol exists.
// NewDimension will panic if the input symbol matches an existing symbol.
//
// NewDimension should only be called for unit types that are actually orthogonal
// to the base dimensions defined in this package. See the package-level
// documentation for further explanation.
func NewDimension(symbol string) Dimension {
	_, ok := dimensions[symbol]
	if ok {
		panic("unit: dimension string \"" + symbol + "\" already used")
	}
	d := Dimension(len(symbols))
	symbols = append(symbols, symbol)
	dimensions[symbol] = d
	return d
}

// SymoblExists returns whether the given symbol is already in use.
func SymbolExists(symbol string) bool {
	_, ok := dimensions[symbol]
	return ok
}

// Unit represents a dimensional value. The dimensions will typically be in SI
// units, but can also include dimensions created with NewDimension. The Unit type
// is most useful for ensuring dimensional consistency when manipulating types
// with different units, for example, by multiplying  an acceleration with a
// mass to get a force. See the package documentation for further explanation.
type Unit struct {
	dimensions Dimensions
	formatted  string
	value      float64
}

// New creates a new variable of type Unit which has the value and dimensions
// specified by the inputs. The built-in dimensions are always in SI units
// (meters, kilograms, etc.).
func New(value float64, d Dimensions) *Unit {
	u := &Unit{
		dimensions: make(map[Dimension]int),
		value:      value,
	}
	for key, val := range d {
		if val != 0 {
			u.dimensions[key] = val
		}
	}
	return u
}

// DimensionsMatch checks if the dimensions of two Uniters are the same.
func DimensionsMatch(a, b Uniter) bool {
	aUnit := a.Unit()
	bUnit := b.Unit()
	if len(aUnit.dimensions) != len(bUnit.dimensions) {
		return false
	}
	for key, val := range aUnit.dimensions {
		if bUnit.dimensions[key] != val {
			return false
		}
	}
	return true
}

// Add adds the function argument to the receiver. Panics if the units of
// the receiver and the argument don't match.
func (u *Unit) Add(uniter Uniter) *Unit {
	a := uniter.Unit()
	if !DimensionsMatch(u, a) {
		panic("unit: mismatched dimensions in addition")
	}
	u.value += a.value
	return u
}

// Unit implements the Uniter interface
func (u *Unit) Unit() *Unit {
	return u
}

// Mul multiply the receiver by the input changing the dimensions
// of the receiver as appropriate. The input is not changed.
func (u *Unit) Mul(uniter Uniter) *Unit {
	a := uniter.Unit()
	for key, val := range a.dimensions {
		if d := u.dimensions[key]; d == -val {
			delete(u.dimensions, key)
		} else {
			u.dimensions[key] = d + val
		}
	}
	u.formatted = ""
	u.value *= a.value
	return u
}

// Div divides the receiver by the argument changing the
// dimensions of the receiver as appropriate.
func (u *Unit) Div(uniter Uniter) *Unit {
	a := uniter.Unit()
	u.value /= a.value
	for key, val := range a.dimensions {
		if d := u.dimensions[key]; d == val {
			delete(u.dimensions, key)
		} else {
			u.dimensions[key] = d - val
		}
	}
	u.formatted = ""
	return u
}

// Value return the raw value of the unit as a float64. Use of this
// method is, in general, not recommended, though it can be useful
// for printing. Instead, the From type of a specific dimension
// should be used to guarantee dimension consistency.
func (u *Unit) Value() float64 {
	return u.value
}

// Format makes Unit satisfy the fmt.Formatter interface. The unit is formatted
// with dimensions appended. If the power if the dimension is not zero or one,
// symbol^power is appended, if the power is one, just the symbol is appended
// and if the power is zero, nothing is appended. Dimensions are appended
// in order by symbol name with positive powers ahead of negative powers.
func (u *Unit) Format(fs fmt.State, c rune) {
	if u == nil {
		fmt.Fprint(fs, "<nil>")
	}
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "&%#v", *u)
			return
		}
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		p, pOk := fs.Precision()
		w, wOk := fs.Width()
		switch {
		case pOk && wOk:
			fmt.Fprintf(fs, "%*.*"+string(c), w, p, u.value)
		case pOk:
			fmt.Fprintf(fs, "%.*"+string(c), p, u.value)
		case wOk:
			fmt.Fprintf(fs, "%*"+string(c), w, u.value)
		default:
			fmt.Fprintf(fs, "%"+string(c), u.value)
		}
	default:
		fmt.Fprintf(fs, "%%!%c(*Unit=%g)", c, u)
		return
	}
	if u.formatted == "" && len(u.dimensions) > 0 {
		u.formatted = u.dimensions.String()
	}
	fmt.Fprintf(fs, " %s", u.formatted)
}
