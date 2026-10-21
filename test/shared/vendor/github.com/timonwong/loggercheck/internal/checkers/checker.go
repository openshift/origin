package checkers

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"golang.org/x/tools/go/analysis"

	"github.com/timonwong/loggercheck/internal/nilness"
)

type Config struct {
	RequireStringKey bool
	NoPrintfLike     bool
	NilStringer      bool
}

type CallContext struct {
	Expr      *ast.CallExpr
	Func      *types.Func
	Signature *types.Signature
	// Nilness is optional; without it every candidate value is reported.
	Nilness *nilness.Index
}

type Checker interface {
	FilterKeyAndValues(pass *analysis.Pass, keyAndValues []ast.Expr) []ast.Expr
	CheckLoggingKey(pass *analysis.Pass, keyAndValues []ast.Expr)
	CheckPrintfLikeSpecifier(pass *analysis.Pass, args []ast.Expr)
}

var stringerType = func() *types.Interface {
	stringMethod := types.NewFunc(token.NoPos, nil, "String", types.NewSignatureType(
		nil,
		nil,
		nil,
		types.NewTuple(),
		types.NewTuple(types.NewVar(token.NoPos, nil, "", types.Typ[types.String])),
		false,
	))
	iface := types.NewInterfaceType([]*types.Func{stringMethod}, nil)
	iface.Complete()
	return iface
}()

func checkStringerValues(pass *analysis.Pass, call CallContext, keyAndValues []ast.Expr) {
	for i := 1; i < len(keyAndValues); i += 2 {
		arg := keyAndValues[i]
		typ := types.Unalias(pass.TypesInfo.TypeOf(arg))
		ptr, ok := typ.(*types.Pointer)
		if !ok {
			continue
		}

		elem := types.Unalias(ptr.Elem())
		if !types.Implements(elem, stringerType) || !types.Implements(ptr, stringerType) {
			continue
		}

		if call.Nilness.NonNilVariadicArg(call.Expr, slices.Index(call.Expr.Args, arg)) {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:      arg.Pos(),
			End:      arg.End(),
			Category: DiagnosticCategory,
			Message:  "logging value may panic when nil because its element type implements fmt.Stringer",
		})
	}
}

func ExecuteChecker(c Checker, pass *analysis.Pass, call CallContext, cfg Config) {
	params := call.Signature.Params()
	nparams := params.Len() // variadic => nonzero
	startIndex := nparams - 1

	if len(call.Expr.Args) < startIndex {
		// A multi-valued expression may expand to multiple arguments, but its
		// individual values cannot be represented by separate AST expressions.
		// Skip calls whose syntactic arguments cannot cover the fixed params.
		return
	}

	iface, ok := types.Unalias(params.At(startIndex).Type().(*types.Slice).Elem()).(*types.Interface)
	if !ok || !iface.Empty() {
		return // final (args) param is not ...interface{}
	}

	keyValuesArgs := c.FilterKeyAndValues(pass, call.Expr.Args[startIndex:])

	if len(keyValuesArgs)%2 != 0 {
		firstArg := keyValuesArgs[0]
		lastArg := keyValuesArgs[len(keyValuesArgs)-1]
		pass.Report(analysis.Diagnostic{
			Pos:      firstArg.Pos(),
			End:      lastArg.End(),
			Category: DiagnosticCategory,
			Message:  "odd number of arguments passed as key-value pairs for logging",
		})
	}

	if cfg.RequireStringKey {
		c.CheckLoggingKey(pass, keyValuesArgs)
	}

	if cfg.NilStringer {
		checkStringerValues(pass, call, keyValuesArgs)
	}

	if cfg.NoPrintfLike {
		// Check all args
		c.CheckPrintfLikeSpecifier(pass, call.Expr.Args)
	}
}
