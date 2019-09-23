package calls

// go generate -import github.com/mesos/mesos-go/api/v1/lib/agent -type C:agent.Call
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"

	"github.com/mesos/mesos-go/api/v1/lib/agent"
)

func TestNonStreaming(t *testing.T) {
	c := new(agent.Call)
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

	c := new(agent.Call)

	f = f.Push(c)
	if x := f.Marshaler(); x == nil {
		t.Fatal("expected non-nil Marshaler")
	}
	if x := f.Marshaler(); x != nil {
		t.Fatalf("expected nil Marshaler instead of %#v", x)
	}

	c2 := new(agent.Call)

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

	ch := make(chan *agent.Call, 2)
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
