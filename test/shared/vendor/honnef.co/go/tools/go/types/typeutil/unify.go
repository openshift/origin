// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The code in this file is copied from
// x/tools/gopls/internal/golang/implementation.go

package typeutil

import (
	"fmt"
	"go/types"
	"iter"
	"reflect"
)

// Unify reports whether the types of x and y match.
//
// If unifier is nil, unify reports only whether it succeeded.
// If unifier is non-nil, it is populated with the values
// of type parameters determined during a successful unification.
// If unification succeeds without binding a type parameter, that parameter
// will not be present in the map.
//
// On entry, the unifier's contents are treated as the values of already-bound type
// parameters, constraining the unification.
//
// For example, if unifier is an empty (not nil) map on entry, then the types
//
//	func[T any](T, int)
//
// and
//
//	func[U any](bool, U)
//
// will unify, with T=bool and U=int.
// That is, the contents of unifier after unify returns will be
//
//	{T: bool, U: int}
//
// where "T" is the type parameter T and "bool" is the basic type for bool.
//
// But if unifier is {T: int} is int on entry, then unification will fail, because T
// does not unify with bool.
//
// Unify does not preserve aliases. For example, given the following:
//
//	type String = string
//	type A[T] = T
//
// unification succeeds with T bound to string, not String.
//
// See also: unify in cache/methodsets/fingerprint, which implements
// unification for type fingerprints, for the global index.
//
// BUG: literal interfaces are not handled properly. But this function is currently
// used only for signatures, where such types are very rare.
func Unify(x, y types.Type, unifier map[*types.TypeParam]types.Type) bool {
	// bindings[tp] is the binding for type parameter tp.
	// Although type parameters are nominally bound to types, each bindings[tp]
	// is a pointer to a type, so unbound variables that unify can share a binding.
	bindings := map[*types.TypeParam]*types.Type{}

	// Bindings is initialized with pointers to the provided types.
	for tp, t := range unifier {
		bindings[tp] = &t
	}

	// bindingFor returns the *types.Type in bindings for tp if tp is not nil,
	// creating one if needed.
	bindingFor := func(tp *types.TypeParam) *types.Type {
		if tp == nil {
			return nil
		}
		b := bindings[tp]
		if b == nil {
			b = new(types.Type)
			bindings[tp] = b
		}
		return b
	}

	// bind sets b to t if b does not occur in t.
	bind := func(b *types.Type, t types.Type) bool {
		for tp := range typeParams(t) {
			if b == bindings[tp] {
				return false // failed "occurs" check
			}
		}
		*b = t
		return true
	}

	// uni performs the actual unification.
	depth := 0
	var uni func(x, y types.Type) bool
	uni = func(x, y types.Type) bool {
		// Panic if recursion gets too deep, to detect bugs before
		// overflowing the stack.
		depth++
		defer func() { depth-- }()
		if depth > 100 {
			panic("unify: max depth exceeded")
		}

		x = types.Unalias(x)
		y = types.Unalias(y)

		tpx, _ := x.(*types.TypeParam)
		tpy, _ := y.(*types.TypeParam)
		if tpx != nil || tpy != nil {
			// Identical type params unify.
			if tpx == tpy {
				return true
			}
			bx := bindingFor(tpx)
			by := bindingFor(tpy)

			// If both args are type params and neither is bound, have them share a binding.
			if bx != nil && by != nil && *bx == nil && *by == nil {
				// Arbitrarily give y's binding to x.
				bindings[tpx] = by
				return true
			}
			// Treat param bindings like original args in what follows.
			if bx != nil && *bx != nil {
				x = *bx
			}
			if by != nil && *by != nil {
				y = *by
			}
			// If the x param is unbound, bind it to y.
			if bx != nil && *bx == nil {
				return bind(bx, y)
			}
			// If the y param is unbound, bind it to x.
			if by != nil && *by == nil {
				return bind(by, x)
			}
			// Unify the binding of a bound parameter.
			return uni(x, y)
		}

		// Neither arg is a type param.

		if reflect.TypeOf(x) != reflect.TypeOf(y) {
			return false // mismatched types
		}

		switch x := x.(type) {
		case *types.Array:
			y := y.(*types.Array)
			return x.Len() == y.Len() &&
				uni(x.Elem(), y.Elem())

		case *types.Basic:
			y := y.(*types.Basic)
			return x.Kind() == y.Kind()

		case *types.Chan:
			y := y.(*types.Chan)
			return x.Dir() == y.Dir() &&
				uni(x.Elem(), y.Elem())

		case *types.Interface:
			y := y.(*types.Interface)
			// TODO(adonovan,jba): fix: for correctness, we must check
			// that both interfaces have the same set of methods
			// modulo type parameters, while avoiding the risk of
			// unbounded interface recursion.
			//
			// Since non-empty interface literals are vanishingly
			// rare in methods signatures, we ignore this for now.
			// If more precision is needed we could compare method
			// names and arities, still without full recursion.
			return x.NumMethods() == y.NumMethods()

		case *types.Map:
			y := y.(*types.Map)
			return uni(x.Key(), y.Key()) &&
				uni(x.Elem(), y.Elem())

		case *types.Named:
			y := y.(*types.Named)
			if x.Origin() != y.Origin() {
				return false // different named types
			}
			xtargs := x.TypeArgs()
			ytargs := y.TypeArgs()
			if xtargs.Len() != ytargs.Len() {
				return false // arity error (ill-typed)
			}
			for i := range xtargs.Len() {
				if !uni(xtargs.At(i), ytargs.At(i)) {
					return false // mismatched type args
				}
			}
			return true

		case *types.Pointer:
			y := y.(*types.Pointer)
			return uni(x.Elem(), y.Elem())

		case *types.Signature:
			y := y.(*types.Signature)
			return x.Variadic() == y.Variadic() &&
				uni(x.Params(), y.Params()) &&
				uni(x.Results(), y.Results())

		case *types.Slice:
			y := y.(*types.Slice)
			return uni(x.Elem(), y.Elem())

		case *types.Struct:
			y := y.(*types.Struct)
			if x.NumFields() != y.NumFields() {
				return false
			}
			for i := range x.NumFields() {
				xf := x.Field(i)
				yf := y.Field(i)
				if xf.Embedded() != yf.Embedded() ||
					xf.Name() != yf.Name() ||
					x.Tag(i) != y.Tag(i) ||
					!xf.Exported() && xf.Pkg() != yf.Pkg() ||
					!uni(xf.Type(), yf.Type()) {
					return false
				}
			}
			return true

		case *types.Tuple:
			y := y.(*types.Tuple)
			if x.Len() != y.Len() {
				return false
			}
			for i := range x.Len() {
				if !uni(x.At(i).Type(), y.At(i).Type()) {
					return false
				}
			}
			return true

		default: // incl. *Union, *TypeParam
			panic(fmt.Sprintf("unexpected Type %#v", x))
		}
	}

	if !uni(x, y) {
		clear(unifier)
		return false
	}

	// Populate the input map with the resulting types.
	if unifier != nil {
		for tparam, tptr := range bindings {
			unifier[tparam] = *tptr
		}
	}
	return true
}

// typeParams yields all the free type parameters within t that are relevant for
// unification.
//
// Note: this function is tailored for the specific needs of the unification algorithm.
// Don't try to use it for other purposes, see [typeparams.Free] instead.
func typeParams(t types.Type) iter.Seq[*types.TypeParam] {
	return func(yield func(*types.TypeParam) bool) {
		seen := map[*types.TypeParam]bool{} // yield each type param only once

		// tps(t) yields each TypeParam in t and returns false to stop.
		var tps func(types.Type) bool
		tps = func(t types.Type) bool {
			t = types.Unalias(t)

			switch t := t.(type) {
			case *types.TypeParam:
				if seen[t] {
					return true
				}
				seen[t] = true
				return yield(t)

			case *types.Basic:
				return true

			case *types.Array:
				return tps(t.Elem())

			case *types.Chan:
				return tps(t.Elem())

			case *types.Interface:
				// TODO(jba): implement.
				return true

			case *types.Map:
				return tps(t.Key()) && tps(t.Elem())

			case *types.Named:
				if t.Origin() == t {
					// generic type: look at type params
					return every(t.TypeParams().TypeParams(),
						func(tp *types.TypeParam) bool { return tps(tp) })
				}
				// instantiated type: look at type args
				return every(t.TypeArgs().Types(), tps)

			case *types.Pointer:
				return tps(t.Elem())

			case *types.Signature:
				return tps(t.Params()) && tps(t.Results())

			case *types.Slice:
				return tps(t.Elem())

			case *types.Struct:
				return every(t.Fields(),
					func(v *types.Var) bool { return tps(v.Type()) })

			case *types.Tuple:
				return every(t.Variables(),
					func(v *types.Var) bool { return tps(v.Type()) })

			default: // incl. *Union
				panic(fmt.Sprintf("unexpected Type %#v", t))
			}
		}

		tps(t)
	}
}

// every reports whether every pred(t) for t in seq returns true,
// stopping at the first false element.
func every[T any](seq iter.Seq[T], pred func(T) bool) bool {
	for t := range seq {
		if !pred(t) {
			return false
		}
	}
	return true
}
