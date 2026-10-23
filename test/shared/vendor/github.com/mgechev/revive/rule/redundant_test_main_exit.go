package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// RedundantTestMainExitRule suggests removing redundant [os.Exit] or [syscall.Exit] calls in TestMain function.
type RedundantTestMainExitRule struct{}

// Apply applies the rule to given file.
func (*RedundantTestMainExitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	if !file.IsTest() || !file.Pkg.IsAtLeastGoVersion(lint.Go115) {
		// skip analysis for non-test files or for Go versions before 1.15
		return failures
	}

	for _, decl := range file.AST.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv != nil || fd.Body == nil || !astutils.FuncSignatureIs(fd, "TestMain", []string{"*testing.M"}, nil) || len(fd.Type.Params.List[0].Names) != 1 {
			continue
		}

		w := &lintRedundantTestMainExit{
			mName:      fd.Type.Params.List[0].Names[0].Name,
			writes:     map[string]int{},
			runResults: map[string]token.Pos{},
			onFailure: func(failure lint.Failure) {
				failures = append(failures, failure)
			},
		}
		ast.Inspect(fd.Body, w.countWrites)
		w.collectRunResults(fd.Body)
		ast.Inspect(fd.Body, w.checkExitCalls)
	}

	return failures
}

// Name returns the rule name.
func (*RedundantTestMainExitRule) Name() string {
	return "redundant-test-main-exit"
}

type lintRedundantTestMainExit struct {
	mName      string               // name of the *testing.M parameter
	writes     map[string]int       // number of writes to each variable
	runResults map[string]token.Pos // end of the unconditional statement assigning the result of m.Run to each variable
	onFailure  func(lint.Failure)
}

// countWrites counts every write to a variable: only a variable written exactly once, from m.Run, holds its result.
func (w *lintRedundantTestMainExit) countWrites(node ast.Node) bool {
	switch n := node.(type) {
	case *ast.AssignStmt:
		for _, lhs := range n.Lhs {
			w.countWrite(lhs)
		}
	case *ast.ValueSpec:
		if len(n.Values) == 0 {
			return true // declaration without a value is not a write
		}
		for _, name := range n.Names {
			w.countWrite(name)
		}
	case *ast.IncDecStmt:
		w.countWrite(n.X)
	case *ast.UnaryExpr:
		if n.Op == token.AND {
			w.countWrite(n.X)
		}
	case *ast.RangeStmt:
		w.countWrite(n.Key)
		w.countWrite(n.Value)
	}

	return true
}

func (w *lintRedundantTestMainExit) countWrite(target ast.Expr) {
	id, ok := ast.Unparen(target).(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}

	w.writes[id.Name]++
}

// collectRunResults records variables assigned from m.Run by a top-level statement of the TestMain body,
// i.e. one that is guaranteed to execute before any statement that follows it.
func (w *lintRedundantTestMainExit) collectRunResults(body *ast.BlockStmt) {
	for _, stmt := range body.List {
		var name *ast.Ident
		var value ast.Expr
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) != 1 || len(s.Rhs) != 1 || (s.Tok != token.ASSIGN && s.Tok != token.DEFINE) {
				continue
			}
			name, _ = ast.Unparen(s.Lhs[0]).(*ast.Ident)
			value = s.Rhs[0]
		case *ast.DeclStmt:
			gd, ok := s.Decl.(*ast.GenDecl)
			if !ok || len(gd.Specs) != 1 {
				continue
			}
			vs, ok := gd.Specs[0].(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			name = vs.Names[0]
			value = vs.Values[0]
		default:
			continue
		}

		if name != nil && w.isRunCall(value) {
			w.runResults[name.Name] = stmt.End()
		}
	}
}

// checkExitCalls reports [os.Exit] and [syscall.Exit] calls whose argument is the result of m.Run.
func (w *lintRedundantTestMainExit) checkExitCalls(node ast.Node) bool {
	ce, ok := node.(*ast.CallExpr)
	if !ok {
		return true
	}

	var pkg string
	switch {
	case astutils.IsPkgDotName(ce.Fun, "os", "Exit"):
		pkg = "os"
	case astutils.IsPkgDotName(ce.Fun, "syscall", "Exit"):
		pkg = "syscall"
	default:
		return true
	}

	if len(ce.Args) != 1 || !w.isRunResult(ce.Args[0]) {
		return true
	}

	w.onFailure(lint.Failure{
		Confidence: 1,
		Node:       ce,
		Category:   lint.FailureCategoryStyle,
		Failure:    fmt.Sprintf("redundant call to %s.Exit in TestMain function, the test runner will handle it automatically as of Go 1.15", pkg),
	})

	return true
}

// isRunResult returns true if expr is a call to m.Run or a variable whose only write, before expr, is the result of m.Run.
func (w *lintRedundantTestMainExit) isRunResult(expr ast.Expr) bool {
	expr = ast.Unparen(expr)
	id, ok := expr.(*ast.Ident)
	if !ok {
		return w.isRunCall(expr)
	}

	assignedAt, ok := w.runResults[id.Name]
	return ok && w.writes[id.Name] == 1 && assignedAt < expr.Pos()
}

func (w *lintRedundantTestMainExit) isRunCall(expr ast.Expr) bool {
	ce, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}

	se, ok := ce.Fun.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != "Run" {
		return false
	}

	return astutils.IsIdent(ast.Unparen(se.X), w.mName)
}
