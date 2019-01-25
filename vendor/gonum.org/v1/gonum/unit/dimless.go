// Copyright ©2013 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit

import (
	"errors"
	"fmt"
	"math"
)

// Dimless represents a dimensionless constant
type Dimless float64

const (
	One Dimless = 1.0
)

// Unit converts the Dimless to a unit
func (d Dimless) Unit() *Unit {
	return New(float64(d), Dimensions{})
}

// Dimless allows Dimless to implement a Dimlesser interface
func (d Dimless) Dimless() Dimless {
	return d
}

// From converts the unit to a dimless. Returns an error if there
// is a mismatch in dimension
func (d *Dimless) From(u *Unit) error {
	if !DimensionsMatch(u, One) {
		(*d) = Dimless(math.NaN())
		return errors.New("Dimension mismatch")
	}
	(*d) = Dimless(u.Unit().Value())
	return nil
}

func (d Dimless) Format(fs fmt.State, c rune) {
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "%T(%v)", d, float64(d))
			return
		}
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		p, pOk := fs.Precision()
		w, wOk := fs.Width()
		switch {
		case pOk && wOk:
			fmt.Fprintf(fs, "%*.*"+string(c), w, p, float64(d))
		case pOk:
			fmt.Fprintf(fs, "%.*"+string(c), p, float64(d))
		case wOk:
			fmt.Fprintf(fs, "%*"+string(c), w, float64(d))
		default:
			fmt.Fprintf(fs, "%"+string(c), float64(d))
		}
	default:
		fmt.Fprintf(fs, "%%!%c(%T=%g)", c, d, float64(d))
		return
	}
}
