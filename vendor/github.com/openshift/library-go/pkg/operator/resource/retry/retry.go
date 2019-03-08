package retry

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// ignoreConnectionErrors is a wrapper for condition function that will cause to retry on all errors like
// connection refused, EOF, no route to host, etc. but also all 50x API server errors.
// This wrapper will return immediately on HTTP 40x client errors and those will not be retried.
func ignoreConnectionErrors(lastError *error, fn ConditionWithContextFunc) ConditionWithContextFunc {
	return func(ctx context.Context) (bool, error) {
		done, err := fn(ctx)
		switch {
		case done:
			return true, err
		case err == nil:
			return true, nil
		case IsHTTPClientError(err):
			return false, err
		default:
			*lastError = err
			return false, nil
		}
	}
}

// RetryOnConnectionErrors will take context and condition function and retry the condition function until:
// 1) no error is returned
// 2) a client (4xx) HTTP error is returned
// 3) the context passed to the condition function is done
// 4) numbers of steps in the exponential backoff are met
// In case of 3) or 4) the error returned will be the last observed error from the condition function.
func RetryOnConnectionErrors(ctx context.Context, fn ConditionWithContextFunc) error {
	var lastRetryErr error
	err := ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, ignoreConnectionErrors(&lastRetryErr, fn))
	switch err {
	case wait.ErrWaitTimeout:
		if lastRetryErr != nil {
			return lastRetryErr
		}
		return err
	default:
		return err
	}
}

// IsHTTPClientError indicates whether the error passes is an 4xx API server error (client error).
func IsHTTPClientError(err error) bool {
	switch t := err.(type) {
	case errors.APIStatus:
		return t.Status().Code >= 400 && t.Status().Code < 500
	default:
		return false
	}
}
