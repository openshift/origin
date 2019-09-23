// +build ignore

package main

import (
	"os"
	"text/template"
)

func main() {
	Run(srcTemplate, testTemplate, os.Args...)
}

var srcTemplate = template.Must(template.New("").Parse(`package {{.Package}}

// go generate {{.Args}}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/encoding"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

{{.RequireType "C" -}}{{/* C is assumed to be a struct or primitive type */ -}}
type (
	// Request generates a Call that's sent to a Mesos agent. Subsequent invocations are expected to
	// yield equivalent calls. Intended for use w/ non-streaming requests to an agent.
	Request interface {
		Call() *{{.Type "C"}}
	}

	// RequestFunc is the functional adaptation of Request.
	RequestFunc func() *{{.Type "C"}}

	// RequestStreaming generates a Call that's send to a Mesos agent. Subsequent invocations MAY generate
	// different Call objects. No more Call objects are expected once a nil is returned to signal the end of
	// of the request stream.
	RequestStreaming interface {
		Request
		IsStreaming()
	}

	// RequestStreamingFunc is the functional adaptation of RequestStreaming.
	RequestStreamingFunc func() *{{.Type "C"}}

	// Send issues a Request to a Mesos agent and properly manages Call-specific mechanics.
	Sender interface {
		Send(context.Context, Request) (mesos.Response, error)
	}

	// SenderFunc is the functional adaptation of the Sender interface
	SenderFunc func(context.Context, Request) (mesos.Response, error)
)

func (f RequestFunc) Call() *{{.Type "C"}} { return f() }

func (f RequestFunc) Marshaler() encoding.Marshaler {
	// avoid returning (*{{.Type "C"}})(nil) for interface type
	if call := f(); call != nil {
		return call
	}
	return nil
}

func (f RequestStreamingFunc) Push(c ...*{{.Type "C"}}) RequestStreamingFunc { return Push(f, c...) }

func (f RequestStreamingFunc) Marshaler() encoding.Marshaler {
	// avoid returning (*{{.Type "C"}})(nil) for interface type
	if call := f(); call != nil {
		return call
	}
	return nil
}

func (f RequestStreamingFunc) IsStreaming() {}

func (f RequestStreamingFunc) Call() *{{.Type "C"}} { return f() }

// Push prepends one or more calls onto a request stream. If no calls are given then the original stream is returned.
func Push(r RequestStreaming, c ...*{{.Type "C"}}) RequestStreamingFunc {
	return func() *{{.Type "C"}} {
		if len(c) == 0 {
			return r.Call()
		}
		head := c[0]
		c = c[1:]
		return head
	}
}

// Empty generates a stream that always returns nil.
func Empty() RequestStreamingFunc { return func() *{{.Type "C"}} { return nil } }

var (
	_ = Request(RequestFunc(nil))
	_ = RequestStreaming(RequestStreamingFunc(nil))
	_ = Sender(SenderFunc(nil))
)

// NonStreaming returns a RequestFunc that always generates the same Call.
func NonStreaming(c *{{.Type "C"}}) RequestFunc { return func() *{{.Type "C"}} { return c } }

// FromChan returns a streaming request that fetches calls from the given channel until it closes.
// If a nil chan is specified then the returned func will always generate nil.
func FromChan(ch <-chan *{{.Type "C"}}) RequestStreamingFunc {
	if ch == nil {
		// avoid blocking forever if we're handed a nil chan
		return func() *{{.Type "C"}} { return nil }
	}
	return func() *{{.Type "C"}} {
		if m, ok := <-ch; ok {
			return m
		}
		return nil
	}
}

// Send implements the Sender interface for SenderFunc
func (f SenderFunc) Send(ctx context.Context, r Request) (mesos.Response, error) {
	return f(ctx, r)
}

// IgnoreResponse generates a sender that closes any non-nil response received by Mesos.
func IgnoreResponse(s Sender) SenderFunc {
	return func(ctx context.Context, r Request) (mesos.Response, error) {
		resp, err := s.Send(ctx, r)
		if resp != nil {
			resp.Close()
		}
		return nil, err
	}
}

// SendNoData is a convenience func that executes the given Call using the provided Sender
// and always drops the response data.
func SendNoData(ctx context.Context, sender Sender, r Request) (err error) {
	_, err = IgnoreResponse(sender).Send(ctx, r)
	return
}
{{if .Type "O"}}{{/* O is a functional call option, C.With(...O) must be defined elsewhere */}}
// SendWith injects the given options for all calls.
func SenderWith(s Sender, opts ...{{.Type "O"}}) SenderFunc {
	if len(opts) == 0 {
		return s.Send
	}
        return func(ctx context.Context, r Request) (mesos.Response, error) {
                f := func() (c *{{.Type "C"}}) {
                        if c = r.Call(); c != nil {
                                c = c.With(opts...)
                        }
                        return
                }
                switch r.(type) {
                case RequestStreaming:
                        return s.Send(ctx, RequestStreamingFunc(f))
                default:
                        return s.Send(ctx, RequestFunc(f))
                }
        }
}
{{end -}}
`))

var testTemplate = template.Must(template.New("").Parse(`package {{.Package}}

// go generate {{.Args}}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
{{range .Imports}}
	{{ printf "%q" . -}}
{{end}}
)

func TestNonStreaming(t *testing.T) {
	c := new({{.Type "C"}})
	f := NonStreaming(c)
	if x := f.Call(); x != c {
		t.Fatalf("expected %#v instead of %#v", c, x)
	}
	if x := f.Marshaler(); x == nil {
		t.Fatal("expected non-nil Marshaler")
	}
	f = NonStreaming(nil)
	if x := f.Marshaler(); x != nil {
		t.Fatalf("expected nil Marshaler instead of %#v", x)
	}
}

func TestStreaming(t *testing.T) {
	f := Empty()

	f.IsStreaming()

	if x := f.Call(); x != nil {
		t.Fatalf("expected nil Call instead of %#v", x)
	}
	if x := f.Marshaler(); x != nil {
		t.Fatalf("expected nil Call instead of %#v", x)
	}

	c := new({{.Type "C"}})

	f = f.Push(c)
	if x := f.Marshaler(); x == nil {
		t.Fatal("expected non-nil Marshaler")
	}
	if x := f.Marshaler(); x != nil {
		t.Fatalf("expected nil Marshaler instead of %#v", x)
	}

	c2 := new({{.Type "C"}})

	f = Empty().Push(c, c2)
	if x := f.Call(); x != c {
		t.Fatalf("expected %#v instead of %#v", c, x)
	}
	if x := f.Call(); x != c2 {
		t.Fatalf("expected %#v instead of %#v", c2, x)
	}
	if x := f.Call(); x != nil {
		t.Fatalf("expected nil Call instead of %#v", x)
	}

	ch := make(chan *{{.Type "C"}}, 2)
	ch <- c
	ch <- c2
	close(ch)
	f = FromChan(ch)
	if x := f.Call(); x != c {
		t.Fatalf("expected %#v instead of %#v", c, x)
	}
	if x := f.Call(); x != c2 {
		t.Fatalf("expected %#v instead of %#v", c2, x)
	}
	if x := f.Call(); x != nil {
		t.Fatalf("expected nil Call instead of %#v", x)
	}

	f = FromChan(nil)
	if x := f.Call(); x != nil {
		t.Fatalf("expected nil Call instead of %#v", x)
	}
}

func TestIgnoreResponse(t *testing.T) {
	var closed bool

	IgnoreResponse(SenderFunc(func(_ context.Context, _ Request) (mesos.Response, error) {
		return &mesos.ResponseWrapper{Closer: mesos.CloseFunc(func() error {
			closed = true
			return nil
		})}, nil
	})).Send(nil, nil)

	if !closed {
		t.Fatal("expected response to be closed")
	}
}
{{if .Type "O"}}{{/* O is a functional call option, C.With(...O) must be defined elsewhere */}}
func TestSenderWith(t *testing.T) {
	var (
		s = SenderFunc(func(_ context.Context, r Request) (mesos.Response, error) {
			_ = r.Call() // need to invoke this to invoke SenderWith call decoration
			return nil, nil
		})
		ignore = func(_ mesos.Response, _ error) {}
		c = new({{.Type "C"}})
	)

	for ti, tc := range []Request{NonStreaming(c), Empty().Push(c, c)} {
		var (
			invoked bool
			opt = func(c *{{.Type "C"}}) { invoked = true }
		)

		// sanity check (w/o any options)
		ignore(SenderWith(s).Send(context.Background(), tc))
		if invoked {
			t.Fatalf("test case %d failed: unexpected option invocation", ti)
		}

		ignore(SenderWith(s, opt).Send(context.Background(), tc))
		if !invoked {
			t.Fatalf("test case %d failed: expected option invocation", ti)
		}
	}
}
{{end -}}
`))
