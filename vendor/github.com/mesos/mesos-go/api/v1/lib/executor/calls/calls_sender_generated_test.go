package calls

// go generate -import github.com/mesos/mesos-go/api/v1/lib/executor -type C:executor.Call -type O:executor.CallOpt -output calls_sender_generated.go
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"

	"github.com/mesos/mesos-go/api/v1/lib/executor"
)

func TestNonStreaming(t *testing.T) {
	c := new(executor.Call)
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

	c := new(executor.Call)

	f = f.Push(c)
	if x := f.Marshaler(); x == nil {
		t.Fatal("expected non-nil Marshaler")
	}
	if x := f.Marshaler(); x != nil {
		t.Fatalf("expected nil Marshaler instead of %#v", x)
	}

	c2 := new(executor.Call)

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

	ch := make(chan *executor.Call, 2)
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

func TestSenderWith(t *testing.T) {
	var (
		s = SenderFunc(func(_ context.Context, r Request) (mesos.Response, error) {
			_ = r.Call() // need to invoke this to invoke SenderWith call decoration
			return nil, nil
		})
		ignore = func(_ mesos.Response, _ error) {}
		c = new(executor.Call)
	)

	for ti, tc := range []Request{NonStreaming(c), Empty().Push(c, c)} {
		var (
			invoked bool
			opt = func(c *executor.Call) { invoked = true }
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
