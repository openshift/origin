package sa6001

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/go/ir"
	"honnef.co/go/tools/go/types/typeutil"
	"honnef.co/go/tools/internal/passes/buildir"

	"golang.org/x/tools/go/analysis"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA6001",
		Run:      run,
		Requires: []*analysis.Analyzer{buildir.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Missing an optimization opportunity when indexing maps by byte slices`,

		Text: `Map keys must be comparable, which precludes the use of byte slices.
This usually leads to using string keys and converting byte slices to
strings.

Normally, a conversion of a byte slice to a string needs to copy the data and
causes allocations. The compiler, however, recognizes \'m[string(b)]\' and
uses the data of \'b\' directly, without copying it, because it knows that
the data can't change during the map lookup. This leads to the
counter-intuitive situation that

    k := string(b)
    println(m[k])
    println(m[k])

will be less efficient than

    println(m[string(b)])
    println(m[string(b)])

because the first version needs to copy and allocate, while the second
one does not.

For some history on this optimization, check out commit
f5f5a8b6209f84961687d993b93ea0d397f5d5bf in the Go repository.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	for _, fn := range pass.ResultOf[buildir.Analyzer].(*buildir.IR).SrcFuncs {
		for _, b := range fn.Blocks {
		insLoop:
			for _, ins := range b.Instrs {
				var fromType types.Type
				var toType types.Type

				// find []byte -> string conversions
				switch ins := ins.(type) {
				case *ir.Convert:
					fromType = ins.X.Type()
					toType = ins.Type()
				case *ir.MultiConvert:
					fromType = ins.X.Type()
					toType = ins.Type()
				default:
					continue
				}
				if toType != types.Universe.Lookup("string").Type() {
					continue
				}
				tset := typeutil.NewTypeSet(fromType)
				// If at least one of the types is []byte, then it's more efficient to inline the conversion
				if !tset.Any(func(term *types.Term) bool {
					s, ok := term.Type().Underlying().(*types.Slice)
					return ok && s.Elem().Underlying() == types.Universe.Lookup("byte").Type()
				}) {
					continue
				}
				refs := ins.Referrers()
				anyUses := false
				for _, ref := range *refs {
					switch ref := ref.(type) {
					case *ir.MapLookup:
						anyUses = true
						if src, ok := ref.Source().(*ast.IndexExpr); ok {
							if _, ok := src.Index.(*ast.Ident); !ok {
								continue insLoop
							}
						} else {
							continue insLoop
						}
					default:
						// the string is used somewhere else than a
						// map lookup
						continue insLoop
					}
				}

				if !anyUses {
					continue
				}
				report.Report(pass, ins, "m[string(key)] would be more efficient than k := string(key); m[k]")
			}
		}
	}
	return nil, nil
}
