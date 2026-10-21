package nilness

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strings"

	"honnef.co/go/tools/analysis/dfa"
	"honnef.co/go/tools/analysis/dfa/dense"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/exp/typeparams"
	"golang.org/x/tools/go/analysis"
)

// TODO(dh): The analysis is currently entirely forward, which means that for
//
// 	x := s[:0]
// 	y := s[:1]
// 	z := s[:0]
//
// x will have MaybeNil at every program point and s will have MaybeNil before
// execution of y, even though executing y without panicing tells us that s has
// been non-nil for all 3 instructions.

type nilnessFact struct {
	Rets []ValueNilness
}

func (*nilnessFact) AFact() {}
func (fact *nilnessFact) String() string {
	return fmt.Sprintf("nilness: %v", fact.Rets)
}

type ValueNilness struct {
	// Undefined for non-interface values and dynamically nil interface values.
	// For interface values, whether the stored value may be nil.
	// Even when Outer == MaybeNil, Inner may still offer precise information
	// for the cases when Outer is dynamically not nil. For example, {NeverNil,
	// MaybeNil} states that the interface value might be nil, but if it isn't,
	// it will definitely contain a non-nil value.
	Inner Nilness
	// For non-interface values, whether the value may be nil.
	// For interface values, whether the interface value may be nil.
	Outer Nilness
}

type Result struct {
	m map[*types.Func][]ValueNilness
}

var Analysis = &analysis.Analyzer{
	Name:       "nilness",
	Doc:        "Annotates return values with their nilness",
	Run:        run,
	Requires:   []*analysis.Analyzer{buildir.Analyzer},
	FactTypes:  []analysis.Fact{(*nilnessFact)(nil)},
	ResultType: reflect.TypeFor[*Result](),
}

// Nilness returns nilness information for return value ret of fn.
func (r *Result) Nilness(fn *types.Func, ret int) ValueNilness {
	typ := fn.Type().(*types.Signature).Results().At(ret).Type()
	if !typeutil.MaybePointerLike(typ) {
		return ValueNilness{Outer: NeverNil}
	}
	if len(r.m[fn]) == 0 {
		return ValueNilness{Inner: MaybeNil, Outer: MaybeNil}
	}

	return normalize(r.m[fn][ret], typ)
}

func normalize(v ValueNilness, typ types.Type) ValueNilness {
	if v.Inner == 0 || !types.IsInterface(typ) {
		v.Inner = MaybeNil
	}
	if v.Outer == 0 {
		v.Outer = MaybeNil
	}
	return v
}

func run(pass *analysis.Pass) (any, error) {
	seen := map[*ir.Function]struct{}{}
	out := &Result{
		m: map[*types.Func][]ValueNilness{},
	}

	// TODO(dh): instead of recursion and giving up on mutual recursion, we
	// should compute the DFA over the call graph, at least until we have
	// proper function summaries.
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		impl(pass, fn, seen)
	}

	for _, fact := range pass.AllObjectFacts() {
		out.m[fact.Object.(*types.Func)] = fact.Fact.(*nilnessFact).Rets
	}

	return out, nil
}

type Nilness uint8

func (n Nilness) IsGlobal() bool {
	return n == AlwaysNilGlobal || n == MaybeNilGlobal
}

const (
	// The value is never nil.
	NeverNil Nilness = iota + 1
	// The value is always nil.
	AlwaysNil
	// The value might be nil, but only because of the value of a global
	// variable.
	MaybeNilGlobal
	// The value might be nil.
	MaybeNil
	// The value is always nil and loaded from a global
	AlwaysNilGlobal
)

func (n Nilness) String() string {
	switch n {
	case 0:
		return "NoNilness"
	case NeverNil:
		return "NeverNil"
	case AlwaysNil:
		return "AlwaysNil"
	case AlwaysNilGlobal:
		return "AlwaysNilGlobal"
	case MaybeNilGlobal:
		return "MaybeNilGlobal"
	case MaybeNil:
		return "MaybeNil"
	default:
		return "InvalidNilness"
	}
}

type state struct {
	cloned bool
	m      []ValueNilness
	n      numbering
}

func (s *state) get(v ir.Value) ValueNilness {
	if !typeutil.MaybePointerLike(v.Type()) {
		// All non-pointer-like types are always {_ NeverNil}.
		return ValueNilness{Outer: NeverNil}
	}
	num := s.n.number(v)
	if num < len(s.m) {
		return s.m[num]
	}

	switch v.(type) {
	case *ir.Parameter:
		return ValueNilness{Inner: MaybeNil, Outer: MaybeNil}
	case *ir.Builtin:
		return ValueNilness{Outer: NeverNil}
	case *ir.FreeVar:
		return ValueNilness{Inner: MaybeNil, Outer: MaybeNil}
	case *ir.Function:
		return ValueNilness{Outer: NeverNil}
	case *ir.Global:
		// Globals are addresses, not the values stored in them. The addresses
		// cannot be nil.
		return ValueNilness{Outer: NeverNil}
	}

	return lattice{}.Ident()
}

func (s *state) set(key ir.Value, value ValueNilness) {
	if !typeutil.MaybePointerLike(key.Type()) {
		// No point in recording state for non-pointer-like types. They're
		// always {_ NeverNil}.
		return
	}

	if value == (lattice{}.Ident()) {
		// No point in storing the default value.
		return
	}
	num := s.n.number(key)
	if !s.cloned {
		if num < len(s.m) && s.m[num] == value {
			// Don't clone if the value already matches.
			return
		}
		s.cloned = true
		s.m = slices.Clone(s.m)
	}
	if num >= len(s.m) {
		s.m = append(s.m, make([]ValueNilness, num-len(s.m)+1)...)
	}
	s.m[num] = value
}

func (s *state) setInner(key ir.Value, value Nilness) {
	if !typeutil.MaybePointerLike(key.Type()) {
		return
	}
	if value == (lattice{}.Ident().Inner) {
		return
	}
	num := s.n.number(key)
	if !s.cloned {
		if num < len(s.m) && s.m[num].Inner == value {
			// Don't clone if the value already matches.
			return
		}
		s.cloned = true
		s.m = slices.Clone(s.m)
	}
	if num >= len(s.m) {
		dflt := s.get(key)
		s.m = append(s.m, make([]ValueNilness, num-len(s.m)+1)...)
		s.m[num] = dflt
	}
	v := s.m[num]
	v.Inner = value
	s.m[num] = v
}

func (s *state) setOuter(key ir.Value, value Nilness) {
	if !typeutil.MaybePointerLike(key.Type()) {
		return
	}
	if value == (lattice{}).Ident().Outer {
		return
	}
	num := s.n.number(key)
	if !s.cloned {
		if num < len(s.m) && s.m[num].Outer == value {
			// Don't clone if the value already matches.
			return
		}
		s.cloned = true
		s.m = slices.Clone(s.m)
	}
	if num >= len(s.m) {
		dflt := s.get(key)
		s.m = append(s.m, make([]ValueNilness, num-len(s.m)+1)...)
		s.m[num] = dflt
	}
	v := s.m[num]
	v.Outer = value
	s.m[num] = v
}

func defaultNilnessForSignature(pass *analysis.Pass, typ *types.Signature) []ValueNilness {
	n := typ.Results().Len()
	if n == 0 {
		return nil
	}
	out := make([]ValueNilness, n)
	for i := range n {
		out[i] = defaultNilness(pass, typ.Results().At(i).Type())
	}
	return out
}

func defaultNilness(pass *analysis.Pass, typ types.Type) ValueNilness {
	if typeutil.MaybePointerLike(typ) {
		// IsPointerLike handles type parameters with type sets, too.
		return ValueNilness{MaybeNil, MaybeNil}
	} else {
		return ValueNilness{NeverNil, NeverNil}
	}
}

func impl(pass *analysis.Pass, fn *ir.Function, seenFns map[*ir.Function]struct{}) []ValueNilness {
	goto start
bailout:
	return defaultNilnessForSignature(pass, fn.Signature)

start:
	if fn.Signature.Results().Len() == 0 {
		return nil
	}
	if fn.Object() == nil {
		// TODO(dh): support closures
		goto bailout
	}
	if fact := new(nilnessFact); pass.ImportObjectFact(fn.Object(), fact) {
		return fact.Rets
	}
	if fn.Pkg != pass.ResultOf[buildir.Analyzer].(*buildir.IR).Pkg {
		goto bailout
	}
	if fn.Blocks == nil {
		goto bailout
	}
	if _, ok := seenFns[fn]; ok {
		// break recursion
		goto bailout
	}

	seenFns[fn] = struct{}{}

	anyPointers := false
	for ret := range fn.Signature.Results().Variables() {
		if typeutil.MaybePointerLike(ret.Type()) {
			anyPointers = true
			break
		}
	}

	if !anyPointers {
		goto bailout
	}

	n := numbering{}

	processBlock := func(from, to *ir.BasicBlock, s state) state {
		handleReturnValue := func(v ir.Value, call *ir.Call, idx int) {
			typ := call.Common().Signature().Results().At(idx).Type()
			if !typeutil.MaybePointerLike(typ) {
				s.setOuter(v, NeverNil)
				return
			}

			if callee, ok := call.Call.Value.(*ir.Builtin); ok {
				switch callee.Name() {
				case "append":
					// TODO(dh): if we knew that the varargs had non-zero
					// length, we'd know that the resulting slice is non-nil.
					switch an := s.get(call.Call.Args[0]).Outer; an {
					case MaybeNil, MaybeNilGlobal, NeverNil:
						s.setOuter(v, an)
					case AlwaysNil:
						s.setOuter(v, MaybeNil)
					case AlwaysNilGlobal:
						s.setOuter(v, MaybeNilGlobal)
					}
				case "UnsafeSlice":
					// If len is negative, or if ptr is nil and len is not
					// zero, unsafe.Slice panics. This implies that a non-nil
					// pointer cannot become nil, and vice versa.
					s.set(v, s.get(call.Call.Args[0]))
				case "UnsafeStringData":
					// TODO(dh): if we had string length information we could
					// return better information.
					s.setOuter(v, MaybeNil)
				case "UnsafeSliceData":
					// When the slice is non-nil but has zero capacity, the
					// returned pointer is still non-nil, so we don't have to
					// worry about that.
					s.set(v, s.get(call.Call.Args[0]))
				case "UnsafeAdd":
					// TODO(dh): a positive addend can never result in a nil pointer.

					// Pointer arithmetic can turn nil pointers into non-nil
					// ones and vice versa.
					s.setOuter(v, MaybeNil)
				case "ssa:deferstack":
					s.setOuter(v, NeverNil)
				case "ssa:wrapnilchk":
					s.setOuter(v, NeverNil)
				case "recover":
					s.setOuter(v, MaybeNil)
				default:
					panic(fmt.Sprintf("internal error: unhandled builtin %s", callee.Name()))
				}
				return
			}

			callee := call.Common().StaticCallee()
			if callee == nil {
				// We don't know which function is being called.
				s.set(v, ValueNilness{MaybeNil, MaybeNil})
				return
			}
			calleeNilness := impl(pass, callee, seenFns)
			if len(calleeNilness) > idx {
				s.set(v, normalize(calleeNilness[idx], typ))
			} else {
				s.set(v, ValueNilness{MaybeNil, MaybeNil})
			}
		}

		for _, instr := range from.Instrs {
			// It is tempting to return early when instr is an ir.Value that
			// doesn't have pointer type. However, instructions like ir.Load
			// tell us something about the value being operated on.

			switch v := instr.(type) {
			case *ir.Convert:
				s.set(v, s.get(v.X))
			case *ir.SliceToArrayPointer:
				// Go does not currently allow (*T)(s) where T is a type
				// parameter with a type set consisting of array types, but it
				// does allow (T)(s) where T is a type parameter with a type
				// set consisting of pointers to array types.

				allNonZero := typeutil.All(v.Type(), func(term *types.Term) bool {
					ptr := term.Type().Underlying().(*types.Pointer).Elem()
					return typeutil.All(ptr, func(innerTerm *types.Term) bool {
						return innerTerm.Type().Underlying().(*types.Array).Len() != 0
					})
				})

				if allNonZero {
					// converting a slice to an array pointer of length > 0
					// panics if the slice is nil
					s.setOuter(v, NeverNil)
					s.setOuter(v.X, NeverNil)
				} else {
					s.set(v, s.get(v.X))
				}
			case *ir.SliceToArray:
				// Pretty much the same logic as SliceToArrayPointer, minus the
				// pointer.

				allNonZero := typeutil.All(v.Type(), func(term *types.Term) bool {
					return term.Type().Underlying().(*types.Array).Len() != 0
				})

				if allNonZero {
					// converting a slice to an array of length > 0
					// panics if the slice is nil
					s.setOuter(v.X, NeverNil)
				}
			case *ir.Slice:
				if typeutil.All(v.X.Type(), typeutil.IsType[*types.Array]) {
					// Slicing arrays never results in a nil slice.
					s.setOuter(v, NeverNil)
					continue
				}

				// checkBound returns true if one of the bounds (low, high,
				// capacity) has non-zero value.
				checkBound := func(v ir.Value) bool {
					if v == nil {
						return false
					}
					// TODO(dh): this is where integration with constant
					// propagation and value range analysis would be useful.
					if k, ok := v.(*ir.Const); ok {
						kv, ok := constant.Int64Val(k.Value)
						return !ok || kv != 0
					}
					return false
				}
				if checkBound(v.Low) || checkBound(v.High) || checkBound(v.Max) {
					// One of the indices is non-zero, which means slicing can
					// only succeed if the slicee is not nil.
					s.setOuter(v, NeverNil)
					s.setOuter(v.X, NeverNil)
				} else {
					// The new slice is as nilly as the slicee.
					s.set(v, s.get(v.X))
				}

			case *ir.If:
				cond := v.Cond
				binop, ok := cond.(*ir.BinOp)
				if !ok {
					continue
				}
				isNil := func(v ir.Value) bool {
					k, ok := v.(*ir.Const)
					if !ok {
						return false
					}
					return k.IsNil()
				}
				var target ir.Value
				if isNil(binop.X) {
					target = binop.Y
				} else if isNil(binop.Y) {
					target = binop.X
				} else {
					continue
				}
				op := binop.Op
				if to != from.Succs[0] {
					// we're in the false branch, negate op
					switch op {
					case token.EQL:
						op = token.NEQ
					case token.NEQ:
						op = token.EQL
					default:
						panic(fmt.Sprintf("internal error: unhandled token %v", op))
					}
				}
				switch op {
				case token.EQL:
					if s.get(target).Outer.IsGlobal() {
						s.setOuter(target, AlwaysNilGlobal)
					} else {
						s.setOuter(target, AlwaysNil)
					}
				case token.NEQ:
					s.setOuter(target, NeverNil)
				default:
					panic(fmt.Sprintf("internal error: unhandled token %v", op))
				}

				// TODO(dh): also handle comparison of two non-nil values. The
				// true branch of neverNil == nilly makes the nilly value neverNil.
			case *ir.ChangeType:
				s.set(v, s.get(v.X))
			case *ir.MultiConvert:
				s.set(v, s.get(v.X))
			case *ir.Load:
				if _, ok := v.X.(*ir.Global); ok {
					s.set(v, ValueNilness{MaybeNilGlobal, MaybeNilGlobal})
				} else {
					s.set(v, ValueNilness{MaybeNil, MaybeNil})
				}
				s.setOuter(v.X, NeverNil)
			case *ir.FieldAddr:
				s.setOuter(v.X, NeverNil)
				s.setOuter(v, NeverNil)
			case *ir.IndexAddr:
				s.setOuter(v.X, NeverNil)
				s.setOuter(v, NeverNil)
			case *ir.Alloc, *ir.MakeMap, *ir.MakeSlice, *ir.MakeClosure, *ir.MakeChan:
				s.setOuter(v.(ir.Value), NeverNil)
			case *ir.MapUpdate:
				s.setOuter(v.Map, NeverNil)
			case *ir.Store:
				s.setOuter(v.Addr, NeverNil)
			case ir.CallInstruction:
				// go/defer/calling a nil function fatals/panics
				if !v.Common().IsInvoke() {
					s.setOuter(v.Common().Value, NeverNil)
				}
				_, ok := v.(*ir.Call)
				if !ok {
					// Defer and Go don't produce values
					continue
				}
				if v.Common().Signature().Results().Len() != 1 {
					// If the called function doesn't return any values then we
					// don't care about it. If it has more than one return
					// value, they'll be handled by Extract.
					continue
				}

				handleReturnValue(v.(ir.Value), v.(*ir.Call), 0)
			case *ir.Send:
				s.setOuter(v.Chan, NeverNil)
			case *ir.Recv:
				s.setOuter(v.Chan, NeverNil)
				s.set(v, ValueNilness{MaybeNil, MaybeNil})
			case *ir.MakeInterface:
				s.set(v, ValueNilness{
					Inner: s.get(v.X).Outer,
					Outer: NeverNil,
				})
			case *ir.ChangeInterface:
				s.set(v, s.get(v.X))
			case *ir.TypeAssert:
				if !v.CommaOk {
					// The interface value cannot have been nil, or the type
					// assertion would have panicked.
					s.setOuter(v.X, NeverNil)

					if types.IsInterface(v.Type()) && !typeparams.IsTypeParam(v.Type()) {
						// Type asserting to another interface doesn't succeed
						// if the assertee was nil. It also results in a new
						// interface value.
						s.setOuter(v, NeverNil)
						s.setInner(v, s.get(v.X).Inner)
					} else {
						// We've extracted the interface value's inner value.
						s.setOuter(v, s.get(v.X).Inner)
					}
				} else {
					// In a comma-ok type assertion, the return type is a
					// tuple. There'll be Extract instructions getting the
					// individual values, to which we'll attach the nilness
					// info.
				}
			case *ir.TypeSwitch:
				// Handled in Extract
			case *ir.MapLookup:
				if s.get(v.X).Outer == AlwaysNil {
					s.set(v, ValueNilness{AlwaysNil, AlwaysNil})
				} else if s.get(v.X).Outer == AlwaysNilGlobal {
					s.set(v, ValueNilness{AlwaysNilGlobal, AlwaysNilGlobal})
				} else {
					s.set(v, ValueNilness{MaybeNil, MaybeNil})
				}
			case *ir.Field:
				s.set(v.X, ValueNilness{NeverNil, NeverNil})
				s.set(v, ValueNilness{MaybeNil, MaybeNil})
			case *ir.Index:
				s.set(v.X, ValueNilness{NeverNil, NeverNil})
				s.set(v, ValueNilness{MaybeNil, MaybeNil})

			case *ir.Extract:
				switch tuple := v.Tuple.(type) {
				case *ir.TypeAssert:
					// When we get here, the type assertion used the comma-ok
					// form, and we don't yet know anything about the result of
					// the type assertion.
					if v.Index == 0 {
						s.set(v, ValueNilness{MaybeNil, MaybeNil})
					}

					// TODO(dh): We should set v's nilness in the true and
					// false branches of checks on the ok value. However, ok can be
					// used in arbitrary ways, and we're also not set up to handle
					// relational facts (ok being true or false affects the value
					// of the other Extract).

				case *ir.Call:
					handleReturnValue(v, tuple, v.Index)

				case *ir.TypeSwitch:
					if v.Index == 0 {
						// Index 0 is an integer and not interesting.
						continue
					}
					idx := v.Index - 1
					if idx >= len(tuple.Conds) {
						// Default branch

						// If there is an untyped nil case, then being in the
						// default branch tells us that the interface value
						// isn't nil.
						hasNil := slices.ContainsFunc(tuple.Conds, func(typ types.Type) bool {
							if typ, ok := typ.(*types.Basic); ok && typ.Kind() == types.UntypedNil {
								return true
							}
							return false
						})
						if hasNil {
							s.setOuter(tuple.Tag, NeverNil)
						} else {
							s.setOuter(tuple.Tag, MaybeNil)
						}
						s.setOuter(v, s.get(tuple.Tag).Inner)
					} else {
						// There is no Extract for the 'untyped nil' case,
						// which means that executing any Extract from a type
						// switch implies that the switched-over value wasn't a
						// nil interface value.
						s.setOuter(tuple.Tag, NeverNil)
						typ := tuple.Conds[idx]
						if types.IsInterface(typ) && !typeparams.IsTypeParam(typ) {
							// Succesfully type asserting to an interface type
							// always produces a non-nil interface value.
							s.setInner(v, s.get(tuple.Tag).Inner)
							s.setOuter(v, NeverNil)
						} else {
							s.setOuter(v, s.get(tuple.Tag).Inner)
						}
					}
				default:
					s.set(v, ValueNilness{MaybeNil, MaybeNil})
				}
			case *ir.Select:
				if v.Blocking && len(v.States) == 1 {
					// If the select doesn't have a default branch and only has
					// one state, that state's channel cannot have been nil if
					// we finished execution the select.
					s.setOuter(v.States[0].Chan, NeverNil)
				}
			case *ir.Jump, *ir.BlankStore, *ir.Phi,
				*ir.Panic, *ir.Return, *ir.RunDefers, *ir.Unreachable, *ir.ConstantSwitch,
				*ir.UnOp, *ir.BinOp, *ir.CompositeValue, *ir.Range, *ir.Next:
			default:
				posn := pass.Fset.PositionFor(v.Pos(), false)
				panic(fmt.Sprintf("internal error: unhandled type %T at %s", v, posn))
			}
		}
		return s
	}

	processPhis := func(b *ir.BasicBlock, i int, s state) state {
		for _, instr := range b.Instrs {
			if instr, ok := instr.(*ir.Phi); ok {
				s.set(instr, s.get(instr.Edges[i]))
			} else {
				break
			}
		}
		return s
	}

	// Populate default state for non-instruction values we encounter. We
	// cannot defer this logic to state.get because control flow merges use
	// simple merges of lattice values and won't know about value-specific
	// defaults.
	entrys := state{cloned: true, n: n}
	for _, param := range fn.Params {
		if typeutil.MaybePointerLike(param.Type()) {
			entrys.set(param, ValueNilness{Inner: MaybeNil, Outer: MaybeNil})
		} else {
			// We never track nilness for value types, so they don't have to be
			// present in the entry state, either.
		}
	}
	if strings.HasPrefix(fn.Synthetic, "bound method wrapper") {
		// This is a bound method and the bound receiver might be nil
		for _, fvar := range fn.FreeVars {
			entrys.set(fvar, ValueNilness{Outer: MaybeNil})
		}
	} else {
		// This is a closure, and closed over variables are allocs, which
		// cannot be nil.
		for _, fvar := range fn.FreeVars {
			entrys.set(fvar, ValueNilness{Outer: NeverNil})
		}
	}
	var ops []*ir.Value
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			ops = instr.Operands(ops[:0])
			for _, pop := range ops {
				if op, ok := (*pop).(*ir.Const); ok {
					if op.IsNil() {
						entrys.set(op, ValueNilness{Inner: AlwaysNil, Outer: AlwaysNil})
					} else if typeutil.MaybePointerLike(op.Type()) {
						entrys.set(op, ValueNilness{Inner: MaybeNil, Outer: MaybeNil})
					} else {
						entrys.set(op, ValueNilness{Inner: NeverNil, Outer: NeverNil})
					}
				}
			}
		}
	}
	res := dense.Forward[dfa.DenseMapLattice[ValueNilness, lattice]](
		fn,
		map[int][]ValueNilness{0: entrys.m},
		func(fromID, toID int, in []ValueNilness) []ValueNilness {
			from := fn.Blocks[fromID]
			to := fn.Blocks[toID]
			s := state{n: n, m: in}
			s = processBlock(from, to, s)
			i := slices.Index(to.Preds, from)
			s = processPhis(to, i, s)
			return s.m
		},
	)

	retNilness := make([]ValueNilness, fn.Signature.Results().Len())
	for b := range fn.Returns() {
		ret := b.Control().(*ir.Return)
		s := state{n: n, m: res.In(b.Index)}
		s = processBlock(b, nil, s)
		for i, res := range ret.Results {
			retNilness[i] = lattice{}.Merge(retNilness[i], s.get(res))
		}
	}

	interesting := false
	for i := range retNilness {
		typ := fn.Signature.Results().At(i).Type()
		if !typeutil.MaybePointerLike(typ) {
			retNilness[i] = ValueNilness{NeverNil, NeverNil}
			continue
		}
		retNilness[i] = normalize(retNilness[i], typ)
		if retNilness[i] != (ValueNilness{MaybeNil, MaybeNil}) {
			interesting = true
		}
	}

	if interesting {
		pass.ExportObjectFact(fn.Object(), &nilnessFact{retNilness})
	}

	return retNilness
}

type lattice struct{}

var _ dfa.Semilattice[ValueNilness] = lattice{}

// Equals implements [dfa.Semilattice].
func (l lattice) Equals(a, b ValueNilness) bool {
	return a == b
}

// Ident implements [dfa.Semilattice].
func (l lattice) Ident() ValueNilness {
	return ValueNilness{}
}

var latticeMerge = [6][6]Nilness{
	0: {
		0:               0,
		NeverNil:        NeverNil,
		AlwaysNil:       AlwaysNil,
		AlwaysNilGlobal: AlwaysNilGlobal,
		MaybeNilGlobal:  MaybeNilGlobal,
		MaybeNil:        MaybeNil,
	},
	NeverNil: {
		0:               NeverNil,
		NeverNil:        NeverNil,
		AlwaysNil:       MaybeNil,
		AlwaysNilGlobal: MaybeNilGlobal,
		MaybeNilGlobal:  MaybeNilGlobal,
		MaybeNil:        MaybeNil,
	},
	AlwaysNil: {
		0:               AlwaysNil,
		NeverNil:        MaybeNil,
		AlwaysNil:       AlwaysNil,
		AlwaysNilGlobal: AlwaysNil,
		MaybeNilGlobal:  MaybeNil,
		MaybeNil:        MaybeNil,
	},
	AlwaysNilGlobal: {
		0:               AlwaysNilGlobal,
		NeverNil:        MaybeNilGlobal,
		AlwaysNil:       AlwaysNil,
		AlwaysNilGlobal: AlwaysNilGlobal,
		MaybeNilGlobal:  MaybeNilGlobal,
		MaybeNil:        MaybeNil,
	},
	MaybeNilGlobal: {
		0:               MaybeNilGlobal,
		NeverNil:        MaybeNilGlobal,
		AlwaysNil:       MaybeNil,
		AlwaysNilGlobal: MaybeNilGlobal,
		MaybeNilGlobal:  MaybeNilGlobal,
		MaybeNil:        MaybeNil,
	},
	MaybeNil: {
		0:               MaybeNil,
		NeverNil:        MaybeNil,
		AlwaysNil:       MaybeNil,
		AlwaysNilGlobal: MaybeNil,
		MaybeNilGlobal:  MaybeNil,
		MaybeNil:        MaybeNil,
	},
}

// Merge implements [dfa.Semilattice].
func (l lattice) Merge(a, b ValueNilness) ValueNilness {
	o := latticeMerge[a.Outer][b.Outer]
	i := latticeMerge[a.Inner][b.Inner]

	// When an interface value is always nil, then there is no stored value and
	// Inner is technically undefined, regardless of what concrete nilness
	// value was assigned to it.
	//
	// For example, {AlwaysNil, AlwaysNil} ∨ {NeverNil, NeverNil} should
	// produce {NeverNil, MaybeNil} instead of {MaybeNil, MaybeNil}, to denote
	// that when the interface value isn't nil, the stored value is never nil.
	//
	// {AlwaysNilGlobal, _} v {AlwaysNil, _}, however, should result in
	// {AlwaysNil, _}, regardless of the outer value.
	if i != AlwaysNil && i != AlwaysNilGlobal {
		if a.Outer == AlwaysNil || a.Outer == AlwaysNilGlobal {
			i = b.Inner
		} else if b.Outer == AlwaysNil || b.Outer == AlwaysNilGlobal {
			i = a.Inner
		}
	}
	return ValueNilness{
		Inner: i,
		Outer: o,
	}
}

type numbering map[ir.Value]int

func (n numbering) number(v ir.Value) int {
	i, ok := n[v]
	if !ok {
		i = len(n)
		n[v] = i
	}
	return i
}
