// Copyright ©2014 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"bytes"
	"go/format"
	"log"
	"math"
	"os"
	"strings"
	"text/template"

	"gonum.org/v1/gonum/unit"
)

const (
	elementaryCharge = 1.602176634e-19
	fineStructure    = 7.2973525693e-3
	lightSpeed       = 2.99792458e8
	planck           = 6.62607015e-34
)

var constants = []Constant{
	{
		Name: "AtomicMass", Value: 1.66053906660e-27,
		Dimensions:  []Dimension{{massName, 1}},
		Comment:     "AtomicMass is the atomic mass constant (mᵤ), one twelfth of the mass of an unbound atom of carbon-12 at rest and in its ground state.",
		Uncertainty: 0.00000000050e-27,
	},
	{
		Name: "Avogadro", Value: 6.02214076e23,
		Dimensions: []Dimension{{moleName, -1}},
		Comment:    "Avogadro is the Avogadro constant (A), the number of constituent particles contained in one mole of a substance.",
	},
	{
		Name: "Boltzmann", Value: 1.380649e-23,
		Dimensions: []Dimension{{massName, 1}, {lengthName, 2}, {timeName, -2}, {temperatureName, -1}},
		Comment:    "Boltzmann is the Boltzmann constant (k), it relates the average relative kinetic energy of particles in a gas with the temperature of the gas.",
	},
	{
		Name: "ElectricConstant", Value: 1 / (4 * math.Pi * 1e-7 * lightSpeed * lightSpeed),
		Dimensions:  []Dimension{{currentName, 2}, {timeName, 4}, {massName, -1}, {lengthName, -3}},
		Comment:     "ElectricConstant is the electric constant (ε₀), the value of the absolute dielectric permittivity of classical vacuum.",
		Uncertainty: 0.0000000013e-12,
	},
	{
		Name: "ElementaryCharge", Value: elementaryCharge,
		Dimensions: []Dimension{{currentName, 1}, {timeName, 1}},
		Comment:    "ElementaryCharge, is the elementary charge constant (e), the magnitude of electric charge carried by a single proton or electron.",
	},
	{
		Name: "Faraday", Value: 96485.33212,
		Dimensions: []Dimension{{currentName, 1}, {timeName, 1}, {moleName, -1}},
		Comment:    "Faraday is the Faraday constant, the magnitude of electric charge per mole of electrons.",
	},
	{
		Name: "FineStructure", Value: fineStructure,
		Comment:     "FineStructure is the fine structure constant (α), it describes the strength of the electromagnetic interaction between elementary charged particles.",
		Uncertainty: 0.0000000011e-3,
	},
	{
		Name: "Gravitational", Value: 6.67430e-11,
		Dimensions:  []Dimension{{massName, -1}, {lengthName, 3}, {timeName, -2}},
		Comment:     "Gravitational is the universal gravitational constant (G), the proportionality constant connecting the gravitational force between two bodies.",
		Uncertainty: 0.00015e-11,
	},
	{
		Name: "LightSpeedInVacuum", Value: lightSpeed,
		Dimensions: []Dimension{{lengthName, 1}, {timeName, -1}},
		Comment:    "LightSpeedInVacuum is the c constant, the speed of light in a vacuum.",
	},
	{
		Name: "MagneticConstant", Value: 2 * fineStructure * planck / (elementaryCharge * elementaryCharge * lightSpeed),
		Dimensions:  []Dimension{{currentName, 2}, {timeName, 4}, {massName, -1}, {lengthName, -3}},
		Comment:     "MagneticConstant is the magnetic constant (μ₀), the magnetic permeability in a classical vacuum.",
		Uncertainty: 0.00000000019e-6,
	},
	{
		Name: "Planck", Value: planck,
		Dimensions: []Dimension{{massName, 1}, {lengthName, 2}, {timeName, -1}},
		Comment:    "Planck is the Planck constant (h), it relates the energy carried by a photon to its frequency.",
	},
	{
		Name: "StandardGravity", Value: 9.80665,
		Dimensions: []Dimension{{lengthName, 1}, {timeName, -2}},
		Comment:    "StandardGravity is the standard gravity constant (g₀), the nominal gravitational acceleration of an object in a vacuum near the surface of the Earth",
	},
}

const (
	angleName             = "AngleDim"
	currentName           = "CurrentDim"
	lengthName            = "LengthDim"
	luminousIntensityName = "LuminousIntensityDim"
	massName              = "MassDim"
	moleName              = "MoleDim"
	temperatureName       = "TemperatureDim"
	timeName              = "TimeDim"
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
	Name        string
	Value       float64
	Dimensions  []Dimension
	Comment     string
	Uncertainty float64
}

type Dimension struct {
	Name  string
	Power int
}

func (c Constant) IsDefined() bool {
	return definedEquivalentOf(unit.New(1, c.dimensions())) != ""
}

func (c Constant) Type() string {
	typ := definedEquivalentOf(unit.New(1, c.dimensions()))
	if typ == "" {
		return strings.ToLower(c.Name[:1]) + c.Name[1:] + "Units"
	}
	return typ
}

func (c Constant) Units() string {
	return c.dimensions().String()
}

func (c Constant) dimensions() unit.Dimensions {
	dims := make(unit.Dimensions)
	for _, d := range c.Dimensions {
		dims[dimOf[d.Name]] = d.Power
	}
	return dims
}

// Generate a file for each of the constants.
func main() {
	for _, c := range constants {
		generate(c)
		generateTest(c)
	}
}

const baseUnitTemplate = `// Code generated by "go generate gonum.org/v1/gonum/unit/constant”; DO NOT EDIT.

// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package constant

import "gonum.org/v1/gonum/unit"

// {{.Comment}}{{if .Dimensions}}
// {{$n := len .Dimensions}}The dimension{{if gt $n 1}}s{{end}} of {{.Name}} {{if eq $n 1}}is{{else}}are{{end}} {{.Units}}.{{end}} {{if not .Uncertainty}}The constant is exact.{{else}}The standard uncertainty of the constant is {{.Uncertainty}} {{.Units}}.{{end}}
const {{.Name}} = {{.Type}}({{.Value}})
`

var baseUnit = template.Must(template.New("base").Parse(baseUnitTemplate))

const methodTemplate = `// Code generated by "go generate gonum.org/v1/gonum/unit/constant”; DO NOT EDIT.

// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package constant

import (
	"fmt"

	"gonum.org/v1/gonum/unit"
)

// {{.Comment}}
// {{$n := len .Dimensions}}The dimension{{if gt $n 1}}s{{end}} of {{.Name}} {{if eq $n 1}}is{{else}}are{{end}} {{.Units}}. {{if not .Uncertainty}}The constant is exact.{{else}}The standard uncertainty of the constant is {{.Uncertainty}} {{.Units}}.{{end}}
const {{.Name}} = {{.Type}}({{.Value}})

type {{.Type}} float64

// Unit converts the {{.Type}} to a *unit.Unit
func (cnst {{.Type}}) Unit() *unit.Unit {
	return unit.New(float64(cnst), unit.Dimensions{
		{{range .Dimensions}} unit.{{.Name}}: {{.Power}},
		{{end}}
		})
}

func (cnst {{.Type}}) Format(fs fmt.State, c rune) {
	switch c {
	case 'v':
		if fs.Flag('#') {
			fmt.Fprintf(fs, "%T(%v)", cnst, float64(cnst))
			return
		}
		fallthrough
	case 'e', 'E', 'f', 'F', 'g', 'G':
		p, pOk := fs.Precision()
		w, wOk := fs.Width()
		switch {
		case pOk && wOk:
			fmt.Fprintf(fs, "%*.*"+string(c), w, p, cnst.Unit())
		case pOk:
			fmt.Fprintf(fs, "%.*"+string(c), p, cnst.Unit())
		case wOk:
			fmt.Fprintf(fs, "%*"+string(c), w, cnst.Unit())
		default:
			fmt.Fprintf(fs, "%"+string(c), cnst.Unit())
		}
	default:
		fmt.Fprintf(fs, "%%!"+string(c)+"(constant.{{.Type}}=%v {{.Units}})", float64(cnst))
	}
}
`

var methods = template.Must(template.New("methods").Parse(methodTemplate))

func generate(c Constant) {
	lowerName := strings.ToLower(c.Name)
	filename := lowerName + ".go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var buf bytes.Buffer

	if c.IsDefined() {
		err = baseUnit.Execute(&buf, c)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		err = methods.Execute(&buf, c)
		if err != nil {
			log.Fatal(err)
		}
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format.
		log.Fatalf("error formatting %q: %s", c.Name, err)
	}

	f.Write(b)
}

const testTemplate = `// Code generated by "go generate gonum.org/v1/gonum/unit/constant”; DO NOT EDIT.

// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package constant

import (
	"fmt"
	"testing"
)

func Test{{.Name}}Format(t *testing.T) {
	for _, test := range []struct{
		format string
		want   string
	}{
		{"%v", "{{.Value}} {{.Units}}"},
		{"%.1v", "{{printf "%.1v" .Value}} {{.Units}}"},
		{"%50.1v", "{{$s := printf "%.1v %s" .Value .Units}}{{printf "%50s" $s}}"},
		{"%50v", "{{$s := printf "%v %s" .Value .Units}}{{printf "%50s" $s}}"},
		{"%1v", "{{.Value}} {{.Units}}"},
		{"%#v", "constant.{{.Type}}({{.Value}})"},
		{"%s", "%!s(constant.{{.Type}}={{.Value}} {{.Units}})"},
	} {
		got := fmt.Sprintf(test.format, {{.Name}})
		if got != test.want {
			t.Errorf("Format %q: got: %q want: %q", test.format, got, test.want)
		}
	}
}
`

var tests = template.Must(template.New("test").Parse(testTemplate))

func generateTest(c Constant) {
	if c.IsDefined() {
		return
	}

	lowerName := strings.ToLower(c.Name)
	filename := lowerName + "_test.go"
	f, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	var buf bytes.Buffer

	err = tests.Execute(&buf, c)
	if err != nil {
		log.Fatal(err)
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		f.Write(buf.Bytes()) // This is here to debug bad format.
		log.Fatalf("error formatting test for %q: %s", c.Name, err)
	}

	f.Write(b)
}
