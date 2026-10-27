package identical

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

// Analyzer detects interfaces within the same package that have identical methods or type constraints.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "identical",
		Doc:      "Detects interfaces within the same package that have identical methods or type constraints.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/identical",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      r.run,
	}

	analyzer.Flags.BoolVar(&r.debug, "nerd", false, "enable nerd mode")

	return analyzer
}

type runner struct {
	debug bool
}

func (r *runner) run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Collect interface type declarations
	ifaceDecls := make(map[string]token.Pos)
	ifaceTypes := make(map[string]*types.Interface)

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
				// this code is unreachable since we already have guard the token type
				continue
			}

			r.debugf("  -> ts.Type %T\n", ts.Type)

			ifaceType, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}

			if r.debug {
				fmt.Fprintln(os.Stderr, "  -> Interface declaration:", ts.Name.Name, ts.Pos(), len(ifaceType.Methods.List))

				for j, field := range ifaceType.Methods.List {
					switch ft := field.Type.(type) {
					case *ast.FuncType:
						fmt.Fprintf(os.Stderr, "  [%d] Field: func %s %T %v\n", j, field.Names[0].Name, ft, field.Pos())
					case *ast.Ident:
						fmt.Fprintf(os.Stderr, "  [%d] Field: iface %s %T %v\n", j, ft.Name, ft, field.Pos())
					default:
						fmt.Fprintf(os.Stderr, "  [%d] Field: unknown %T\n", j, ft)
					}
				}
			}

			if directive.ShouldIgnore(ts.Doc, pass.Analyzer.Name) {
				continue
			}

			ifaceDecls[ts.Name.Name] = ts.Pos()

			obj := pass.TypesInfo.Defs[ts.Name]
			if obj == nil {
				continue
			}

			iface, ok := obj.Type().Underlying().(*types.Interface)
			if !ok {
				continue
			}

			ifaceTypes[ts.Name.Name] = iface
		}
	})

	identicals := make(map[string][]string)

	for name, typ := range ifaceTypes {
		for otherName, otherTyp := range ifaceTypes {
			if name == otherName {
				continue
			}

			if !types.Identical(typ, otherTyp) {
				continue
			}

			r.debugln("Identical interface:", name, "and", otherName)

			identicals[name] = append(identicals[name], otherName)
		}
	}

	for name, others := range identicals {
		slices.Sort(others)
		otherNames := strings.Join(others, ", ")
		pass.Reportf(ifaceDecls[name], "interface '%s' contains identical methods or type constraints with another interface, causing redundancy (see: %s)", name, otherNames)
	}

	return nil, nil
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
