// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package irutil

// This file defines utility functions for constructing programs in IR form.

import (
	"go/ast"
	"go/token"
	"go/types"

	"honnef.co/go/tools/go/ir"

	"golang.org/x/tools/go/packages"
)

// Packages creates an IR program for a set of packages.
//
// The packages must have been loaded from source syntax using the
// golang.org/x/tools/go/packages.Load function in LoadSyntax or
// LoadAllSyntax mode.
//
// Packages creates an IR package for each well-typed package in the
// initial list, plus all their dependencies. The resulting list of
// packages corresponds to the list of initial packages, and may contain
// a nil if IR code could not be constructed for the corresponding initial
// package due to type errors.
//
// Code for bodies of functions is not built until Build is called on
// the resulting Program. IR code is constructed only for the initial
// packages with well-typed syntax trees.
//
// The mode parameter controls diagnostics and checking during IR construction.
func Packages(initial []*packages.Package, mode ir.BuilderMode) (*ir.Program, []*ir.Package) {
	// TODO(adonovan): opt: this calls CreatePackage far more than
	// necessary: for all dependencies, not just the (non-initial)
	// direct dependencies of the initial packages.
	//
	// But can it reasonably be changed without breaking the
	// spirit and/or letter of the law above? Clients may notice
	// if we call CreatePackage less, as methods like
	// Program.FuncValue will return nil. Or must we provide a new
	// function (and perhaps deprecate this one)? Is it worth it?
	//
	// Tim King makes the interesting point that it would be
	// possible to entirely alleviate the client from the burden
	// of calling CreatePackage for non-syntax packages, if we
	// were to treat vars and funcs lazily in the same way we now
	// treat methods. (In essence, try to move away from the
	// notion of ir.Packages, and make the Program answer
	// all reasonable questions about any types.Object.)

	return doPackages(initial, mode, false)
}

// AllPackages creates an IR program for a set of packages plus all
// their dependencies.
//
// The packages must have been loaded from source syntax using the
// golang.org/x/tools/go/packages.Load function in LoadAllSyntax mode.
//
// AllPackages creates an IR package for each well-typed package in the
// initial list, plus all their dependencies. The resulting list of
// packages corresponds to the list of initial packages, and may contain
// a nil if IR code could not be constructed for the corresponding
// initial package due to type errors.
//
// Code for bodies of functions is not built until Build is called on
// the resulting Program. IR code is constructed for all packages with
// well-typed syntax trees.
//
// The mode parameter controls diagnostics and checking during IR construction.
func AllPackages(initial []*packages.Package, mode ir.BuilderMode) (*ir.Program, []*ir.Package) {
	return doPackages(initial, mode, true)
}

func doPackages(initial []*packages.Package, mode ir.BuilderMode, deps bool) (*ir.Program, []*ir.Package) {
	var fset *token.FileSet
	if len(initial) > 0 {
		fset = initial[0].Fset
	}

	prog := ir.NewProgram(fset, mode)

	isInitial := make(map[*packages.Package]bool, len(initial))
	for _, p := range initial {
		isInitial[p] = true
	}

	irmap := make(map[*packages.Package]*ir.Package)
	packages.Visit(initial, nil, func(p *packages.Package) {
		if p.Types != nil && !p.IllTyped {
			var files []*ast.File
			var info *types.Info
			if deps || isInitial[p] {
				files = p.Syntax
				info = p.TypesInfo
			}
			irmap[p] = prog.CreatePackage(p.Types, files, info, true)
		}
	})

	var irpkgs []*ir.Package
	for _, p := range initial {
		irpkgs = append(irpkgs, irmap[p]) // may be nil
	}
	return prog, irpkgs
}

// BuildPackage builds an IR program with IR intermediate
// representation (IR) for all functions of a single package.
//
// It populates pkg by type-checking the specified file ASTs.  All
// dependencies are loaded using the importer specified by tc, which
// typically loads compiler export data; IR code cannot be built for
// those packages.  BuildPackage then constructs an ir.Program with all
// dependency packages created, and builds and returns the IR package
// corresponding to pkg.
//
// The caller must have set pkg.Path() to the import path.
//
// The operation fails if there were any type-checking or import errors.
//
// See ../ir/example_test.go for an example.
func BuildPackage(tc *types.Config, fset *token.FileSet, pkg *types.Package, files []*ast.File, mode ir.BuilderMode) (*ir.Package, *types.Info, error) {
	if fset == nil {
		panic("no token.FileSet")
	}
	if pkg.Path() == "" {
		panic("package has no import path")
	}

	info := &types.Info{
		Types:        make(map[ast.Expr]types.TypeAndValue),
		Defs:         make(map[*ast.Ident]types.Object),
		Uses:         make(map[*ast.Ident]types.Object),
		Implicits:    make(map[ast.Node]types.Object),
		Scopes:       make(map[ast.Node]*types.Scope),
		Selections:   make(map[*ast.SelectorExpr]*types.Selection),
		Instances:    make(map[*ast.Ident]types.Instance),
		FileVersions: make(map[*ast.File]string),
	}
	if err := types.NewChecker(tc, fset, pkg, info).Files(files); err != nil {
		return nil, nil, err
	}

	prog := ir.NewProgram(fset, mode)

	// Create IR packages for all imports.
	// Order is not significant.
	created := make(map[*types.Package]bool)
	var createAll func(pkgs []*types.Package)
	createAll = func(pkgs []*types.Package) {
		for _, p := range pkgs {
			if !created[p] {
				created[p] = true
				prog.CreatePackage(p, nil, nil, true)
				createAll(p.Imports())
			}
		}
	}
	createAll(pkg.Imports())

	// TODO(adonovan): we could replace createAll with just:
	//
	// // Create IR packages for all imports.
	// for _, p := range pkg.Imports() {
	// 	prog.CreatePackage(p, nil, nil, true)
	// }
	//
	// (with minor changes to changes to ../builder_test.go as
	// shown in CL 511715 PS 10.) But this would strictly violate
	// the letter of the doc comment above, which says "all
	// dependencies created".
	//
	// Tim makes the good point with some extra work we could
	// remove the need for any CreatePackage calls except the
	// ones with syntax (i.e. primary packages). Of course
	// You wouldn't have ir.Packages and Members for as
	// many things but no-one really uses that anyway.
	// I wish I had done this from the outset.

	// Create and build the primary package.
	irpkg := prog.CreatePackage(pkg, files, info, false)
	irpkg.Build()
	return irpkg, info, nil
}
