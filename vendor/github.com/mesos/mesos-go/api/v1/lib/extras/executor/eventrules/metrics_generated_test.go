package eventrules

// go generate -import github.com/mesos/mesos-go/api/v1/lib/executor -type E:*executor.Event:&executor.Event{} -type ET:executor.Event_Type -output metrics_generated.go
// GENERATED CODE FOLLOWS; DO NOT EDIT.

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/executor"
)

func TestMetrics(t *testing.T) {
	var (
		i   int
		ctx = context.Background()
		p   = &executor.Event{}
		a   = errors.New("a")
		h   = func(f func() error, _ ...string) error {
			i++
			return f()
		}
	)
	for ti, tc := range []struct {
		ctx context.Context
		e   *executor.Event
		err error
	}{
		{ctx, p, a},
		{ctx, p, nil},
		{ctx, nil, a},
	} {
		for ri, r := range []Rule{
			Metrics(h, nil), // default labeler
			Metrics(h, func(_ context.Context, _ *executor.Event) []string { return nil }), // custom labeler
		} {
			c, e, err := r.Eval(tc.ctx, tc.e, tc.err, ChainIdentity)
			if !reflect.DeepEqual(c, tc.ctx) {
				t.Errorf("test case %d: expected context %q instead of %q", ti, tc.ctx, c)
			}
			if !reflect.DeepEqual(e, tc.e) {
				t.Errorf("test case %d: expected event %q instead of %q", ti, tc.e, e)
			}
			if !reflect.DeepEqual(err, tc.err) {
				t.Errorf("test case %d: expected error %q instead of %q", ti, tc.err, err)
			}
			if y := (ti * 2) + ri + 1; y != i {
				t.Errorf("test case %d: expected count %q instead of %q", ti, y, i)
			}
		}
	}
	func() {
		defer func() {
			if x := recover(); x != nil {
				t.Log("intercepted expected panic", x)
			}
		}()
		_ = Metrics(nil, nil)
		t.Fatalf("expected a panic because nil harness is not allowed")
	}()
}
