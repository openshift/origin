package util

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestTimeoutAfter(t *testing.T) {
	type testCase struct {
		fn      func(*time.Timer) error
		msg     string
		timeout time.Duration
		expect  interface{}
	}
	table := []testCase{
		{
			fn:      func(timer *time.Timer) error { time.Sleep(1 * time.Second); return nil },
			timeout: 50 * time.Millisecond,
			expect:  &TimeoutError{after: 50 * time.Millisecond},
		},
		{
			fn:      func(timer *time.Timer) error { return fmt.Errorf("foo") },
			timeout: 50 * time.Millisecond,
			expect:  fmt.Errorf("foo"),
		},
		{
			fn:      func(timer *time.Timer) error { time.Sleep(1 * time.Second); return fmt.Errorf("foo") },
			msg:     "bar",
			timeout: 50 * time.Millisecond,
			expect:  fmt.Errorf("bar timed out after 50ms"),
		},
		{
			fn:      func(timer *time.Timer) error { return nil },
			timeout: 50 * time.Millisecond,
			expect:  nil,
		},
	}
	for _, item := range table {
		got := TimeoutAfter(item.timeout, item.msg, item.fn)
		if len(item.msg) > 0 {
			expect, ok := item.expect.(error)
			if !ok {
				t.Errorf("expect must be an error, got %+v", item.expect)
			}
			if expect.Error() != got.Error() {
				t.Errorf("expected message %q, got %q", item.msg, got.Error())
			}
			continue
		}
		if !reflect.DeepEqual(item.expect, got) {
			t.Errorf("expected %+v, got %+v", item.expect, got)
		}
		if _, ok := item.expect.(*TimeoutError); ok {
			if !IsTimeoutError(got) {
				t.Errorf("expected %+v to be timeout error", got)
			}
		}
	}
}
