// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ir

// This file implements the Function type.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"iter"
	"os"
	"strings"
)

// Like ObjectOf, but panics instead of returning nil.
// Only valid during f's create and build phases.
func (f *Function) objectOf(id *ast.Ident) types.Object {
	if o := f.info.ObjectOf(id); o != nil {
		return o
	}
	panic(fmt.Sprintf("no types.Object for ast.Ident %s @ %s",
		id.Name, f.Prog.Fset.Position(id.Pos())))
}

// instrs returns an iterator that returns each reachable instruction of the SSA function.
func (f *Function) instrs() iter.Seq[Instruction] {
	return func(yield func(i Instruction) bool) {
		for _, block := range f.Blocks {
			for _, instr := range block.Instrs {
				if !yield(instr) {
					return
				}
			}
		}
	}
}

// Like TypeOf, but panics instead of returning nil.
// Only valid during f's create and build phases.
func (f *Function) typeOf(e ast.Expr) types.Type {
	if T := f.info.TypeOf(e); T != nil {
		return f.typ(T)
	}
	panic(fmt.Sprintf("no type for %T @ %s", e, f.Prog.Fset.Position(e.Pos())))
}

// typ is the locally instantiated type of T.
// If f is not an instantiation, then f.typ(T)==T.
func (f *Function) typ(T types.Type) types.Type {
	return f.subst.typ(T)
}

// If id is an Instance, returns info.Instances[id].Type.
// Otherwise returns f.typeOf(id).
func (f *Function) instanceType(id *ast.Ident) types.Type {
	if t, ok := f.info.Instances[id]; ok {
		return t.Type
	}
	return f.typeOf(id)
}

// selection returns a *selection corresponding to f.info.Selections[selector]
// with potential updates for type substitution.
func (f *Function) selection(selector *ast.SelectorExpr) *selection {
	sel := f.info.Selections[selector]
	if sel == nil {
		return nil
	}

	switch sel.Kind() {
	case types.MethodExpr, types.MethodVal:
		if recv := f.typ(sel.Recv()); recv != sel.Recv() {
			// recv changed during type substitution.
			pkg := f.declaredPackage().Pkg
			obj, index, indirect := types.LookupFieldOrMethod(recv, true, pkg, sel.Obj().Name())

			// sig replaces sel.Type(). See (types.Selection).Typ() for details.
			sig := obj.Type().(*types.Signature)
			sig = changeRecv(sig, newVar(sig.Recv().Name(), recv))
			if sel.Kind() == types.MethodExpr {
				sig = recvAsFirstArg(sig)
			}
			return &selection{
				kind:     sel.Kind(),
				recv:     recv,
				typ:      sig,
				obj:      obj,
				index:    index,
				indirect: indirect,
			}
		}
	}
	return toSelection(sel)
}

// Destinations associated with unlabelled for/switch/select stmts.
// We push/pop one of these as we enter/leave each construct and for
// each BranchStmt we scan for the innermost target of the right type.
type targets struct {
	tail         *targets // rest of stack
	_break       *BasicBlock
	_continue    *BasicBlock
	_fallthrough *BasicBlock
}

// Destinations associated with a labelled block.
// We populate these as labels are encountered in forward gotos or
// labelled statements.
// Forward gotos are resolved once it is known which statement they
// are associated with inside the Function.
type lblock struct {
	label     *types.Label // Label targeted by the blocks.
	resolved  bool         // _goto block encountered (back jump or resolved fwd jump)
	_goto     *BasicBlock
	_break    *BasicBlock
	_continue *BasicBlock
}

// label returns the symbol denoted by a label identifier.
//
// label should be a non-blank identifier (label.Name != "_").
func (f *Function) label(label *ast.Ident) *types.Label {
	return f.objectOf(label).(*types.Label)
}

// lblockOf returns the branch target associated with the
// specified label, creating it if needed.
func (f *Function) lblockOf(label *types.Label) *lblock {
	lb := f.lblocks[label]
	if lb == nil {
		lb = &lblock{
			label: label,
			_goto: f.newBasicBlock(label.Name()),
		}
		if f.lblocks == nil {
			f.lblocks = make(map[*types.Label]*lblock)
		}
		f.lblocks[label] = lb
	}
	return lb
}

// labelledBlock searches f for the block of the specified label.
//
// If f is a yield function, it additionally searches ancestor Functions
// corresponding to enclosing range-over-func statements within the
// same source function, so the returned block may belong to a different Function.
func labelledBlock(f *Function, label *types.Label, tok token.Token) *BasicBlock {
	if lb := f.lblocks[label]; lb != nil {
		var block *BasicBlock
		switch tok {
		case token.BREAK:
			block = lb._break
		case token.CONTINUE:
			block = lb._continue
		case token.GOTO:
			block = lb._goto
		}
		if block != nil {
			return block
		}
	}
	// Search ancestors if this is a yield function.
	if f.jump != nil {
		return labelledBlock(f.parent, label, tok)
	}
	return nil
}

// targetedBlock looks for the nearest block in f.targets
// (and f's ancestors) that matches tok's type, and returns
// the block and function it was found in.
func targetedBlock(f *Function, tok token.Token) *BasicBlock {
	if f == nil {
		return nil
	}
	for t := f.targets; t != nil; t = t.tail {
		var block *BasicBlock
		switch tok {
		case token.BREAK:
			block = t._break
		case token.CONTINUE:
			block = t._continue
		case token.FALLTHROUGH:
			block = t._fallthrough
		}
		if block != nil {
			return block
		}
	}
	// Search f's ancestors (in case f is a yield function).
	return targetedBlock(f.parent, tok)
}

// addResultVar adds a result for a variable v to f.results and v to f.returnVars.
func (f *Function) addResultVar(v *types.Var, source ast.Node) {
	name := v.Name()
	if name == "" {
		name = fmt.Sprintf("res.%d", len(f.results))
	}
	result := emitLocalVar(f, v, source)
	result.comment = name
	f.results = append(f.results, result)
	f.returnVars = append(f.returnVars, v)
}

// addParamVar adds a parameter to f.Params.
func (f *Function) addParamVar(v *types.Var, source ast.Node) *Parameter {
	name := v.Name()
	if name == "" {
		name = fmt.Sprintf("arg%d", len(f.Params))
	}
	param := &Parameter{name: name, typ: f.typ(v.Type()), parent: f}
	param.setSource(source)
	param.object = v
	f.Params = append(f.Params, param)
	return param
}

// addSpilledParam declares a parameter that is pre-spilled to the
// stack; the function body will load/store the spilled location.
// Subsequent lifting will eliminate spills where possible.
func (f *Function) addSpilledParam(obj *types.Var, source ast.Node) {
	param := f.addParamVar(obj, source)
	spill := emitLocalVar(f, obj, source)
	emitStore(f, spill, param, source)
}

// startBody initializes the function prior to generating IR code for its body.
// Precondition: f.Type() already set.
func (f *Function) startBody() {
	entry := f.newBasicBlock("entry")
	f.currentBlock = entry
	f.vars = make(map[*types.Var]Value) // needed for some synthetics, e.g. init
}

func (f *Function) blockset(i int) *BlockSet {
	bs := &f.blocksets[i]
	if len(bs.values) != len(f.Blocks) {
		if cap(bs.values) >= len(f.Blocks) {
			bs.values = bs.values[:len(f.Blocks)]
			bs.Clear()
		} else {
			bs.values = make([]bool, len(f.Blocks))
		}
	} else {
		bs.Clear()
	}
	return bs
}

// createSyntacticParams populates f.Params and generates code (spills
// and named result locals) for all the parameters declared in the
// syntax.  In addition it populates the f.objects mapping.
//
// Preconditions:
// f.startBody() was called. f.info != nil.
// Postcondition:
// len(f.Params) == len(f.Signature.Params) + (f.Signature.Recv() ? 1 : 0)
func (f *Function) createSyntacticParams(recv *ast.FieldList, functype *ast.FuncType) {
	// Receiver (at most one inner iteration).
	if recv != nil {
		for _, field := range recv.List {
			for _, n := range field.Names {
				f.addSpilledParam(identVar(f, n), n)
			}
			// Anonymous receiver?  No need to spill.
			if field.Names == nil {
				f.addParamVar(f.Signature.Recv(), field)
			}
		}
	}

	// Parameters.
	if functype.Params != nil {
		n := len(f.Params) // 1 if has recv, 0 otherwise
		for _, field := range functype.Params.List {
			for _, n := range field.Names {
				f.addSpilledParam(identVar(f, n), n)
			}
			// Anonymous parameter?  No need to spill.
			if field.Names == nil {
				f.addParamVar(f.Signature.Params().At(len(f.Params)-n), field)
			}
		}
	}

	// Results.
	if functype.Results != nil {
		for _, field := range functype.Results.List {
			// Implicit "var" decl of locals for named results.
			for _, n := range field.Names {
				v := identVar(f, n)
				f.addResultVar(v, n)
			}
			// Implicit "var" decl of local for an unnamed result.
			if field.Names == nil {
				v := f.Signature.Results().At(len(f.results))
				f.addResultVar(v, field.Type)
			}
		}
	}
}

// createDeferStack initializes fn.deferstack to a local variable
// initialized to a ssa:deferstack() call.
func (fn *Function) createDeferStack() {
	// Each syntactic function makes a call to ssa:deferstack,
	// which is spilled to a local. Unused ones are later removed.
	fn.deferstack = newVar("defer$stack", tDeferStack)
	call := &Call{Call: CallCommon{Value: vDeferStack}}
	call.setType(tDeferStack)
	deferstack := fn.emit(call, nil)
	spill := emitLocalVar(fn, fn.deferstack, nil)
	emitStore(fn, spill, deferstack, nil)
}

func numberNodes(f *Function) {
	var base ID
	for _, b := range f.Blocks {
		for _, instr := range b.Instrs {
			if instr == nil {
				continue
			}
			base++
			instr.setID(base)
		}
	}
}

func updateOperandsReferrers(instr Instruction, ops []*Value) {
	for _, op := range ops {
		if r := *op; r != nil {
			if refs := (*op).Referrers(); refs != nil {
				if len(*refs) == 0 {
					// per median, each value has two referrers, so we can avoid one call into growslice
					//
					// Note: we experimented with allocating
					// sequential scratch space, but we
					// couldn't find a value that gave better
					// performance than making many individual
					// allocations
					*refs = make([]Instruction, 1, 2)
					(*refs)[0] = instr
				} else {
					*refs = append(*refs, instr)
				}
			}
		}
	}
}

// buildReferrers populates the def/use information in all non-nil
// Value.Referrers slice.
// Precondition: all such slices are initially empty.
func buildReferrers(f *Function) {
	var rands []*Value

	for _, b := range f.Blocks {
		for _, instr := range b.Instrs {
			rands = instr.Operands(rands[:0]) // recycle storage
			updateOperandsReferrers(instr, rands)
		}
	}
}

// finishBody() finalizes the contents of the function after IR code generation of its body.
//
// The function is not done being built until done() is called.
func (f *Function) finishBody() {
	f.currentBlock = nil
	f.lblocks = nil
	f.returnVars = nil
	f.jump = nil
	f.source = nil
	f.exits = nil

	// Remove from f.Locals any Allocs that escape to the heap.
	j := 0
	for _, l := range f.Locals {
		if !l.Heap {
			f.Locals[j] = l
			j++
		}
	}
	// Nil out f.Locals[j:] to aid GC.
	for i := j; i < len(f.Locals); i++ {
		f.Locals[i] = nil
	}
	f.Locals = f.Locals[:j]

	optimizeBlocks(f)
	buildReferrers(f)
	buildDomTree(f)
	buildSCCs(f)

	if f.Prog.mode&NaiveForm == 0 {
		for lift(f) {
		}
		if doSimplifyConstantCompositeValues {
			for simplifyConstantCompositeValues(f) {
			}
		}
	}

	if f.debugInfo() {
		// Turn debug references into a map
		f.exprToValue = map[ast.Expr]struct {
			v      Value
			isAddr bool
		}{}
		for _, b := range f.Blocks {
			newInstrs := b.Instrs[:0]
			for _, instr := range b.Instrs {
				if instr, ok := instr.(*debugRef); ok {
					if refs := instr.X.Referrers(); refs != nil {
						*refs = removeInstr(*refs, instr)
					}
					if _, ok := f.exprToValue[instr.Expr]; !ok {
						f.exprToValue[instr.Expr] = struct {
							v      Value
							isAddr bool
						}{instr.X, instr.IsAddr}
					}
					continue
				}
				newInstrs = append(newInstrs, instr)
			}
			b.Instrs = newInstrs
		}
	}

	// clear remaining builder state
	f.results = nil    // (used by lifting)
	f.deferstack = nil // (used by lifting)
	f.vars = nil       // (used by lifting)

	numberNodes(f)

	// clear out other function state (keep consistent with buildParamsOnly)
	f.subst = nil

	// Release the package's type information, unless
	// instances of f will later be built from it.
	if !f.isGenericOrigin() {
		f.info = nil
	}

	numberNodes(f)
}

// done marks the building of f's SSA body complete,
// along with any nested functions, and optionally prints them.
func (f *Function) done() {
	assert(f.parent == nil, "done called on an anonymous function")

	var visit func(*Function)
	visit = func(f *Function) {
		for _, anon := range f.AnonFuncs {
			visit(anon) // anon is done building before f.
		}

		f.uniq = 0    // done with uniq
		f.build = nil // function is built

		if f.Prog.mode&PrintFunctions != 0 {
			printMu.Lock()
			f.WriteTo(os.Stdout)
			printMu.Unlock()
		}

		if f.Prog.mode&SanityCheckFunctions != 0 {
			mustSanityCheck(f, nil)
		}
	}
	visit(f)
}

func isUselessPhi(phi *Phi) (Value, bool) {
	var v0 Value
	if len(phi.Edges) == 0 {
		panic("φ with no edges")
	}
	for _, e := range phi.Edges {
		if e == nil {
			panic(fmt.Sprintf("φ has nil edge: %v", phi))
		}
		if e == phi {
			continue
		}
		if v0 == nil {
			v0 = e
		}
		if v0 != e {
			if v0, ok := v0.(*Const); ok {
				if e, ok := e.(*Const); ok {
					if v0.typ == e.typ && v0.Value == e.Value {
						continue
					}
				}
			}
			return nil, false
		}
	}
	if v0 == nil {
		panic("trying to replace φ with nil")
	}
	return v0, true
}

// removeNilBlocks eliminates nils from f.Blocks and updates each
// BasicBlock.Index.  Use this after any pass that may delete blocks.
func (f *Function) removeNilBlocks() {
	j := 0
	for _, b := range f.Blocks {
		if b != nil {
			b.Index = j
			f.Blocks[j] = b
			j++
		}
	}
	// Nil out f.Blocks[j:] to aid GC.
	for i := j; i < len(f.Blocks); i++ {
		f.Blocks[i] = nil
	}
	f.Blocks = f.Blocks[:j]
}

// SetDebugMode sets the debug mode for package pkg.  If true, all its
// functions will include full debug info.  This greatly increases the
// size of the instruction stream, and causes Functions to depend upon
// the ASTs, potentially keeping them live in memory for longer.
func (pkg *Package) SetDebugMode(debug bool) {
	pkg.debug = debug
}

// debugInfo reports whether debug info is wanted for this function.
func (f *Function) debugInfo() bool {
	// debug info for instantiations follows the debug info of their origin.
	p := f.declaredPackage()
	return p != nil && p.debug
}

// lookup returns the address of the named variable identified by obj
// that is local to function f or one of its enclosing functions.
// If escaping, the reference comes from a potentially escaping pointer
// expression and the referent must be heap-allocated.
// We assume the referent is a *Alloc or *Phi.
// (The only Phis at this stage are those created directly by go1.22 "for" loops.)
func (f *Function) lookup(obj *types.Var, escaping bool) Value {
	if v, ok := f.vars[obj]; ok {
		if escaping {
			switch v := v.(type) {
			case *Alloc:
				v.Heap = true
			case *Phi:
				for _, edge := range v.Edges {
					if alloc, ok := edge.(*Alloc); ok {
						alloc.Heap = true
					}
				}
			}
		}
		return v // function-local var (address)
	}

	// Definition must be in an enclosing function;
	// plumb it through intervening closures.
	if f.parent == nil {
		panic("no ir.Value for " + obj.String())
	}
	outer := f.parent.lookup(obj, true) // escaping
	v := &FreeVar{
		name:   obj.Name(),
		typ:    outer.Type(),
		outer:  outer,
		parent: f,
	}
	f.vars[obj] = v
	f.FreeVars = append(f.FreeVars, v)
	return v
}

// emit emits the specified instruction to function f.
func (f *Function) emit(instr Instruction, source ast.Node) Value {
	return f.currentBlock.emit(instr, source)
}

// RelString returns the full name of this function, qualified by
// package name, receiver type, etc.
//
// The specific formatting rules are not guaranteed and may change.
//
// Examples:
//
//	"math.IsNaN"                  // a package-level function
//	"(*bytes.Buffer).Bytes"       // a declared method or a wrapper
//	"(*bytes.Buffer).Bytes$thunk" // thunk (func wrapping method; receiver is param 0)
//	"(*bytes.Buffer).Bytes$bound" // bound (func wrapping method; receiver supplied by closure)
//	"main.main$1"                 // an anonymous function in main
//	"main.init#1"                 // a declared init function
//	"main.init"                   // the synthesized package initializer
//
// When these functions are referred to from within the same package
// (i.e. from == f.Pkg.Object), they are rendered without the package path.
// For example: "IsNaN", "(*Buffer).Bytes", etc.
//
// All non-synthetic functions have distinct package-qualified names.
// (But two methods may have the same name "(T).f" if one is a synthetic
// wrapper promoting a non-exported method "f" from another package; in
// that case, the strings are equal but the identifiers "f" are distinct.)
func (f *Function) RelString(from *types.Package) string {
	// Anonymous?
	if f.parent != nil {
		// An anonymous function's Name() looks like "parentName$1",
		// but its String() should include the type/package/etc.
		parent := f.parent.RelString(from)
		for i, anon := range f.parent.AnonFuncs {
			if anon == f {
				return fmt.Sprintf("%s$%d", parent, 1+i)
			}
		}

		return f.name // should never happen
	}

	// Method (declared or wrapper)?
	if recv := f.Signature.Recv(); recv != nil {
		return f.relMethod(from, recv.Type())
	}

	// Thunk?
	if f.method != nil {
		return f.relMethod(from, f.method.recv)
	}

	// Bound?
	if len(f.FreeVars) == 1 && strings.HasSuffix(f.name, "$bound") {
		return f.relMethod(from, f.FreeVars[0].Type())
	}

	// Package-level function?
	// Prefix with package name for cross-package references only.
	if p := f.relPkg(); p != nil && p != from {
		return fmt.Sprintf("%s.%s", p.Path(), f.name)
	}

	// Unknown.
	return f.name
}

func (f *Function) relMethod(from *types.Package, recv types.Type) string {
	return fmt.Sprintf("(%s).%s", relType(recv, from), f.name)
}

// writeSignature writes to buf the signature sig in declaration syntax.
func writeSignature(buf *bytes.Buffer, from *types.Package, name string, sig *types.Signature) {
	buf.WriteString("func ")
	if recv := sig.Recv(); recv != nil {
		buf.WriteString("(")
		if name := recv.Name(); name != "" {
			buf.WriteString(name)
			buf.WriteString(" ")
		}
		types.WriteType(buf, recv.Type(), types.RelativeTo(from))
		buf.WriteString(") ")
	}
	buf.WriteString(name)
	types.WriteSignature(buf, sig, types.RelativeTo(from))
}

// declaredPackage returns the package fn is declared in or nil if the
// function is not declared in a package.
func (fn *Function) declaredPackage() *Package {
	switch {
	case fn.Pkg != nil:
		return fn.Pkg // non-generic function  (does that follow??)
	case fn.topLevelOrigin != nil:
		return fn.topLevelOrigin.Pkg // instance of a named generic function
	case fn.parent != nil:
		return fn.parent.declaredPackage() // instance of an anonymous [generic] function
	default:
		return nil // function is not declared in a package, e.g. a wrapper.
	}
}

// relPkg returns types.Package fn is printed in relationship to.
func (fn *Function) relPkg() *types.Package {
	if p := fn.declaredPackage(); p != nil {
		return p.Pkg
	}
	return nil
}

var _ io.WriterTo = (*Function)(nil) // *Function implements io.Writer

func (f *Function) WriteTo(w io.Writer) (int64, error) {
	var buf bytes.Buffer
	WriteFunction(&buf, f)
	n, err := w.Write(buf.Bytes())
	return int64(n), err
}

// WriteFunction writes to buf a human-readable "disassembly" of f.
func WriteFunction(buf *bytes.Buffer, f *Function) {
	fmt.Fprintf(buf, "# Name: %s\n", f.String())
	if f.Pkg != nil {
		fmt.Fprintf(buf, "# Package: %s\n", f.Pkg.Pkg.Path())
	}
	if syn := f.Synthetic; syn != "" {
		fmt.Fprintln(buf, "# Synthetic:", syn)
	}
	if pos := f.Pos(); pos.IsValid() {
		fmt.Fprintf(buf, "# Location: %s\n", f.Prog.Fset.Position(pos))
	}

	if f.parent != nil {
		fmt.Fprintf(buf, "# Parent: %s\n", f.parent.Name())
	}

	from := f.relPkg()

	// NB. column calculations are confused by non-ASCII
	// characters
	const punchcard = 80 // for old time's sake.
	const padding = "        "

	if f.FreeVars != nil {
		buf.WriteString("# Free variables:\n")
		for i, fv := range f.FreeVars {
			fmt.Fprintf(buf, "# % 3d:%s%s %s\n", i, padding, fv.Name(), relType(fv.Type(), from))
		}
	}

	if len(f.Locals) > 0 {
		buf.WriteString("# Locals:\n")
		for i, l := range f.Locals {
			fmt.Fprintf(buf, "# % 3d:%s%s %s\n", i, padding, l.Name(), relType(deref(l.Type()), from))
		}
	}
	writeSignature(buf, from, f.Name(), f.Signature)
	buf.WriteString(":\n")

	if f.Blocks == nil {
		buf.WriteString(padding)
		buf.WriteString("(external)\n")
	}

	for _, b := range f.Blocks {
		if b == nil {
			// Corrupt CFG.
			fmt.Fprintf(buf, ".nil:\n")
			continue
		}
		n, _ := fmt.Fprintf(buf, "%d:", b.Index)
		// (|predecessors|, |successors|, immediate dominator)
		bmsg := fmt.Sprintf("%s P:%d S:%d", b.Comment, len(b.Preds), len(b.Succs))
		if b.Idom() != nil {
			bmsg = fmt.Sprintf("%s idom:%d", bmsg, b.Idom().Index)
		}
		fmt.Fprintf(buf, "%*s%s\n", punchcard-1-n-len(bmsg), "", bmsg)

		if false { // CFG debugging
			fmt.Fprintf(buf, "%s# CFG: %s --> %s --> %s\n", padding, b.Preds, b, b.Succs)
		}
		for _, instr := range b.Instrs {
			buf.WriteString(padding)
			switch v := instr.(type) {
			case Value:
				l := punchcard - len(padding)
				// Left-align the instruction.
				if name := v.Name(); name != "" {
					n, _ := fmt.Fprintf(buf, "%s = ", name)
					l -= n
				}
				n, _ := buf.WriteString(instr.String())
				l -= n
				// Right-align the type if there's space.
				if t := v.Type(); t != nil {
					buf.WriteByte(' ')
					ts := relType(t, from)
					l -= len(ts) + len("  ") // (spaces before and after type)
					if l > 0 {
						fmt.Fprintf(buf, "%*s", l, "")
					}
					buf.WriteString(ts)
				}
			case nil:
				// Be robust against bad transforms.
				buf.WriteString("<deleted>")
			default:
				buf.WriteString(instr.String())
			}
			// -mode=S: show line numbers
			if f.Prog.mode&LogSource != 0 {
				if pos := instr.Pos(); pos.IsValid() {
					fmt.Fprintf(buf, " L%d", f.Prog.Fset.Position(pos).Line)
				}
			}
			buf.WriteString("\n")
		}
	}
	buf.WriteString("\n")
}

// newBasicBlock adds to f a new basic block and returns it.  It does
// not automatically become the current block for subsequent calls to emit.
// comment is an optional string for more readable debugging output.
func (f *Function) newBasicBlock(comment string) *BasicBlock {
	b := &BasicBlock{
		Index:   len(f.Blocks),
		Comment: comment,
		parent:  f,
	}
	b.Succs = b.succs2[:0]
	f.Blocks = append(f.Blocks, b)
	return b
}

// NewFunction returns a new synthetic Function instance belonging to
// prog, with its name and signature fields set as specified.
//
// The caller is responsible for initializing the remaining fields of
// the function object, e.g. Pkg, Params, Blocks.
//
// It is practically impossible for clients to construct well-formed
// IR functions/packages/programs directly, so we assume this is the
// job of the Builder alone.  NewFunction exists to provide clients a
// little flexibility.  For example, analysis tools may wish to
// construct fake Functions for the root of the callgraph, a fake
// "reflect" package, etc.
//
// TODO(adonovan): think harder about the API here.
func (prog *Program) NewFunction(name string, sig *types.Signature, provenance string) *Function {
	return &Function{Prog: prog, name: name, Signature: sig, Synthetic: provenance}
}

//lint:ignore U1000 we may make use of this for functions loaded from export data
type extentNode [2]token.Pos

func (n extentNode) Pos() token.Pos { return n[0] }
func (n extentNode) End() token.Pos { return n[1] }

func killInstruction(instr Instruction) {
	ops := instr.Operands(nil)
	for _, op := range ops {
		if refs := (*op).Referrers(); refs != nil {
			*refs = removeInstr(*refs, instr)
		}
	}
}

// Syntax returns the function's syntax (*ast.Func{Decl,Lit})
// if it was produced from syntax or an *ast.RangeStmt if
// it is a range-over-func yield function.
func (f *Function) Syntax() ast.Node { return f.syntax }

// identVar returns the variable defined by id.
func identVar(fn *Function, id *ast.Ident) *types.Var {
	return fn.info.Defs[id].(*types.Var)
}

// unique returns a unique positive int within the source tree of f.
// The source tree of f includes all of f's ancestors by parent and all
// of the AnonFuncs contained within these.
func unique(f *Function) int64 {
	f.uniq++
	return f.uniq
}

// exit is a change of control flow going from a range-over-func
// yield function to an ancestor function caused by a break, continue,
// goto, or return statement.
//
// There are 3 types of exits:
// * return from the source function (from ReturnStmt),
// * jump to a block (from break and continue statements [labelled/unlabelled]),
// * go to a label (from goto statements).
//
// As the builder does one pass over the ast, it is unclear whether
// a forward goto statement will leave a range-over-func body.
// The function being exited to is unresolved until the end
// of building the range-over-func body.
type exit struct {
	id     int64     // unique value for exit within from and to
	from   *Function // the function the exit starts from
	to     *Function // the function being exited to (nil if unresolved)
	source ast.Node

	block *BasicBlock  // basic block within to being jumped to.
	label *types.Label // forward label being jumped to via goto.
	// block == nil && label == nil => return
}

// storeVar emits to function f code to store a value v to a *types.Var x.
func storeVar(f *Function, x *types.Var, v Value, source ast.Node) {
	emitStore(f, f.lookup(x, true), v, source)
}

// labelExit creates a new exit to a yield fn to exit the function using a label.
func labelExit(fn *Function, label *types.Label, source ast.Node) *exit {
	e := &exit{
		id:     unique(fn),
		from:   fn,
		to:     nil,
		source: source,
		label:  label,
	}
	fn.exits = append(fn.exits, e)
	return e
}

// blockExit creates a new exit to a yield fn that jumps to a basic block.
func blockExit(fn *Function, block *BasicBlock, source ast.Node) *exit {
	e := &exit{
		id:     unique(fn),
		from:   fn,
		to:     block.parent,
		source: source,
		block:  block,
	}
	fn.exits = append(fn.exits, e)
	return e
}

// returnExit creates a new exit to a yield fn that returns to the source function.
func returnExit(fn *Function, source ast.Node) *exit {
	e := &exit{
		id:     unique(fn),
		from:   fn,
		to:     fn.source,
		source: source,
	}
	fn.exits = append(fn.exits, e)
	return e
}

// isGenericOrigin reports whether f is an uninstantiated generic
// function or a method of an uninstantiated generic type.
func (f *Function) isGenericOrigin() bool {
	return (f.typeparams.Len() > 0 && len(f.typeargs) == 0) ||
		(f.recvtypeparams.Len() > 0 && len(f.recvtypeargs) == 0)
}
