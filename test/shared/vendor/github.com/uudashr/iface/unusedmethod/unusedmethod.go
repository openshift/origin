package unusedmethod

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"slices"
	"strings"

	"github.com/uudashr/iface/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "unusedmethod",
		Doc:      "Detects interface methods that are never used anywhere in the same package where they are defined. A method is considered used only when invoked or referenced through a value of the interface type; merely implementing the interface does not count as a use. Exported methods are reported without a suggested fix, since they may be used by other packages.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/unusedmethod",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      r.run,
	}

	analyzer.Flags.BoolVar(&r.debug, "nerd", false, "enable nerd mode")
	analyzer.Flags.StringVar(&r.exclude, "exclude", "", "comma-separated list of packages to exclude from the check")

	return analyzer
}

type methodEntry struct {
	ifaceName string
	field     *ast.Field
}

type runner struct {
	debug   bool
	exclude string
}

func (r *runner) run(pass *analysis.Pass) (any, error) {
	var excludes []string

	if r.exclude != "" {
		for _, pkg := range strings.Split(r.exclude, ",") {
			if p := strings.TrimSpace(pkg); p != "" {
				excludes = append(excludes, p)
			}
		}
	}

	if slices.Contains(excludes, pass.Pkg.Path()) {
		return nil, nil
	}

	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	unusedMethods := make(map[*types.Func]methodEntry)

	nodeFilter := []ast.Node{
		(*ast.GenDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		decl, ok := n.(*ast.GenDecl)
		if !ok {
			return
		}

		if r.debug {
			fmt.Fprintf(os.Stderr, "GenDecl: %v specs=%d\n", decl.Tok, len(decl.Specs))
		}

		if decl.Tok != token.TYPE {
			return
		}

		if directive.ShouldIgnore(decl.Doc, pass.Analyzer.Name) {
			return
		}

		for i, spec := range decl.Specs {
			r.debugf(" spec[%d]: %v %T\n", i, spec, spec)

			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			ifaceType, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			r.debugln(" -> Interface type declaration:", ts.Name.Name)

			if directive.ShouldIgnore(ts.Doc, pass.Analyzer.Name) {
				continue
			}

			for j, field := range ifaceType.Methods.List {
				switch ft := field.Type.(type) {
				case *ast.FuncType:
					name := field.Names[0]
					obj := pass.TypesInfo.Defs[name]

					if r.debug {
						fmt.Fprintf(os.Stderr, "  [%d] Field: func %s %T\n", j, name.Name, ft)
						fmt.Fprintf(os.Stderr, "   obj: %v %T %p\n", obj, obj, obj)
					}

					if directive.ShouldIgnore(field.Doc, pass.Analyzer.Name) {
						continue
					}

					if directive.ShouldIgnore(field.Comment, pass.Analyzer.Name) {
						continue
					}

					if fn, ok := obj.(*types.Func); ok {
						unusedMethods[fn.Origin()] = methodEntry{
							ifaceName: ts.Name.Name,
							field:     field,
						}
					}
				default:
					r.debugf("  [%d] Field: unknown %v %T\n", j, ft, ft)
				}
			}
		}
	})

	// Method is always SelectorExpr
	nodeFilter = []ast.Node{
		(*ast.SelectorExpr)(nil),
	}

	r.debugln("Method usage")
	inspect.Preorder(nodeFilter, func(n ast.Node) {
		r.debugf(" n %v %T\n", n, n)

		selExp, ok := n.(*ast.SelectorExpr)
		if !ok {
			return
		}

		r.debugf("  selExp sel: %v %T, x: %v %T\n", selExp.Sel, selExp.Sel, selExp.X, selExp.X)

		sel, ok := pass.TypesInfo.Selections[selExp]
		if !ok {
			return
		}

		if r.debug {
			fmt.Fprintf(os.Stderr, "   sel -> %v %T, kind: %v %T, obj: %v %T\n", sel, sel, sel.Kind(), sel.Kind(), sel.Obj(), sel.Obj())
		}

		fn, ok := sel.Obj().(*types.Func)
		if !ok {
			return
		}

		delete(unusedMethods, fn.Origin())
	})

	if r.debug {
		fmt.Fprintf(os.Stderr, "Unused methods %d\n", len(unusedMethods))
	}

	for fn, entry := range unusedMethods {
		if r.debug {
			fmt.Fprintf(os.Stderr, " %v %T %s\n", fn, fn, fn.Name())
		}

		msg := fmt.Sprintf("method '%s()' is declared on interface '%s' but not used within the package", fn.Name(), entry.ifaceName)

		diag := analysis.Diagnostic{
			Pos:     entry.field.Pos(),
			Message: msg,
		}

		if fix := removalFix(entry.field); fix != nil {
			diag.SuggestedFixes = []analysis.SuggestedFix{*fix}
		}

		pass.Report(diag)
	}

	return nil, nil
}

func removalFix(field *ast.Field) *analysis.SuggestedFix {
	if field.Names[0].IsExported() {
		return nil
	}

	pos := field.Pos()
	if doc := field.Doc; doc != nil {
		pos = doc.Pos()
	}

	end := field.End()
	if comment := field.Comment; comment != nil {
		end = comment.End()
	}

	return &analysis.SuggestedFix{
		Message: "Remove the unused method",
		TextEdits: []analysis.TextEdit{
			{
				Pos:     pos,
				End:     end,
				NewText: []byte{},
			},
		},
	}
}

func (r *runner) debugln(a ...any) {
	if r.debug {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func (r *runner) debugf(format string, a ...any) {
	if r.debug {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}
