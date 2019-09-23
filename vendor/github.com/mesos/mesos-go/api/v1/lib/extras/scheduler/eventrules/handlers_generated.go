package eventrules

// go generate -import github.com/mesos/mesos-go/api/v1/lib/scheduler -import github.com/mesos/mesos-go/api/v1/lib/scheduler/events -type E:*scheduler.Event -type H:events.Handler -type HF:events.HandlerFunc -output handlers_generated.go
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/events"
)

// Handle generates a rule that executes the given events.Handler.
func Handle(h events.Handler) Rule {
	if h == nil {
		return nil
	}
	return func(ctx context.Context, e *scheduler.Event, err error, chain Chain) (context.Context, *scheduler.Event, error) {
		newErr := h.HandleEvent(ctx, e)
		return chain(ctx, e, Error2(err, newErr))
	}
}

// HandleF is the functional equivalent of Handle
func HandleF(h events.HandlerFunc) Rule {
	return Handle(events.Handler(h))
}

// Handle returns a Rule that invokes the receiver, then the given events.Handler
func (r Rule) Handle(h events.Handler) Rule {
	return Rules{r, Handle(h)}.Eval
}

// HandleF is the functional equivalent of Handle
func (r Rule) HandleF(h events.HandlerFunc) Rule {
	return r.Handle(events.Handler(h))
}

// HandleEvent implements events.Handler for Rule
func (r Rule) HandleEvent(ctx context.Context, e *scheduler.Event) (err error) {
	if r == nil {
		return nil
	}
	_, _, err = r(ctx, e, nil, ChainIdentity)
	return
}

// HandleEvent implements events.Handler for Rules
func (rs Rules) HandleEvent(ctx context.Context, e *scheduler.Event) error {
	return Rule(rs.Eval).HandleEvent(ctx, e)
}
