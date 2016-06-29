package util

import (
	"fmt"
	"time"
)

// TimeoutError is error returned after timeout occurred.
type TimeoutError struct {
	after   time.Duration
	message string
}

// Error implements the Go error interface.
func (t *TimeoutError) Error() string {
	if len(t.message) > 0 {
		return fmt.Sprintf(t.message, t.after)
	}
	return fmt.Sprintf("calling the function timeout after %v", t.after)
}

// TimeoutAfter executes the provide function and return the TimeoutError in
// case when the execution time of the provided function is bigger than provided
// time duration.
func TimeoutAfter(t time.Duration, errorMsg string, fn func() error) error {
	c := make(chan error, 1)
	go func() {
		defer close(c)
		c <- fn()
	}()
	select {
	case err := <-c:
		return err
	case <-time.After(t):
		return &TimeoutError{after: t, message: errorMsg}
	}
}

// IsTimeoutError checks if the provided error is timeout.
func IsTimeoutError(e error) bool {
	_, ok := e.(*TimeoutError)
	return ok
}
