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

{{.RequireType "C" -}}
type (
	// Caller is the public interface this framework scheduler's should consume
	Caller interface {
		// Call issues a call to Mesos and properly manages call-specific HTTP response headers & data.
		Call(context.Context, {{.Type "C"}}) (mesos.Response, error)
	}

	// CallerFunc is the functional adaptation of the Caller interface
	CallerFunc func(context.Context, {{.Type "C"}}) (mesos.Response, error)
)

// Call implements the Caller interface for CallerFunc
func (f CallerFunc) Call(ctx context.Context, c {{.Type "C"}}) (mesos.Response, error) {
	return f(ctx, c)
}

// CallNoData is a convenience func that executes the given Call using the provided Caller
// and always drops the response data.
func CallNoData(ctx context.Context, caller Caller, call {{.Type "C"}}) error {
	resp, err := caller.Call(ctx, call)
	if resp != nil {
		resp.Close()
	}
	return err
}
`))
