package sa9010

import (
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA9010",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Returned function should be called in defer`,
		Text: `
If you have a function such as:

    func f() func() {
        // Do something.
        return func() {
            // Do something.
        }
    }

Then calling that in defer:

    defer f()

Is almost always a mistake, since you typically want to call the returned
function:

    defer f()()
`,
		Since:    "2026.2",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAll,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (any, error) {
	fn := func(n ast.Node) {
		def := n.(*ast.DeferStmt)
		if _, ok := pass.TypesInfo.TypeOf(def.Call).Underlying().(*types.Signature); ok {
			report.Report(pass, def, "deferred return function not called")
		}
	}

	code.Preorder(pass, fn, (*ast.DeferStmt)(nil))
	return nil, nil
}
