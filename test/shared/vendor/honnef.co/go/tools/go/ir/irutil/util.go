package irutil

import (
	"go/types"
	"slices"
	"strings"

	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/types/typeutil"
)

func Reachable(from, to *ir.BasicBlock) bool {
	if from == to {
		return true
	}
	if from.Dominates(to) {
		return true
	}

	found := false
	Walk(from, func(b *ir.BasicBlock) bool {
		if b == to {
			found = true
			return false
		}
		return true
	})
	return found
}

func Walk(b *ir.BasicBlock, fn func(*ir.BasicBlock) bool) {
	seen := map[*ir.BasicBlock]bool{}
	wl := []*ir.BasicBlock{b}
	for len(wl) > 0 {
		b := wl[len(wl)-1]
		wl = wl[:len(wl)-1]
		if seen[b] {
			continue
		}
		seen[b] = true
		if !fn(b) {
			continue
		}
		wl = append(wl, b.Succs...)
	}
}

func Vararg(x *ir.Slice) ([]ir.Value, bool) {
	var out []ir.Value
	alloc, ok := x.X.(*ir.Alloc)
	if !ok {
		return nil, false
	}
	for _, ref := range *alloc.Referrers() {
		if ref == x {
			continue
		}
		if ref.Block() != x.Block() {
			return nil, false
		}
		switch ref := ref.(type) {
		case *ir.IndexAddr:
			idx := ref
			if len(*idx.Referrers()) != 1 {
				return nil, false
			}
			store, ok := (*idx.Referrers())[0].(*ir.Store)
			if !ok {
				return nil, false
			}
			out = append(out, store.Val)
		default:
			return nil, false
		}
	}
	return out, true
}

func CallName(call *ir.CallCommon) string {
	if call.IsInvoke() {
		return ""
	}
	switch v := call.Value.(type) {
	case *ir.Function:
		fn, ok := v.Object().(*types.Func)
		if !ok {
			return ""
		}
		return typeutil.FuncName(fn)
	case *ir.Builtin:
		return v.Name()
	}
	return ""
}

func IsCallTo(call *ir.CallCommon, name string) bool { return CallName(call) == name }

func IsCallToAny(call *ir.CallCommon, names ...string) bool {
	q := CallName(call)
	return slices.Contains(names, q)
}

func IsExample(fn *ir.Function) bool {
	if !strings.HasPrefix(fn.Name(), "Example") {
		return false
	}
	f := fn.Prog.Fset.File(fn.Pos())
	if f == nil {
		return false
	}
	return strings.HasSuffix(f.Name(), "_test.go")
}

// Flatten recursively returns the underlying value of an
// ir.Phi node. If all edges in an ir.Phi node are the same (after
// flattening), the flattened edge will get returned. If flattening is
// not possible, nil is returned.
func Flatten(v ir.Value) ir.Value {
	failed := false
	seen := map[ir.Value]struct{}{}
	var out ir.Value
	var dfs func(v ir.Value)
	dfs = func(v ir.Value) {
		if failed {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}

		switch v := v.(type) {
		case *ir.Phi:
			for _, e := range v.Edges {
				dfs(e)
			}
		default:
			if out == nil {
				out = v
			} else if out != v {
				failed = true
			}
		}
	}
	dfs(v)

	if failed {
		return nil
	}
	return out
}
