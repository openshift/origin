package httpexec

// go generate -import github.com/mesos/mesos-go/api/v1/lib/executor -import github.com/mesos/mesos-go/api/v1/lib/executor/calls -type C:executor.Call:executor.Call{Type:executor.Call_MESSAGE}
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib"
	"github.com/mesos/mesos-go/api/v1/lib/client"
	"github.com/mesos/mesos-go/api/v1/lib/httpcli"

	"github.com/mesos/mesos-go/api/v1/lib/executor"
	"github.com/mesos/mesos-go/api/v1/lib/executor/calls"
)

func TestNewSender(t *testing.T) {
	ch := make(chan client.Request, 1)
	cf := ClientFunc(func(r client.Request, _ client.ResponseClass, _ ...httpcli.RequestOpt) (_ mesos.Response, _ error) {
		ch <- r
		return
	})
	check := func(_ mesos.Response, err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	sent := func() client.Request {
		select {
		case r := <-ch:
			return r
		default:
			t.Fatal("no request was sent")
		}
		return nil
	}
	sender := NewSender(cf)
	c := &executor.Call{Type:executor.Call_MESSAGE}

	check(sender.Send(context.Background(), calls.NonStreaming(c)))
	r := sent()
	if _, ok := r.(client.RequestStreaming); ok {
		t.Fatalf("expected non-streaming request instead of %v", r)
	}

	check(sender.Send(context.Background(), calls.Empty().Push(c)))
	r = sent()
	if _, ok := r.(client.RequestStreaming); !ok {
		t.Fatalf("expected streaming request instead of %v", r)
	}

	// expect this to fail because newly created call structs don't have a type
	// that can be used for classifying an expected response type.
	_, err := sender.Send(context.Background(), calls.Empty().Push(new(executor.Call)))
	if err == nil {
		t.Fatal("expected send to fail w/ malformed call")
	}
}
