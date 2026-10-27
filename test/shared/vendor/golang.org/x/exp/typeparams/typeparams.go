// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package typeparams

import (
	"go/ast"
	"go/types"
)

// IndexListExpr is an alias for ast.IndexListExpr.
//
// Deprecated: Use [go/ast.IndexListExpr] instead.
//
//go:fix inline
type IndexListExpr = ast.IndexListExpr

// ForTypeSpec returns n.TypeParams.
func ForTypeSpec(n *ast.TypeSpec) *ast.FieldList {
	if n == nil {
		return nil
	}
	return n.TypeParams
}

// ForFuncType returns n.TypeParams.
func ForFuncType(n *ast.FuncType) *ast.FieldList {
	if n == nil {
		return nil
	}
	return n.TypeParams
}

// TypeParam is an alias for types.TypeParam
//
// Deprecated: Use [go/types.TypeParam] instead.
//
//go:fix inline
type TypeParam = types.TypeParam

// TypeParamList is an alias for types.TypeParamList
//
// Deprecated: Use [go/types.TypeParamList] instead.
//
//go:fix inline
type TypeParamList = types.TypeParamList

// TypeList is an alias for types.TypeList
//
// Deprecated: Use [go/types.TypeList] instead.
//
//go:fix inline
type TypeList = types.TypeList

// NewTypeParam calls types.NewTypeParam.
//
// Deprecated: Use [go/types.NewTypeParam] instead.
//
//go:fix inline
func NewTypeParam(name *types.TypeName, constraint types.Type) *TypeParam {
	return types.NewTypeParam(name, constraint)
}

// NewSignatureType calls types.NewSignatureType.
//
// Deprecated: Use [go/types.NewSignatureType] instead.
//
//go:fix inline
func NewSignatureType(recv *types.Var, recvTypeParams, typeParams []*TypeParam, params, results *types.Tuple, variadic bool) *types.Signature {
	return types.NewSignatureType(recv, recvTypeParams, typeParams, params, results, variadic)
}

// ForSignature returns sig.TypeParams()
//
// Deprecated: Use sig.TypeParams() instead.
//
//go:fix inline
func ForSignature(sig *types.Signature) *TypeParamList {
	return sig.TypeParams()
}

// RecvTypeParams returns sig.RecvTypeParams().
//
// Deprecated: Use sig.RecvTypeParams() instead.
//
//go:fix inline
func RecvTypeParams(sig *types.Signature) *TypeParamList {
	return sig.RecvTypeParams()
}

// IsComparable calls iface.IsComparable().
//
// Deprecated: Use iface.IsComparable() instead.
//
//go:fix inline
func IsComparable(iface *types.Interface) bool {
	return iface.IsComparable()
}

// IsMethodSet calls iface.IsMethodSet().
//
// Deprecated: Use iface.IsMethodSet() instead.
//
//go:fix inline
func IsMethodSet(iface *types.Interface) bool {
	return iface.IsMethodSet()
}

// IsImplicit calls iface.IsImplicit().
//
// Deprecated: Use iface.IsImplicit() instead.
//
//go:fix inline
func IsImplicit(iface *types.Interface) bool {
	return iface.IsImplicit()
}

// MarkImplicit calls iface.MarkImplicit().
//
// Deprecated: Use iface.MarkImplicit() instead.
//
//go:fix inline
func MarkImplicit(iface *types.Interface) {
	iface.MarkImplicit()
}

// ForNamed extracts the (possibly empty) type parameter object list from
// named.
//
// Deprecated: Use named.TypeParams() instead.
//
//go:fix inline
func ForNamed(named *types.Named) *TypeParamList {
	return named.TypeParams()
}

// SetForNamed sets the type params tparams on n. Each tparam must be of
// dynamic type *types.TypeParam.
//
// Deprecated: Use n.SetTypeParams(...) instead.
//
//go:fix inline
func SetForNamed(n *types.Named, tparams []*TypeParam) {
	n.SetTypeParams(tparams)
}

// NamedTypeArgs returns named.TypeArgs().
//
// Deprecated: Use named.TypeArgs() instead.
//
//go:fix inline
func NamedTypeArgs(named *types.Named) *TypeList {
	return named.TypeArgs()
}

// NamedTypeOrigin returns named.Orig().
//
// Deprecated: Use named.Origin() instead.
//
//go:fix inline
func NamedTypeOrigin(named *types.Named) types.Type {
	return named.Origin()
}

// Term is an alias for types.Term.
//
// Deprecated: Use [go/types.Term] instead.
//
//go:fix inline
type Term = types.Term

// NewTerm calls types.NewTerm.
//
// Deprecated: Use [go/types.NewTerm] instead.
//
//go:fix inline
func NewTerm(tilde bool, typ types.Type) *Term {
	return types.NewTerm(tilde, typ)
}

// Union is an alias for types.Union
//
// Deprecated: Use [go/types.Union] instead.
//
//go:fix inline
type Union = types.Union

// NewUnion calls types.NewUnion.
//
// Deprecated: Use [go/types.NewUnion] instead.
//
//go:fix inline
func NewUnion(terms []*Term) *Union {
	return types.NewUnion(terms)
}

// InitInstances initializes info to record information about type and function
// instances.
//
// Deprecated: Use info.Instances = make(...) instead.
//
//go:fix inline
func InitInstances(info *types.Info) {
	info.Instances = make(map[*ast.Ident]types.Instance)
}

// Instance is an alias for types.Instance.
//
// Deprecated: Use [go/types.Instance] instead.
//
//go:fix inline
type Instance = types.Instance

// GetInstances returns info.Instances.
//
// Deprecated: Use info.Instances instead.
//
//go:fix inline
func GetInstances(info *types.Info) map[*ast.Ident]Instance {
	return info.Instances
}

// Context is an alias for types.Context.
//
// Deprecated: Use [go/types.Context] instead.
//
//go:fix inline
type Context = types.Context

// NewContext calls types.NewContext.
//
// Deprecated: Use [go/types.NewContext] instead.
//
//go:fix inline
func NewContext() *Context {
	return types.NewContext()
}

// Instantiate calls types.Instantiate.
//
// Deprecated: Use [go/types.Instantiate] instead.
//
//go:fix inline
func Instantiate(ctxt *Context, typ types.Type, targs []types.Type, validate bool) (types.Type, error) {
	return types.Instantiate(ctxt, typ, targs, validate)
}
