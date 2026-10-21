package rule

import (
	"go/ast"

	"github.com/mgechev/revive/lint"
)

// MultilineIfInitRule flags if statements whose init clause spans multiple lines.
// A multi-line init defeats the purpose of the if-init idiom, which exists for tight one-liners.
// When it wraps, the reader has to visually parse a struct literal or call chain to
// find where the init ends and the condition begins.
type MultilineIfInitRule struct{}

// Apply applies the rule to given file.
func (*MultilineIfInitRule) Apply(file *lint.File, _ lint.Arguments) []lint.Failure {
	var failures []lint.Failure

	onFailure := func(failure lint.Failure) {
		failures = append(failures, failure)
	}

	ast.Walk(lintMultilineIfInit{
		file:      file,
		onFailure: onFailure,
	}, file.AST)
	return failures
}

// Name returns the rule name.
func (*MultilineIfInitRule) Name() string {
	return "multiline-if-init"
}

type lintMultilineIfInit struct {
	file      *lint.File
	onFailure func(lint.Failure)
}

func (w lintMultilineIfInit) Visit(n ast.Node) ast.Visitor {
	ifStmt, ok := n.(*ast.IfStmt)
	if !ok || ifStmt.Init == nil {
		return w
	}

	initStart := w.file.ToPosition(ifStmt.Init.Pos())
	initEnd := w.file.ToPosition(ifStmt.Init.End())

	if initEnd.Line-initStart.Line > 0 {
		w.onFailure(lint.Failure{
			Confidence: 1,
			Node:       ifStmt,
			Category:   lint.FailureCategoryStyle,
			Failure:    "if-init statement should not span multiple lines",
		})
	}

	return w
}
