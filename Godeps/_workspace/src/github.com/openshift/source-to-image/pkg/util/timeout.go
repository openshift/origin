package util

import (
	"fmt"
	"time"
)

// TimeoutError is error returned after timeout occured.
type TimeoutError struct {
	after time.Duration
}

// Error implements the Go error interface.
func (t *TimeoutError) Error() string {
	return fmt.Sprintf("calling the function timeout after %v", t.after)
}

// TimeoutAfter executes the provide function and return the TimeoutError in
// case when the execution time of the provided function is bigger than provided
// time duration.
func TimeoutAfter(t time.Duration, fn func() error) error {
	c := make(chan error, 1)
	go func() {
		defer close(c)
		c <- fn()
	}()
	select {
	case err := <-c:
		return err
	case <-time.After(t):
		return &TimeoutError{after: t}
	}
}

// IsTimeoutError checks if the provided error is timeout.
func IsTimeoutError(e error) bool {
	_, ok := e.(*TimeoutError)
	return ok
}
