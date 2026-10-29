package rule

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/mgechev/revive/internal/astutils"
	"github.com/mgechev/revive/lint"
)

// IdenticalSwitchBranchesRule warns on identical switch branches.
type IdenticalSwitchBranchesRule struct {
	allowIdenticalDefault bool // allow the default clause to be identical to a case clause
}

var _ lint.ConfigurableRule = (*IdenticalSwitchBranchesRule)(nil)

// Configure validates the rule configuration, and configures the rule accordingly.
//
// Configuration implements the [lint.ConfigurableRule] interface.
func (r *IdenticalSwitchBranchesRule) Configure(arguments lint.Arguments) error {
	r.allowIdenticalDefault = false

	if len(arguments) < 1 {
		return nil // use defaults
	}

	argKV, ok := arguments[0].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid argument to the %s rule. Expecting a k,v map, got %T", r.Name(), arguments[0])
	}

	for k, v := range argKV {
		switch {
		case isRuleOption(k, "allowIdenticalDefault"):
			allow, ok := v.(bool)
			if !ok {
				return fmt.Errorf("invalid configuration value for %q in %s rule; need bool but got %T", k, r.Name(), v)
			}
			r.allowIdenticalDefault = allow
		default:
			return fmt.Errorf(`invalid argument %q for rule %s; expected "allow-identical-default"`, k, r.Name())
		}
	}

	return nil
}

// Apply applies the rule to given file.
func (r *IdenticalSwitchBranchesRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	getStmtLine := func(s ast.Stmt) int {
		return file.ToPosition(s.Pos()).Line
	}

	w := &lintIdenticalSwitchBranches{
		getStmtLine:           getStmtLine,
		onFailure:             onFailure,
		allowIdenticalDefault: r.allowIdenticalDefault,
	}
	for _, decl := range file.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		ast.Walk(w, fn.Body)
	}

	return failures
}

// Name returns the rule name.
func (*IdenticalSwitchBranchesRule) Name() string {
	return "identical-switch-branches"
}

type lintIdenticalSwitchBranches struct {
	getStmtLine           func(ast.Stmt) int
	onFailure             func(lint.Failure)
	allowIdenticalDefault bool
}

func (w *lintIdenticalSwitchBranches) Visit(node ast.Node) ast.Visitor {
	switchStmt, ok := node.(*ast.SwitchStmt)
	if !ok {
		return w
	}

	if switchStmt.Tag == nil {
		return w // do not lint untagged switches (order of case evaluation might be important)
	}

	doesFallthrough := func(stmts []ast.Stmt) bool {
		if len(stmts) == 0 {
			return false
		}

		ft, ok := stmts[len(stmts)-1].(*ast.BranchStmt)
		return ok && ft.Tok == token.FALLTHROUGH
	}

	// A case clause with no expression list is the default clause.
	isDefault := func(cc *ast.CaseClause) bool { return cc.List == nil }

	hashes := map[string]int{} // map hash(branch code) -> branch line
	for _, cc := range switchStmt.Body.List {
		caseClause := cc.(*ast.CaseClause)
		if doesFallthrough(caseClause.Body) {
			continue // skip fallthrough branches
		}

		if w.allowIdenticalDefault && isDefault(caseClause) {
			// Spelling out a fallback that repeats a listed case is a documented choice: it says
			// which values are explicitly handled and that anything else lands on the same code.
			// Walk into it so nested switches are still analyzed, but neither report it nor record
			// its hash, so a later clause cannot be reported against it either.
			ast.Walk(w, &ast.BlockStmt{List: caseClause.Body})
			continue
		}
		branch := &ast.BlockStmt{
			List: caseClause.Body,
		}
		hash := astutils.NodeHash(branch)
		branchLine := w.getStmtLine(caseClause)
		if matchLine, ok := hashes[hash]; ok {
			w.onFailure(lint.Failure{
				Confidence: 1.0,
				Node:       node,
				Category:   lint.FailureCategoryLogic,
				Failure:    fmt.Sprintf(`"switch" with identical branches (lines %d and %d)`, matchLine, branchLine),
			})
		}

		hashes[hash] = branchLine
		ast.Walk(w, branch)
	}

	return nil // switch branches already analyzed
}
