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
{{.RequireType "ET" -}}
type (
	// Handler is invoked upon the occurrence of some scheduler event that is generated
	// by some other component in the Mesos ecosystem (e.g. master, agent, executor, etc.)
	Handler interface {
		HandleEvent(context.Context, {{.Type "E"}}) error
	}

	// HandlerFunc is a functional adaptation of the Handler interface
	HandlerFunc func(context.Context, {{.Type "E"}}) error

	// Handlers executes an event Handler according to the event's type
	Handlers map[{{.Type "ET"}}]Handler

	// HandlerFuncs executes an event HandlerFunc according to the event's type
	HandlerFuncs map[{{.Type "ET"}}]HandlerFunc
)

// HandleEvent implements Handler for HandlerFunc
func (f HandlerFunc) HandleEvent(ctx context.Context, e {{.Type "E"}}) error { return f(ctx, e) }

type noopHandler int

func (noopHandler) HandleEvent(_ context.Context, _ {{.Type "E"}}) error { return nil }

// NoopHandler is a Handler that does nothing and always returns nil
const NoopHandler = noopHandler(0)

// HandleEvent implements Handler for Handlers
func (hs Handlers) HandleEvent(ctx context.Context, e {{.Type "E"}}) (err error) {
	if h := hs[e.GetType()]; h != nil {
		return h.HandleEvent(ctx, e)
	}
	return nil
}

// HandleEvent implements Handler for HandlerFuncs
func (hs HandlerFuncs) HandleEvent(ctx context.Context, e {{.Type "E"}}) (err error) {
	if h := hs[e.GetType()]; h != nil {
		return h.HandleEvent(ctx, e)
	}
	return nil
}

// Otherwise returns a HandlerFunc that attempts to process an event with the Handlers map; unmatched event types are
// processed by the given HandlerFunc. A nil HandlerFunc parameter is effecitvely a noop.
func (hs Handlers) Otherwise(f HandlerFunc) HandlerFunc {
	if f == nil {
		return hs.HandleEvent
	}
	return func(ctx context.Context, e {{.Type "E"}}) error {
		if h := hs[e.GetType()]; h != nil {
			return h.HandleEvent(ctx, e)
		}
		return f(ctx, e)
	}
}

// Otherwise returns a HandlerFunc that attempts to process an event with the HandlerFuncs map; unmatched event types
// are processed by the given HandlerFunc. A nil HandlerFunc parameter is effecitvely a noop.
func (hs HandlerFuncs) Otherwise(f HandlerFunc) HandlerFunc {
	if f == nil {
		return hs.HandleEvent
	}
	return func(ctx context.Context, e {{.Type "E"}}) error {
		if h := hs[e.GetType()]; h != nil {
			return h.HandleEvent(ctx, e)
		}
		return f(ctx, e)
	}
}

var (
	_ = Handler(Handlers(nil))
	_ = Handler(HandlerFunc(nil))
	_ = Handler(HandlerFuncs(nil))
)
`))
