package opaque

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

	"github.com/uudashr/iface/internal/directive"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer detects functions that return an interface type, but only ever return a single concrete implementation.
var Analyzer = newAnalyzer()

func newAnalyzer() *analysis.Analyzer {
	r := runner{}

	analyzer := &analysis.Analyzer{
		Name:     "opaque",
		Doc:      "Detects functions that return an interface type, but only ever return a single concrete implementation.",
		URL:      "https://pkg.go.dev/github.com/uudashr/iface/opaque",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      r.run,
	}

	analyzer.Flags.BoolVar(&r.debug, "nerd", false, "enable nerd mode")

	return analyzer
}

type runner struct {
	debug bool
}

func (r *runner) run(pass *analysis.Pass) (any, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	// Find function declarations that return an interface

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		funcDecl := n.(*ast.FuncDecl)

		if funcDecl.Recv != nil {
			// Skip methods because their return types may be dictated by interface
			// contracts that the receiver type satisfies. Unlike standalone functions,
			// methods are often created to fulfill an interface, so the return type
			// is not a free design choice. Flagging these would produce false positives.
			return
		}

		if funcDecl.Body == nil {
			// skip functions without body
			return
		}

		if funcDecl.Type.Results == nil {
			// skip functions without return values
			return
		}

		if r.debug {
			fmt.Fprintf(os.Stderr, "Function declaration %s\n", funcDecl.Name.Name)
			fmt.Fprintf(os.Stderr, " Results len=%d\n", len(funcDecl.Type.Results.List))
		}

		if directive.ShouldIgnore(funcDecl.Doc, pass.Analyzer.Name) {
			return
		}

		var (
			hasInterfaceReturnType bool
			outCount               int
		)

		namedReturnObjs := make(map[*types.Var]int)
		{
			idx := 0

			for i, result := range funcDecl.Type.Results.List {
				if r.debug {
					fmt.Fprintf(os.Stderr, "  result[%d] %v %T names=%v\n", i, result.Type, result.Type, result.Names)
				}

				for j, name := range result.Names {
					if obj := pass.TypesInfo.Defs[name]; obj != nil {
						r.debugf("   name[%d] %v def=%v defAddr=%p\n", j, name, obj, obj)

						if v, ok := obj.(*types.Var); ok {
							namedReturnObjs[v] = idx
						}
					} else {
						r.debugf("   name[%d] %v def=unknown\n", j, name)
					}

					idx++
				}

				if len(result.Names) == 0 {
					idx++
				}

				outCount += max(1, len(result.Names))

				if !hasInterfaceReturnType {
					typ := pass.TypesInfo.TypeOf(result.Type)
					hasInterfaceReturnType = typ != nil && types.IsInterface(typ)
				}
			}
		}

		r.debugf("  hasInterface=%t outCount=%d\n", hasInterfaceReturnType, outCount)

		if !hasInterfaceReturnType {
			// skip, since it has no interface return type
			return
		}

		// Collect types on every return statement
		retStmtTypes := make([]map[types.Type]struct{}, outCount)
		for i := range retStmtTypes {
			retStmtTypes[i] = make(map[types.Type]struct{})
		}

		r.debugln(" Body")
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncLit:
				r.debugln("  FuncLit")

				return false
			case *ast.AssignStmt:
				if n.Tok != token.ASSIGN {
					return true
				}

				if r.debug {
					r.debugf("  AssignStmt token %q lhsLen=%d rhsLen=%d\n", n.Tok, len(n.Lhs), len(n.Rhs))
				}

				for i, lhs := range n.Lhs {
					r.debugf("   lhs[%d] %v %T\n", i, lhs, lhs)

					ident, ok := lhs.(*ast.Ident)
					if !ok {
						continue
					}

					obj := pass.TypesInfo.Uses[ident]
					r.debugf("    -> typeObj %v\n", obj)

					if obj == nil {
						continue
					}

					v, ok := obj.(*types.Var)
					r.debugf("    -> types.Var=%t\n", ok)

					if !ok {
						continue
					}

					pos, found := namedReturnObjs[v]
					r.debugf("    -> namedFound=%t\n", found)

					if !found {
						continue
					}

					var typ types.Type

					if r.debug {
						fmt.Fprintf(os.Stderr, "    -> rhsLen=%d\n", len(n.Rhs))
					}

					switch {
					case len(n.Rhs) == 1:
						typ = pass.TypesInfo.TypeOf(n.Rhs[0])
						if tuple, ok := typ.(*types.Tuple); ok {
							if i < tuple.Len() {
								typ = tuple.At(i).Type()
							} else {
								typ = nil
							}
						}

						r.debugf("     -> rhs[0] typ %v %T\n", typ, typ)
					case i < len(n.Rhs):
						typ = pass.TypesInfo.TypeOf(n.Rhs[i])
						r.debugf("     -> rhs[%d] typ %v %T\n", i, typ, typ)
					default:
						typ = nil

						r.debugf("    -> rhs default\n")
					}

					if typ != nil && !isUntypedNil(typ) {
						retStmtTypes[pos][typ] = struct{}{}
					}
				}

				return true
			case *ast.ReturnStmt:
				if r.debug {
					fmt.Fprintf(os.Stderr, "  ReturnStmt results %v len=%d\n", n.Results, len(n.Results))
				}

				for i, result := range n.Results {
					r.debugf("   [%d] %v %T\n", i, result, result)

					switch res := result.(type) {
					case *ast.CallExpr:
						// Multi-return calls produce a Tuple; unwrap each element to
						// record types by position. Single-value calls fall to default.
						r.debugf("       CallExpr Fun: %v %T\n", res.Fun, res.Fun)

						typ := pass.TypesInfo.TypeOf(res)
						switch typ := typ.(type) {
						case *types.Tuple:
							for j := range typ.Len() {
								v := typ.At(j)
								vTyp := v.Type()
								retStmtTypes[j][vTyp] = struct{}{}

								if r.debug {
									fmt.Fprintf(os.Stderr, "          Tuple [%d]: %v %T | %v %T interface=%t\n", j, v, v, vTyp, vTyp, types.IsInterface(vTyp))
								}
							}
						default:
							retStmtTypes[i][typ] = struct{}{}
						}

					case *ast.Ident:
						// Skip untyped nil — not a concrete type. All other identifiers
						// (variables, constants, etc.) are recorded as-is.
						r.debugf("       Ident: %v %T\n", res, res)

						if obj := pass.TypesInfo.Uses[res]; obj != nil {
							if v, ok := obj.(*types.Var); ok {
								if _, found := namedReturnObjs[v]; found {
									break
								}
							}
						}

						typ := pass.TypesInfo.TypeOf(res)
						isNilStmt := isUntypedNil(typ)

						if r.debug {
							fmt.Fprintf(os.Stderr, "        Ident type: %v %T interface=%t, untypedNil=%t\n", typ, typ, types.IsInterface(typ), isNilStmt)
						}

						if !isNilStmt {
							retStmtTypes[i][typ] = struct{}{}
						}
					default:
						// Catches everything else: UnaryExpr (`&foo`), SelectorExpr,
						// TypeAssertExpr, CompositeLit, etc. pass.TypesInfo.TypeOf(res)
						// resolves the type correctly regardless of AST node kind.
						r.debugf("       OtherExpr: %v %T\n", res, res)

						typ := pass.TypesInfo.TypeOf(res)
						retStmtTypes[i][typ] = struct{}{}
					}
				}

				return false
			default:
				return true
			}
		})

		// Compare func return types with the return statement types
		var nextIdx int

		for _, result := range funcDecl.Type.Results.List {
			resType := result.Type
			typ := pass.TypesInfo.TypeOf(resType)

			consumeCount := 1
			if len(result.Names) > 0 {
				consumeCount = len(result.Names)
			}

			currentIdx := nextIdx
			nextIdx += consumeCount

			// Check return type
			if !types.IsInterface(typ) {
				// it is a concrete type
				continue
			}

			errorType := types.Universe.Lookup("error").Type()
			anyType := types.Universe.Lookup("any").Type()

			if types.Identical(typ, errorType) || types.Identical(typ, anyType) {
				continue
			}

			if !fromSamePackage(pass, typ) {
				// ignore interface from other package
				continue
			}

			// Check statement type
			stmtTyps := retStmtTypes[currentIdx]

			stmtTypsSize := len(stmtTyps)
			if stmtTypsSize > 1 {
				// it has multiple implementation
				continue
			}

			if stmtTypsSize == 0 {
				// function use named return value, while return statement is empty
				continue
			}

			var stmtTyp types.Type
			for t := range stmtTyps {
				// expect only one, we don't have to break it
				stmtTyp = t
			}

			if types.IsInterface(stmtTyp) {
				// not concrete type, skip
				continue
			}

			if r.debug {
				fmt.Fprintf(os.Stderr, "stmtType: %v %T | %v %T\n", stmtTyp, stmtTyp, stmtTyp.Underlying(), stmtTyp.Underlying())
			}

			switch stmtTyp := stmtTyp.(type) {
			case *types.Basic:
				if stmtTyp.Kind() == types.UntypedNil {
					// ignore nil
					continue
				}
			case *types.Named:
				if _, ok := stmtTyp.Underlying().(*types.Signature); ok {
					// skip function type
					continue
				}
			}

			retTypeName := typ.String()
			if fromSamePackage(pass, typ) {
				retTypeName = removePkgPrefix(retTypeName)
			}

			stmtTypName := stmtTyp.String()
			if fromSamePackage(pass, stmtTyp) {
				stmtTypName = removePkgPrefix(stmtTypName)
			}

			msg := fmt.Sprintf("'%s' function return '%s' interface at the %s result, abstract a single concrete implementation of '%s'",
				funcDecl.Name.Name,
				retTypeName,
				positionStr(currentIdx),
				stmtTypName)

			pass.Report(analysis.Diagnostic{
				Pos:     result.Type.Pos(),
				Message: msg,
				SuggestedFixes: []analysis.SuggestedFix{
					{
						Message: "Replace the interface return type with the concrete type",
						TextEdits: []analysis.TextEdit{
							{
								Pos:     result.Type.Pos(),
								End:     result.Type.End(),
								NewText: []byte(stmtTypName),
							},
						},
					},
				},
			})
		}
	})

	return nil, nil
}

func isUntypedNil(typ types.Type) bool {
	if b, ok := typ.(*types.Basic); ok {
		return b.Kind() == types.UntypedNil
	}

	return false
}

func positionStr(idx int) string {
	switch idx {
	case 0:
		return "1st"
	case 1:
		return "2nd"
	case 2:
		return "3rd"
	default:
		return fmt.Sprintf("%dth", idx+1)
	}
}

func fromSamePackage(pass *analysis.Pass, typ types.Type) bool {
	switch typ := typ.(type) {
	case *types.Named:
		currentPkg := pass.Pkg
		ifacePkg := typ.Obj().Pkg()

		return currentPkg == ifacePkg
	case *types.Pointer:
		return fromSamePackage(pass, typ.Elem())
	default:
		return false
	}
}

func removePkgPrefix(typeStr string) string {
	if len(typeStr) == 0 {
		return typeStr
	}

	if typeStr[0] == '*' {
		return "*" + removePkgPrefix(typeStr[1:])
	}

	if lastDot := strings.LastIndex(typeStr, "."); lastDot != -1 {
		return typeStr[lastDot+1:]
	}

	return typeStr
}

func (r *runner) debugf(format string, a ...any) {
	if r.debug {
		fmt.Fprintf(os.Stderr, format, a...)
	}
}

func (r *runner) debugln(a ...any) {
	if r.debug {
		fmt.Fprintln(os.Stderr, a...)
	}
}
