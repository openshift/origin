/*
Package predeclared provides a static analysis that finds
declarations that shadow any of Go's predeclared identifiers. The
list of predeclared identifiers can be found in the language
specification.

	https://golang.org/ref/spec#Predeclared_identifiers

# Flags

The analysis may be configured with flags. The synopsis is:

	[-pkglevel] [-q] [-ignore string] [-mode string]

The flags are described below.

The -mode flag argument specifies the categories of issues that
the analysis should report. The argument is a string containing
one or more of the following letters. The default argument is the
string "s".

	d    declaration has same name as a predeclared identifier
	s    declaration shadows a predeclared identifier

Note that shadowing necessarily implies that the identifier in
the declaration has the same name as the predeclared identifier
being shadowed. In general the declarations reported for mode 's'
will be a subset of the declarations reported for mode 'd'.

The -ignore flag argument is a comma-separated list of
predeclared identifier names for which the analysis will not
report issues. For example, given the following flag argument,
the analysis will not report issues for the predeclared
identifiers new, min, and max. The default argument is an empty
string.

	-ignore "new,min,max"

The -pkglevel flag restricts the analysis to package-level
declarations and import declarations. By default all
declarations are analyzed.

The -q flag extends the analysis to field names (in struct type
declarations) and method names (in method declarations and
interface type declarations). This is applicable if the
-mode argument includes 'd'.
*/
package predeclared

import (
	"errors"
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var (
	fMode     = string(shadow)
	fIgnore   = ""
	fPkgLevel = false
	fField    = false
)

func resetFlags() {
	fMode = string(shadow)
	fIgnore = ""
	fPkgLevel = false
	fField = false
}

func init() {
	Analyzer.Flags.StringVar(&fMode, "mode", fMode, "select the categories of issues to report, any subset of \"ds\"")
	Analyzer.Flags.StringVar(&fIgnore, "ignore", fIgnore, "ignore these predeclared identifiers, comma-separated list")
	Analyzer.Flags.BoolVar(&fPkgLevel, "pkglevel", fPkgLevel, "analyze package-level declarations and import declarations only")
	Analyzer.Flags.BoolVar(&fField, "q", fField, "analyze field names and method names")
}

var Analyzer = &analysis.Analyzer{
	Name: "predeclared",
	Doc:  "find declarations that shadow Go's predeclared identifiers",
	Run:  run,
}

type mode rune

const (
	declare mode = 'd'
	shadow  mode = 's'
)

func run(pass *analysis.Pass) (any, error) {
	ignore := make(map[string]struct{})
	if fIgnore != "" {
		for name := range strings.SplitSeq(fIgnore, ",") {
			ignore[strings.TrimSpace(name)] = struct{}{}
		}
	}

	modes, err := parseMode(fMode)
	if err != nil {
		return nil, fmt.Errorf("invalid value for flag -mode: %s", err)
	}

	cfg := config{
		modes:        modes,
		pkgLevelOnly: fPkgLevel,
		checkFields:  fField,
		ignore:       ignore,
	}
	for _, file := range pass.Files {
		check(pass, &cfg, file)
	}
	return nil, nil
}

func parseMode(s string) ([]mode, error) {
	if s == "" {
		return nil, errors.New("empty string")
	}
	var ret []mode
	for _, v := range s {
		switch v := mode(v); v {
		case declare, shadow:
			if !slices.Contains(ret, v) {
				ret = append(ret, v)
			}
		default:
			return nil, fmt.Errorf("invalid letter %c", v)
		}
	}
	slices.Sort(ret)
	return ret, nil
}

func messagePrefix(m mode) string {
	switch m {
	case declare:
		return "same name as"
	case shadow:
		return "shadows"
	default:
		panic(fmt.Sprintf("internal error: unknown mode %v", m))
	}
}

type config struct {
	modes        []mode
	pkgLevelOnly bool
	checkFields  bool // whether to check fields and methods
	ignore       map[string]struct{}
}

func assert(v bool) {
	if !v {
		panic("assertion failed")
	}
}

func shadowsPredeclared(obj types.Object) bool {
	if !doc.IsPredeclared(obj.Name()) {
		panic("bad object name")
	}

	u := types.Universe.Lookup(obj.Name())
	assert(u != nil) // should not fail; see doc.IsPredeclared guard above.

	// Note that if obj is a label, field, or method, then c
	// will be nil. The validity of the 'c == u' result still
	// holds because u is always non-nil.
	c := lookupWhile(obj.Parent(), obj.Name(), obj.Pos(), func(obj2 types.Object) bool { return obj2 == obj })
	return c == u
}

func check(pass *analysis.Pass, cfg *config, file *ast.File) {
	// Report an object (by position) at most once per mode.
	//
	// This can happen in practice when handling implicitly
	// declared objects, such as those in type switch case
	// clauses. Note that, in these scenarios, the objects
	// themselves might be distinct (have different pointer
	// values) but obj.Pos() values and obj.Name() values
	// are the same.
	//
	reported := make(map[mode]map[token.Pos]struct{})
	for _, mode := range cfg.modes {
		reported[mode] = make(map[token.Pos]struct{})
	}

	reportAtMostOnce := func(mode mode, pos token.Pos, name string) {
		if _, ok := reported[mode][pos]; ok {
			return
		}
		pass.Reportf(pos, "%s: %s predeclared identifier", name, messagePrefix(mode))
		reported[mode][pos] = struct{}{}
	}

	for n := range ast.Preorder(file) {
		switch x := n.(type) {
		case *ast.Ident:
			// go/types: "Defs maps identifiers to the objects they define (including
			// package names, dots "." of dot-imports, and blank "_" identifiers).
			// For identifiers that do not denote objects (e.g., the package name
			// in package clauses, or symbolic variables t in t := x.(type) of
			// type switch headers), the corresponding objects are nil."
			if obj, ok := pass.TypesInfo.Defs[x]; ok && obj != nil {
				handleObject(obj, cfg, reportAtMostOnce)
			}
		case *ast.ImportSpec, *ast.CaseClause, *ast.Field:
			if obj, ok := pass.TypesInfo.Implicits[x]; ok && obj != nil {
				handleObject(obj, cfg, reportAtMostOnce)
			}
		}
	}
}

func handleObject(obj types.Object, cfg *config, report func(mode, token.Pos, string)) {
	if _, ok := cfg.ignore[obj.Name()]; ok {
		return
	}

	if !doc.IsPredeclared(obj.Name()) {
		return
	}

	if cfg.pkgLevelOnly {
		switch obj.(type) {
		case *types.PkgName:
			// *types.PkgName represents imported Go
			// packages (in import declarations).
			// Proceed below.
			_ = obj // Dummy statement to verify code coverage.
		default:
			// constant, variable, type, or function.
			if !isPkgLevel(obj) {
				return
			}
		}
	}

modeloop:
	for _, mode := range cfg.modes {
		switch mode {
		case declare:
			switch obj := obj.(type) {
			case *types.Var:
				if obj.IsField() && !cfg.checkFields {
					continue modeloop
				}
			case *types.Func:
				// go/types documentation for (*types.Func).Parent:
				// "Parent returns the scope in which the object
				// is declared. The result is nil for methods and
				// struct fields."
				if obj.Parent() == nil && !cfg.checkFields {
					continue modeloop
				}
			}
		case shadow:
			switch obj := obj.(type) {
			case *types.Label:
				// Labels cannot shadow the predeclared identifiers.
				// See the example in 'testdata/*' file.
				// Go spec: "In contrast to other identifiers, labels are
				// not block scoped and do not conflict with identifiers
				// that are not labels."
				continue modeloop
			case *types.Var:
				if obj.IsField() {
					continue modeloop
				}
			case *types.Func:
				if obj.Parent() == nil {
					continue modeloop
				}
			}
			if !shadowsPredeclared(obj) {
				continue modeloop
			}
		}
		report(mode, obj.Pos(), obj.Name())
	}
}

func lookupWhile(s *types.Scope, name string, pos token.Pos, f func(types.Object) bool) types.Object {
	for {
		_, c := s.LookupParent(name, pos)
		if f(c) && s != nil {
			s = s.Parent()
			continue
		}
		return c
	}
}

func isPkgLevel(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Pkg().Scope().Lookup(obj.Name()) == obj
}
