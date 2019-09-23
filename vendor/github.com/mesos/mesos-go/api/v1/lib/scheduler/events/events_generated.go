package events

// go generate -import github.com/mesos/mesos-go/api/v1/lib/scheduler -type E:*scheduler.Event:&scheduler.Event{} -type ET:scheduler.Event_Type
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
)

type (
	// Handler is invoked upon the occurrence of some scheduler event that is generated
	// by some other component in the Mesos ecosystem (e.g. master, agent, executor, etc.)
	Handler interface {
		HandleEvent(context.Context, *scheduler.Event) error
	}

	// HandlerFunc is a functional adaptation of the Handler interface
	HandlerFunc func(context.Context, *scheduler.Event) error

	// Handlers executes an event Handler according to the event's type
	Handlers map[scheduler.Event_Type]Handler

	// HandlerFuncs executes an event HandlerFunc according to the event's type
	HandlerFuncs map[scheduler.Event_Type]HandlerFunc
)

// HandleEvent implements Handler for HandlerFunc
func (f HandlerFunc) HandleEvent(ctx context.Context, e *scheduler.Event) error { return f(ctx, e) }

type noopHandler int

func (noopHandler) HandleEvent(_ context.Context, _ *scheduler.Event) error { return nil }

// NoopHandler is a Handler that does nothing and always returns nil
const NoopHandler = noopHandler(0)

// HandleEvent implements Handler for Handlers
func (hs Handlers) HandleEvent(ctx context.Context, e *scheduler.Event) (err error) {
	if h := hs[e.GetType()]; h != nil {
		return h.HandleEvent(ctx, e)
	}
	return nil
}

// HandleEvent implements Handler for HandlerFuncs
func (hs HandlerFuncs) HandleEvent(ctx context.Context, e *scheduler.Event) (err error) {
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
	return func(ctx context.Context, e *scheduler.Event) error {
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
	return func(ctx context.Context, e *scheduler.Event) error {
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
