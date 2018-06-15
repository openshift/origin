// Copyright ©2016 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// generate_lapacke creates a lapacke.go file from the provided C header file
// with optionally added documentation from the documentation package.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/cznic/cc"

	"github.com/gonum/internal/binding"
)

const (
	header = "lapacke.h"
	target = "lapacke.go"

	prefix = "LAPACKE_"
	suffix = "_work"
)

const (
	elideRepeat = true
	noteOrigin  = false
)

var skip = map[string]bool{
	// Deprecated.
	"LAPACKE_cggsvp_work": true,
	"LAPACKE_dggsvp_work": true,
	"LAPACKE_sggsvp_work": true,
	"LAPACKE_zggsvp_work": true,
	"LAPACKE_cggsvd_work": true,
	"LAPACKE_dggsvd_work": true,
	"LAPACKE_sggsvd_work": true,
	"LAPACKE_zggsvd_work": true,
	"LAPACKE_cgeqpf_work": true,
	"LAPACKE_dgeqpf_work": true,
	"LAPACKE_sgeqpf_work": true,
	"LAPACKE_zgeqpf_work": true,
}

// needsInt is a list of routines that need to return the integer info value and
// and cannot convert to a success boolean.
var needsInt = map[string]bool{
	"hseqr": true,
	"geev":  true,
	"geevx": true,
}

// allUplo is a list of routines that allow 'A' for their uplo argument.
// The list keys are truncated by one character to cover all four numeric types.
var allUplo = map[string]bool{
	"lacpy": true,
	"laset": true,
}

var cToGoType = map[string]string{
	"char":           "byte",
	"int_must":       "int",
	"int_must32":     "int32",
	"int":            "bool",
	"float":          "float32",
	"double":         "float64",
	"float complex":  "complex64",
	"double complex": "complex128",
}

var cToGoTypeConv = map[string]string{
	"int_must":       "int",
	"int":            "isZero",
	"float":          "float32",
	"double":         "float64",
	"float complex":  "complex64",
	"double complex": "complex128",
}

var cgoEnums = map[string]*template.Template{}

var byteTypes = map[string]string{
	"compq": "lapack.Comp",
	"compz": "lapack.Comp",

	"d": "blas.Diag",

	"job":    "lapack.Job",
	"joba":   "lapack.Job",
	"jobr":   "lapack.Job",
	"jobp":   "lapack.Job",
	"jobq":   "lapack.Job",
	"jobt":   "lapack.Job",
	"jobu":   "lapack.Job",
	"jobu1":  "lapack.Job",
	"jobu2":  "lapack.Job",
	"jobv":   "lapack.Job",
	"jobv1t": "lapack.Job",
	"jobv2t": "lapack.Job",
	"jobvl":  "lapack.Job",
	"jobvr":  "lapack.Job",
	"jobvt":  "lapack.Job",
	"jobz":   "lapack.Job",

	"side": "blas.Side",

	"trans":  "blas.Transpose",
	"trana":  "blas.Transpose",
	"tranb":  "blas.Transpose",
	"transr": "blas.Transpose",

	"ul": "blas.Uplo",

	"balanc": "byte",
	"cmach":  "byte",
	"direct": "byte",
	"dist":   "byte",
	"equed":  "byte",
	"eigsrc": "byte",
	"fact":   "byte",
	"howmny": "byte",
	"id":     "byte",
	"initv":  "byte",
	"norm":   "byte",
	"order":  "byte",
	"pack":   "byte",
	"sense":  "byte",
	"signs":  "byte",
	"storev": "byte",
	"sym":    "byte",
	"typ":    "byte",
	"rng":    "byte",
	"vect":   "byte",
	"way":    "byte",
}

func typeForByte(n string) string {
	t, ok := byteTypes[n]
	if !ok {
		return fmt.Sprintf("<unknown %q>", n)
	}
	return t
}

var intTypes = map[string]string{
	"forwrd": "int32",

	"ijob": "lapack.Job",

	"wantq": "int32",
	"wantz": "int32",
}

func typeForInt(n string) string {
	t, ok := intTypes[n]
	if !ok {
		return "int"
	}
	return t
}

// TODO(kortschak): convForInt* are for #define types,
// so they could go away. Kept here now for diff reduction.

func convForInt(n string) string {
	switch n {
	case "rowMajor":
		return "C.int"
	case "forwrd", "wantq", "wantz":
		return "C.lapack_logical"
	default:
		return "C.lapack_int"
	}
}

func convForIntSlice(n string) string {
	switch n {
	case "bwork", "tryrac":
		return "*C.lapack_logical"
	default:
		return "*C.lapack_int"
	}
}

var goTypes = map[binding.TypeKey]*template.Template{
	{Kind: cc.Char}:                           template.Must(template.New("byte").Funcs(map[string]interface{}{"typefor": typeForByte}).Parse("{{typefor .}}")),
	{Kind: cc.Int}:                            template.Must(template.New("int").Funcs(map[string]interface{}{"typefor": typeForInt}).Parse("{{typefor .}}")),
	{Kind: cc.Char, IsPointer: true}:          template.Must(template.New("[]byte").Parse("[]byte")),
	{Kind: cc.Int, IsPointer: true}:           template.Must(template.New("[]int32").Parse("[]int32")),
	{Kind: cc.FloatComplex, IsPointer: true}:  template.Must(template.New("[]complex64").Parse("[]complex64")),
	{Kind: cc.DoubleComplex, IsPointer: true}: template.Must(template.New("[]complex128").Parse("[]complex128")),
}

var cgoTypes = map[binding.TypeKey]*template.Template{
	{Kind: cc.Char}:                           template.Must(template.New("char").Parse("(C.char)({{.}})")),
	{Kind: cc.Int}:                            template.Must(template.New("int").Funcs(map[string]interface{}{"conv": convForInt}).Parse(`({{conv .}})({{.}})`)),
	{Kind: cc.Float}:                          template.Must(template.New("float").Parse("(C.float)({{.}})")),
	{Kind: cc.Double}:                         template.Must(template.New("double").Parse("(C.double)({{.}})")),
	{Kind: cc.FloatComplex}:                   template.Must(template.New("lapack_complex_float").Parse("(C.lapack_complex_float)({{.}})")),
	{Kind: cc.DoubleComplex}:                  template.Must(template.New("lapack_complex_double").Parse("(C.lapack_complex_double)({{.}})")),
	{Kind: cc.Char, IsPointer: true}:          template.Must(template.New("char*").Parse("(*C.char)(unsafe.Pointer(_{{.}}))")),
	{Kind: cc.Int, IsPointer: true}:           template.Must(template.New("int*").Funcs(map[string]interface{}{"conv": convForIntSlice}).Parse("({{conv .}})(_{{.}})")),
	{Kind: cc.Float, IsPointer: true}:         template.Must(template.New("float").Parse("(*C.float)(_{{.}})")),
	{Kind: cc.Double, IsPointer: true}:        template.Must(template.New("double").Parse("(*C.double)(_{{.}})")),
	{Kind: cc.FloatComplex, IsPointer: true}:  template.Must(template.New("lapack_complex_float*").Parse("(*C.lapack_complex_float)(_{{.}})")),
	{Kind: cc.DoubleComplex, IsPointer: true}: template.Must(template.New("lapack_complex_double*").Parse("(*C.lapack_complex_double)(_{{.}})")),
}

var names = map[string]string{
	"matrix_layout": "rowMajor",
	"uplo":          "ul",
	"range":         "rng",
	"diag":          "d",
	"select":        "sel",
	"type":          "typ",
}

func shorten(n string) string {
	s, ok := names[n]
	if ok {
		return s
	}
	return n
}

func join(a []string) string {
	return strings.Join(a, " ")
}

func main() {
	decls, err := binding.Declarations(header)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer

	h, err := template.New("handwritten").
		Funcs(map[string]interface{}{"join": join}).
		Parse(handwritten)
	if err != nil {
		log.Fatal(err)
	}
	err = h.Execute(&buf, struct {
		Header string
		Lib    []string
	}{
		Header: header,
		Lib:    os.Args[1:],
	})
	if err != nil {
		log.Fatal(err)
	}

	for _, d := range decls {
		if !strings.HasPrefix(d.Name, prefix) || !strings.HasSuffix(d.Name, suffix) || skip[d.Name] {
			continue
		}
		lapackeName := strings.TrimSuffix(strings.TrimPrefix(d.Name, prefix), suffix)
		switch {
		case strings.HasSuffix(lapackeName, "fsx"):
			continue
		case strings.HasSuffix(lapackeName, "vxx"):
			continue
		case strings.HasSuffix(lapackeName, "rook"):
			continue
		}
		if hasFuncParameter(d) {
			continue
		}

		goSignature(&buf, d)
		if noteOrigin {
			fmt.Fprintf(&buf, "\t// %s %s %s ...\n\n", d.Position(), d.Return, d.Name)
		}
		parameterChecks(&buf, d, parameterCheckRules)
		buf.WriteByte('\t')
		cgoCall(&buf, d)
		buf.WriteString("}\n")
	}

	b, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	err = ioutil.WriteFile(target, b, 0664)
	if err != nil {
		log.Fatal(err)
	}
}

// This removes select and selctg parameterised functions.
func hasFuncParameter(d binding.Declaration) bool {
	for _, p := range d.Parameters() {
		if p.Kind() != cc.Ptr {
			continue
		}
		if p.Elem().Kind() == cc.Function {
			return true
		}
	}
	return false
}

func goSignature(buf *bytes.Buffer, d binding.Declaration) {
	lapackeName := strings.TrimSuffix(strings.TrimPrefix(d.Name, prefix), suffix)
	goName := binding.UpperCaseFirst(lapackeName)

	parameters := d.Parameters()

	fmt.Fprintf(buf, "\n// See http://www.netlib.org/cgi-bin/netlibfiles.txt?format=txt&filename=/lapack/lapack_routine/%s.f.\n", lapackeName)
	fmt.Fprintf(buf, "func %s(", goName)
	c := 0
	for i, p := range parameters {
		if p.Name() == "matrix_layout" {
			continue
		}
		if c != 0 {
			buf.WriteString(", ")
		}
		c++

		n := shorten(binding.LowerCaseFirst(p.Name()))
		var this, next string

		if p.Kind() == cc.Enum {
			this = binding.GoTypeForEnum(p.Type(), n)
		} else {
			this = binding.GoTypeFor(p.Type(), n, goTypes)
		}

		if elideRepeat && i < len(parameters)-1 && p.Type().Kind() == parameters[i+1].Type().Kind() {
			p := parameters[i+1]
			n := shorten(binding.LowerCaseFirst(p.Name()))
			if p.Kind() == cc.Enum {
				next = binding.GoTypeForEnum(p.Type(), n)
			} else {
				next = binding.GoTypeFor(p.Type(), n, goTypes)
			}
		}
		if next == this {
			buf.WriteString(n)
		} else {
			fmt.Fprintf(buf, "%s %s", n, this)
		}
	}
	if d.Return.Kind() != cc.Void {
		var must string
		if needsInt[lapackeName[1:]] {
			must = "_must"
		}
		fmt.Fprintf(buf, ") %s {\n", cToGoType[d.Return.String()+must])
	} else {
		buf.WriteString(") {\n")
	}
}

func parameterChecks(buf *bytes.Buffer, d binding.Declaration, rules []func(*bytes.Buffer, binding.Declaration, binding.Parameter) bool) {
	done := make(map[int]bool)
	for _, p := range d.Parameters() {
		for i, r := range rules {
			if done[i] {
				continue
			}
			done[i] = r(buf, d, p)
		}
	}
}

func cgoCall(buf *bytes.Buffer, d binding.Declaration) {
	if d.Return.Kind() != cc.Void {
		lapackeName := strings.TrimSuffix(strings.TrimPrefix(d.Name, prefix), suffix)
		var must string
		if needsInt[lapackeName[1:]] {
			must = "_must"
		}
		fmt.Fprintf(buf, "return %s(", cToGoTypeConv[d.Return.String()+must])
	}
	fmt.Fprintf(buf, "C.%s(", d.Name)
	for i, p := range d.Parameters() {
		if i != 0 {
			buf.WriteString(", ")
		}
		if p.Type().Kind() == cc.Enum {
			buf.WriteString(binding.CgoConversionForEnum(shorten(binding.LowerCaseFirst(p.Name())), p.Type()))
		} else {
			buf.WriteString(binding.CgoConversionFor(shorten(binding.LowerCaseFirst(p.Name())), p.Type(), cgoTypes))
		}
	}
	if d.Return.Kind() != cc.Void {
		buf.WriteString(")")
	}
	buf.WriteString(")\n")
}

var parameterCheckRules = []func(*bytes.Buffer, binding.Declaration, binding.Parameter) bool{
	uplo,
	diag,
	side,
	trans,
	address,
}

func uplo(buf *bytes.Buffer, d binding.Declaration, p binding.Parameter) bool {
	if p.Name() != "uplo" {
		return false
	}
	lapackeName := strings.TrimSuffix(strings.TrimPrefix(d.Name, prefix), suffix)
	if allUplo[lapackeName[1:]] {
		fmt.Fprint(buf, `	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		ul = 'A'
	}
`)
	} else {
		fmt.Fprint(buf, `	switch ul {
	case blas.Upper:
		ul = 'U'
	case blas.Lower:
		ul = 'L'
	default:
		panic("lapack: illegal triangle")
	}
`)
	}
	return true
}

func diag(buf *bytes.Buffer, d binding.Declaration, p binding.Parameter) bool {
	if p.Name() != "diag" {
		return false
	}
	fmt.Fprint(buf, `	switch d {
	case blas.Unit:
		d = 'U'
	case blas.NonUnit:
		d = 'N'
	default:
		panic("lapack: illegal diagonal")
	}
`)
	return true
}

func side(buf *bytes.Buffer, d binding.Declaration, p binding.Parameter) bool {
	if p.Name() != "side" {
		return false
	}
	fmt.Fprint(buf, `	switch side {
	case blas.Left:
		side = 'L'
	case blas.Right:
		side = 'R'
	default:
		panic("lapack: bad side")
	}
`)
	return true
}

func trans(buf *bytes.Buffer, d binding.Declaration, p binding.Parameter) bool {
	n := shorten(binding.LowerCaseFirst(p.Name()))
	if !strings.HasPrefix(n, "tran") {
		return false
	}
	fmt.Fprintf(buf, `	switch %[1]s {
	case blas.NoTrans:
		%[1]s = 'N'
	case blas.Trans:
		%[1]s = 'T'
	case blas.ConjTrans:
		%[1]s = 'C'
	default:
		panic("lapack: bad trans")
	}
`, n)
	return false
}

var addrTypes = map[string]string{
	"char":           "byte",
	"int":            "int32",
	"float":          "float32",
	"double":         "float64",
	"float complex":  "complex64",
	"double complex": "complex128",
}

func address(buf *bytes.Buffer, d binding.Declaration, p binding.Parameter) bool {
	n := shorten(binding.LowerCaseFirst(p.Name()))
	if p.Type().Kind() == cc.Ptr {
		t := strings.TrimPrefix(p.Type().Element().String(), "const ")
		fmt.Fprintf(buf, `	var _%[1]s *%[2]s
	if len(%[1]s) > 0 {
		_%[1]s = &%[1]s[0]
	}
`, n, addrTypes[t])
	}
	return false
}

const handwritten = `// Code generated by "go generate github.com/gonum/lapack/cgo/lapacke" from {{.Header}}; DO NOT EDIT.

// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package lapacke provides bindings to the LAPACKE C Interface to LAPACK.
//
// Links are provided to the NETLIB fortran implementation/dependencies for each function.
package lapacke

/*
#cgo CFLAGS: -g -O2{{if .Lib}}
#cgo LDFLAGS: {{join .Lib}}{{end}}
#include "{{.Header}}"
*/
import "C"

import (
	"unsafe"

	"github.com/gonum/blas"
	"github.com/gonum/lapack"
)

// Type order is used to specify the matrix storage format. We still interact with
// an API that allows client calls to specify order, so this is here to document that fact.
type order int

const (
	rowMajor order = 101 + iota
	colMajor
)

func isZero(ret C.int) bool { return ret == 0 }
`
