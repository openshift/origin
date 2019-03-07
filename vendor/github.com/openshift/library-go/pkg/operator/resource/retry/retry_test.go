package retry

import (
	"context"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestRetryOnConnectionErrors(t *testing.T) {
	tests := []struct {
		name           string
		contextTimeout time.Duration
		jobDuration    time.Duration
		jobError       error
		jobDone        bool
		evalError      func(*testing.T, error)
		evalAttempts   func(*testing.T, int)
	}{
		{
			name:           "timeout on context deadline",
			contextTimeout: 200 * time.Millisecond,
			jobDuration:    500 * time.Millisecond,
			jobError:       errors.NewInternalError(fmt.Errorf("internal error")),
			evalError: func(t *testing.T, e error) {
				if !errors.IsInternalError(e) {
					t.Errorf("expected internal server error, got %v", e)
				}
			},
			evalAttempts: func(t *testing.T, attempts int) {
				if attempts != 1 {
					t.Errorf("expected only one attempt, got %d", attempts)
				}
			},
		},
		{
			name:           "retry on internal server error",
			contextTimeout: 500 * time.Millisecond,
			jobDuration:    200 * time.Millisecond,
			jobError:       errors.NewInternalError(fmt.Errorf("internal error")),
			evalError: func(t *testing.T, e error) {
				if !errors.IsInternalError(e) {
					t.Errorf("expected internal server error, got %v", e)
				}
			},
			evalAttempts: func(t *testing.T, attempts int) {
				if attempts <= 1 {
					t.Errorf("expected more than one attempt, got %d", attempts)
				}
			},
		},
		{
			name:           "return on not found error",
			contextTimeout: 500 * time.Millisecond,
			jobDuration:    100 * time.Millisecond,
			jobError:       errors.NewNotFound(schema.GroupResource{Resource: "pods"}, "test-pod"),
			evalError: func(t *testing.T, e error) {
				if !errors.IsNotFound(e) {
					t.Errorf("expected not found error, got %v", e)
				}
			},
			evalAttempts: func(t *testing.T, attempts int) {
				if attempts != 1 {
					t.Errorf("expected only one attempt, got %d", attempts)
				}
			},
		},
		{
			name:           "return on no error",
			contextTimeout: 500 * time.Millisecond,
			jobDuration:    50 * time.Millisecond,
			evalError: func(t *testing.T, e error) {
				if e != nil {
					t.Errorf("expected no error, got %v", e)
				}
			},
			evalAttempts: func(t *testing.T, attempts int) {
				if attempts != 1 {
					t.Errorf("expected only one attempt, got %d", attempts)
				}
			},
		},
		{
			name:           "return on done",
			contextTimeout: 500 * time.Millisecond,
			jobDuration:    50 * time.Millisecond,
			jobDone:        true,
			evalError: func(t *testing.T, e error) {
				if e != nil {
					t.Errorf("expected no error, got %v", e)
				}
			},
			evalAttempts: func(t *testing.T, attempts int) {
				if attempts != 1 {
					t.Errorf("expected only one attempt, got %d", attempts)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.TODO(), test.contextTimeout)
			defer cancel()
			attempts := 0
			err := RetryOnConnectionErrors(ctx, func(context.Context) (bool, error) {
				time.Sleep(test.jobDuration)
				attempts++
				if test.jobError != nil {
					return test.jobDone, test.jobError
				}
				return test.jobDone, nil
			})
			test.evalError(t, err)
			test.evalAttempts(t, attempts)
		})
	}

}
