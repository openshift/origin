// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

// This file defines utilities for population of method sets.

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/tools/internal/typesinternal"
)

// MethodValue returns the Function implementing method sel, building
// wrapper methods on demand. It returns nil if sel denotes an
// interface or generic method, or a method of a generic type.
//
// Precondition: sel.Kind() == MethodVal.
//
// Thread-safe.
//
// Acquires prog.methodsMu.
func (prog *Program) MethodValue(sel *types.Selection) *Function {
	if sel.Kind() != types.MethodVal {
		panic(fmt.Sprintf("MethodValue(%s) kind != MethodVal", sel))
	}

	method := sel.Obj().(*types.Func)
	if method.Signature().TypeParams().Len() > 0 {
		return nil // generic method
	}

	T := sel.Recv()
	if types.IsInterface(T) {
		return nil // interface method or type parameter
	}

	// We can avoid materializing sel.Type(): it will be parameterized iff
	// the receiver type T is too (see go.dev/issue/81308).
	if prog.isParameterized(T) {
		return nil // method on generic type
	}

	if prog.mode&LogSource != 0 {
		defer logStack("MethodValue %s %v", T, sel)()
	}

	key := methodKeyOf(method.Pkg(), method.Name())

	// The critical section returns a builder only if it created a
	// new Function. In the common case, a method created by an
	// earlier call, it allocates nothing: a builder declared out
	// here would escape to the heap on every call.
	fn, b := func() (*Function, *builder) {
		prog.methodsMu.Lock()
		defer prog.methodsMu.Unlock()

		// Get or create SSA method set.
		mset, ok := prog.methodSets.At(T).(*methodSet)
		if !ok {
			mset = &methodSet{mapping: make(map[methodKey]*Function)}
			prog.methodSets.Set(T, mset)
		}

		// Get or create SSA method.
		if fn, ok := mset.mapping[key]; ok {
			return fn, nil
		}
		b := new(builder)
		var fn *Function
		needsPromotion := len(sel.Index()) > 1
		needsIndirection := !isPointer(recvType(method)) && isPointer(T)
		if needsPromotion || needsIndirection {
			fn = createWrapper(prog, toSelection(sel), nil)
			fn.buildshared = b.shared()
			b.enqueue(fn)
		} else {
			fn = prog.objectMethod(method, nil, b)
		}
		if fn.Signature.Recv() == nil {
			panic(fn)
		}
		mset.mapping[key] = fn
		return fn, b
	}()

	if b != nil {
		b.iterate()
	} else if !fn.buildshared.isTransitivelyDone() {
		// fn was created by an earlier call and another builder
		// is still building it: wait for that to finish.
		var b builder
		b.waitForSharedFunction(fn)
		b.iterate()
	}

	return fn
}

// objectMethod returns the Function for a given method symbol.
// The symbol may be an instance of a generic function. It need not
// belong to an existing SSA package created by a call to
// prog.CreatePackage.
//
// objectMethod panics if the function is not a method.
//
// Acquires prog.objectMethodsMu.
func (prog *Program) objectMethod(obj *types.Func, targs []types.Type, b *builder) *Function {
	sig := obj.Type().(*types.Signature)
	if sig.Recv() == nil {
		panic("not a method: " + obj.String())
	}

	// Instantiation of generic?
	if orig := obj.Origin(); orig != obj || len(targs) > 0 {
		return prog.objectMethod(orig, nil, b).instance(receiverTypeArgs(obj), targs, b)
	}

	// Belongs to a created package?
	if fn := prog.FuncValue(obj); fn != nil {
		return fn
	}

	// Consult/update cache of methods created from types.Func.
	prog.objectMethodsMu.Lock()
	defer prog.objectMethodsMu.Unlock()
	fn, ok := prog.objectMethods[obj]
	if !ok {
		fn = createFunction(prog, obj, obj.Name(), nil, nil, "")
		fn.Synthetic = "from type information (on demand)"
		fn.buildshared = b.shared()
		b.enqueue(fn)

		if prog.objectMethods == nil {
			prog.objectMethods = make(map[*types.Func]*Function)
		}
		prog.objectMethods[obj] = fn
	} else {
		b.waitForSharedFunction(fn)
	}
	return fn
}

// LookupMethod returns the implementation of the method of type T
// identified by (pkg, name).  It returns nil if the method exists but
// is an interface method or generic method, and panics if T has no such method.
func (prog *Program) LookupMethod(T types.Type, pkg *types.Package, name string) *Function {
	// Fast path: the method was created and built by an earlier
	// call. RTA calls LookupMethod once per (call site, concrete
	// type) pair, so this is the common case; it avoids computing
	// the method set of T and searching it, which dominates the
	// cost of the slow path below.
	if fn := prog.existingMethod(T, methodKeyOf(pkg, name)); fn != nil {
		return fn
	}

	sel := prog.MethodSets.MethodSet(T).Lookup(pkg, name)
	if sel == nil {
		panic(fmt.Sprintf("%s has no method %s", T, types.Id(pkg, name)))
	}
	return prog.MethodValue(sel)
}

// existingMethod returns the Function implementing the method of
// the concrete type T identified by key, if it has already been
// created and built, or nil. A recorded method implies that T was
// found to be concrete and non-parameterized and the method
// non-generic when it was created.
//
// Acquires prog.methodsMu.
func (prog *Program) existingMethod(T types.Type, key methodKey) *Function {
	prog.methodsMu.Lock()
	defer prog.methodsMu.Unlock()
	if mset, ok := prog.methodSets.At(T).(*methodSet); ok {
		if fn := mset.mapping[key]; fn != nil && fn.buildshared.isTransitivelyDone() {
			return fn
		}
	}
	return nil
}

// methodSet contains the (concrete) methods of a concrete type (non-interface, non-parameterized).
type methodSet struct {
	mapping map[methodKey]*Function // populated lazily
}

// methodKey identifies a method within a method set the same way
// types.Id does, by name for exported methods and by package path
// and name for unexported ones, without building a string.
type methodKey struct {
	path string // package path, or "" if name is exported
	name string
}

func methodKeyOf(pkg *types.Package, name string) methodKey {
	if token.IsExported(name) {
		return methodKey{"", name}
	}
	path := "_" // as types.Id does when pkg is nil
	if pkg != nil {
		path = pkg.Path()
	}
	return methodKey{path, name}
}

// RuntimeTypes returns a new unordered slice containing all types in
// the program for which a runtime type is required.
//
// A runtime type is required for any non-parameterized, non-interface
// type that is converted to an interface, or for any type (including
// interface types) derivable from one through reflection.
//
// The methods of such types may be reachable through reflection or
// interface calls even if they are never called directly.
//
// Thread-safe.
//
// Acquires prog.makeInterfaceTypesMu.
func (prog *Program) RuntimeTypes() []types.Type {
	prog.makeInterfaceTypesMu.Lock()
	defer prog.makeInterfaceTypesMu.Unlock()

	// Compute the derived types on demand, since many SSA clients
	// never call RuntimeTypes, and those that do typically call
	// it once (often within ssautil.AllFunctions, which will
	// eventually not use it; see Go issue #69291.) This
	// eliminates the need to eagerly compute all the element
	// types during SSA building.
	var runtimeTypes []types.Type
	var set typeutil.Map // for de-duping identical types
	for t := range prog.makeInterfaceTypes {
		typesinternal.ForEachElement(prog.MethodSets.MethodSet, t, func(t types.Type, access bool) bool {
			if !access {
				return false // inaccessible to reflection
			}
			seen, _ := set.Set(t, true).(bool)
			if !seen {
				runtimeTypes = append(runtimeTypes, t)
			}
			return seen
		})
	}

	return runtimeTypes
}
