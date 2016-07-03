package util

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestTimeoutAfter(t *testing.T) {
	type testCase struct {
		fn      func() error
		msg     string
		timeout time.Duration
		expect  interface{}
	}
	table := []testCase{
		{
			fn:      func() error { time.Sleep(1 * time.Second); return nil },
			timeout: 50 * time.Millisecond,
			expect:  &TimeoutError{after: 50 * time.Millisecond},
		},
		{
			fn:      func() error { return fmt.Errorf("foo") },
			timeout: 50 * time.Millisecond,
			expect:  fmt.Errorf("foo"),
		},
		{
			fn:      func() error { time.Sleep(1 * time.Second); return fmt.Errorf("foo") },
			msg:     "bar %v",
			timeout: 50 * time.Millisecond,
			expect:  fmt.Errorf("bar 50ms"),
		},
		{
			fn:      func() error { return nil },
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
