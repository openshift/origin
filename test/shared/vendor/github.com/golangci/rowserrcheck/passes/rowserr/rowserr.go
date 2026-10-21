package rowserr

import (
	"go/ast"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

const (
	errMethod = "Err"
	rowsName  = "Rows"
)

func NewAnalyzer(sqlPkgs ...string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "rowserrcheck",
		Doc:  "Checks whether Rows.Err of rows is checked successfully.",
		Run:  NewRun(sqlPkgs...),
		Requires: []*analysis.Analyzer{
			buildssa.Analyzer,
		},
	}
}

type runner struct {
	rowsTyp       *types.Pointer
	rowsInterface *types.Interface
	rowsObj       types.Object
	skipFile      map[*ast.File]bool
}

func NewRun(pkgs ...string) func(pass *analysis.Pass) (any, error) {
	sqlPkgs := slices.Concat(pkgs, []string{"database/sql"})

	return func(pass *analysis.Pass) (any, error) {
		for _, pkg := range sqlPkgs {
			r := new(runner)
			r.run(pass, pkg)
		}

		return nil, nil
	}
}

// run executes an analysis for the pass. The receiver is passed
// by value because this func is called in parallel for different passes.
func (r *runner) run(pass *analysis.Pass, pkgPath string) {
	pssa := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	funcs := pssa.SrcFuncs

	pkg := pssa.Pkg.Prog.ImportedPackage(pkgPath)
	if pkg == nil {
		// skip
		return
	}

	rowsType := pkg.Type(rowsName)
	if rowsType == nil {
		// skip checking
		return
	}

	r.rowsObj = rowsType.Object()
	if r.rowsObj == nil {
		// skip checking
		return
	}

	resNamed, ok := r.rowsObj.Type().(*types.Named)
	if !ok {
		return
	}

	rowsInterface, ok := r.rowsObj.Type().Underlying().(*types.Interface)
	if ok {
		r.rowsInterface = rowsInterface
	}

	r.rowsTyp = types.NewPointer(resNamed)
	r.skipFile = map[*ast.File]bool{}

	for _, f := range funcs {
		// skip if the function is just referenced
		var isRefFunc bool

		for v := range f.Signature.Results().Variables() {
			if types.Identical(v.Type(), r.rowsTyp) {
				isRefFunc = true
			}
		}

		if isRefFunc {
			continue
		}

		for _, b := range f.Blocks {
			for i := range b.Instrs {
				if r.errCallMissing(b, i) {
					pass.Reportf(b.Instrs[i].Pos(), "rows.Err must be checked")
				}
			}
		}
	}
}

func (r *runner) errCallMissing(b *ssa.BasicBlock, i int) (ret bool) {
	call, ok := r.getCallReturnsRow(b.Instrs[i])
	if !ok {
		return false
	}

	if call.Referrers() == nil {
		return false
	}

	for _, cRef := range *call.Referrers() {
		val, ok := r.getRowsVal(cRef)
		if !ok {
			continue
		}

		if val.Referrers() == nil {
			return false
		}

		resRefs := *val.Referrers()

		if len(resRefs) == 0 {
			continue
		}

		if slices.ContainsFunc(resRefs, r.isErrCalled) {
			return false
		}
	}

	return true
}

func (r *runner) getCallReturnsRow(instr ssa.Instruction) (*ssa.Call, bool) {
	call, ok := instr.(*ssa.Call)
	if !ok {
		return nil, false
	}

	res := call.Call.Signature().Results()

	for v := range res.Variables() {
		typeToCheck := v.Type()

		if types.Identical(typeToCheck, r.rowsTyp) {
			return call, true
		}

		if r.rowsInterface != nil && types.Implements(typeToCheck, r.rowsInterface) {
			return call, true
		}
	}

	return nil, false
}

func (r *runner) getRowsVal(instr ssa.Instruction) (ssa.Value, bool) {
	switch instr := instr.(type) {
	case *ssa.Call:
		if len(instr.Call.Args) != 1 {
			return nil, false
		}

		if types.Identical(instr.Call.Args[0].Type(), r.rowsTyp) {
			return instr.Call.Args[0], true
		}

		if r.rowsInterface != nil && types.Implements(instr.Call.Args[0].Type(), r.rowsInterface) {
			return instr.Call.Args[0], true
		}

	case ssa.Value:
		if types.Identical(instr.Type(), r.rowsTyp) {
			return instr, true
		}

		if r.rowsInterface != nil && types.Implements(instr.Type(), r.rowsInterface) {
			return instr, true
		}
	}

	return nil, false
}

func (r *runner) getBodyOp(instr ssa.Instruction) (*ssa.UnOp, bool) {
	op, ok := instr.(*ssa.UnOp)
	if !ok {
		return nil, false
	}
	// fix: try to check type
	// if op.Type() != r.rowsObj.Type() {
	// 	return nil, false
	// }
	return op, true
}

func (r *runner) isErrCalled(inst ssa.Instruction) bool {
	return r.isErrCalledVisited(inst, map[ssa.Instruction]bool{})
}

func (r *runner) isErrCalledVisited(inst ssa.Instruction, visited map[ssa.Instruction]bool) bool {
	if visited[inst] {
		return false
	}

	visited[inst] = true

	isErrCalled := func(i ssa.Instruction) bool {
		return r.isErrCalledVisited(i, visited)
	}

	switch resRef := inst.(type) {
	case *ssa.Phi:
		if resRef.Referrers() == nil {
			return false
		}

		if slices.ContainsFunc(*resRef.Referrers(), isErrCalled) {
			return true
		}

	case *ssa.Store: // Call in Closure function
		if resRef.Addr.Referrers() == nil {
			return false
		}

		for _, aref := range *resRef.Addr.Referrers() {
			switch c := aref.(type) {
			case *ssa.MakeClosure:
				f := c.Fn.(*ssa.Function)

				called := r.isClosureCalled(c)
				if r.calledInFunc(f, called) {
					return true
				}
			case *ssa.UnOp:
				if c.Referrers() == nil {
					continue
				}

				if slices.ContainsFunc(*c.Referrers(), isErrCalled) {
					return true
				}
			}
		}

	case *ssa.Call: // Indirect function call
		if r.isErrInstruction(resRef) {
			return true
		}

		if f, ok := resRef.Call.Value.(*ssa.Function); ok {
			for _, b := range f.Blocks {
				for i := range b.Instrs {
					if !r.errCallMissing(b, i) {
						return true
					}
				}
			}
		}

	case *ssa.FieldAddr:
		if resRef.Referrers() == nil {
			return false
		}

		for _, bRef := range *resRef.Referrers() {
			bOp, ok := r.getBodyOp(bRef)
			if !ok {
				continue
			}

			if bOp.Referrers() == nil {
				continue
			}

			if slices.ContainsFunc(*bOp.Referrers(), r.isErrInstruction) {
				return true
			}
		}
	}

	return false
}

func (r *runner) isErrInstruction(inst ssa.Instruction) bool {
	switch ccall := inst.(type) {
	case *ssa.Defer:
		return r.isErrCall(ccall.Call)
	case *ssa.Call:
		return r.isErrCall(ccall.Call)
	default:
		return false
	}
}

func (r *runner) isErrCall(call ssa.CallCommon) bool {
	if call.Value != nil && call.Value.Name() == errMethod {
		return true
	}

	if call.Method != nil && call.Method.Name() == errMethod {
		return true
	}

	return false
}

func (r *runner) isClosureCalled(c *ssa.MakeClosure) bool {
	if c.Referrers() == nil {
		return false
	}

	for _, ref := range *c.Referrers() {
		switch ref.(type) {
		case *ssa.Call, *ssa.Defer:
			return true
		}
	}

	return false
}

func (r *runner) calledInFunc(f *ssa.Function, called bool) bool {
	for _, b := range f.Blocks {
		for i, instr := range b.Instrs {
			switch instr := instr.(type) {
			case *ssa.UnOp:
				if instr.Referrers() == nil {
					continue
				}

				for _, ref := range *instr.Referrers() {
					v, ok := ref.(ssa.Value)
					if !ok {
						continue
					}

					vCall, ok := v.(*ssa.Call)
					if !ok {
						continue
					}

					if vCall.Call.Value == nil || vCall.Call.Value.Name() != errMethod {
						continue
					}

					if called {
						return true
					}
				}

			default:
				if r.errCallMissing(b, i) || !called {
					return false
				}
			}
		}
	}

	return false
}
