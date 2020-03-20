// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"go/format"
	"log"
	"os"
	"strings"
	"text/template"

	"gonum.org/v1/gonum/unit"
)

type Unit struct {
	DimensionName string
	Receiver      string
	PowerOffset   int    // from normal (for example, mass base unit is kg, not g)
	PrintString   string // print string for the unit (kg for mass)
	ExtraConstant []Constant
	Name          string
	TypeComment   string // text to comment the type
	Dimensions    []Dimension
	ErForm        string // for Xxxer interface
}

type Dimension struct {
	Name  string
	Power int
}

func (u Unit) Units() string {
	dims := make(unit.Dimensions)
	for _, d := range u.Dimensions {
		dims[dimOf[d.Name]] = d.Power
	}
	return dims.String()
}

const (
	AngleName             string = "AngleDim"
	CurrentName           string = "CurrentDim"
	LengthName            string = "LengthDim"
	LuminousIntensityName string = "LuminousIntensityDim"
	MassName              string = "MassDim"
	MoleName              string = "MoleDim"
	TemperatureName       string = "TemperatureDim"
	TimeName              string = "TimeDim"
)

var dimOf = map[string]unit.Dimension{
	"AngleDim":             unit.AngleDim,
	"CurrentDim":           unit.CurrentDim,
	"LengthDim":            unit.LengthDim,
	"LuminousIntensityDim": unit.LuminousIntensityDim,
	"MassDim":              unit.MassDim,
	"MoleDim":              unit.MoleDim,
	"TemperatureDim":       unit.TemperatureDim,
	"TimeDim":              unit.TimeDim,
}

type Constant struct {
	Name  string
	Value string
}

type Prefix struct {
	Name  string
	Power int
}

var Units = []Unit{
	// Base units.
	{
		DimensionName: "Angle",
		Receiver:      "a",
		PrintString:   "rad",
		Name:          "Rad",
		TypeComment:   "Angle represents an angle in radians",
		Dimensions: []Dimension{
			{Name: AngleName, Power: 1},
		},
	},
	{
		DimensionName: "Current",
		Receiver:      "i",
		PrintString:   "A",
		Name:          "Ampere",
		TypeComment:   "Current represents a current in Amperes",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 1},
		},
	},
	{
		DimensionName: "Length",
		Receiver:      "l",
		PrintString:   "m",
		Name:          "Metre",
		TypeComment:   "Length represents a length in metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
		},
	},
	{
		DimensionName: "LuminousIntensity",
		Receiver:      "j",
		PrintString:   "cd",
		Name:          "Candela",
		TypeComment:   "Candela represents a luminous intensity in candela",
		Dimensions: []Dimension{
			{Name: LuminousIntensityName, Power: 1},
		},
	},
	{
		DimensionName: "Mass",
		Receiver:      "m",
		PowerOffset:   -3,
		PrintString:   "kg",
		Name:          "Gram",
		TypeComment:   "Mass represents a mass in kilograms",
		ExtraConstant: []Constant{
			{Name: "Kilogram", Value: "Kilo * Gram"},
		},
		Dimensions: []Dimension{
			{Name: MassName, Power: 1},
		},
	},
	{
		DimensionName: "Mole",
		Receiver:      "n",
		PrintString:   "mol",
		Name:          "Mol",
		TypeComment:   "Mole represents an amount in moles",
		Dimensions: []Dimension{
			{Name: MoleName, Power: 1},
		},
	},
	{
		DimensionName: "Temperature",
		Receiver:      "t",
		PrintString:   "K",
		Name:          "Kelvin",
		TypeComment:   "Temperature represents a temperature in Kelvin",
		Dimensions: []Dimension{
			{Name: TemperatureName, Power: 1},
		},
		ErForm: "Temperaturer",
	},
	{
		DimensionName: "Time",
		Receiver:      "t",
		PrintString:   "s",
		Name:          "Second",
		TypeComment:   "Time represents a duration in seconds",
		ExtraConstant: []Constant{
			{Name: "Minute", Value: "60 * Second"},
			{Name: "Hour", Value: "60 * Minute"},
		},
		Dimensions: []Dimension{
			{Name: TimeName, Power: 1},
		},
		ErForm: "Timer",
	},

	// Derived units.
	{
		DimensionName: "AbsorbedRadioactiveDose",
		Receiver:      "a",
		PrintString:   "Gy",
		Name:          "Gray",
		TypeComment:   "AbsorbedRadioactiveDose is a measure of absorbed dose of ionizing radiation in grays",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "Acceleration",
		Receiver:      "a",
		PrintString:   "m s^-2",
		TypeComment:   "Acceleration represents an acceleration in metres per second squared",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "Area",
		Receiver:      "a",
		PrintString:   "m^2",
		TypeComment:   "Area represents an area in square metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
		},
	},
	{
		DimensionName: "Radioactivity",
		Receiver:      "r",
		PrintString:   "Bq",
		Name:          "Becquerel",
		TypeComment:   "Radioactivity represents a rate of radioactive decay in becquerels",
		Dimensions: []Dimension{
			{Name: TimeName, Power: -1},
		},
	},
	{
		DimensionName: "Capacitance",
		Receiver:      "cp",
		PrintString:   "F",
		Name:          "Farad",
		TypeComment:   "Capacitance represents an electrical capacitance in Farads",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 2},
			{Name: LengthName, Power: -2},
			{Name: MassName, Power: -1},
			{Name: TimeName, Power: 4},
		},
		ErForm: "Capacitancer",
	},
	{
		DimensionName: "Charge",
		Receiver:      "ch",
		PrintString:   "C",
		Name:          "Coulomb",
		TypeComment:   "Charge represents an electric charge in Coulombs",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 1},
			{Name: TimeName, Power: 1},
		},
		ErForm: "Charger",
	},
	{
		DimensionName: "Conductance",
		Receiver:      "co",
		PrintString:   "S",
		Name:          "Siemens",
		TypeComment:   "Conductance represents an electrical conductance in Siemens",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 2},
			{Name: LengthName, Power: -2},
			{Name: MassName, Power: -1},
			{Name: TimeName, Power: 3},
		},
		ErForm: "Conductancer",
	},
	{
		DimensionName: "EquivalentRadioactiveDose",
		Receiver:      "a",
		PrintString:   "Sy",
		Name:          "Sievert",
		TypeComment:   "EquivalentRadioactiveDose is a measure of equivalent dose of ionizing radiation in sieverts",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "Energy",
		Receiver:      "e",
		PrintString:   "J",
		Name:          "Joule",
		TypeComment:   "Energy represents a quantity of energy in Joules",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "Frequency",
		Receiver:      "f",
		PrintString:   "Hz",
		Name:          "Hertz",
		TypeComment:   "Frequency represents a frequency in Hertz",
		Dimensions: []Dimension{
			{Name: TimeName, Power: -1},
		},
	},
	{
		DimensionName: "Force",
		Receiver:      "f",
		PrintString:   "N",
		Name:          "Newton",
		TypeComment:   "Force represents a force in Newtons",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Forcer",
	},
	{
		DimensionName: "Inductance",
		Receiver:      "i",
		PrintString:   "H",
		Name:          "Henry",
		TypeComment:   "Inductance represents an electrical inductance in Henry",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -2},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Inductancer",
	},
	{
		DimensionName: "Power",
		Receiver:      "pw",
		PrintString:   "W",
		Name:          "Watt",
		TypeComment:   "Power represents a power in Watts",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
	},
	{
		DimensionName: "Resistance",
		Receiver:      "r",
		PrintString:   "Ω",
		Name:          "Ohm",
		TypeComment:   "Resistance represents an electrical resistance, impedance or reactance in Ohms",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -2},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
		ErForm: "Resistancer",
	},
	{
		DimensionName: "MagneticFlux",
		Receiver:      "m",
		PrintString:   "Wb",
		Name:          "Weber",
		TypeComment:   "MagneticFlux represents a magnetic flux in Weber",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "MagneticFluxDensity",
		Receiver:      "m",
		PrintString:   "T",
		Name:          "Tesla",
		TypeComment:   "MagneticFluxDensity represents a magnetic flux density in Tesla",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		DimensionName: "Pressure",
		Receiver:      "pr",
		PrintString:   "Pa",
		Name:          "Pascal",
		TypeComment:   "Pressure represents a pressure in Pascals",
		Dimensions: []Dimension{
			{Name: LengthName, Power: -1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Pressurer",
	},
	{
		DimensionName: "Torque",
		Receiver:      "t",
		PrintString:   "N m",
		Name:          "Newtonmetre",
		TypeComment:   "Torque represents a torque in Newton metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Torquer",
	},
	{
		DimensionName: "Velocity",
		Receiver:      "v",
		PrintString:   "m s^-1",
		TypeComment:   "Velocity represents a velocity in metres per second",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: TimeName, Power: -1},
		},
	},
	{
		DimensionName: "Voltage",
		Receiver:      "v",
		PrintString:   "V",
		Name:          "Volt",
		TypeComment:   "Voltage represents a voltage in Volts",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
		ErForm: "Voltager",
	},
	{
		DimensionName: "Volume",
		Receiver:      "v",
		PowerOffset:   -3,
		PrintString:   "m^3",
		Name:          "Litre",
		TypeComment:   "Volume represents a volume in cubic metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 3},
		},
	},
}

// Generate generates a file for each of the units
func main() {
	for _, unit := range Units {
		generate(unit)
		generateTest(unit)
	}
}

const headerTemplate = `// Code generated by "go generate gonum.org/v1/gonum/unit”; DO NOT EDIT.

// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

// {{.TypeComment}}.
type {{.DimensionName}} float64
`

var header = template.Must(template.New("header").Parse(headerTemplate))

const constTemplate = `
const {{if .ExtraConstant}}({{end}}
	{{.Name}} {{.DimensionName}} = {{if .PowerOffset}} 1e{{.PowerOffset}} {{else}} 1 {{end}}
	{{$name := .Name}}
	{{range .ExtraConstant}} {{.Name}} = {{.Value}}
	{{end}}
{{if .ExtraConstant}}){{end}}
`

var prefix = template.Must(template.New("prefix").Parse(constTemplate))

const methodTemplate = `
// Unit converts the {{.DimensionName}} to a *Unit
func ({{.Receiver}} {{.DimensionName}}) Unit() *Unit {
	return New(float64({{.Receiver}}), Dimensions{
		{{range .Dimensions}} {{.Name}}: {{.Power}},
		{{end}}
		})
}

// {{.DimensionName}} allows {{.DimensionName}} to implement a {{if .ErForm}}{{.ErForm}}{{else}}{{.DimensionName}}er{{end}} interface
func ({{.Receiver}} {{.DimensionName}}) {{.DimensionName}}() {{.DimensionName}} {
	return {{.Receiver}}
}

// From converts the unit into the receiver. From returns an
// error if there is a mismatch in dimension
func ({{.Receiver}} *{{.DimensionName}}) From(u Uniter) error {
	if !DimensionsMatch(u, {{if .Name}}{{.Name}}{{else}}{{.DimensionName}}(0){{end}}){
		*{{.Receiver}} = {{.DimensionName}}(math.NaN())
		return errors.New("Dimension mismatch")
	}
	*{{.Receiver}} = {{.DimensionName}}(u.Unit().Value())
	return nil
}
`

var methods = template.Must(template.New("methods").Parse(methodTemplate))

const formatTemplate = `
func ({{.Receiver}} {{.DimensionName}}) Format(fs fmt.State, c rune) {
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "%T(%v)", {{.Receiver}}, float64({{.Receiver}}))
			return
		}
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		p, pOk := fs.Precision()
		w, wOk := fs.Width()
		const unit = " {{.PrintString}}"
		switch {
		case pOk && wOk:
			fmt.Fprintf(fs, "%*.*"+string(c), pos(w-utf8.RuneCount([]byte(unit))), p, float64({{.Receiver}}))
		case pOk:
			fmt.Fprintf(fs, "%.*"+string(c), p, float64({{.Receiver}}))
		case wOk:
			fmt.Fprintf(fs, "%*"+string(c), pos(w-utf8.RuneCount([]byte(unit))), float64({{.Receiver}}))
		default:
			fmt.Fprintf(fs, "%"+string(c), float64({{.Receiver}}))
		}
		fmt.Fprint(fs, unit)
	default:
		fmt.Fprintf(fs, "%%!%c(%T=%g {{.PrintString}})", c, {{.Receiver}}, float64({{.Receiver}}))
	}
}
`

var form = template.Must(template.New("format").Parse(formatTemplate))

func generate(unit Unit) {
	lowerName := strings.ToLower(unit.DimensionName)
	filename := lowerName + ".go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var buf bytes.Buffer

	err = header.Execute(&buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	if unit.Name != "" {
		err = prefix.Execute(&buf, unit)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = methods.Execute(&buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	err = form.Execute(&buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format
		log.Fatalf("error formatting %q: %s", unit.DimensionName, err)
	}

	f.Write(b)
}

const testTemplate = `// Code generated by "go generate gonum.org/v1/gonum/unit; DO NOT EDIT.

// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unit

import (
	"fmt"
	"testing"
)

func Test{{.DimensionName}}Format(t *testing.T) {
	for _, test := range []struct{
		value  {{.DimensionName}}
		format string
		want   string
	}{
		{1.23456789, "%v", "1.23456789 {{.PrintString}}"},
		{1.23456789, "%.1v", "1 {{.PrintString}}"},
		{1.23456789, "%20.1v", "{{$s := printf "1 %s" .PrintString}}{{printf "%20s" $s}}"},
		{1.23456789, "%20v", "{{$s := printf "1.23456789 %s" .PrintString}}{{printf "%20s" $s}}"},
		{1.23456789, "%1v", "1.23456789 {{.PrintString}}"},
		{1.23456789, "%#v", "unit.{{.DimensionName}}(1.23456789)"},
		{1.23456789, "%s", "%!s(unit.{{.DimensionName}}=1.23456789 {{.PrintString}})"},
	} {
		got := fmt.Sprintf(test.format, test.value)
		if got != test.want {
			t.Errorf("Format %q %v: got: %q want: %q", test.format, float64(test.value), got, test.want)
		}
	}
}
`

var tests = template.Must(template.New("test").Parse(testTemplate))

func generateTest(unit Unit) {
	lowerName := strings.ToLower(unit.DimensionName)
	filename := lowerName + "_test.go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var buf bytes.Buffer
	err = tests.Execute(&buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format.
		log.Fatalf("error formatting test for %q: %s", unit.DimensionName, err)
	}

	f.Write(b)
}
