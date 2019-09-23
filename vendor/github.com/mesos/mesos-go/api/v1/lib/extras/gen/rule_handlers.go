// +build ignore

package main

import (
	"os"
	"text/template"
)

func main() {
	Run(handlersTemplate, nil, os.Args...)
}

var handlersTemplate = template.Must(template.New("").Parse(`package {{.Package}}

// go generate {{.Args}}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

{{.RequireType "E" -}}
{{.RequireType "H" -}}
{{.RequireType "HF" -}}
// Handle generates a rule that executes the given {{.Type "H"}}.
func Handle(h {{.Type "H"}}) Rule {
	if h == nil {
		return nil
	}
	return func(ctx context.Context, e {{.Type "E"}}, err error, chain Chain) (context.Context, {{.Type "E"}}, error) {
		newErr := h.HandleEvent(ctx, e)
		return chain(ctx, e, Error2(err, newErr))
	}
}

// HandleF is the functional equivalent of Handle
func HandleF(h {{.Type "HF"}}) Rule {
	return Handle({{.Type "H"}}(h))
}

// Handle returns a Rule that invokes the receiver, then the given {{.Type "H"}}
func (r Rule) Handle(h {{.Type "H"}}) Rule {
	return Rules{r, Handle(h)}.Eval
}

// HandleF is the functional equivalent of Handle
func (r Rule) HandleF(h {{.Type "HF"}}) Rule {
	return r.Handle({{.Type "H"}}(h))
}

// HandleEvent implements {{.Type "H"}} for Rule
func (r Rule) HandleEvent(ctx context.Context, e {{.Type "E"}}) (err error) {
	if r == nil {
		return nil
	}
	_, _, err = r(ctx, e, nil, ChainIdentity)
	return
}

// HandleEvent implements {{.Type "H"}} for Rules
func (rs Rules) HandleEvent(ctx context.Context, e {{.Type "E"}}) error {
	return Rule(rs.Eval).HandleEvent(ctx, e)
}
`))
