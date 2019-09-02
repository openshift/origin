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
	Name          string
	Receiver      string
	Offset        int    // From normal (for example, mass base unit is kg, not kg)
	PrintString   string // print string for the unit (kg for mass)
	ExtraConstant []Constant
	Suffix        string
	Singular      string
	TypeComment   string // Text to comment the type
	Dimensions    []Dimension
	ErForm        string //For Xxxer interface
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

var Prefixes = []Prefix{
	{
		Name:  "Yotta",
		Power: 24,
	},
	{
		Name:  "Zetta",
		Power: 21,
	},
	{
		Name:  "Exa",
		Power: 18,
	},
	{
		Name:  "Peta",
		Power: 15,
	},
	{
		Name:  "Tera",
		Power: 12,
	},
	{
		Name:  "Giga",
		Power: 9,
	},
	{
		Name:  "Mega",
		Power: 6,
	},
	{
		Name:  "Kilo",
		Power: 3,
	},
	{
		Name:  "Hecto",
		Power: 2,
	},
	{
		Name:  "Deca",
		Power: 1,
	},
	{
		Name:  "",
		Power: 0,
	},
	{
		Name:  "Deci",
		Power: -1,
	},
	{
		Name:  "Centi",
		Power: -2,
	},
	{
		Name:  "Milli",
		Power: -3,
	},
	{
		Name:  "Micro",
		Power: -6,
	},
	{
		Name:  "Nano",
		Power: -9,
	},
	{
		Name:  "Pico",
		Power: -12,
	},
	{
		Name:  "Femto",
		Power: -15,
	},
	{
		Name:  "Atto",
		Power: -18,
	},
	{
		Name:  "Zepto",
		Power: -21,
	},
	{
		Name:  "Yocto",
		Power: -24,
	},
}

var Units = []Unit{
	// Base units.
	{
		Name:        "Angle",
		Receiver:    "a",
		PrintString: "rad",
		Suffix:      "rad",
		Singular:    "Rad",
		TypeComment: "Angle represents an angle in radians",
		Dimensions: []Dimension{
			{Name: AngleName, Power: 1},
		},
	},
	{
		Name:        "Current",
		Receiver:    "i",
		PrintString: "A",
		Suffix:      "ampere",
		Singular:    "Ampere",
		TypeComment: "Current represents a current in Amperes",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 1},
		},
	},
	{
		Name:        "Length",
		Receiver:    "l",
		PrintString: "m",
		Suffix:      "metre",
		Singular:    "Metre",
		TypeComment: "Length represents a length in metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
		},
	},
	{
		Name:        "LuminousIntensity",
		Receiver:    "j",
		PrintString: "cd",
		Suffix:      "candela",
		Singular:    "Candela",
		TypeComment: "Candela represents a luminous intensity in candela",
		Dimensions: []Dimension{
			{Name: LuminousIntensityName, Power: 1},
		},
	},
	{
		Name:        "Mass",
		Receiver:    "m",
		Offset:      -3,
		PrintString: "kg",
		Suffix:      "gram",
		Singular:    "Gram",
		TypeComment: "Mass represents a mass in kilograms",
		Dimensions: []Dimension{
			{Name: MassName, Power: 1},
		},
	},
	{
		Name:        "Mole",
		Receiver:    "n",
		PrintString: "mol",
		Suffix:      "mol",
		Singular:    "mol",
		TypeComment: "Mole represents an amount in moles",
		Dimensions: []Dimension{
			{Name: MoleName, Power: 1},
		},
	},
	{
		Name:        "Temperature",
		Receiver:    "t",
		PrintString: "K",
		Suffix:      "kelvin",
		Singular:    "Kelvin",
		TypeComment: "Temperature represents a temperature in Kelvin",
		Dimensions: []Dimension{
			{Name: TemperatureName, Power: 1},
		},
		ErForm: "Temperaturer",
	},
	{
		Name:        "Time",
		Receiver:    "t",
		PrintString: "s",
		Suffix:      "second",
		Singular:    "Second",
		TypeComment: "Time represents a time in seconds",
		ExtraConstant: []Constant{
			{Name: "Hour", Value: "3600"},
			{Name: "Minute", Value: "60"},
		},
		Dimensions: []Dimension{
			{Name: TimeName, Power: 1},
		},
		ErForm: "Timer",
	},

	// Derived units.
	{
		Name:        "AbsorbedRadioactiveDose",
		Receiver:    "a",
		PrintString: "Gy",
		Suffix:      "gray",
		Singular:    "Gray",
		TypeComment: "AbsorbedRadioactiveDose is a measure of absorbed dose of ionizing radiation in grays",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "Acceleration",
		Receiver:    "a",
		PrintString: "m s^-2",
		TypeComment: "Acceleration represents an acceleration in metres per second squared",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "Area",
		Receiver:    "a",
		PrintString: "m^2",
		TypeComment: "Area represents and area in square metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
		},
	},
	{
		Name:        "Radioactivity",
		Receiver:    "r",
		PrintString: "Bq",
		Suffix:      "becquerel",
		Singular:    "Becquerel",
		TypeComment: "Radioactivity represents a rate of radioactive decay in becquerels",
		Dimensions: []Dimension{
			{Name: TimeName, Power: -1},
		},
	},
	{
		Name:        "Capacitance",
		Receiver:    "cp",
		PrintString: "F",
		Suffix:      "farad",
		Singular:    "Farad",
		TypeComment: "Capacitance represents an electrical capacitance in Farads",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 2},
			{Name: LengthName, Power: -2},
			{Name: MassName, Power: -1},
			{Name: TimeName, Power: 4},
		},
		ErForm: "Capacitancer",
	},
	{
		Name:        "Charge",
		Receiver:    "ch",
		PrintString: "C",
		Suffix:      "coulomb",
		Singular:    "coulomb",
		TypeComment: "Charge represents an electric charge in Coulombs",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 1},
			{Name: TimeName, Power: 1},
		},
		ErForm: "Charger",
	},
	{
		Name:        "Conductance",
		Receiver:    "co",
		PrintString: "S",
		Suffix:      "siemens",
		Singular:    "Siemens",
		TypeComment: "Conductance represents an electrical conductance in Siemens",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: 2},
			{Name: LengthName, Power: -2},
			{Name: MassName, Power: -1},
			{Name: TimeName, Power: 3},
		},
		ErForm: "Conductancer",
	},
	{
		Name:        "EquivalentRadioactiveDose",
		Receiver:    "a",
		PrintString: "Sy",
		Suffix:      "sievert",
		Singular:    "Sievert",
		TypeComment: "EquivalentRadioactiveDose is a measure of equivalent dose of ionizing radiation in sieverts",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "Energy",
		Receiver:    "e",
		PrintString: "J",
		Suffix:      "joule",
		Singular:    "Joule",
		TypeComment: "Energy represents a quantity of energy in Joules",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "Frequency",
		Receiver:    "f",
		PrintString: "Hz",
		Suffix:      "hertz",
		Singular:    "Hertz",
		TypeComment: "Frequency represents a frequency in Hertz",
		Dimensions: []Dimension{
			{Name: TimeName, Power: -1},
		},
	},
	{
		Name:        "Force",
		Receiver:    "f",
		PrintString: "N",
		Suffix:      "newton",
		Singular:    "Newton",
		TypeComment: "Force represents a force in Newtons",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Forcer",
	},
	{
		Name:        "Inductance",
		Receiver:    "i",
		PrintString: "H",
		Suffix:      "henry",
		Singular:    "Henry",
		TypeComment: "Inductance represents an electrical inductance in Henry",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -2},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Inductancer",
	},
	{
		Name:        "Power",
		Receiver:    "pw",
		PrintString: "W",
		Suffix:      "watt",
		Singular:    "Watt",
		TypeComment: "Power represents a power in Watts",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
	},
	{
		Name:        "Resistance",
		Receiver:    "r",
		PrintString: "Ω",
		Suffix:      "ohm",
		Singular:    "Ohm",
		TypeComment: "Resistance represents an electrical resistance, impedance or reactance in Ohms",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -2},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
		ErForm: "Resistancer",
	},
	{
		Name:        "MagneticFlux",
		Receiver:    "m",
		PrintString: "Wb",
		Suffix:      "weber",
		Singular:    "Weber",
		TypeComment: "MagneticFlux represents a magnetic flux in Weber",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "MagneticFluxDensity",
		Receiver:    "m",
		PrintString: "T",
		Suffix:      "tesla",
		Singular:    "Tesla",
		TypeComment: "MagneticFluxDensity represents a magnetic flux density in Tesla",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
	},
	{
		Name:        "Pressure",
		Receiver:    "pr",
		PrintString: "Pa",
		Suffix:      "pascal",
		Singular:    "Pascal",
		TypeComment: "Pressure represents a pressure in Pascals",
		Dimensions: []Dimension{
			{Name: LengthName, Power: -1},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Pressurer",
	},
	{
		Name:        "Torque",
		Receiver:    "t",
		PrintString: "N m",
		Suffix:      "newtonmetre",
		Singular:    "Newtonmetre",
		TypeComment: "Torque represents a torque in Newton metres",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -2},
		},
		ErForm: "Torquer",
	},
	{
		Name:        "Velocity",
		Receiver:    "v",
		PrintString: "m s^-1",
		TypeComment: "Velocity represents a velocity in metres per second",
		Dimensions: []Dimension{
			{Name: LengthName, Power: 1},
			{Name: TimeName, Power: -1},
		},
	},
	{
		Name:        "Voltage",
		Receiver:    "v",
		PrintString: "V",
		Suffix:      "volt",
		Singular:    "Volt",
		TypeComment: "Voltage represents a voltage in Volts",
		Dimensions: []Dimension{
			{Name: CurrentName, Power: -1},
			{Name: LengthName, Power: 2},
			{Name: MassName, Power: 1},
			{Name: TimeName, Power: -3},
		},
		ErForm: "Voltager",
	},
	{
		Name:        "Volume",
		Receiver:    "v",
		Offset:      -3,
		PrintString: "m^3",
		Suffix:      "litre",
		Singular:    "Litre",
		TypeComment: "Volume represents a volume in cubic metres",
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
type {{.Name}} float64
`

var header = template.Must(template.New("header").Parse(headerTemplate))

const constTemplate = `
const(
	{{$unit := .Unit}}
	{{range $unit.ExtraConstant}} {{.Name}} {{$unit.Name}} = {{.Value}}
	{{end}}
	{{$prefixes := .Prefixes}}
	{{range $prefixes}} {{if .Name}} {{.Name}}{{$unit.Suffix}} {{else}} {{$unit.Singular}} {{end}} {{$unit.Name}} = {{if .Power}} 1e{{.Power}} {{else}} 1.0 {{end}}
	{{end}}
)
`

var prefix = template.Must(template.New("prefix").Parse(constTemplate))

const methodTemplate = `
// Unit converts the {{.Name}} to a *Unit
func ({{.Receiver}} {{.Name}}) Unit() *Unit {
	return New(float64({{.Receiver}}), Dimensions{
		{{range .Dimensions}} {{.Name}}: {{.Power}},
		{{end}}
		})
}

// {{.Name}} allows {{.Name}} to implement a {{if .ErForm}}{{.ErForm}}{{else}}{{.Name}}er{{end}} interface
func ({{.Receiver}} {{.Name}}) {{.Name}}() {{.Name}} {
	return {{.Receiver}}
}

// From converts the unit into the receiver. From returns an
// error if there is a mismatch in dimension
func ({{.Receiver}} *{{.Name}}) From(u Uniter) error {
	if !DimensionsMatch(u, {{if .Singular}}{{.Singular}}{{else}}{{.Name}}(0){{end}}){
		*{{.Receiver}} = {{.Name}}(math.NaN())
		return errors.New("Dimension mismatch")
	}
	*{{.Receiver}} = {{.Name}}(u.Unit().Value())
	return nil
}
`

var methods = template.Must(template.New("methods").Parse(methodTemplate))

const formatTemplate = `
func ({{.Receiver}} {{.Name}}) Format(fs fmt.State, c rune) {
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
	lowerName := strings.ToLower(unit.Name)
	filename := lowerName + ".go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// Need to define new prefixes because text/template can't do math.
	// Need to do math because kilogram = 1 not 10^3

	prefixes := make([]Prefix, len(Prefixes))
	for i, p := range Prefixes {
		prefixes[i].Name = p.Name
		prefixes[i].Power = p.Power + unit.Offset
	}

	data := struct {
		Prefixes []Prefix
		Unit     Unit
	}{
		prefixes,
		unit,
	}

	buf := bytes.NewBuffer(make([]byte, 0))

	err = header.Execute(buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	if unit.Singular != "" {
		err = prefix.Execute(buf, data)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = methods.Execute(buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	err = form.Execute(buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format
		log.Fatalf("error formatting %q: %s", unit.Name, err)
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

func Test{{.Name}}Format(t *testing.T) {
	for _, test := range []struct{
		value  {{.Name}}
		format string
		want   string
	}{
		{1.23456789, "%v", "1.23456789 {{.PrintString}}"},
		{1.23456789, "%.1v", "1 {{.PrintString}}"},
		{1.23456789, "%20.1v", "{{$s := printf "1 %s" .PrintString}}{{printf "%20s" $s}}"},
		{1.23456789, "%20v", "{{$s := printf "1.23456789 %s" .PrintString}}{{printf "%20s" $s}}"},
		{1.23456789, "%1v", "1.23456789 {{.PrintString}}"},
		{1.23456789, "%#v", "unit.{{.Name}}(1.23456789)"},
		{1.23456789, "%s", "%!s(unit.{{.Name}}=1.23456789 {{.PrintString}})"},
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
	lowerName := strings.ToLower(unit.Name)
	filename := lowerName + "_test.go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	buf := bytes.NewBuffer(make([]byte, 0))

	err = tests.Execute(buf, unit)
	if err != nil {
		log.Fatal(err)
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format.
		log.Fatalf("error formatting test for %q: %s", unit.Name, err)
	}

	f.Write(b)
}
