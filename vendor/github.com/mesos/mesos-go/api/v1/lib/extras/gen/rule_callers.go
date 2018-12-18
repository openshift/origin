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

	"github.com/mesos/mesos-go/api/v1/lib"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

{{.RequireType "E" -}}
{{.RequireType "C" -}}
{{.RequireType "CF" -}}
// Call returns a Rule that invokes the given Caller
func Call(caller {{.Type "C"}}) Rule {
	if caller == nil {
		return nil
	}
	return func(ctx context.Context, c {{.Type "E"}}, _ mesos.Response, _ error, ch Chain) (context.Context, {{.Type "E"}}, mesos.Response, error) {
		resp, err := caller.Call(ctx, c)
		return ch(ctx, c, resp, err)
	}
}

// CallF returns a Rule that invokes the given CallerFunc
func CallF(cf {{.Type "CF"}}) Rule {
	return Call({{.Type "C"}}(cf))
}

// Caller returns a Rule that invokes the receiver and then calls the given Caller
func (r Rule) Caller(caller {{.Type "C"}}) Rule {
	return Rules{r, Call(caller)}.Eval
}

// CallerF returns a Rule that invokes the receiver and then calls the given CallerFunc
func (r Rule) CallerF(cf {{.Type "CF"}}) Rule {
	return r.Caller({{.Type "C"}}(cf))
}

// Call implements the Caller interface for Rule
func (r Rule) Call(ctx context.Context, c {{.Type "E"}}) (mesos.Response, error) {
	if r == nil {
		return nil, nil
	}
	_, _, resp, err := r(ctx, c, nil, nil, ChainIdentity)
	return resp, err
}

// Call implements the Caller interface for Rules
func (rs Rules) Call(ctx context.Context, c {{.Type "E"}}) (mesos.Response, error) {
	return Rule(rs.Eval).Call(ctx, c)
}

var (
	_ = {{.Type "C"}}(Rule(nil))
	_ = {{.Type "C"}}(Rules(nil))
)
`))
