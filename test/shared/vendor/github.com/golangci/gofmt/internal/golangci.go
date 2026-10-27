package internal

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
)

/*
 The goal of this file is:
 - to expose the unexported constants and the function.
 - set the parserMode.
*/

const (
	TabWidth    = tabWidth
	PrinterMode = printerMode
)

// https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/cmd/gofmt/gofmt.go#L81-L91
// https://github.com/golang/go/blob/1b291b70dff51732415da5b68debe323704d8e8d/src/go/format/format.go#L41
const parserMode = parser.ParseComments | parser.SkipObjectResolution

func Parse(fset *token.FileSet, filename string, src []byte, fragmentOk bool) (
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	err error,
) {
	return parse(fset, filename, src, fragmentOk)
}

func Format(
	fset *token.FileSet,
	file *ast.File,
	sourceAdj func(src []byte, indent int) []byte,
	indentAdj int,
	src []byte,
	cfg printer.Config,
) ([]byte, error) {
	return format(fset, file, sourceAdj, indentAdj, src, cfg)
}

func Simplify(f *ast.File) {
	simplify(f)
}

func RewriteFile(fileSet *token.FileSet, pattern, replace ast.Expr, p *ast.File) *ast.File {
	return rewriteFile(fileSet, pattern, replace, p)
}
