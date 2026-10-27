package canonicalheader

import (
	"flag"
	"fmt"
	"go/ast"
	"go/types"
	"net/http"

	"github.com/go-toolsmith/astcast"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	httpHeaderPkg  = "net/http"
	httpHeaderName = "Header"
)

//nolint:gochecknoglobals // for backward compatibility.
var Analyzer = New()

func New() *analysis.Analyzer {
	c := &canonicalHeader{}

	a := &analysis.Analyzer{
		Name:     "canonicalheader",
		Doc:      "canonicalheader checks whether net/http.Header uses canonical header",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      c.run,
	}

	a.Flags.Init("canonicalheader", flag.ExitOnError)
	a.Flags.Var(&c.exclusions, "exclusions", "comma-separated list of exclusion rules")
	a.Flags.BoolVar(&c.useDefaultExclusion, "useDefaultExclusion", true, "use default exclusion rules")

	return a
}

type canonicalHeader struct {
	useDefaultExclusion bool
	exclusions          stringSet
}

type argumenter interface {
	diagnostic(canonicalHeader string) analysis.Diagnostic
	value() string
}

func (c *canonicalHeader) buildExclusions() map[string]string {
	var defEx map[string]string
	if c.useDefaultExclusion {
		defEx = initialism()
	}

	result := make(map[string]string, len(c.exclusions)+len(defEx))

	for canonical, noCanonical := range defEx {
		result[canonical] = noCanonical
	}

	for _, ex := range c.exclusions {
		result[http.CanonicalHeaderKey(ex)] = ex
	}

	return result
}

func (c *canonicalHeader) run(pass *analysis.Pass) (any, error) {
	spctor, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, fmt.Errorf("want %T, got %T", spctor, pass.ResultOf[inspect.Analyzer])
	}

	exclusions := c.buildExclusions()

	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
	}

	spctor.Preorder(nodeFilter, func(n ast.Node) {
		callExp, ok := n.(*ast.CallExpr)
		if !ok {
			return
		}

		var (
			// gotType type receiver.
			gotType       types.Type
			gotMethodName string
		)

		switch t := typeutil.Callee(pass.TypesInfo, callExp).(type) {
		// Direct call method.
		case *types.Func:
			fn := t
			// Find net/http.Header{} by function call.
			signature, ok := fn.Type().(*types.Signature)
			if !ok {
				return
			}

			recv := signature.Recv()

			// It's a func, not a method.
			if recv == nil {
				return
			}

			gotType = recv.Type()

			sel := astcast.ToSelectorExpr(callExp.Fun).Sel
			if sel == nil {
				return
			}

			gotMethodName = sel.Name

		// h := http.Header{}
		// f := h.Get
		// v("Test-Value").
		case *types.Var:
			ident, ok := callExp.Fun.(*ast.Ident)
			if !ok {
				return
			}

			if ident.Obj == nil {
				return
			}

			// f := h.Get.
			assign, ok := ident.Obj.Decl.(*ast.AssignStmt)
			if !ok {
				return
			}

			// For case `i, v := 0, h.Get`.
			// indexAssign--^.
			indexAssign := -1
			for i, lh := range assign.Lhs {
				// Find by name of variable.
				if astcast.ToIdent(lh).Name == ident.Name {
					indexAssign = i
				}
			}

			// Not found.
			if indexAssign == -1 {
				return
			}

			if len(assign.Rhs) <= indexAssign {
				return
			}

			sel, ok := assign.Rhs[indexAssign].(*ast.SelectorExpr)
			if !ok {
				return
			}

			if sel.Sel == nil {
				return
			}

			gotMethodName = sel.Sel.Name
			ident, ok = sel.X.(*ast.Ident)
			if !ok {
				return
			}

			obj := pass.TypesInfo.ObjectOf(ident)
			gotType = obj.Type()

		default:
			return
		}

		// It is not net/http.Header{}.
		if !isHTTPHeader(gotType) {
			return
		}

		// Search for known methods where the key is the first arg.
		if !isValidMethod(gotMethodName) {
			return
		}

		// Should be more than one. Because get the value by index it
		// will not be superfluous.
		if len(callExp.Args) == 0 {
			return
		}

		callArg := callExp.Args[0]

		// Check for type casting from myString to string.
		// it could be: Get(string(string(string(myString)))).
		// need this node------------------------^^^^^^^^.
		for {
			// If it is not *ast.CallExpr, this is a value.
			c, ok := callArg.(*ast.CallExpr)
			if !ok {
				break
			}

			// Some function is called, skip this case.
			if len(c.Args) == 0 {
				return
			}

			f, ok := c.Fun.(*ast.Ident)
			if !ok {
				break
			}

			obj := pass.TypesInfo.ObjectOf(f)
			// nil may be by code, but not by logic.
			// TypeInfo should contain of type.
			if obj == nil {
				break
			}

			// This is function.
			// Skip this method call.
			_, ok = obj.Type().(*types.Signature)
			if ok {
				return
			}

			callArg = c.Args[0]
		}

		var arg argumenter
		switch t := callArg.(type) {
		case *ast.BasicLit:
			lString, err := newLiteralString(t)
			if err != nil {
				return
			}

			arg = lString

		case *ast.Ident:
			constString, err := newConstantKey(pass.TypesInfo, t)
			if err != nil {
				return
			}

			arg = constString

		default:
			return
		}

		argValue := arg.value()

		headerKeyCanonical := canonicalHeaderKey(argValue, exclusions)
		if argValue == headerKeyCanonical {
			return
		}

		pass.Report(arg.diagnostic(headerKeyCanonical))
	})

	//nolint:nilnil // This is expected by the signature of the callback.
	return nil, nil
}

func canonicalHeaderKey(s string, m map[string]string) string {
	canonical := http.CanonicalHeaderKey(s)

	wellKnown, ok := m[canonical]
	if !ok {
		return canonical
	}

	return wellKnown
}

func isValidMethod(name string) bool {
	switch name {
	case "Get", "Set", "Add", "Del", "Values":
		return true
	default:
		return false
	}
}

func isHTTPHeader(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()
	if obj == nil {
		return false
	}

	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}

	return pkg.Path() == httpHeaderPkg && obj.Name() == httpHeaderName
}
