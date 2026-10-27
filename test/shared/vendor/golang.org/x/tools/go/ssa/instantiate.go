// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ssa

import (
	"fmt"
	"go/types"
	"slices"
	"sync"
)

// A generic records information about a generic origin function,
// including a cache of existing instantiations.
type generic struct {
	instancesMu sync.Mutex
	instances   map[*typeList]*Function // canonical type arguments to an instance.
}

// instance returns a Function that is the instantiation of generic
// origin function fn with the type arguments rtargs and targs.
//
// Any created instance is added to cr.
//
// Acquires fn.generic.instancesMu.
func (fn *Function) instance(rtargs, targs []types.Type, b *builder) *Function {
	key := fn.Prog.canon.List(slices.Concat(rtargs, targs))

	gen := fn.generic

	gen.instancesMu.Lock()
	defer gen.instancesMu.Unlock()
	inst, ok := gen.instances[key]
	if !ok {
		inst = createInstance(fn, rtargs, targs)
		inst.buildshared = b.shared()
		b.enqueue(inst)

		if gen.instances == nil {
			gen.instances = make(map[*typeList]*Function)
		}
		gen.instances[key] = inst
	} else {
		b.waitForSharedFunction(inst)
	}
	return inst
}

// createInstance returns the instantiation of generic function fn using targs.
// If fn is a method on a generic type, fn's receiver type will be instantiated
// using rtargs.
//
// Requires fn.generic.instancesMu.
func createInstance(fn *Function, rtargs, targs []types.Type) *Function {
	prog := fn.Prog

	// Compute signature.
	var sig *types.Signature
	var obj *types.Func
	if recv := fn.Signature.Recv(); recv != nil {
		// method, len(rtargs) > 0 || len(targs) > 0
		if len(rtargs) > 0 {
			// possibly generic method on generic type
			obj = prog.canon.instantiateMethod(fn.object, rtargs, prog.ctxt)
		} else {
			// generic method on non-generic type
			obj = fn.object // instantiation does not exist yet
		}
		if len(targs) > 0 {
			// generic method
			instSig, err := types.Instantiate(prog.ctxt, obj.Signature(), targs, false)
			if err != nil {
				panic(err)
			}
			instance, ok := instSig.(*types.Signature)
			if !ok {
				panic("Instantiate of a Signature returned a non-signature")
			}
			// Do not canonicalize generic methods, because the receiver is not
			// part of a Signature's type identity. For example,
			// (*G[int]).m[int] and (G[int]).n[int] may be identical types even
			// though their receiver types differ.
			sig = instance
		} else {
			// non-generic method on generic type
			sig = obj.Signature()
		}
	} else {
		// function, len(rtargs) == 0 && len(targs) > 0
		instSig, err := types.Instantiate(prog.ctxt, fn.Signature, targs, false)
		if err != nil {
			panic(err)
		}
		instance, ok := instSig.(*types.Signature)
		if !ok {
			panic("Instantiate of a Signature returned a non-signature")
		}
		obj = fn.object // instantiation does not exist yet
		sig = prog.canon.Type(instance).(*types.Signature)
	}

	// Choose strategy (instance or wrapper).
	var (
		synthetic string
		subst     *subster
		build     buildFunc
	)
	if prog.mode&InstantiateGenerics != 0 && !prog.isParameterized(slices.Concat(rtargs, targs)...) {
		synthetic = fmt.Sprintf("instance of %s", fn.Name())
		if fn.syntax != nil {
			subst = makeSubster(prog.ctxt, obj, fn.recvtypeparams, rtargs, fn.typeparams, targs)
			build = (*builder).buildFromSyntax
		} else {
			build = (*builder).buildParamsOnly
		}
	} else {
		synthetic = fmt.Sprintf("instantiation wrapper of %s", fn.Name())
		build = (*builder).buildInstantiationWrapper
	}

	name := fn.Name()
	if len(targs) > 0 {
		name = fmt.Sprintf("%s%s", name, targstr(targs)) // may not be unique
	}

	/* generic instance or instantiation wrapper */
	return &Function{
		name:           name,
		object:         obj,
		Signature:      sig,
		Synthetic:      synthetic,
		syntax:         fn.syntax,    // \
		info:           fn.info,      //  } empty for non-created packages
		goversion:      fn.goversion, // /
		build:          build,
		topLevelOrigin: fn,
		pos:            obj.Pos(),
		Pkg:            nil,
		Prog:           fn.Prog,
		recvtypeparams: fn.recvtypeparams, // share with origin
		recvtypeargs:   rtargs,
		typeparams:     fn.typeparams, // share with origin
		typeargs:       targs,
		subst:          subst,
	}
}

// isParameterized reports whether any of the specified types contains
// a free type parameter. It is safe to call concurrently.
func (prog *Program) isParameterized(ts ...types.Type) bool {
	prog.hasParamsMu.Lock()
	defer prog.hasParamsMu.Unlock()

	// TODO(adonovan): profile. If this operation is expensive,
	// handle the most common but shallow cases such as T, pkg.T,
	// *T without consulting the cache under the lock.

	return slices.ContainsFunc(ts, prog.hasParams.Has)
}
