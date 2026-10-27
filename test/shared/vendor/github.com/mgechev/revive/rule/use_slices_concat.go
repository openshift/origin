package rule

import (
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// UseSlicesConcatRule spots appends to an empty slice that can be replaced by a call to [slices.Concat].
type UseSlicesConcatRule struct{}

// Apply applies the rule to given file.
func (*UseSlicesConcatRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	if !file.Pkg.IsAtLeastGoVersion(lint.Go122) {
		return nil
	}

	var failures []lint.Failure

	walker := lintUseSlicesConcat{
		onFailure: func(failure lint.Failure) {
			failures = append(failures, failure)
		},
	}

	ast.Walk(walker, file.AST)

	return failures
}

// Name returns the rule name.
func (*UseSlicesConcatRule) Name() string {
	return "use-slices-concat"
}

type lintUseSlicesConcat struct {
	onFailure func(lint.Failure)
}

func (w lintUseSlicesConcat) Visit(n ast.Node) ast.Visitor {
	switch n := n.(type) {
	case *ast.BlockStmt:
		w.checkConsecutiveAppends(n.List)
	case *ast.CaseClause:
		w.checkConsecutiveAppends(n.Body)
	case *ast.CommClause:
		w.checkConsecutiveAppends(n.Body)
	case *ast.CallExpr:
		appended := appendedSlices(n)
		if len(appended) > 1 && allSideEffectFree(appended[1:]) {
			w.addFailure(n, "replace nested appends by a call to slices.Concat")
			// only walk the appended slices, to not report the nested appends again
			for _, slice := range appended {
				ast.Walk(w, slice)
			}

			return nil
		}
	}

	return w
}

// checkConsecutiveAppends spots the
//
//	s := append([]T{}, s1...)
//	s = append(s, s2...)
//
// idiom that can be replaced by a call to [slices.Concat].
func (w lintUseSlicesConcat) checkConsecutiveAppends(stmts []ast.Stmt) {
	for i := 0; i+1 < len(stmts); i++ {
		name, ok := appendToEmptySliceTarget(stmts[i])
		if !ok {
			continue
		}

		if appendsToTarget(stmts[i+1], name) {
			w.addFailure(stmts[i], "replace consecutive appends by a call to slices.Concat")
		}
	}
}

func (w lintUseSlicesConcat) addFailure(node ast.Node, msg string) {
	w.onFailure(lint.Failure{
		Category:   lint.FailureCategoryMaintenance,
		Node:       node,
		Confidence: 0.8,
		Failure:    msg,
	})
}

// appendToEmptySliceTarget returns the name of the variable defined by a
// s := append([]T{}, s1...) statement, if the statement is of that form.
func appendToEmptySliceTarget(stmt ast.Stmt) (name string, ok bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.DEFINE || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return "", false
	}

	target, ok := assign.Lhs[0].(*ast.Ident)
	if !ok || target.Name == "_" {
		return "", false
	}

	// more than one nested append is already reported as nested appends
	if len(appendedSlices(assign.Rhs[0])) != 1 {
		return "", false
	}

	return target.Name, true
}

// appendsToTarget returns true if the statement is of the form target = append(target, s...), false otherwise.
func appendsToTarget(stmt ast.Stmt, target string) bool {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || assign.Tok != token.ASSIGN || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return false
	}

	if !astutils.IsIdent(assign.Lhs[0], target) {
		return false
	}

	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isVariadicAppend(call) || !astutils.IsIdent(call.Args[0], target) {
		return false
	}

	// target = append(target, f(target)...) can not be replaced by a call to slices.Concat
	// because the target is not defined yet in the replacement
	return !usesIdent(call.Args[1], target) && isSideEffectFree(call.Args[1])
}

// allSideEffectFree returns true if none of the given expressions can have side effects, false otherwise.
func allSideEffectFree(exprs []ast.Expr) bool {
	for _, expr := range exprs {
		if !isSideEffectFree(expr) {
			return false
		}
	}

	return true
}

// isSideEffectFree returns true if evaluating the given expression can not modify the program state, false otherwise.
// Appends are merged into a single call to [slices.Concat] that copies the slices only once all of them are evaluated,
// thus an appended slice that can mutate the previously appended ones must not be reported.
func isSideEffectFree(expr ast.Expr) bool {
	switch expr := expr.(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return isSideEffectFree(expr.X)
	case *ast.StarExpr:
		return isSideEffectFree(expr.X)
	case *ast.SelectorExpr:
		return isSideEffectFree(expr.X)
	case *ast.IndexExpr:
		return isSideEffectFree(expr.X) && isSideEffectFree(expr.Index)
	case *ast.SliceExpr:
		return isSideEffectFree(expr.X) &&
			(expr.Low == nil || isSideEffectFree(expr.Low)) &&
			(expr.High == nil || isSideEffectFree(expr.High)) &&
			(expr.Max == nil || isSideEffectFree(expr.Max))
	case *ast.CallExpr:
		// only a conversion to a slice type, e.g. []byte("a string"), is not a call to a function
		_, isSliceConversion := expr.Fun.(*ast.ArrayType)

		return isSliceConversion && len(expr.Args) == 1 && isSideEffectFree(expr.Args[0])
	default:
		return false
	}
}

// usesIdent returns true if the identifier name occurs in the given expression, false otherwise.
func usesIdent(expr ast.Expr, name string) bool {
	idents := astutils.PickNodes(expr, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)

		return ok && ident.Name == name
	})

	return len(idents) > 0
}

// appendedSlices returns the slices spread by nested calls to append into an empty slice literal,
// e.g. it returns [s1, s2] for append(append([]T{}, s1...), s2...).
// It returns nil if the expression is not of that form.
func appendedSlices(expr ast.Expr) []ast.Expr {
	call, ok := expr.(*ast.CallExpr)
	if !ok || !isVariadicAppend(call) {
		return nil
	}

	if isEmptySliceLiteral(call.Args[0]) {
		return []ast.Expr{call.Args[1]}
	}

	nested := appendedSlices(call.Args[0])
	if nested == nil {
		return nil
	}

	return append(nested, call.Args[1])
}

// isVariadicAppend returns true if the call is of the form append(s1, s2...), false otherwise.
// Appends of a string literal to a byte slice, e.g. append(b, "str"...), are excluded because [slices.Concat]
// does not accept a string. Strings given by a variable or by a call are not excluded: telling them from a
// slice requires type information.
func isVariadicAppend(call *ast.CallExpr) bool {
	if !astutils.IsIdent(call.Fun, "append") || !call.Ellipsis.IsValid() || len(call.Args) != 2 {
		return false
	}

	return !astutils.IsStringLiteral(call.Args[1])
}

// isEmptySliceLiteral returns true if the expression is an empty slice literal, e.g. []T{}, false otherwise.
func isEmptySliceLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 0 {
		return false
	}

	sliceType, ok := lit.Type.(*ast.ArrayType)

	return ok && sliceType.Len == nil
}
