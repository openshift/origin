package gofmt

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"

	"github.com/golangci/gofmt/internal"
)

type Options struct {
	NeedSimplify bool
	RewriteRules []RewriteRule
}

type RewriteRule struct {
	Pattern     string
	Replacement string
}

// Source formats the code like gofmt.
// Empty string `rewrite` will be ignored.
// https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/cmd/gofmt/gofmt.go#L236-L300
// https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/go/format/format.go#L101-L115
func Source(filename string, src []byte, opts Options) ([]byte, error) {
	fset := token.NewFileSet()

	file, sourceAdj, indentAdj, err := internal.Parse(fset, filename, src, false)
	if err != nil {
		return nil, err
	}

	file, err = rewriteFileContent(fset, file, opts.RewriteRules)
	if err != nil {
		return nil, err
	}

	ast.SortImports(fset, file)

	if opts.NeedSimplify {
		internal.Simplify(file)
	}

	return internal.Format(fset, file, sourceAdj, indentAdj, src, printer.Config{Mode: internal.PrinterMode, Tabwidth: internal.TabWidth})
}

func rewriteFileContent(fset *token.FileSet, file *ast.File, rewriteRules []RewriteRule) (*ast.File, error) {
	for _, rewriteRule := range rewriteRules {
		pattern, err := parseExpression(rewriteRule.Pattern, "pattern")
		if err != nil {
			return nil, err
		}

		replacement, err := parseExpression(rewriteRule.Replacement, "replacement")
		if err != nil {
			return nil, err
		}

		file = internal.RewriteFile(fset, pattern, replacement, file)
	}

	return file, nil
}

func parseExpression(s, what string) (ast.Expr, error) {
	x, err := parser.ParseExpr(s)
	if err != nil {
		return nil, fmt.Errorf("parsing %s %q at %s\n", what, s, err)
	}
	return x, nil
}
