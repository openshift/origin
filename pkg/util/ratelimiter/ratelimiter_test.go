package ratelimiter

import (
	"testing"
	"time"
)

func TestRateLimitedFunction(t *testing.T) {
	tests := []struct {
		Name     string
		Interval int
		Times    int
	}{
		{
			Name:     "unrated",
			Interval: 0,
			Times:    5,
		},
		{
			Name:     "3PO",
			Interval: 3,
			Times:    10,
		},
		{
			Name:     "five-fer",
			Interval: 5,
			Times:    20,
		},
	}

	keyFunc := func(_ interface{}) (string, error) {
		return "ratelimitertest", nil
	}

	for _, tc := range tests {
		counter := 0
		handler := func() error {
			counter += 1
			return nil
		}

		quit := make(chan struct{})
		rlf := NewRateLimitedFunction(keyFunc, tc.Interval, handler)
		rlf.RunUntil(quit)

		for i := 0; i < tc.Times; i++ {
			go func() {
				if tc.Interval > 0 {
					rlf.Invoke(rlf)
				} else {
					rlf.Invoke(i)
				}
			}()
		}

		select {
		case <-time.After(time.Duration(tc.Interval+2) * time.Second):
			if tc.Interval > 0 && counter >= tc.Times/2 {
				t.Errorf("For coalesced calls, expected number of invocations to be atleast half. Expected: < %v  Got: %v",
					tc.Times/2, counter)
			}
		}
	}
}
