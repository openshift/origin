package limiter

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type handler struct {
	count int
	sync.Mutex
}

func (h *handler) handle() error {
	h.Lock()
	defer h.Unlock()
	h.count += 1
	return nil
}

func (h *handler) counter() int {
	h.Lock()
	defer h.Unlock()
	return h.count
}

func TestCoalescingSerializingRateLimiter(t *testing.T) {

	fmt.Println("start")

	tests := []struct {
		Name     string
		Interval time.Duration
		Times    int
	}{
		{
			Name:     "3PO",
			Interval: 3 * time.Second,
			Times:    10,
		},
		{
			Name:     "five-fer",
			Interval: 5 * time.Second,
			Times:    20,
		},
		{
			Name:     "longjob",
			Interval: 2 * time.Second,
			Times:    20,
		},
	}

	for _, tc := range tests {
		h := &handler{}
		rlf := NewCoalescingSerializingRateLimiter(tc.Interval, h.handle)

		for i := 0; i < tc.Times; i++ {
			fmt.Println("start")
			rlf.RegisterChange()
			fmt.Println("end")
		}

		select {
		case <-time.After(tc.Interval + 2*time.Second):
			fmt.Println("after")

			counter := h.counter()
			if tc.Interval > 0 && counter >= tc.Times/2 {
				t.Errorf("For coalesced calls, expected number of invocations to be at least half. Expected: < %v  Got: %v",
					tc.Times/2, counter)
			}
		}
	}
}
