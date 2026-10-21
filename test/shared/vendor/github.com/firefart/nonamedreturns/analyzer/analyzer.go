package analyzer

import (
	"errors"
	"flag"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const (
	FlagReportErrorInDefer      = "report-error-in-defer"
	FlagAllowUnusedNamedReturns = "allow-unused-named-returns"
)

var Analyzer = &analysis.Analyzer{
	Name:     "nonamedreturns",
	Doc:      "Reports all named returns",
	Flags:    flags(),
	Run:      run,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
}

func flags() flag.FlagSet {
	fs := flag.FlagSet{}
	fs.Bool(FlagReportErrorInDefer, false, "report named error if it is assigned inside defer")
	fs.Bool(FlagAllowUnusedNamedReturns, false, "allow named returns in the signature but report them if referenced in the body or used by a naked return")
	return fs
}

func run(pass *analysis.Pass) (any, error) {
	reportErrorInDefer := pass.Analyzer.Flags.Lookup(FlagReportErrorInDefer).Value.String() == "true"
	allowUnusedNamedReturns := pass.Analyzer.Flags.Lookup(FlagAllowUnusedNamedReturns).Value.String() == "true"
	errorType := types.Universe.Lookup("error").Type()

	inspector, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("failed to get inspector")
	}

	// only filter function definitions
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	inspector.Preorder(nodeFilter, func(node ast.Node) {
		var funcResults *ast.FieldList
		var funcBody *ast.BlockStmt

		switch n := node.(type) {
		case *ast.FuncLit:
			funcResults = n.Type.Results
			funcBody = n.Body
		case *ast.FuncDecl:
			funcResults = n.Type.Results
			funcBody = n.Body
		default:
			return
		}

		// Function without body, ex: https://github.com/golang/go/blob/master/src/internal/syscall/unix/net.go
		if funcBody == nil {
			return
		}

		// no return values
		if funcResults == nil {
			return
		}

		if allowUnusedNamedReturns {
			var referenced map[types.Object]bool
			var hasNakedReturn bool
			usageComputed := false

			for _, p := range funcResults.List {
				if len(p.Names) == 0 {
					continue
				}

				for _, n := range p.Names {
					if n.Name == "_" {
						continue
					}

					if !usageComputed {
						referenced, hasNakedReturn = collectNamedReturnUsage(funcBody, pass.TypesInfo)
						usageComputed = true
					}

					obj := pass.TypesInfo.ObjectOf(n)
					if referenced[obj] || hasNakedReturn {
						pass.Reportf(n.Pos(), "named return %q with type %q must not be referenced or used by a naked return", n.Name, types.ExprString(p.Type))
					}
				}
			}

			return
		}

		// existing behavior: report every named return (subject to the
		// error-in-defer exemption)

		// deferUsed, assigned and hasReturnWithResults are computed lazily and
		// at most once per function: the relatively expensive body walk only
		// happens when there is an error-typed named return whose defer
		// exemption we need to check.
		var deferUsed, assigned map[types.Object]bool
		var hasReturnWithResults bool

		for _, p := range funcResults.List {
			if len(p.Names) == 0 {
				// all good, the parameter is not named
				continue
			}

			isError := types.Identical(pass.TypesInfo.TypeOf(p.Type), errorType)

			for _, n := range p.Names {
				if n.Name == "_" {
					continue
				}

				// A named error return is allowed when it is referenced inside a defer
				// (e.g. to inspect or modify it before returning) as long as it is also
				// assigned somewhere in the function body. The assignment may happen
				// inside the defer itself, anywhere else in the function, or implicitly
				// via a `return` statement with result values (which assigns every
				// named return before the defers run).
				if !reportErrorInDefer && isError {
					if deferUsed == nil {
						deferUsed, assigned, hasReturnWithResults = collectDeferUsageAndAssignments(funcBody, pass.TypesInfo)
					}

					obj := pass.TypesInfo.ObjectOf(n)
					if deferUsed[obj] && (assigned[obj] || hasReturnWithResults) {
						continue
					}
				}

				pass.Reportf(n.Pos(), "named return %q with type %q found", n.Name, types.ExprString(p.Type))
			}
		}
	})

	return nil, nil // nolint:nilnil
}

// collectDeferUsageAndAssignments walks body a single time and returns:
//   - deferUsed: objects referenced (read or written) inside a deferred func literal
//   - assigned:  objects that appear on the left-hand side of an assignment
//     (including `for ... = range` statements) anywhere in the body, including
//     inside a deferred func literal
//   - hasReturnWithResults: true if the body contains a `return` statement with
//     result values at the top level of the function. Such a return implicitly
//     assigns every named return before the defers run, so it counts as an
//     assignment for all of them. Returns inside nested closures (including the
//     deferred closure itself) populate the closure's own results and are
//     therefore excluded.
//
// Variable shadowing is handled naturally: a shadowed declaration introduces a
// distinct types.Object, so it does not match the outer named return.
func collectDeferUsageAndAssignments(body *ast.BlockStmt, info *types.Info) (map[types.Object]bool, map[types.Object]bool, bool) {
	deferUsed := make(map[types.Object]bool)
	assigned := make(map[types.Object]bool)
	hasReturnWithResults := false

	markAssigned := func(expr ast.Expr) {
		if i, ok := expr.(*ast.Ident); ok {
			if obj := info.ObjectOf(i); obj != nil {
				assigned[obj] = true
			}
		}
	}

	var walk func(node ast.Node, inClosure bool)
	walk = func(node ast.Node, inClosure bool) {
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, lh := range x.Lhs {
					markAssigned(lh)
				}
			case *ast.RangeStmt:
				if x.Tok == token.ASSIGN {
					if x.Key != nil {
						markAssigned(x.Key)
					}
					if x.Value != nil {
						markAssigned(x.Value)
					}
				}
			case *ast.ReturnStmt:
				// a return inside a closure populates the closure's own
				// results, not the enclosing function's named returns
				if !inClosure && len(x.Results) > 0 {
					hasReturnWithResults = true
				}
			case *ast.DeferStmt:
				if fn, ok := x.Call.Fun.(*ast.FuncLit); ok {
					ast.Inspect(fn.Body, func(inner ast.Node) bool {
						if i, ok := inner.(*ast.Ident); ok {
							if obj := info.ObjectOf(i); obj != nil {
								deferUsed[obj] = true
							}
						}
						return true
					})
				}
				// keep descending (via the FuncLit case below) so assignments
				// and nested defers inside the deferred closure are still
				// collected by the outer walk
			case *ast.FuncLit:
				// everything inside a closure (deferred or not) is in-closure
				// for return purposes, but assignments to captured named
				// returns still count
				walk(x.Body, true)
				return false
			}
			return true
		})
	}
	walk(body, false)

	return deferUsed, assigned, hasReturnWithResults
}

// collectNamedReturnUsage walks body a single time and returns:
//   - referenced:     objects referenced (read or written) anywhere in the body,
//     including inside nested closures (which may capture the outer named return)
//   - hasNakedReturn: true if the body contains a return statement with no result
//     expressions at the top level of the function (naked returns inside nested
//     closures are excluded because they populate the closure's own results, not
//     the enclosing function's named returns)
//
// Shadowing is handled naturally: a shadowed declaration introduces a distinct
// types.Object, so it does not match the outer named return.
func collectNamedReturnUsage(body *ast.BlockStmt, info *types.Info) (map[types.Object]bool, bool) {
	referenced := make(map[types.Object]bool)
	hasNakedReturn := false

	var walk func(node ast.Node, inClosure bool)
	walk = func(node ast.Node, inClosure bool) {
		ast.Inspect(node, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if obj := info.ObjectOf(x); obj != nil {
					referenced[obj] = true
				}
			case *ast.ReturnStmt:
				// a naked return inside a closure populates the closure's
				// own results, not the enclosing function's named returns
				if !inClosure && len(x.Results) == 0 {
					hasNakedReturn = true
				}
			case *ast.FuncLit:
				// keep collecting referenced identifiers (closures can
				// capture the outer named return), but everything inside
				// the closure is in-closure for naked-return purposes
				walk(x.Body, true)
				return false
			}
			return true
		})
	}
	walk(body, false)

	return referenced, hasNakedReturn
}
