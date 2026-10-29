package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"go/version"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"dev.gaijin.team/go/exhaustruct/v5/internal/directive"
	"dev.gaijin.team/go/exhaustruct/v5/internal/structure"
)

// missingFieldsVisitor checks struct literals for missing field initializations.
type missingFieldsVisitor struct {
	pass      *analysis.Pass
	config    *Config
	processor *structure.Processor
}

func newMissingFieldsVisitor(
	pass *analysis.Pass,
	config *Config,
	processor *structure.Processor,
) *missingFieldsVisitor {
	return &missingFieldsVisitor{
		pass:      pass,
		config:    config,
		processor: processor,
	}
}

func (v *missingFieldsVisitor) run() {
	insp := v.pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert

	insp.WithStack([]ast.Node{(*ast.CompositeLit)(nil)}, v.visit)
}

func (v *missingFieldsVisitor) visit(n ast.Node, push bool, stack []ast.Node) bool {
	if !push {
		return true
	}

	lit, ok := n.(*ast.CompositeLit)
	if !ok {
		return true
	}

	lv := literalVisitor{missingFieldsVisitor: v, lit: lit, stack: stack}
	lv.process()

	return true
}

// literalVisitor carries context for processing a single composite literal.
type literalVisitor struct {
	*missingFieldsVisitor

	lit   *ast.CompositeLit
	stack []ast.Node
}

// literal holds resolved info for a struct literal being checked.
type literal struct {
	strct    *structure.Struct
	ignored  bool
	enforced bool
}

// shouldCheck implements checking decision priority.
func (l literal) shouldCheck(explicitMode bool) bool {
	if l.ignored {
		return false
	}

	if l.enforced {
		return true
	}

	if l.strct.IsIgnored() {
		return false
	}

	if l.strct.IsEnforced() {
		return true
	}

	return !explicitMode
}

func (lv literalVisitor) process() {
	lit, ok := lv.resolveLiteral()
	if !ok {
		return
	}

	if len(lv.lit.Elts) == 0 && lv.checkEmptyAllowed(lit.strct) {
		return
	}

	if pos, msg := lv.checkLiteral(lit); pos != nil {
		lv.pass.Reportf(*pos, "%s", msg)
	}
}

// resolveLiteral extracts struct type information from the composite literal,
// retrieves cached metadata, and looks up directives.
//
// The named result carries the zero value on the paths that report ok=false,
// where the caller discards it and there is no struct to describe.
func (lv literalVisitor) resolveLiteral() (lit literal, ok bool) {
	typeName, strct, pos := lv.resolveLiteralType()
	if strct == nil {
		return lit, false
	}

	s := lv.processor.ResolveStruct(lv.pass.Fset, typeName, strct, pos, lv.pass.Pkg)
	if s == nil {
		return lit, false
	}

	dirs := lv.useSiteDirectives()

	return literal{
		strct:    s,
		ignored:  dirs.Contains(directive.Ignore),
		enforced: dirs.Contains(directive.Enforce),
	}, true
}

// useSiteDirectives collects the directives that apply to the literal.
//
// A directive is written above the statement holding a literal, not above every
// literal nested inside it, so the search climbs out of the expression the
// literal sits in and reads the statement holding it. Enumerating the node
// kinds a literal may be written within cannot be complete -- parentheses,
// selectors, operators, calls, sends and index expressions each nest literals,
// and the list grew by one every time a shape was missed -- so the walk asks
// what a node is instead: it climbs through expressions, and through the specs
// and declarations a var block is built from, until it reaches a statement.
//
// It stops there. A statement is where a directive is written, and going above
// one would let a directive on an `if` or a `for` reach every literal in the
// block below it.
func (lv literalVisitor) useSiteDirectives() directive.Directives {
	var dirs directive.Directives

	for _, node := range slices.Backward(lv.stack) {
		if !carriesUseSiteDirective(node) {
			break
		}

		found := lv.processor.Directives().LookupPos(lv.pass.Fset, node.Pos())

		for _, d := range found {
			if !dirs.Contains(d) {
				dirs = append(dirs, d)
			}
		}

		if _, isStatement := node.(ast.Stmt); isStatement {
			break
		}
	}

	return dirs
}

// carriesUseSiteDirective reports whether a directive written above node can
// apply to a composite literal nested inside it.
func carriesUseSiteDirective(node ast.Node) bool {
	switch node.(type) {
	case ast.Expr, ast.Spec, ast.Decl, ast.Stmt:
		return true

	default:
		return false
	}
}

// resolveLiteralType resolves the composite literal's type and definition position.
func (lv literalVisitor) resolveLiteralType() (name *types.TypeName, strct *types.Struct, pos token.Pos) {
	typ := lv.pass.TypesInfo.TypeOf(lv.lit)

	// A literal may elide `&T` when the element type is a pointer, including a
	// pointer reached through an alias or a defined pointer type. Such a
	// pointer has a declaration of its own, which carries the directives and
	// the patterns that select the literal -- an alias to a pointer answers
	// like an alias to a struct.
	name = typeNameOf(typ)

	if ptr, ok := types.Unalias(typ).Underlying().(*types.Pointer); ok {
		typ = ptr.Elem()

		// A plain *T is written at the use site and declares nothing, so the
		// struct it points at names the literal.
		if name == nil {
			name = typeNameOf(typ)
		}
	}

	typ = types.Unalias(typ)

	switch t := typ.(type) {
	case *types.Named:
		var ok bool

		// Every instantiation of a generic type shares one declaration, and the
		// declaration is what carries the fields and the directives. Resolving
		// the origin makes G[int] and G[string] answer as the type they are
		// written from, out of one cache entry.
		if strct, ok = t.Origin().Underlying().(*types.Struct); !ok {
			return nil, nil, token.NoPos
		}

		return name, strct, name.Pos()

	case *types.TypeParam:
		// A literal of a type parameter is written against its constraint's
		// core type, which the type checker has already validated the keys
		// against.
		if strct = coreStruct(lv.pass.Fset, lv.processor.Directives(), t); strct == nil {
			return nil, nil, token.NoPos
		}

		// A type parameter is declared in a signature, and the core it resolves
		// to is a shape its constraint names rather than a type anyone declared.
		// There is no declaration to read type-level directives from, and the
		// parameter's own line would read the directives of the function.
		return name, strct, token.NoPos

	case *types.Struct:
		// An alias to an anonymous struct has a declaration of its own; only a
		// genuinely unnamed struct needs its position recovered from the AST.
		if name != nil {
			return name, t, name.Pos()
		}

		return name, t, lv.findAnonymousStructPos()

	default:
		return nil, nil, token.NoPos
	}
}

// typeNameOf returns the declaration a type is written under, or nil when the
// type is unnamed.
func typeNameOf(typ types.Type) *types.TypeName {
	switch t := typ.(type) {
	case *types.Alias:
		return t.Obj()

	case *types.Named:
		return t.Obj()

	case *types.TypeParam:
		return t.Obj()

	default:
		return nil
	}
}

// coreStruct returns the struct type every term of a type parameter's
// constraint shares, or nil when the constraint has no such core type and a
// composite literal of the parameter is therefore not possible.
func coreStruct(fset *token.FileSet, directives *directive.Scanner, tp *types.TypeParam) *types.Struct {
	r := coreResolver{
		fset:       fset,
		directives: directives,
		walking:    map[*types.Interface]bool{},
		done:       map[*types.Interface]coreResult{},
	}

	strct, _ := r.constraintCore(tp.Constraint())

	return strct
}

// coreResolver walks the interfaces a constraint embeds. walking holds the ones
// on the path being walked, so a cycle ends without a second branch of a
// diamond reading as one; done holds the ones the walk finished, so a diamond
// costs one visit per interface rather than one per path down to it. Stacked
// diamonds otherwise multiply: n layers open 2^n paths over 3n+1 declarations.
//
// constraint: Go rejects a recursive interface, so an entry in done can never
// hold a result the walking guard truncated. That guard is there so a type
// built outside the type checker cannot recurse without end.
type coreResolver struct {
	fset *token.FileSet
	// directives reads what a term's fields are annotated with, which is what
	// tells two declarations of one shape apart. Nil where no scanner is at
	// hand, and two such declarations then answer as one core.
	directives *directive.Scanner
	walking    map[*types.Interface]bool
	done       map[*types.Interface]coreResult
}

// coreResult is what one interface resolved to, kept so a branch reaching it a
// second time answers without walking it again.
type coreResult struct {
	core *types.Struct
	ok   bool
}

// constraintCore resolves the struct shared by every term of a constraint,
// descending into the interfaces it embeds: a constraint may name its terms
// itself or inherit them from another interface, and both describe one type
// set.
//
// A constraint restricts methods as well as types, and the two are separate
// contributions: a method-only interface narrows the type set without naming a
// type, so it leaves the core of its siblings standing. ok is false only for a
// term that names a type no literal can be written for, which leaves the whole
// constraint without a core.

func (r coreResolver) constraintCore(typ types.Type) (core *types.Struct, ok bool) {
	iface, isIface := typ.Underlying().(*types.Interface)
	if !isIface {
		return nil, false
	}

	// An interface embedding itself contributes no core of its own; whichever
	// branch opened it carries the answer.
	if r.walking[iface] {
		return nil, true
	}

	if res, walked := r.done[iface]; walked {
		return res.core, res.ok
	}

	r.walking[iface] = true

	defer func() {
		delete(r.walking, iface)

		r.done[iface] = coreResult{core: core, ok: ok}
	}()

	for embedded := range iface.EmbeddedTypes() {
		for _, term := range unionTerms(embedded) {
			strct, termOK := r.termCore(term)
			if !termOK {
				return nil, false
			}

			if strct == nil {
				continue
			}

			if core != nil && !r.sameCore(core, strct) {
				return nil, false
			}

			core = strct
		}
	}

	return core, true
}

// sameCore reports whether two terms answer as one core: the same struct, which
// an alias and a defined type both share, or two declarations of one shape
// whose fields are annotated alike. Fields annotated apart make two cores, and
// reading either declaration's would answer by the order the union lists its
// terms.
func (r coreResolver) sameCore(a, b *types.Struct) bool {
	if a == b {
		return true
	}

	if !types.Identical(a, b) {
		return false
	}

	if r.directives == nil {
		return true
	}

	for i := range a.NumFields() {
		if r.fieldOptionality(a.Field(i)) != r.fieldOptionality(b.Field(i)) {
			return false
		}
	}

	return true
}

// fieldOptionality reads what one field of a term is annotated with, at the
// position it is declared at, through the projection the field metadata itself
// is built from.
func (r coreResolver) fieldOptionality(f *types.Var) directive.Optionality {
	return r.directives.LookupPos(r.fset, f.Pos()).Optionality()
}

// termCore resolves one term of a constraint to the struct it stands for: the
// term's own type, or the core of another interface it names.
func (r coreResolver) termCore(term types.Type) (*types.Struct, bool) {
	if _, ok := term.Underlying().(*types.Interface); ok {
		return r.constraintCore(term)
	}

	strct, ok := term.Underlying().(*types.Struct)

	return strct, ok
}

// unionTerms splits a constraint element into its terms. A single-term element
// is not wrapped in a union, so it stands for itself.
func unionTerms(typ types.Type) []types.Type {
	union, ok := typ.(*types.Union)
	if !ok {
		return []types.Type{typ}
	}

	terms := make([]types.Type, 0, union.Len())

	for term := range union.Terms() {
		terms = append(terms, term.Type())
	}

	return terms
}

// findAnonymousStructPos finds the position of the struct keyword for anonymous structs.
func (lv literalVisitor) findAnonymousStructPos() token.Pos {
	if lv.lit.Type != nil {
		if st, ok := lv.lit.Type.(*ast.StructType); ok {
			return st.Struct
		}

		return token.NoPos
	}

	// Key and value of a map literal both elide their type, and their types come
	// from opposite sides of the map type — track which side the literal is on.
	mapKey := false

	for i := len(lv.stack) - 2; i >= 0; i-- { //nolint:mnd
		switch parent := lv.stack[i].(type) {
		case *ast.KeyValueExpr:
			mapKey = parent.Key == lv.stack[i+1]

			continue

		case *ast.CompositeLit:
			return structPosFromType(parent.Type, mapKey)

		default:
			return token.NoPos
		}
	}

	return token.NoPos
}

func structPosFromType(typ ast.Expr, mapKey bool) token.Pos {
	if typ == nil {
		return token.NoPos
	}

	switch t := typ.(type) {
	case *ast.ArrayType:
		return structPosFromExpr(t.Elt)

	case *ast.MapType:
		if mapKey {
			return structPosFromExpr(t.Key)
		}

		return structPosFromExpr(t.Value)
	}

	return token.NoPos
}

func structPosFromExpr(expr ast.Expr) token.Pos {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}

	if st, ok := expr.(*ast.StructType); ok {
		return st.Struct
	}

	return token.NoPos
}

func (lv literalVisitor) checkEmptyAllowed(s *structure.Struct) bool {
	if lv.config.AllowEmpty {
		return true
	}

	if s.AllowEmptyDecl {
		return true
	}

	if ret, ok := lv.getParentReturnStmt(); ok {
		if lv.config.AllowEmptyReturns {
			return true
		}

		if lv.isErrorReturnStatement(ret) {
			return true
		}
	}

	if lv.isChildOfVariableDeclaration() && lv.config.AllowEmptyDeclarations {
		return true
	}

	if lv.config.AllowEmptyBlankAssignments && lv.isBlankAssignment() {
		return true
	}

	return false
}

func (lv literalVisitor) checkLiteral(lit literal) (*token.Pos, string) {
	if !lit.shouldCheck(lv.config.ExplicitMode) {
		return nil, ""
	}

	strct := lit.strct

	missingFields := strct.SkippedFields(lv.lit, lv.pass.Pkg.Path(), lv.canNamePromoted())

	if len(missingFields) == 0 {
		return nil, ""
	}

	pos := lv.lit.Pos()

	displayName := strct.PackageName + "." + strct.Name
	if lv.config.ReportFullTypePath {
		displayName = strct.FullPath
	}

	if len(missingFields) == 1 {
		return &pos, fmt.Sprintf("%s is missing field %s", displayName, structure.FormatFieldNames(missingFields))
	}

	return &pos, fmt.Sprintf("%s is missing fields %s", displayName, structure.FormatFieldNames(missingFields))
}

// promotedKeysVersion is the first language version that lets a composite
// literal name a promoted field in place of the embedded field carrying it
// (golang/go#77245).
const promotedKeysVersion = "go1.27"

// canNamePromoted reports whether the literal may write a promoted field as a
// key. The version of the file holding it decides, not the one the module
// declares: a //go:build constraint sets a single file apart from its package.
func (lv literalVisitor) canNamePromoted() bool {
	return canNamePromotedIn(lv.fileGoVersion())
}

// canNamePromotedIn reports whether a file of the given language version may
// write a promoted field as a key. A version that is missing or not a Go
// version reads as the newest one, as go/types reads it: the literal has
// already type-checked with whatever keys it names.
func canNamePromotedIn(goVersion string) bool {
	return !version.IsValid(goVersion) || version.Compare(goVersion, promotedKeysVersion) >= 0
}

// fileGoVersion returns the language version of the file the literal is written
// in, falling back to the package's own when the file carries none.
func (lv literalVisitor) fileGoVersion() string {
	if len(lv.stack) > 0 {
		if file, ok := lv.stack[0].(*ast.File); ok {
			if v := lv.pass.TypesInfo.FileVersions[file]; v != "" {
				return v
			}
		}
	}

	return lv.pass.Pkg.GoVersion()
}

// enclosingOfLiteral returns the node the literal is written into, climbing out
// of the wrappers that leave what it is written as unchanged: parentheses, and
// the address-of that makes a `&T{}`. The expression returned beside it is the
// one that node holds directly: the literal itself, or the outermost wrapper
// around it. Both are nil where no such node encloses the literal.
func (lv literalVisitor) enclosingOfLiteral() (parent ast.Node, child ast.Expr) {
	child = lv.lit

	for i := len(lv.stack) - 1; i > 0; i-- {
		switch p := lv.stack[i-1].(type) {
		case *ast.ParenExpr:
			child = p

			continue

		case *ast.UnaryExpr:
			if p.Op == token.AND {
				child = p

				continue
			}

			return nil, nil

		default:
			return p, child
		}
	}

	return nil, nil
}

// isChildOfVariableDeclaration reports whether the literal is the value a
// variable is declared with.
func (lv literalVisitor) isChildOfVariableDeclaration() bool {
	parent, _ := lv.enclosingOfLiteral()

	switch p := parent.(type) {
	case *ast.AssignStmt:
		return p.Tok == token.DEFINE

	case *ast.ValueSpec:
		return true

	default:
		return false
	}
}

// isBlankAssignment reports whether the literal is a value the blank identifier
// receives, in a declaration or an assignment. A value is matched to its name
// by position: `var _, x = T{}, U{}` discards the first literal and binds the
// second.
func (lv literalVisitor) isBlankAssignment() bool {
	parent, child := lv.enclosingOfLiteral()

	switch p := parent.(type) {
	case *ast.ValueSpec:
		i := slices.Index(p.Values, child)

		return i >= 0 && i < len(p.Names) && isBlank(p.Names[i])

	case *ast.AssignStmt:
		i := slices.Index(p.Rhs, child)

		return i >= 0 && i < len(p.Lhs) && isBlank(p.Lhs[i])

	default:
		return false
	}
}

// isBlank reports whether expr is the blank identifier, parenthesized or not,
// as go/types reads an assignment target.
func isBlank(expr ast.Expr) bool {
	ident, ok := unparen(expr).(*ast.Ident)

	return ok && ident.Name == "_"
}

// getParentReturnStmt returns the return statement the literal is a result of.
func (lv literalVisitor) getParentReturnStmt() (*ast.ReturnStmt, bool) {
	parent, _ := lv.enclosingOfLiteral()

	ret, ok := parent.(*ast.ReturnStmt)

	return ret, ok
}

//nolint:forcetypeassert,gochecknoglobals
var builtinErrorInterface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func (lv literalVisitor) isErrorReturnStatement(n *ast.ReturnStmt) bool {
	if len(n.Results) == 0 {
		return false
	}

	for _, ri := range slices.Backward(n.Results) {
		ri = unparen(ri)

		if ri == lv.lit {
			continue
		}

		switch ri := ri.(type) {
		case *ast.Ident:
			if ri.Name == "nil" {
				continue
			}

		case *ast.UnaryExpr:
			if unparen(ri.X) == lv.lit {
				continue
			}

		default:
		}

		resultType := lv.pass.TypesInfo.TypeOf(ri)
		if resultType != nil && types.Implements(resultType, builtinErrorInterface) {
			return true
		}
	}

	return false
}

// unparen strips redundant parentheses, which gofmt keeps and which would
// otherwise break the walks that match a literal against its enclosing
// expression.
func unparen(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}

		expr = paren.X
	}
}
