// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit

import (
	"bytes"
	"fmt"
	"sort"
	"sync"
	"unicode/utf8"
)

// Uniter is a type that can be converted to a Unit.
type Uniter interface {
	Unit() *Unit
}

// Dimension is a type representing an SI base dimension or a distinct
// orthogonal dimension. Non-SI dimensions can be created using the NewDimension
// function, typically within an init function.
type Dimension int

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
	defer mu.Unlock()
	mu.Lock()
	_, ok := dimensions[symbol]
	if ok {
		panic("unit: dimension string \"" + symbol + "\" already used")
	}
	d := Dimension(len(symbols))
	symbols = append(symbols, symbol)
	dimensions[symbol] = d
	return d
}

// String returns the string for the dimension.
func (d Dimension) String() string {
	if d == reserved {
		return "reserved"
	}
	defer mu.RUnlock()
	mu.RLock()
	if int(d) < len(symbols) {
		return symbols[d]
	}
	panic("unit: illegal dimension")
}

// SymbolExists returns whether the given symbol is already in use.
func SymbolExists(symbol string) bool {
	mu.RLock()
	_, ok := dimensions[symbol]
	mu.RUnlock()
	return ok
}

const (
	// SI Base Units
	reserved Dimension = iota
	CurrentDim
	LengthDim
	LuminousIntensityDim
	MassDim
	MoleDim
	TemperatureDim
	TimeDim
	// Other common SI Dimensions
	AngleDim // e.g. radians
)

var (
	// mu protects symbols and dimensions for concurrent use.
	mu      sync.RWMutex
	symbols = []string{
		CurrentDim:           "A",
		LengthDim:            "m",
		LuminousIntensityDim: "cd",
		MassDim:              "kg",
		MoleDim:              "mol",
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
		"mol": MoleDim,
		"K":   TemperatureDim,
		"s":   TimeDim,
		"rad": AngleDim,

		// Reserve common SI symbols
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

func (d Dimensions) clone() Dimensions {
	if d == nil {
		return nil
	}
	c := make(Dimensions, len(d))
	for dim, pow := range d {
		if pow != 0 {
			c[dim] = pow
		}
	}
	return c
}

// matches reports whether the dimensions of d and o match. Zero power
// dimensions in d an o must be removed, otherwise matches may incorrectly
// report a mismatch.
func (d Dimensions) matches(o Dimensions) bool {
	if len(d) != len(o) {
		return false
	}
	for dim, pow := range d {
		if o[dim] != pow {
			return false
		}
	}
	return true
}

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

// Unit represents a dimensional value. The dimensions will typically be in SI
// units, but can also include dimensions created with NewDimension. The Unit type
// is most useful for ensuring dimensional consistency when manipulating types
// with different units, for example, by multiplying  an acceleration with a
// mass to get a force. See the package documentation for further explanation.
type Unit struct {
	dimensions Dimensions
	value      float64
}

// New creates a new variable of type Unit which has the value and dimensions
// specified by the inputs. The built-in dimensions are always in SI units
// (metres, kilograms, etc.).
func New(value float64, d Dimensions) *Unit {
	return &Unit{
		dimensions: d.clone(),
		value:      value,
	}
}

// DimensionsMatch checks if the dimensions of two Uniters are the same.
func DimensionsMatch(a, b Uniter) bool {
	return a.Unit().dimensions.matches(b.Unit().dimensions)
}

// Dimensions returns a copy of the dimensions of the unit.
func (u *Unit) Dimensions() Dimensions {
	return u.dimensions.clone()
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
	return u
}

// Value return the raw value of the unit as a float64. Use of this
// method is, in general, not recommended, though it can be useful
// for printing. Instead, the From method of a specific dimension
// should be used to guarantee dimension consistency.
func (u *Unit) Value() float64 {
	return u.value
}

// SetValue sets the value of the unit.
func (u *Unit) SetValue(v float64) {
	u.value = v
}

// Format makes Unit satisfy the fmt.Formatter interface. The unit is formatted
// with dimensions appended. If the power of the dimension is not zero or one,
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
		units := u.dimensions.String()
		switch {
		case pOk && wOk:
			fmt.Fprintf(fs, "%*.*"+string(c), pos(w-utf8.RuneCount([]byte(units))-1), p, u.value)
		case pOk:
			fmt.Fprintf(fs, "%.*"+string(c), p, u.value)
		case wOk:
			fmt.Fprintf(fs, "%*"+string(c), pos(w-utf8.RuneCount([]byte(units))-1), u.value)
		default:
			fmt.Fprintf(fs, "%"+string(c), u.value)
		}
		fmt.Fprintf(fs, " %s", units)
	default:
		fmt.Fprintf(fs, "%%!%c(*Unit=%g)", c, u)
	}
}

func pos(a int) int {
	if a < 0 {
		return 0
	}
	return a
}
