package sa4003

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/types/typeutil"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4003",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `Comparing unsigned values against negative values is pointless`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	isobj := func(expr ast.Expr, name string) bool {
		if name == "" {
			return false
		}
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		return typeutil.IsObject(pass.TypesInfo.ObjectOf(sel.Sel), name)
	}

	fn := func(node ast.Node) {
		expr := node.(*ast.BinaryExpr)
		tx := pass.TypesInfo.TypeOf(expr.X)
		tset := typeutil.NewTypeSet(tx)

		// We only check for the math constants and integer literals, not for
		// all constant expressions. This is to avoid
		// false positives when constant values differ under different build tags.
		var (
			maxMathConst string
			minMathConst string
			maxLiteral   constant.Value
			minLiteral   constant.Value
		)

		allUnsigned := tset.All(func(t *types.Term) bool {
			if basic, ok := t.Type().Underlying().(*types.Basic); ok {
				return basic.Info()&types.IsUnsigned != 0
			}
			return false
		})

		if allUnsigned {
			isZeroLiteral := func(expr ast.Expr) bool {
				return code.IsIntegerLiteral(pass, expr, constant.MakeInt64(0))
			}
			if (expr.Op == token.LSS && isZeroLiteral(expr.Y)) ||
				(expr.Op == token.GTR && isZeroLiteral(expr.X)) {
				report.Report(
					pass,
					expr,
					fmt.Sprintf("no value of type %s is less than 0", tx),
					report.FilterGenerated(),
				)
			}
			if expr.Op == token.GEQ && isZeroLiteral(expr.Y) ||
				expr.Op == token.LEQ && isZeroLiteral(expr.X) {
				report.Report(
					pass,
					expr,
					fmt.Sprintf("every value of type %s is >= 0", tx),
					report.FilterGenerated(),
				)
			}
		}

		core := tset.CoreType()
		if core == nil {
			// All remaining checks are only relevant when the type set
			// contains a single underlying type.
			//
			// If we had a 'var x uint8 | uint16',
			// then the type checker wouldn't permit a check such as
			// 'if x <= math.MaxUint16', because the constant cannot be converted to all
			// types in the type set.
			return
		}

		basic, ok := core.(*types.Basic)
		if !ok {
			return
		}

		switch basic.Kind() {
		case types.Uint8:
			maxMathConst = "math.MaxUint8"
			minLiteral = constant.MakeUint64(0)
			maxLiteral = constant.MakeUint64(math.MaxUint8)
		case types.Uint16:
			maxMathConst = "math.MaxUint16"
			minLiteral = constant.MakeUint64(0)
			maxLiteral = constant.MakeUint64(math.MaxUint16)
		case types.Uint32:
			maxMathConst = "math.MaxUint32"
			minLiteral = constant.MakeUint64(0)
			maxLiteral = constant.MakeUint64(math.MaxUint32)
		case types.Uint64:
			maxMathConst = "math.MaxUint64"
			minLiteral = constant.MakeUint64(0)
			maxLiteral = constant.MakeUint64(math.MaxUint64)
		case types.Uint:
			// TODO(dh): we could chose 32 bit vs 64 bit depending on the
			// file's build tags
			maxMathConst = "math.MaxUint64"
			minLiteral = constant.MakeUint64(0)
			maxLiteral = constant.MakeUint64(math.MaxUint64)

		case types.Int8:
			minMathConst = "math.MinInt8"
			maxMathConst = "math.MaxInt8"
			minLiteral = constant.MakeInt64(math.MinInt8)
			maxLiteral = constant.MakeInt64(math.MaxInt8)
		case types.Int16:
			minMathConst = "math.MinInt16"
			maxMathConst = "math.MaxInt16"
			minLiteral = constant.MakeInt64(math.MinInt16)
			maxLiteral = constant.MakeInt64(math.MaxInt16)
		case types.Int32:
			minMathConst = "math.MinInt32"
			maxMathConst = "math.MaxInt32"
			minLiteral = constant.MakeInt64(math.MinInt32)
			maxLiteral = constant.MakeInt64(math.MaxInt32)
		case types.Int64:
			minMathConst = "math.MinInt64"
			maxMathConst = "math.MaxInt64"
			minLiteral = constant.MakeInt64(math.MinInt64)
			maxLiteral = constant.MakeInt64(math.MaxInt64)
		case types.Int:
			// TODO(dh): we could chose 32 bit vs 64 bit depending on the
			// file's build tags
			minMathConst = "math.MinInt64"
			maxMathConst = "math.MaxInt64"
			minLiteral = constant.MakeInt64(math.MinInt64)
			maxLiteral = constant.MakeInt64(math.MaxInt64)
		}

		isLiteral := func(expr ast.Expr, c constant.Value) bool {
			if c == nil {
				return false
			}
			return code.IsIntegerLiteral(pass, expr, c)
		}

		x, y, op := expr.X, expr.Y, expr.Op
		switch op {
		case token.GEQ, token.GTR:
		case token.LEQ:
			x, y = y, x
			op = token.GEQ
		case token.LSS:
			x, y = y, x
			op = token.GTR
		default:
			return
		}

		if isobj(y, maxMathConst) || isLiteral(y, maxLiteral) {
			report.Report(
				pass,
				expr,
				fmt.Sprintf("no value of type %s is greater than %s", tx, maxMathConst),
				report.FilterGenerated(),
			)
		}
		if op == token.GEQ && (isobj(x, maxMathConst) || isLiteral(x, maxLiteral)) {
			report.Report(
				pass,
				expr,
				fmt.Sprintf("every value of type %s is <= %s", tx, maxMathConst),
				report.FilterGenerated(),
			)
		}

		if !allUnsigned {
			if isobj(x, minMathConst) || isLiteral(x, minLiteral) {
				report.Report(
					pass,
					expr,
					fmt.Sprintf("no value of type %s is less than %s", tx, minMathConst),
					report.FilterGenerated(),
				)
			}
			if op == token.GEQ && (isobj(y, minMathConst) || isLiteral(y, minLiteral)) {
				report.Report(
					pass,
					expr,
					fmt.Sprintf("every value of type %s is >= %s", tx, minMathConst),
					report.FilterGenerated(),
				)
			}
		}

	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
