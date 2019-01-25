package callrules

// go generate -import github.com/mesos/mesos-go/api/v1/lib/scheduler -import github.com/mesos/mesos-go/api/v1/lib/scheduler/calls -type E:*scheduler.Call -type C:calls.Caller -type CF:calls.CallerFunc -output callers_generated.go
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"

	"github.com/mesos/mesos-go/api/v1/lib"

	"github.com/mesos/mesos-go/api/v1/lib/scheduler"
	"github.com/mesos/mesos-go/api/v1/lib/scheduler/calls"
)

// Call returns a Rule that invokes the given Caller
func Call(caller calls.Caller) Rule {
	if caller == nil {
		return nil
	}
	return func(ctx context.Context, c *scheduler.Call, _ mesos.Response, _ error, ch Chain) (context.Context, *scheduler.Call, mesos.Response, error) {
		resp, err := caller.Call(ctx, c)
		return ch(ctx, c, resp, err)
	}
}

// CallF returns a Rule that invokes the given CallerFunc
func CallF(cf calls.CallerFunc) Rule {
	return Call(calls.Caller(cf))
}

// Caller returns a Rule that invokes the receiver and then calls the given Caller
func (r Rule) Caller(caller calls.Caller) Rule {
	return Rules{r, Call(caller)}.Eval
}

// CallerF returns a Rule that invokes the receiver and then calls the given CallerFunc
func (r Rule) CallerF(cf calls.CallerFunc) Rule {
	return r.Caller(calls.Caller(cf))
}

// Call implements the Caller interface for Rule
func (r Rule) Call(ctx context.Context, c *scheduler.Call) (mesos.Response, error) {
	if r == nil {
		return nil, nil
	}
	_, _, resp, err := r(ctx, c, nil, nil, ChainIdentity)
	return resp, err
}

// Call implements the Caller interface for Rules
func (rs Rules) Call(ctx context.Context, c *scheduler.Call) (mesos.Response, error) {
	return Rule(rs.Eval).Call(ctx, c)
}

var (
	_ = calls.Caller(Rule(nil))
	_ = calls.Caller(Rules(nil))
)
