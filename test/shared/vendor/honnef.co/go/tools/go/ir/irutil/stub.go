package irutil

import (
	"honnef.co/go/tools/go/ir"
)

// IsStub reports whether a function is a stub. A function is
// considered a stub if it has no instructions or if all it does is
// return a fixed value (either an actual constant or a new allocation).
func IsStub(fn *ir.Function) bool {
	if fn.Source() == nil {
		// External functions have to be assumed to not be stubs.
		return false
	}

	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch instr.(type) {
			case *ir.Panic:
				// panic is a stub if it only uses constants
			case *ir.Return:
				// return is a stub if it only uses constants
			case *ir.Jump:
				// if there are no disallowed instructions, then we're
				// only jumping to the exit block (or possibly
				// somewhere else that's stubby?)
			case *ir.MakeInterface:
				// this can only be wrapping Const and HeapAlloc, which is fine.
			case *ir.Alloc:
				// allocating is fine as long as all we do with the allocation
				// is return it (possibly in an interface value)
			default:
				// all other instructions are assumed to do actual work
				return false
			}
		}
	}
	return true
}

// IsTrivial reports whether a function is trivial. A function is
// considered trivial if it is a stub, only returns a global variable, or calls
// another trivial function.
func IsTrivial(fn *ir.Function) bool {
	return isTrivial(fn, nil)
}

func isTrivial(fn *ir.Function, seen map[*ir.Function]struct{}) bool {
	if fn == nil || fn.Source() == nil {
		// External functions have to be assumed to be nontrivial.
		return false
	}

	if _, ok := seen[fn]; ok {
		// Mutual recursion is not trivial
		return false
	}

	// We delay adding to seen until a call, to avoid creating garbage in the
	// common case.
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			switch instr := instr.(type) {
			case *ir.Panic:
				// panic is a stub if it only uses constants
			case *ir.Return:
				// return is a stub if it only uses constants
			case *ir.Jump:
				// if there are no disallowed instructions, then we're
				// only jumping to the exit block (or possibly
				// somewhere else that's stubby?)
			case *ir.MakeInterface:
				// this can only be wrapping allowed instructions
			case *ir.Alloc:
				// allocating is fine as long as all we do with the allocation
				// is return it (possibly in an interface value)
			case *ir.Load:
				if _, ok := instr.X.(*ir.Global); !ok {
					return false
				}
			case *ir.Call:
				if seen == nil {
					seen = make(map[*ir.Function]struct{})
				}
				seen[fn] = struct{}{}
				callee := instr.Call.StaticCallee()
				if callee == nil || !isTrivial(callee, seen) {
					return false
				}
			default:
				// all other instructions are assumed to do actual work
				return false
			}
		}
	}
	return true
}
